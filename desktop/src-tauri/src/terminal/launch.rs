use super::{session::Emitter, types::TerminalOptions, TerminalManager};
use crate::error::{HostError, HostResult};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    io::Write,
    path::{Path, PathBuf},
    sync::{Arc, Mutex, OnceLock},
    time::{SystemTime, UNIX_EPOCH},
};
use tauri::Manager;

#[derive(Clone, Serialize, Deserialize)]
struct LaunchConfig {
    id: String,
    name: String,
    command: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    url: Option<String>,
}

#[derive(Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct LaunchProcess {
    config_id: String,
    terminal_id: String,
    status: String,
    started_at: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    url: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    exit_code: Option<u64>,
    #[serde(skip)]
    root: String,
    #[serde(skip)]
    workspace: String,
    #[serde(skip)]
    buffer: String,
}

#[derive(Clone, Serialize, Deserialize)]
struct SavedProfiles {
    version: u32,
    configs: HashMap<String, Vec<LaunchConfig>>,
}

struct LaunchState {
    saved: SavedProfiles,
    processes: HashMap<String, LaunchProcess>,
}

pub struct LaunchManager {
    path: PathBuf,
    state: Arc<Mutex<LaunchState>>,
    operations: Mutex<()>,
}

type UrlHandler = Arc<dyn Fn(String) + Send + Sync>;

impl LaunchManager {
    pub fn load(directory: &Path) -> HostResult<Self> {
        std::fs::create_dir_all(directory)?;
        let path = directory.join("launch.json");
        let saved = match std::fs::read(&path) {
            Ok(bytes) => serde_json::from_slice::<SavedProfiles>(&bytes)?,
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => SavedProfiles {
                version: 1,
                configs: HashMap::new(),
            },
            Err(error) => return Err(error.into()),
        };
        if saved.version != 1 {
            return Err(HostError::new(
                "LAUNCH_STORAGE",
                "Unsupported launch profile format",
            ));
        }
        Ok(Self {
            path,
            state: Arc::new(Mutex::new(LaunchState {
                saved,
                processes: HashMap::new(),
            })),
            operations: Mutex::new(()),
        })
    }

    pub fn handle(
        &self,
        app: tauri::AppHandle,
        terminals: &TerminalManager,
        workspace: &str,
        cwd: &Path,
        request: &Value,
    ) -> HostResult<Value> {
        let event_app = app.clone();
        let emit: Emitter = Arc::new(move |workspace, payload| {
            let kind = payload
                .get("__launch")
                .and_then(Value::as_bool)
                .unwrap_or(false);
            if kind {
                crate::events::emit(&event_app, "", "launch", json!({"root":payload["root"]}));
            } else {
                crate::events::emit(&event_app, workspace, "terminal", payload);
            }
        });
        let on_url: UrlHandler = Arc::new(move |url| {
            if let Some(state) = app.try_state::<crate::state::AppState>() {
                let _ = state.browser.navigate(&app, &url, None, false, None);
            }
        });
        self.dispatch(terminals, workspace, cwd, request, emit, on_url)
    }

    fn dispatch(
        &self,
        terminals: &TerminalManager,
        workspace: &str,
        cwd: &Path,
        request: &Value,
        emit: Emitter,
        on_url: UrlHandler,
    ) -> HostResult<Value> {
        let _operation = self.operations.lock().unwrap();
        let root = dunce::canonicalize(cwd)?.to_string_lossy().into_owned();
        let action = super::string(request, "action")?;
        match action {
            "list" => {}
            "save" => {
                let config = validate_config(request.get("input").ok_or_else(|| {
                    HostError::new("INVALID_ARGUMENT", "Launch input is required")
                })?)?;
                let mut state = self.state.lock().unwrap();
                let mut next = state.saved.clone();
                let configs = next.configs.entry(root.clone()).or_default();
                configs.retain(|item| item.id != config.id);
                if configs.len() >= 30 {
                    return Err(HostError::new("LAUNCH_LIMIT", "Too many launch profiles"));
                }
                configs.push(config);
                self.persist(&next)?;
                state.saved = next;
            }
            "remove" => {
                let id = profile_id(request)?;
                self.require_config(&root, id)?;
                self.stop(terminals, workspace, &root, id)?;
                let mut state = self.state.lock().unwrap();
                let mut next = state.saved.clone();
                next.configs
                    .entry(root.clone())
                    .or_default()
                    .retain(|config| config.id != id);
                self.persist(&next)?;
                state.saved = next;
            }
            "start" | "restart" => {
                let id = profile_id(request)?;
                let config = self.require_config(&root, id)?;
                if action == "restart" {
                    self.stop(terminals, workspace, &root, id)?;
                }
                self.start(
                    terminals,
                    workspace,
                    cwd,
                    &root,
                    config,
                    emit.clone(),
                    on_url,
                )?;
            }
            "stop" => {
                let id = profile_id(request)?;
                self.require_config(&root, id)?;
                self.stop(terminals, workspace, &root, id)?;
            }
            _ => return Err(HostError::new("INVALID_ARGUMENT", "Unknown launch action")),
        }
        if action != "list" {
            emit("", json!({"__launch":true,"root":root}));
        }
        Ok(self.snapshot(&root))
    }

