use super::files::{CachedFileIndex, CachedIgnoreRules};
use super::runtime;
use super::types::{Challenge, Credentials, Profile};
use crate::error::{HostError, HostResult};
use russh::client::{self, AuthResult, Config, Handle, KeyboardInteractiveAuthResponse, Msg};
use russh::keys::{
    agent::client::AgentClient, load_secret_key, Algorithm, HashAlg, PrivateKeyWithHashAlg,
    PublicKeyOrCertificate,
};
use russh::{Channel, ChannelMsg, Disconnect, MethodKind};
use russh_sftp::{
    client::{error::Error as SftpError, SftpSession},
    protocol::StatusCode,
};
use serde_json::{json, Value};
use std::{
    path::{Path, PathBuf},
    sync::{Arc, Mutex},
    time::Duration,
};

pub struct ClientHandler {
    host: String,
    port: u16,
    trusted: Option<String>,
    captured_fingerprint: Arc<Mutex<Option<String>>>,
    accepted: Arc<Mutex<bool>>,
}

impl client::Handler for ClientHandler {
    type Error = russh::Error;

    async fn check_server_key(
        &mut self,
        server_public_key: &PublicKeyOrCertificate,
    ) -> Result<bool, Self::Error> {
        let public_key = server_public_key.public_key();
        let fingerprint = public_key.fingerprint(HashAlg::Sha256).to_string();
        *self.captured_fingerprint.lock().unwrap() = Some(fingerprint.clone());
        let is_trusted = self.trusted.as_deref() == Some(fingerprint.as_str())
            || super::known_hosts::check_user_known_hosts(&self.host, self.port, &public_key)
                == super::known_hosts::KnownHostCheck::Matched;
        *self.accepted.lock().unwrap() = is_trusted;
        Ok(is_trusted)
    }
}

pub enum Handshake {
    Trusted(Handle<ClientHandler>),
    Challenge(Challenge),
}

pub struct Connection {
    pub profile: Profile,
    pub session: Arc<Handle<ClientHandler>>,
    pub sftp: SftpSession,
    pub root: String,
    pub credentials: Credentials,
    pub ignore_cache: Mutex<Option<CachedIgnoreRules>>,
    pub file_index_cache: Mutex<Option<CachedFileIndex>>,
}

fn failure(error: impl std::fmt::Display) -> HostError {
    HostError::new("SSH_ERROR", error.to_string())
}

pub(super) fn sftp_failure(error: SftpError) -> HostError {
    match error {
        SftpError::Status(status) if status.status_code == StatusCode::NoSuchFile => {
            let detail = status.error_message.trim();
            if detail.to_ascii_lowercase().contains("no such file") {
                HostError::new("SFTP_ERROR", detail)
            } else if detail.is_empty() {
                HostError::new("SFTP_ERROR", "No such file")
            } else {
                HostError::new("SFTP_ERROR", format!("No such file: {detail}"))
            }
        }
        other => HostError::new("SFTP_ERROR", other.to_string()),
    }
}

pub fn handshake(profile: &Profile, trusted: Option<&str>) -> HostResult<Handshake> {
    let (result, _) = handshake_with_fingerprint(profile, trusted)?;
    Ok(result)
}

