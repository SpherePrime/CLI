use super::types::{Challenge, Credentials, Profile};
use crate::error::{HostError, HostResult};
use base64::Engine;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use ssh2::{Session, Sftp};
use std::{
    io::Read,
    net::{TcpStream, ToSocketAddrs},
    path::Path,
    time::{Duration, Instant},
};

pub enum Handshake {
    Trusted(Session),
    Challenge(Challenge),
}

pub struct Connection {
    pub profile: Profile,
    pub session: Session,
    pub sftp: Sftp,
    pub root: String,
    pub credentials: Credentials,
}

fn failure(error: impl std::fmt::Display) -> HostError {
    HostError::new("SSH_ERROR", error.to_string())
}

pub fn handshake(profile: &Profile, trusted: Option<&str>) -> HostResult<Handshake> {
    let addresses = (profile.host.as_str(), profile.port)
        .to_socket_addrs()?
        .take(8)
        .collect::<Vec<_>>();
    let deadline = Instant::now() + Duration::from_secs(15);
    let mut stream = None;
    for address in addresses {
        let timeout = deadline.saturating_duration_since(Instant::now());
        if timeout.is_zero() {
            break;
        }
        if let Ok(connection) = TcpStream::connect_timeout(&address, timeout) {
            stream = Some(connection);
            break;
        }
    }
    let stream = stream.ok_or_else(|| HostError::new("SSH_CONNECT", "remote.connectionFailed"))?;
    stream.set_read_timeout(Some(Duration::from_secs(15)))?;
    stream.set_write_timeout(Some(Duration::from_secs(15)))?;
    let mut session = Session::new().map_err(failure)?;
    session.set_timeout(15000);
    session.set_tcp_stream(stream);
    session.handshake().map_err(failure)?;
    let key = session
        .host_key()
        .map(|(key, _)| key)
        .ok_or_else(|| HostError::new("SSH_HOST_KEY", "SSH server did not provide a host key"))?;
    let fingerprint = format!(
        "SHA256:{}",
        base64::engine::general_purpose::STANDARD_NO_PAD.encode(Sha256::digest(key))
    );
    if trusted != Some(fingerprint.as_str()) {
        let _ = session.disconnect(None, "Host verification required", None);
        return Ok(Handshake::Challenge(Challenge {
            fingerprint,
            previous: trusted.map(str::to_owned),
        }));
    }
    Ok(Handshake::Trusted(session))
}

impl Connection {
    pub fn authenticate(
        profile: Profile,
        session: Session,
        credentials: &Credentials,
    ) -> HostResult<Self> {
        match profile.auth.as_str() {
            "password" => session
                .userauth_password(
                    &profile.user,
                    credentials
                        .password
                        .as_deref()
                        .filter(|value| !value.is_empty())
                        .ok_or_else(|| {
                            HostError::new("SSH_CREDENTIALS", "remote.credentialsRequired")
                        })?,
                )
                .map_err(failure)?,
            "key" => {
                let path = profile
                    .key_path
                    .as_deref()
                    .ok_or_else(|| HostError::new("SSH_KEY", "remote.invalidKey"))?;
                let metadata = std::fs::metadata(path)?;
                if !metadata.is_file() || metadata.len() > 1024 * 1024 {
                    return Err(HostError::new("SSH_KEY", "remote.invalidKey"));
                }
                session
                    .userauth_pubkey_file(
                        &profile.user,
                        None,
                        Path::new(path),
                        credentials.passphrase.as_deref(),
                    )
                    .map_err(failure)?;
            }
            "agent" => session.userauth_agent(&profile.user).map_err(failure)?,
            _ => return Err(HostError::new("SSH_CREDENTIALS", "remote.invalidProfile")),
        }
        if !session.authenticated() {
            return Err(HostError::new("SSH_AUTH", "remote.credentialsRequired"));
        }
        session.set_keepalive(true, 10);
        let sftp = session.sftp().map_err(failure)?;
        let root = sftp
            .realpath(Path::new(&profile.root))
            .map_err(failure)?
            .to_string_lossy()
            .replace('\\', "/");
        let metadata = sftp.stat(Path::new(&root)).map_err(failure)?;
        if !metadata.is_dir() {
            return Err(HostError::new("SSH_ROOT", "remote.notDirectory"));
        }
        Ok(Self {
            profile,
            session,
            sftp,
            root,
            credentials: credentials.clone(),
        })
    }

    pub fn resolve(&self, relative: &str, missing: bool) -> HostResult<std::path::PathBuf> {
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
        let target = Path::new(&target);
        let resolved =
            match self.sftp.realpath(target) {
                Ok(path) => path,
                Err(error) if missing && matches!(error.code(), ssh2::ErrorCode::SFTP(2)) => {
                    let parent =
                        self.sftp
                            .realpath(target.parent().ok_or_else(|| {
                                HostError::new("REMOTE_PATH", "remote.pathOutside")
                            })?)
                            .map_err(failure)?;
                    parent.join(
                        target
                            .file_name()
                            .ok_or_else(|| HostError::new("REMOTE_PATH", "remote.pathOutside"))?,
                    )
                }
                Err(error) => return Err(failure(error)),
            };
        let normalized = resolved.to_string_lossy().replace('\\', "/");
        if normalized != self.root
            && !normalized.starts_with(&format!("{}/", self.root.trim_end_matches('/')))
        {
            return Err(HostError::new("REMOTE_PATH", "remote.pathOutside"));
        }
        Ok(resolved)
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
        let mut channel = self.session.channel_session().map_err(failure)?;
        channel.exec(&command).map_err(failure)?;
        self.session.set_blocking(false);
        let result = (|| {
            let deadline = Instant::now() + Duration::from_secs(timeout.clamp(1, 600));
            let mut stdout = Vec::new();
            let mut stderr = Vec::new();
            let mut truncated = false;
            let mut buffer = [0u8; 8192];
            loop {
                if Instant::now() >= deadline {
                    return Err(HostError::new("SSH_TIMEOUT", "Remote command timed out"));
                }
                for (stream, output) in [
                    (&mut channel.stream(0), &mut stdout),
                    (&mut channel.stderr(), &mut stderr),
                ] {
                    match stream.read(&mut buffer) {
                        Ok(count) => {
                            let retained =
                                count.min((256 * 1024usize).saturating_sub(output.len()));
                            output.extend_from_slice(&buffer[..retained]);
                            truncated |= retained < count;
                        }
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
                        Err(error) => return Err(error.into()),
                    }
                }
                if channel.eof() {
                    break;
                }
                std::thread::sleep(Duration::from_millis(10));
            }
            loop {
                match channel.wait_close() {
                    Ok(()) => break,
                    Err(error) if error.code() == ssh2::ErrorCode::Session(-37) => {
                        std::thread::sleep(Duration::from_millis(10))
                    }
                    Err(error) => return Err(failure(error)),
                }
                if Instant::now() >= deadline {
                    return Err(HostError::new(
                        "SSH_TIMEOUT",
                        "Remote command did not close",
                    ));
                }
            }
            Ok(
                json!({"stdout":String::from_utf8_lossy(&stdout),"stderr":String::from_utf8_lossy(&stderr),"exitCode":channel.exit_status().map_err(failure)?,"code":channel.exit_status().map_err(failure)?,"truncated":truncated}),
            )
        })();
        let _ = channel.close();
        self.session.set_blocking(true);
        result
    }

    pub fn close(&self) {
        let _ = self
            .session
            .disconnect(None, "Disconnected by application", None);
    }
}

pub fn shell_quote(value: &str) -> String {
    format!("'{}'", value.replace('\'', "'\\''"))
}