    fn persist(&self, profiles: &SavedProfiles) -> HostResult<()> {
        let mut temporary = tempfile::NamedTempFile::new_in(self.path.parent().unwrap())?;
        temporary.write_all(&serde_json::to_vec_pretty(profiles)?)?;
        temporary.as_file().sync_all()?;
        temporary.persist(&self.path).map_err(|error| error.error)?;
        Ok(())
    }

    fn require_config(&self, root: &str, id: &str) -> HostResult<LaunchConfig> {
        self.state
            .lock()
            .unwrap()
            .saved
            .configs
            .get(root)
            .and_then(|configs| configs.iter().find(|config| config.id == id))
            .cloned()
            .ok_or_else(|| HostError::new("LAUNCH_NOT_FOUND", "Launch profile not found"))
    }

    fn snapshot(&self, root: &str) -> Value {
        let state = self.state.lock().unwrap();
        json!({"root":root,"configs":state.saved.configs.get(root).cloned().unwrap_or_default(),"processes":state.processes.values().filter(|process| process.root == root).collect::<Vec<_>>()})
    }

    fn start(
        &self,
        terminals: &TerminalManager,
        workspace: &str,
        cwd: &Path,
        root: &str,
        config: LaunchConfig,
        emit: Emitter,
        on_url: UrlHandler,
    ) -> HostResult<()> {
        let key = format!("{root}\0{}", config.id);
        if self
            .state
            .lock()
            .unwrap()
            .processes
            .get(&key)
            .is_some_and(|process| process.status == "running")
        {
            return Ok(());
        }
        self.stop(terminals, workspace, root, &config.id)?;
        let launch_state = self.state.clone();
        let terminal_emit: Emitter = Arc::new(move |workspace, payload| {
            emit(workspace, payload.clone());
            if let Some((root, url)) = update_process(&launch_state, &payload) {
                emit("", json!({"__launch":true,"root":root}));
                if let Some(url) = url {
                    on_url(url);
                }
            }
        });
        let terminal = terminals.create(
            workspace,
            cwd,
            TerminalOptions {
                cols: Some(120.0),
                rows: Some(30.0),
                title: Some(config.name),
                ..Default::default()
            },
            terminal_emit,
        )?;
        self.state.lock().unwrap().processes.insert(
            key.clone(),
            LaunchProcess {
                config_id: config.id,
                terminal_id: terminal.id.clone(),
                status: "running".into(),
                started_at: SystemTime::now()
                    .duration_since(UNIX_EPOCH)
                    .unwrap_or_default()
                    .as_millis() as u64,
                url: config.url,
                exit_code: None,
                root: root.into(),
                workspace: workspace.into(),
                buffer: String::new(),
            },
        );
        let command = if cfg!(windows) {
            powershell_command(&config.command)
        } else {
            config.command
        };
        let result = terminals.find(workspace, &terminal.id)?.write(
            &format!("{command}{}", if cfg!(windows) { "\r" } else { "\n" }),
            "user",
        );
        if let Err(error) = result {
            self.state.lock().unwrap().processes.remove(&key);
            let _ = terminals.close_one(workspace, &terminal.id);
            return Err(error);
        }
        Ok(())
    }

    fn stop(
        &self,
        terminals: &TerminalManager,
        workspace: &str,
        root: &str,
        id: &str,
    ) -> HostResult<()> {
        let key = format!("{root}\0{id}");
        let process = self.state.lock().unwrap().processes.get(&key).cloned();
        if let Some(process) = process {
            if process.workspace != workspace {
                return Err(HostError::new(
                    "ACCESS_DENIED",
                    "Launch process belongs to another workspace",
                ));
            }
            match terminals.close_one(workspace, &process.terminal_id) {
                Ok(()) => {}
                Err(error) if error.code == "TERMINAL_NOT_FOUND" => {}
                Err(error) => return Err(error),
            }
            self.state.lock().unwrap().processes.remove(&key);
        }
        Ok(())
    }

    pub fn all(&self) -> Value {
        let state = self.state.lock().unwrap();
        Value::Array(
            state
                .processes
                .values()
                .map(|process| {
                    let mut value = serde_json::to_value(process).unwrap();
                    value["key"] = json!(process.root);
                    value["workspaceId"] = json!(process.workspace);
                    value["name"] = json!(state
                        .saved
                        .configs
                        .get(&process.root)
                        .and_then(|configs| configs
                            .iter()
                            .find(|config| config.id == process.config_id))
                        .map(|config| config.name.as_str())
                        .unwrap_or(""));
                    value
                })
                .collect(),
        )
    }
}