pub fn handshake_with_fingerprint(
    profile: &Profile,
    trusted: Option<&str>,
) -> HostResult<(Handshake, Option<String>)> {
    runtime::block_on(async {
        let deadline = tokio::time::Instant::now() + Duration::from_secs(15);
        let addresses = match tokio::time::timeout_at(
            deadline,
            tokio::net::lookup_host((profile.host.as_str(), profile.port)),
        )
        .await
        {
            Ok(Ok(addrs)) => addrs.take(8).collect::<Vec<_>>(),
            Ok(Err(error)) => {
                eprintln!(
                    "SSH DNS lookup failed for {}:{}: {error}",
                    profile.host, profile.port
                );
                return Err(HostError::new("SSH_CONNECT", "remote.connectionFailed"));
            }
            Err(_) => {
                eprintln!(
                    "SSH DNS lookup timed out for {}:{}",
                    profile.host, profile.port
                );
                return Err(HostError::new("SSH_CONNECT", "remote.connectionFailed"));
            }
        };
        let mut stream = None;
        for address in addresses {
            if tokio::time::Instant::now() >= deadline {
                break;
            }
            match tokio::time::timeout_at(deadline, tokio::net::TcpStream::connect(address)).await {
                Ok(Ok(connection)) => {
                    stream = Some(connection);
                    break;
                }
                Ok(Err(error)) => {
                    eprintln!("SSH TCP connect failed for {address}: {error}");
                }
                Err(_) => break,
            }
        }
        let stream =
            stream.ok_or_else(|| HostError::new("SSH_CONNECT", "remote.connectionFailed"))?;
        let _ = stream.set_nodelay(true);

        let config = Arc::new(Config {
            inactivity_timeout: None,
            keepalive_interval: Some(Duration::from_secs(10)),
            keepalive_max: 3,
            ..Default::default()
        });
        let captured_fingerprint = Arc::new(Mutex::new(None));
        let accepted = Arc::new(Mutex::new(false));
        let handler = ClientHandler {
            host: profile.host.clone(),
            port: profile.port,
            trusted: trusted.map(str::to_owned),
            captured_fingerprint: Arc::clone(&captured_fingerprint),
            accepted: Arc::clone(&accepted),
        };
        let connect_result = tokio::time::timeout(
            Duration::from_secs(15),
            russh::client::connect_stream(config, stream, handler),
        )
        .await;

        let captured = captured_fingerprint.lock().unwrap().clone();
        let is_accepted = *accepted.lock().unwrap();
        if let Some(fp) = captured.as_ref() {
            if !is_accepted {
                if let Ok(Ok(session)) = connect_result {
                    let _ = session
                        .disconnect(Disconnect::ByApplication, "Host verification required", "")
                        .await;
                }
                return Ok((
                    Handshake::Challenge(Challenge {
                        fingerprint: fp.clone(),
                        previous: trusted.map(str::to_owned),
                    }),
                    None,
                ));
            }
        }

        match connect_result {
            Ok(Ok(session)) => Ok((Handshake::Trusted(session), captured)),
            Ok(Err(error)) => {
                eprintln!(
                    "SSH handshake failed for {}:{}: {error}",
                    profile.host, profile.port
                );
                Err(HostError::new("SSH_HANDSHAKE", "remote.handshakeFailed"))
            }
            Err(_) => {
                eprintln!(
                    "SSH handshake timed out for {}:{}",
                    profile.host, profile.port
                );
                Err(HostError::new("SSH_HANDSHAKE", "remote.handshakeFailed"))
            }
        }
    })
}

