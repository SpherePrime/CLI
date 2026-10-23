mod connection;
pub mod files;
pub mod forward;
mod store;
pub mod terminal;
pub mod tools;
pub mod types;
mod vault;

use crate::error::{HostError, HostResult};
use connection::{Connection, Handshake};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    collections::{HashMap, HashSet},
    path::Path,
    sync::{Arc, Mutex},
};
use store::Store;
use tauri::Manager;
use types::{ConnectionStatus, Credentials, Profile};

struct State {
    store: Store,
    statuses: HashMap<String, ConnectionStatus>,
    connections: HashMap<String, Arc<Mutex<Connection>>>,
    pending: HashSet<String>,
    generations: HashMap<String, u64>,
}

pub fn workspace_directory(
    state: &crate::state::AppState,
    profile: &Profile,
) -> HostResult<std::path::PathBuf> {
    let identity = format!(
        "{:x}",
        Sha256::digest(serde_json::to_vec(&json!([
            profile.host,
            profile.port,
            profile.user,
            profile.root
        ]))?)
    );
    let directory = state
        .directory
        .join("RemoteWorkspaces")
        .join(&profile.id)
        .join(&identity[..24]);
    std::fs::create_dir_all(directory.join(".prime"))?;
    let config = directory.join(".prime/prime.json");
    let mut value = match std::fs::read(&config) {
        Ok(bytes) => serde_json::from_slice::<Value>(&bytes)?,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => json!({}),
        Err(error) => return Err(error.into()),
    };
    if !value["options"].is_object() {
        value["options"] = json!({});
    }
    value["options"]["remote_workspace"] =
        json!({"host":profile.host,"user":profile.user,"root":profile.root});
    value["options"]["auto_lsp"] = json!(false);
    value["options"]["attribution"] = json!({"trailer_style":"none","generated_with":false});
    use std::io::Write;
    let mut temporary = tempfile::NamedTempFile::new_in(config.parent().unwrap())?;
    temporary.write_all(&serde_json::to_vec_pretty(&value)?)?;
    temporary.as_file().sync_all()?;
    temporary.persist(config).map_err(|error| error.error)?;
    Ok(dunce::canonicalize(directory)?)
}

pub fn workspace_key(profile: &Profile) -> String {
    let root = profile
        .root
        .split('/')
        .map(|part| url::form_urlencoded::byte_serialize(part.as_bytes()).collect::<String>())
        .collect::<Vec<_>>()
        .join("/");
    format!("prime-ssh://{}{root}", profile.id)
}

pub fn profile_from_workspace(path: &str) -> HostResult<String> {
    let url = url::Url::parse(path)
        .map_err(|_| HostError::new("REMOTE_PROFILE", "Invalid SSH workspace"))?;
    if url.scheme() != "prime-ssh"
        || !url.username().is_empty()
        || url.password().is_some()
        || url.port().is_some()
        || url.query().is_some()
        || url.fragment().is_some()
    {
        return Err(HostError::new("REMOTE_PROFILE", "Invalid SSH workspace"));
    }
    let id = url
        .host_str()
        .ok_or_else(|| HostError::new("REMOTE_PROFILE", "Invalid SSH workspace"))?;
    uuid::Uuid::parse_str(id)
        .map_err(|_| HostError::new("REMOTE_PROFILE", "Invalid SSH profile"))?;
    Ok(id.into())
}

pub fn open(app: &tauri::AppHandle, id: &str) -> HostResult<Value> {
    let state = app.state::<crate::state::AppState>();
    if let Some(project) = state
        .projects
        .lock()
        .unwrap()
        .values()
        .find(|project| project["remote"]["profileId"] == id)
        .cloned()
    {
        return Ok(project);
    }
    if state.remotes.connection(id).is_err() {
        state.remotes.connect(app, id, Credentials::default())?;
    }
    let connection = state.remotes.connection(id)?;
    let (profile, root) = {
        let connection = connection.lock().unwrap();
        (connection.profile.clone(), connection.root.clone())
    };
    let directory = workspace_directory(&state, &profile)?;
    let mut project = crate::projects::open(app, &state, directory.clone(), "project")?;
    let workspace_id = project["id"].as_str().unwrap().to_owned();
    let key = workspace_key(&profile);
    project["localPath"] = json!(directory);
    project["path"] = json!(key);
    project["remote"] = json!({"profileId":profile.id,"name":profile.name,"host":profile.host,"port":profile.port,"user":profile.user,"root":root});
    state
        .projects
        .lock()
        .unwrap()
        .insert(workspace_id.clone(), project.clone());
    let mut settings = state.settings.lock().unwrap();
    let mut recent = settings.get()["recent"]
        .as_array()
        .cloned()
        .unwrap_or_default();
    let local_path = directory.to_string_lossy().into_owned();
    recent.retain(|path| path.as_str() != Some(local_path.as_str()) && path.as_str() != Some(&key));
    recent.insert(0, json!(key));
    recent.truncate(64);
    settings.update(&json!({"recent":recent,"activeProject":key}))?;
    drop(settings);
    crate::events::emit(app, &workspace_id, "projectsChanged", json!({}));
    Ok(project)
}