fn profile_id(request: &Value) -> HostResult<&str> {
    let id = super::string(request, "id")?;
    uuid::Uuid::parse_str(id)
        .map_err(|_| HostError::new("INVALID_ARGUMENT", "Invalid launch profile id"))?;
    Ok(id)
}

fn validate_config(input: &Value) -> HostResult<LaunchConfig> {
    let id = match input.get("id") {
        None => uuid::Uuid::new_v4().to_string(),
        Some(value) => {
            let id = value
                .as_str()
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Invalid launch profile id"))?;
            uuid::Uuid::parse_str(id)
                .map_err(|_| HostError::new("INVALID_ARGUMENT", "Invalid launch profile id"))?;
            id.into()
        }
    };
    let name = super::string(input, "name")?.trim().to_string();
    let command = super::string(input, "command")?.trim().to_string();
    if name.is_empty()
        || name.chars().count() > 80
        || command.is_empty()
        || command.chars().count() > 4000
        || command.contains(['\r', '\n', '\0'])
    {
        return Err(HostError::new(
            "INVALID_ARGUMENT",
            "Invalid launch name or command",
        ));
    }
    let url = match input.get("url") {
        None => None,
        Some(value) => {
            let value = value
                .as_str()
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Invalid launch URL"))?
                .trim();
            if value.is_empty() {
                None
            } else {
                let parsed = url::Url::parse(value)
                    .map_err(|_| HostError::new("INVALID_ARGUMENT", "Invalid launch URL"))?;
                if value.len() > 2000 || !matches!(parsed.scheme(), "http" | "https") {
                    return Err(HostError::new(
                        "INVALID_ARGUMENT",
                        "Launch URL must use HTTP or HTTPS",
                    ));
                }
                Some(value.into())
            }
        }
    };
    Ok(LaunchConfig {
        id,
        name,
        command,
        url,
    })
}

fn update_process(state: &Mutex<LaunchState>, payload: &Value) -> Option<(String, Option<String>)> {
    let id = payload
        .get("id")
        .or_else(|| {
            payload
                .get("terminal")
                .and_then(|terminal| terminal.get("id"))
        })?
        .as_str()?;
    let mut state = state.lock().unwrap();
    let key = state
        .processes
        .iter()
        .find(|(_, process)| process.terminal_id == id)
        .map(|(key, _)| key.clone())?;
    let process = state.processes.get_mut(&key)?;
    let root = process.root.clone();
    match payload["action"].as_str()? {
        "data" if process.url.is_none() => {
            process
                .buffer
                .push_str(&strip_ansi(payload["data"].as_str().unwrap_or("")));
            if process.buffer.len() > 8000 {
                let mut start = process.buffer.len() - 8000;
                while !process.buffer.is_char_boundary(start) {
                    start += 1;
                }
                process.buffer.drain(..start);
            }
            if let Some(url) = local_url(&process.buffer) {
                process.url = Some(url.clone());
                return Some((root, Some(url)));
            }
        }
        "exit" => {
            process.status = "exited".into();
            process.exit_code = payload["exitCode"].as_u64();
            return Some((root, None));
        }
        "state" if payload["terminal"]["status"] == "closed" => {
            state.processes.remove(&key);
            return Some((root, None));
        }
        _ => {}
    }
    None
}

fn strip_ansi(text: &str) -> String {
    static ANSI: OnceLock<regex::Regex> = OnceLock::new();
    ANSI.get_or_init(|| {
        regex::Regex::new(r"\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*(?:\x07|\x1b\\)").unwrap()
    })
    .replace_all(text, "")
    .into_owned()
}

fn local_url(text: &str) -> Option<String> {
    static URL: OnceLock<regex::Regex> = OnceLock::new();
    let found = URL.get_or_init(|| regex::Regex::new(r#"(?i)https?://(?:localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1?\]):\d{2,5}(?:/[^\s'"<>)\]]*)?"#).unwrap()).find(text)?;
    let value = found
        .as_str()
        .trim_end_matches(['.', ',', ';', ':'])
        .replace("0.0.0.0", "localhost");
    url::Url::parse(&value).ok()?;
    Some(value)
}

fn powershell_command(command: &str) -> String {
    static START: OnceLock<regex::Regex> = OnceLock::new();
    let pattern = START.get_or_init(|| {
        regex::Regex::new(
            r#"(?i)^start(?:\.exe)?\s+(?:""\s+)?(?:/(?:b|min|max|wait)\s+)*("[^"]+"|\S+)(.*)$"#,
        )
        .unwrap()
    });
    let Some(captures) = pattern.captures(command.trim()) else {
        return command.into();
    };
    let file = &captures[1];
    let target = if file.starts_with('"') {
        file.into()
    } else {
        format!("\"{file}\"")
    };
    let args = captures[2].trim();
    if args.is_empty() {
        format!("Start-Process {target}")
    } else {
        format!(
            "Start-Process {target} -ArgumentList '{}'",
            args.replace('\'', "''")
        )
    }
}

#[cfg(test)]
#[path = "launch_tests.rs"]
mod tests;