impl Connection {
    pub fn authenticate(
        profile: Profile,
        mut session: Handle<ClientHandler>,
        credentials: &Credentials,
    ) -> HostResult<Self> {
        runtime::block_on(async {
            match profile.auth.as_str() {
                "password" => {
                    let password = credentials
                        .password
                        .as_deref()
                        .filter(|value| !value.is_empty())
                        .ok_or_else(|| {
                            HostError::new("SSH_CREDENTIALS", "remote.credentialsRequired")
                        })?;
                    match session
                        .authenticate_password(&profile.user, password)
                        .await
                    {
                        Ok(AuthResult::Success) => {}
                        Ok(AuthResult::Failure {
                            remaining_methods, ..
                        }) if remaining_methods.contains(&MethodKind::KeyboardInteractive) => {
                            let mut response = session
                                .authenticate_keyboard_interactive_start(&profile.user, None)
                                .await
                                .map_err(|error| {
                                    eprintln!(
                                        "SSH keyboard-interactive auth error for {}: {error}",
                                        profile.host
                                    );
                                    HostError::new("SSH_AUTH", "remote.authFailed")
                                })?;
                            let mut attempts = 0usize;
                            let authenticated = loop {
                                match response {
                                    KeyboardInteractiveAuthResponse::Success => break true,
                                    KeyboardInteractiveAuthResponse::Failure { .. } => break false,
                                    KeyboardInteractiveAuthResponse::InfoRequest {
                                        prompts, ..
                                    } => {
                                        attempts += 1;
                                        if attempts > 4 {
                                            break false;
                                        }
                                        let answers =
                                            prompts.iter().map(|_| password.to_owned()).collect();
                                        response = session
                                            .authenticate_keyboard_interactive_respond(answers)
                                            .await
                                            .map_err(|error| {
                                                eprintln!(
                                                    "SSH keyboard-interactive respond error for {}: {error}",
                                                    profile.host
                                                );
                                                HostError::new("SSH_AUTH", "remote.authFailed")
                                            })?;
                                    }
                                }
                            };
                            if !authenticated {
                                return Err(HostError::new("SSH_AUTH", "remote.authFailed"));
                            }
                        }
                        Ok(AuthResult::Failure { .. }) => {
                            return Err(HostError::new("SSH_AUTH", "remote.authFailed"));
                        }
                        Err(error) => {
                            eprintln!(
                                "SSH password auth error for {}: {error}",
                                profile.host
                            );
                            return Err(HostError::new("SSH_AUTH", "remote.authFailed"));
                        }
                    }
                }
                "key" => {
                    let path = profile
                        .key_path
                        .as_deref()
                        .ok_or_else(|| HostError::new("SSH_KEY", "remote.invalidKey"))?;
                    let metadata = std::fs::metadata(path)
                        .map_err(|_| HostError::new("SSH_KEY", "remote.invalidKey"))?;
                    if !metadata.is_file() || metadata.len() > 1024 * 1024 {
                        return Err(HostError::new("SSH_KEY", "remote.invalidKey"));
                    }
                    let passphrase = credentials
                        .passphrase
                        .as_deref()
                        .filter(|value| !value.is_empty());
                    let key = load_secret_key(path, passphrase).map_err(|error| {
                        eprintln!("SSH key load error for {path}: {error}");
                        HostError::new("SSH_KEY", "remote.invalidKey")
                    })?;
                    let rsa_hash = session
                        .best_supported_rsa_hash()
                        .await
                        .map_err(|error| {
                            eprintln!(
                                "SSH best_supported_rsa_hash error for {}: {error}",
                                profile.host
                            );
                            HostError::new("SSH_AUTH", "remote.authFailed")
                        })?
                        .flatten();
                    let result = session
                        .authenticate_publickey(
                            &profile.user,
                            PrivateKeyWithHashAlg::new(Arc::new(key), rsa_hash),
                        )
                        .await
                        .map_err(|error| {
                            eprintln!("SSH key auth error for {}: {error}", profile.host);
                            HostError::new("SSH_AUTH", "remote.authFailed")
                        })?;
                    if !result.success() {
                        return Err(HostError::new("SSH_AUTH", "remote.authFailed"));
                    }
                }
                "agent" => {
                    let rsa_hash = session
                        .best_supported_rsa_hash()
                        .await
                        .ok()
                        .flatten()
                        .flatten();
                    macro_rules! try_agent {
                        ($client:expr) => {{
                            match $client.request_identities().await {
                                Ok(identities) => {
                                    let mut authenticated = false;
                                    for identity in identities {
                                        let public_key = identity.public_key().into_owned();
                                        let key_hash = if matches!(
                                            public_key.algorithm(),
                                            Algorithm::Rsa { .. }
                                        ) {
                                            rsa_hash
                                        } else {
                                            None
                                        };
                                        match session
                                            .authenticate_publickey_with(
                                                &profile.user,
                                                public_key,
                                                key_hash,
                                                &mut $client,
                                            )
                                            .await
                                        {
                                            Ok(result) if result.success() => {
                                                authenticated = true;
                                                break;
                                            }
                                            Ok(_) => {}
                                            Err(error) => {
                                                eprintln!(
                                                    "SSH agent signing error for {}: {error}",
                                                    profile.host
                                                );
                                            }
                                        }
                                    }
                                    authenticated
                                }
                                Err(error) => {
                                    eprintln!("SSH agent identities error: {error}");
                                    false
                                }
                            }
                        }};
                    }

                    #[cfg(windows)]
                    {
                        let mut authenticated = false;
                        if let Ok(mut agent) =
                            AgentClient::connect_named_pipe(r"\\.\pipe\openssh-ssh-agent").await
                        {
                            authenticated = try_agent!(agent);
                        }
                        if !authenticated {
                            if let Ok(mut agent) = AgentClient::connect_pageant().await {
                                authenticated = try_agent!(agent);
                            }
                        }
                        if !authenticated {
                            return Err(HostError::new("SSH_AGENT", "remote.agentUnavailable"));
                        }
                    }

                    #[cfg(not(windows))]
                    {
                        let mut agent = AgentClient::connect_env().await.map_err(|error| {
                            eprintln!("SSH agent connect error: {error}");
                            HostError::new("SSH_AGENT", "remote.agentUnavailable")
                        })?;
                        if !try_agent!(agent) {
                            return Err(HostError::new("SSH_AGENT", "remote.agentUnavailable"));
                        }
                    }
                }
                _ => return Err(HostError::new("SSH_CREDENTIALS", "remote.invalidProfile")),
            }

            let channel = session.channel_open_session().await.map_err(|error| {
                eprintln!(
                    "SSH SFTP channel open error for {}: {error}",
                    profile.host
                );
                HostError::new("SFTP_ERROR", "remote.sftpFailed")
            })?;
            channel
                .request_subsystem(true, "sftp")
                .await
                .map_err(|error| {
                    eprintln!(
                        "SSH SFTP subsystem request error for {}: {error}",
                        profile.host
                    );
                    HostError::new("SFTP_ERROR", "remote.sftpFailed")
                })?;
            let sftp = SftpSession::new(channel.into_stream())
                .await
                .map_err(|error| {
                    eprintln!(
                        "SSH SFTP session initialization error for {}: {error}",
                        profile.host
                    );
                    HostError::new("SFTP_ERROR", "remote.sftpFailed")
                })?;

            let root = match sftp.canonicalize(&profile.root).await {
                Ok(path) => path.replace('\\', "/"),
                Err(SftpError::Status(_)) => {
                    return Err(HostError::new("SSH_ROOT", "remote.notDirectory"));
                }
                Err(error) => {
                    eprintln!(
                        "SSH SFTP canonicalize root {} error: {error}",
                        profile.root
                    );
                    return Err(HostError::new("SFTP_ERROR", "remote.sftpFailed"));
                }
            };
            let metadata = match sftp.metadata(&root).await {
                Ok(metadata) => metadata,
                Err(SftpError::Status(_)) => {
                    return Err(HostError::new("SSH_ROOT", "remote.notDirectory"));
                }
                Err(error) => {
                    eprintln!("SSH SFTP metadata root {} error: {error}", root);
                    return Err(HostError::new("SFTP_ERROR", "remote.sftpFailed"));
                }
            };
            if !metadata.is_dir() {
                return Err(HostError::new("SSH_ROOT", "remote.notDirectory"));
            }

            Ok(Self {
                profile,
                session: Arc::new(session),
                sftp,
                root,
                credentials: credentials.clone(),
                ignore_cache: Mutex::new(None),
                file_index_cache: Mutex::new(None),
            })
        })
    }