pub fn workspace_profile(state: &crate::state::AppState, id: &str) -> Option<String> {
    state
        .projects
        .lock()
        .unwrap()
        .get(id)
        .and_then(|project| project["remote"]["profileId"].as_str())
        .map(str::to_owned)
}

pub fn detach(app: &tauri::AppHandle, profile: &str) -> HostResult<()> {
    let state = app.state::<crate::state::AppState>();
    let workspaces = state
        .projects
        .lock()
        .unwrap()
        .values()
        .filter(|project| project["remote"]["profileId"] == profile)
        .filter_map(|project| project["id"].as_str().map(str::to_owned))
        .collect::<Vec<_>>();
    for workspace in workspaces {
        if let Ok(engine) = state.require_engine() {
            if let Ok(sessions) =
                engine.request("GET", &format!("/v1/workspaces/{workspace}/sessions"), None)
            {
                if let Some(sessions) = sessions.as_array() {
                    for session in sessions {
                        if session["is_busy"].as_bool().unwrap_or(false) {
                            if let Some(id) = session["id"].as_str() {
                                let _ = engine.request(
                                    "POST",
                                    &format!(
                                        "/v1/workspaces/{workspace}/agent/sessions/{id}/cancel"
                                    ),
                                    None,
                                );
                            }
                        }
                    }
                }
            }
        }
        crate::projects::close(app, &state, &workspace)?;
    }
    crate::events::emit(app, "", "projectsChanged", json!({}));
    Ok(())
}

pub fn file_call(
    state: &crate::state::AppState,
    id: &str,
    method: &str,
    args: &[Value],
) -> HostResult<Value> {
    let profile = workspace_profile(state, id)
        .ok_or_else(|| HostError::new("REMOTE_PROFILE", "Workspace is not remote"))?;
    let connection = state.remotes.connection(&profile)?;
    let connection = connection.lock().unwrap();
    let path = args.get(1).and_then(Value::as_str).unwrap_or("");
    match method {
        "files" => files::list(&connection, path),
        "readFile" => files::read(&connection, path),
        "saveFile" => files::save(
            &connection,
            path,
            args.get(2)
                .and_then(Value::as_str)
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing file content"))?,
            args.get(3)
                .and_then(Value::as_str)
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing file revision"))?,
        ),
        "createEntry" => files::create(
            &connection,
            path,
            args.get(2).and_then(Value::as_str).unwrap_or(""),
            &[],
        ),
        "trashEntry" => files::trash(&connection, path),
        _ => Err(HostError::unsupported(&format!("remote {method}"))),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn profile_validation_and_store_round_trip() {
        let directory = tempfile::tempdir().unwrap();
        let profile=serde_json::from_value::<Profile>(json!({"name":"Fixture","host":"localhost","port":22,"user":"fixture","root":"/project/./nested/","auth":"agent","runtime":"bundled"})).unwrap().validate().unwrap();
        assert_eq!(profile.root, "/project/nested");
        let mut store = Store::load(directory.path()).unwrap();
        let mut next = store.value.clone();
        next.profiles.push(profile.clone());
        store.persist(next).unwrap();
        let restored = Store::load(directory.path()).unwrap();
        assert_eq!(restored.value.profiles[0].id, profile.id);
        let mut invalid = profile;
        invalid.root = "/project/../outside".into();
        assert!(invalid.validate().is_err());
    }

    #[test]
    #[ignore]
    fn native_ssh_sftp_round_trip() {
        let key = std::env::var("PRIME_NATIVE_SSH_TEST_KEY")
            .expect("Set PRIME_NATIVE_SSH_TEST_KEY to an isolated SSH fixture identity");
        let root = std::env::var("PRIME_NATIVE_SSH_TEST_ROOT")
            .unwrap_or_else(|_| "/tmp/prime-native-ssh-fixture/project".into());
        let port = std::env::var("PRIME_NATIVE_SSH_TEST_PORT")
            .unwrap_or_else(|_| "22349".into())
            .parse::<u16>()
            .unwrap();
        let profile=serde_json::from_value::<Profile>(json!({"name":"Fixture","host":"127.0.0.1","port":port,"user":"root","root":root,"auth":"key","keyPath":key,"runtime":"bundled"})).unwrap().validate().unwrap();
        let challenge = match connection::handshake(&profile, None).unwrap() {
            Handshake::Challenge(challenge) => challenge,
            Handshake::Trusted(_) => panic!("Unknown SSH host was trusted"),
        };
        let session = match connection::handshake(&profile, Some(&challenge.fingerprint)).unwrap() {
            Handshake::Trusted(session) => session,
            Handshake::Challenge(_) => panic!("Expected trusted fixture key"),
        };
        let connection =
            Connection::authenticate(profile, session, &Credentials::default()).unwrap();
        let result = connection
            .execute("printf 'native Привет'; printf 'stderr' >&2", "", 5)
            .unwrap();
        assert_eq!(result["stdout"], "native Привет");
        assert_eq!(result["stderr"], "stderr");
        assert_eq!(result["exitCode"], 0);
        let original = files::read(&connection, "file.txt").unwrap();
        let saved = files::save(
            &connection,
            "file.txt",
            "native updated",
            original["revision"].as_str().unwrap(),
        )
        .unwrap();
        assert!(files::save(
            &connection,
            "file.txt",
            "stale overwrite",
            original["revision"].as_str().unwrap()
        )
        .is_err());
        assert!(connection.resolve("../outside.txt", false).is_err());
        assert!(connection.resolve("escape.txt", false).is_err());
        let entries = files::list(&connection, "").unwrap();
        assert!(entries
            .as_array()
            .unwrap()
            .iter()
            .any(|entry| entry["name"] == "file.txt"));
        files::save(
            &connection,
            "file.txt",
            original["content"].as_str().unwrap(),
            saved["revision"].as_str().unwrap(),
        )
        .unwrap();
        let name = format!("upload-{}.bin", uuid::Uuid::new_v4());
        files::upload(&connection, &name, &[0, 1, 2, 255], false).unwrap();
        assert_eq!(
            files::bytes(&connection, &name, 1024).unwrap(),
            vec![0, 1, 2, 255]
        );
        assert!(files::upload(&connection, &name, b"replace", false).is_err());
        connection
            .sftp
            .unlink(&connection.resolve(&name, false).unwrap())
            .unwrap();
        connection.close();
    }

    #[test]
    #[ignore]
    fn native_credential_vault_round_trip() {
        let profile=serde_json::from_value::<Profile>(json!({"name":"Fixture","host":"localhost","port":22,"user":"fixture","root":"/project","auth":"password","runtime":"bundled"})).unwrap().validate().unwrap();
        assert!(vault::available());
        let credentials = Credentials {
            password: Some(uuid::Uuid::new_v4().to_string()),
            passphrase: None,
        };
        vault::save(&profile, &credentials).unwrap();
        assert_eq!(
            vault::load(&profile).unwrap().password,
            credentials.password
        );
        vault::remove(&profile).unwrap();
        assert!(!vault::has(&profile));
    }
}

pub struct RemoteManager {
    inner: Mutex<State>,
    pub terminals: terminal::Terminals,
    pub forwards: forward::ForwardManager,
}

impl RemoteManager {
    pub fn load(directory: &Path) -> HostResult<Self> {
        Ok(Self {
            terminals: terminal::Terminals::default(),
            forwards: forward::ForwardManager::default(),
            inner: Mutex::new(State {
                store: Store::load(directory)?,
                statuses: HashMap::new(),
                connections: HashMap::new(),
                pending: HashSet::new(),
                generations: HashMap::new(),
            }),
        })
    }

    pub fn snapshot(&self) -> Value {
        let state = self.inner.lock().unwrap();
        let profiles =
            state
                .store
                .value
                .profiles
                .iter()
                .map(|profile| {
                    let mut value = serde_json::to_value(profile).unwrap();
                    let status = state.statuses.get(&profile.id).cloned().unwrap_or_else(|| {
                        ConnectionStatus {
                            status: "disconnected".into(),
                            ..Default::default()
                        }
                    });
                    value.as_object_mut().unwrap().extend(
                        serde_json::to_value(status)
                            .unwrap()
                            .as_object()
                            .unwrap()
                            .clone(),
                    );
                    value["hasSavedCredentials"] = json!(vault::has(profile));
                    value
                })
                .collect::<Vec<_>>();
        json!({"profiles":profiles,"credentialStorageAvailable":vault::available()})
    }

    fn emit(&self, app: &tauri::AppHandle) {
        crate::events::emit(app, "", "remote", self.snapshot());
    }

    pub fn profile(&self, id: &str) -> HostResult<Profile> {
        self.inner
            .lock()
            .unwrap()
            .store
            .value
            .profiles
            .iter()
            .find(|profile| profile.id == id)
            .cloned()
            .ok_or_else(|| HostError::new("REMOTE_PROFILE", "remote.notFound"))
    }

    pub fn connection(&self, id: &str) -> HostResult<Arc<Mutex<Connection>>> {
        self.inner
            .lock()
            .unwrap()
            .connections
            .get(id)
            .cloned()
            .ok_or_else(|| HostError::new("SSH_DISCONNECTED", "remote.notConnected"))
    }

    pub fn dispatch(&self, app: &tauri::AppHandle, request: &Value) -> HostResult<Value> {
        let id = || {
            request["id"]
                .as_str()
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing SSH profile id"))
        };
        match request["action"].as_str().unwrap_or("") {
            "list" => return Ok(self.snapshot()),
            "save" => {
                let profile =
                    serde_json::from_value::<Profile>(request["input"].clone())?.validate()?;
                self.disconnect(&profile.id)?;
                let mut state = self.inner.lock().unwrap();
                let mut next = state.store.value.clone();
                if let Some(existing) = next
                    .profiles
                    .iter_mut()
                    .find(|saved| saved.id == profile.id)
                {
                    if existing.credential_id() != profile.credential_id() {
                        vault::remove(existing)?;
                    }
                    *existing = profile;
                } else {
                    next.profiles.push(profile);
                }
                state.store.persist(next)?;
            }
            "remove" => {
                let profile = self.profile(id()?)?;
                self.disconnect(&profile.id)?;
                vault::remove(&profile)?;
                let mut state = self.inner.lock().unwrap();
                let mut next = state.store.value.clone();
                next.profiles.retain(|saved| saved.id != profile.id);
                state.store.persist(next)?;
                state.statuses.remove(&profile.id);
            }
            "forgetCredentials" => vault::remove(&self.profile(id()?)?)?,
            "disconnect" => self.disconnect(id()?)?,
            "connect" => {
                let credentials = request
                    .get("credentials")
                    .filter(|value| !value.is_null())
                    .cloned()
                    .map(serde_json::from_value)
                    .transpose()?
                    .unwrap_or_default();
                return self.connect(app, id()?, credentials);
            }
            "trust" => {
                let id = id()?;
                let fingerprint = request["fingerprint"].as_str().ok_or_else(|| {
                    HostError::new("INVALID_ARGUMENT", "Missing host fingerprint")
                })?;
                let mut state = self.inner.lock().unwrap();
                let challenge = state
                    .statuses
                    .get(id)
                    .and_then(|status| status.challenge.as_ref())
                    .ok_or_else(|| HostError::new("SSH_HOST_KEY", "remote.staleChallenge"))?;
                if challenge.fingerprint != fingerprint {
                    return Err(HostError::new("SSH_HOST_KEY", "remote.staleChallenge"));
                }
                let profile = state
                    .store
                    .value
                    .profiles
                    .iter()
                    .find(|profile| profile.id == id)
                    .cloned()
                    .ok_or_else(|| HostError::new("REMOTE_PROFILE", "remote.notFound"))?;
                let mut next = state.store.value.clone();
                next.hosts.insert(profile.host_id(), fingerprint.into());
                state.store.persist(next)?;
                state.statuses.insert(
                    id.into(),
                    ConnectionStatus {
                        status: "disconnected".into(),
                        ..Default::default()
                    },
                );
            }
            _ => return Err(HostError::unsupported("remote action")),
        }
        self.emit(app);
        Ok(self.snapshot())
    }

    pub fn connect(
        &self,
        app: &tauri::AppHandle,
        id: &str,
        credentials: Credentials,
    ) -> HostResult<Value> {
        let profile = self.profile(id)?;
        let (generation, trusted) = {
            let mut state = self.inner.lock().unwrap();
            if state.pending.contains(id) || state.connections.contains_key(id) {
                return Err(HostError::new("SSH_CONNECT", "remote.alreadyConnecting"));
            }
            state.pending.insert(id.into());
            let generation = state.generations.get(id).copied().unwrap_or(0) + 1;
            state.generations.insert(id.into(), generation);
            state.statuses.insert(
                id.into(),
                ConnectionStatus {
                    status: "connecting".into(),
                    ..Default::default()
                },
            );
            (
                generation,
                state.store.value.hosts.get(&profile.host_id()).cloned(),
            )
        };
        self.emit(app);
        let supplied = credentials.supplied();
        let result: HostResult<Result<Connection, types::Challenge>> = (|| {
            let session = match connection::handshake(&profile, trusted.as_deref())? {
                Handshake::Challenge(challenge) => return Ok(Err(challenge)),
                Handshake::Trusted(session) => session,
            };
            let credentials = if supplied {
                credentials
            } else if profile.auth == "agent" || !vault::available() {
                Credentials::default()
            } else {
                vault::load(&profile)?
            };
            let connection = Connection::authenticate(profile.clone(), session, &credentials)?;
            if supplied && vault::available() {
                vault::save(&profile, &credentials)?;
            }
            Ok(Ok(connection))
        })();
        let mut state = self.inner.lock().unwrap();
        state.pending.remove(id);
        if state.generations.get(id) != Some(&generation) {
            if let Ok(Ok(connection)) = result {
                connection.close();
            }
            drop(state);
            return Ok(self.snapshot());
        }
        match result {
            Ok(Ok(connection)) => {
                state
                    .connections
                    .insert(id.into(), Arc::new(Mutex::new(connection)));
                state.statuses.insert(
                    id.into(),
                    ConnectionStatus {
                        status: "connected".into(),
                        ..Default::default()
                    },
                );
            }
            Ok(Err(challenge)) => {
                state.statuses.insert(
                    id.into(),
                    ConnectionStatus {
                        status: "verifying".into(),
                        challenge: Some(challenge),
                        ..Default::default()
                    },
                );
            }
            Err(error) => {
                state.statuses.insert(
                    id.into(),
                    ConnectionStatus {
                        status: "error".into(),
                        error: Some(if error.message.starts_with("remote.") {
                            error.message
                        } else {
                            "remote.connectionFailed".into()
                        }),
                        ..Default::default()
                    },
                );
            }
        }
        drop(state);
        self.emit(app);
        Ok(self.snapshot())
    }

    pub fn disconnect(&self, id: &str) -> HostResult<()> {
        self.forwards.close_profile(id);
        self.terminals.close_profile(id);
        let connection = {
            let mut state = self.inner.lock().unwrap();
            *state.generations.entry(id.into()).or_default() += 1;
            state.statuses.insert(
                id.into(),
                ConnectionStatus {
                    status: "disconnected".into(),
                    ..Default::default()
                },
            );
            state.connections.remove(id)
        };
        if let Some(connection) = connection {
            connection.lock().unwrap().close();
        }
        Ok(())
    }

    pub fn shutdown(&self) {
        self.forwards.shutdown();
        let ids = self
            .inner
            .lock()
            .unwrap()
            .connections
            .keys()
            .cloned()
            .collect::<Vec<_>>();
        for id in ids {
            let _ = self.disconnect(&id);
        }
    }

    pub fn connected(&self) -> Value {
        let state = self.inner.lock().unwrap();
        json!(state
            .store
            .value
            .profiles
            .iter()
            .filter(|profile| state.connections.contains_key(&profile.id))
            .cloned()
            .collect::<Vec<_>>())
    }

    pub fn dedicated_connection(&self, id: &str) -> HostResult<Connection> {
        let existing = self.connection(id)?;
        let credentials = existing.lock().unwrap().credentials.clone();
        let profile = self.profile(id)?;
        let trusted = self
            .inner
            .lock()
            .unwrap()
            .store
            .value
            .hosts
            .get(&profile.host_id())
            .cloned();
        let session = match connection::handshake(&profile, trusted.as_deref())? {
            Handshake::Trusted(session) => session,
            Handshake::Challenge(_) => {
                return Err(HostError::new(
                    "SSH_HOST_KEY",
                    "Host key changed; verify the server again in the SSH panel",
                ))
            }
        };
        Connection::authenticate(profile, session, &credentials)
    }

    pub fn forward(
        self: &Arc<Self>,
        profile: &str,
        remote_port: u16,
        local_port: u16,
        host: &str,
    ) -> HostResult<Value> {
        let weak = Arc::downgrade(self);
        let id = profile.to_owned();
        let connect: forward::Connector = Arc::new(move || {
            weak.upgrade()
                .ok_or_else(|| HostError::new("SSH_CLOSED", "SSH manager is shutting down"))?
                .dedicated_connection(&id)
        });
        self.forwards
            .open(profile, remote_port, local_port, host, connect)
    }
}