    pub fn resolve(&self, relative: &str, missing: bool) -> HostResult<PathBuf> {
        let relative = if relative == self.root {
            ""
        } else if relative.starts_with('/') {
            relative
                .strip_prefix(&format!("{}/", self.root.trim_end_matches('/')))
                .ok_or_else(|| HostError::new("REMOTE_PATH", "remote.pathOutside"))?
        } else {
            relative
        };
        if relative.contains('\\')
            || relative.contains('\0')
            || relative.split('/').any(|part| part == "..")
        {
            return Err(HostError::new("REMOTE_PATH", "remote.pathOutside"));
        }
        let target = format!("{}/{}", self.root.trim_end_matches('/'), relative);
        let target_path = Path::new(&target);
        let normalized = runtime::block_on(async {
            match self.sftp.canonicalize(target.as_str()).await {
                Ok(path) => Ok(path.replace('\\', "/")),
                Err(SftpError::Status(status))
                    if missing && status.status_code == StatusCode::NoSuchFile =>
                {
                    let parent_str = target_path
                        .parent()
                        .ok_or_else(|| HostError::new("REMOTE_PATH", "remote.pathOutside"))?
                        .to_string_lossy()
                        .replace('\\', "/");
                    let file_name = target_path
                        .file_name()
                        .and_then(|name| name.to_str())
                        .ok_or_else(|| HostError::new("REMOTE_PATH", "remote.pathOutside"))?;
                    let parent = self
                        .sftp
                        .canonicalize(parent_str)
                        .await
                        .map_err(sftp_failure)?;
                    Ok(format!(
                        "{}/{}",
                        parent.replace('\\', "/").trim_end_matches('/'),
                        file_name
                    ))
                }
                Err(error) => Err(sftp_failure(error)),
            }
        })?;
        if normalized != self.root
            && !normalized.starts_with(&format!("{}/", self.root.trim_end_matches('/')))
        {
            return Err(HostError::new("REMOTE_PATH", "remote.pathOutside"));
        }
        Ok(PathBuf::from(normalized))
    }

    pub fn execute(&self, command: &str, relative: &str, timeout: u64) -> HostResult<Value> {
        if command.len() > 1024 * 1024 || command.contains('\0') {
            return Err(HostError::new("SSH_COMMAND", "Invalid command"));
        }
        let directory = self
            .resolve(relative, false)?
            .to_string_lossy()
            .replace('\\', "/");
        let command = format!("cd {} && {}", shell_quote(&directory), command);
        let timeout_duration = Duration::from_secs(timeout.clamp(1, 600));
        runtime::block_on(async {
            let mut channel = self
                .session
                .channel_open_session()
                .await
                .map_err(failure)?;
            let run = async {
                channel
                    .exec(true, command.into_bytes())
                    .await
                    .map_err(failure)?;
                let mut stdout = Vec::new();
                let mut stderr = Vec::new();
                let mut truncated = false;
                let mut exit_code: Option<u32> = None;
                while let Some(message) = channel.wait().await {
                    match message {
                        ChannelMsg::Data { data } => {
                            let retained =
                                data.len().min((256 * 1024usize).saturating_sub(stdout.len()));
                            stdout.extend_from_slice(&data[..retained]);
                            truncated |= retained < data.len();
                        }
                        ChannelMsg::ExtendedData { data, ext: 1 } => {
                            let retained =
                                data.len().min((256 * 1024usize).saturating_sub(stderr.len()));
                            stderr.extend_from_slice(&data[..retained]);
                            truncated |= retained < data.len();
                        }
                        ChannelMsg::ExitStatus { exit_status } => {
                            exit_code = Some(exit_status);
                        }
                        ChannelMsg::ExitSignal { .. } if exit_code.is_none() => {
                            exit_code = Some(1);
                        }
                        _ => {}
                    }
                }
                let code = exit_code.unwrap_or(0);
                Ok(json!({
                    "stdout": String::from_utf8_lossy(&stdout),
                    "stderr": String::from_utf8_lossy(&stderr),
                    "exitCode": code,
                    "code": code,
                    "truncated": truncated
                }))
            };
            match tokio::time::timeout(timeout_duration, run).await {
                Ok(result) => {
                    let _ = channel.close().await;
                    result
                }
                Err(_) => {
                    let _ = channel.close().await;
                    Err(HostError::new("SSH_TIMEOUT", "Remote command timed out"))
                }
            }
        })
    }

    pub async fn open_direct_tcpip_async(
        &self,
        host: &str,
        port: u16,
    ) -> HostResult<Channel<Msg>> {
        self.session
            .channel_open_direct_tcpip(host, port as u32, "127.0.0.1", 0)
            .await
            .map_err(|error| HostError::new("SSH_FORWARD_CONNECT", error.to_string()))
    }

    pub fn open_direct_tcpip(&self, host: &str, port: u16) -> HostResult<Channel<Msg>> {
        runtime::block_on(self.open_direct_tcpip_async(host, port))
    }

    pub async fn open_pty_channel_async(
        &self,
        cols: u32,
        rows: u32,
        command: &str,
    ) -> HostResult<Channel<Msg>> {
        let channel = self
            .session
            .channel_open_session()
            .await
            .map_err(|error| HostError::new("SSH_TERMINAL", error.to_string()))?;
        channel
            .request_pty(true, "xterm-256color", cols, rows, 0, 0, &[])
            .await
            .map_err(|error| HostError::new("SSH_TERMINAL", error.to_string()))?;
        channel
            .exec(true, command.as_bytes().to_vec())
            .await
            .map_err(|error| HostError::new("SSH_TERMINAL", error.to_string()))?;
        Ok(channel)
    }

    pub fn open_pty_channel(
        &self,
        cols: u32,
        rows: u32,
        command: &str,
    ) -> HostResult<Channel<Msg>> {
        runtime::block_on(self.open_pty_channel_async(cols, rows, command))
    }

    pub async fn close_async(&self) {
        let _ = self
            .session
            .disconnect(Disconnect::ByApplication, "Disconnected by application", "")
            .await;
    }

    pub fn close(&self) {
        let session = Arc::clone(&self.session);
        if tokio::runtime::Handle::try_current().is_ok() {
            runtime::runtime().spawn(async move {
                let _ = session
                    .disconnect(Disconnect::ByApplication, "Disconnected by application", "")
                    .await;
            });
        } else {
            runtime::block_on(async move {
                let _ = session
                    .disconnect(Disconnect::ByApplication, "Disconnected by application", "")
                    .await;
            });
        }
    }
}

pub fn shell_quote(value: &str) -> String {
    format!("'{}'", value.replace('\'', "'\\''"))
}
