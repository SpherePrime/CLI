pub mod launch;
mod output;
mod process;
mod session;
mod types;

use crate::error::{HostError, HostResult};
use portable_pty::{native_pty_system, CommandBuilder, PtySize};
use serde_json::{json, Value};
use session::{pty_error, Emitter, Session};
use std::{
    collections::{HashMap, HashSet},
    path::Path,
    sync::{Arc, Mutex},
    time::{SystemTime, UNIX_EPOCH},
};
use types::{TerminalOptions, TerminalSnapshot};

const MAX_SESSIONS: usize = 12;

#[derive(Default)]
struct ManagerState {
    sessions: HashMap<String, Arc<Session>>,
    closed_workspaces: HashSet<String>,
    disposed: bool,
}

#[derive(Default)]
pub struct TerminalManager {
    state: Mutex<ManagerState>,
}

impl TerminalManager {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn count(&self) -> usize {
        self.state.lock().unwrap().sessions.len()
    }

    pub fn call(
        &self,
        app: tauri::AppHandle,
        workspace: &str,
        cwd: &Path,
        request: &Value,
    ) -> HostResult<Value> {
        let emit: Emitter = Arc::new(move |workspace, payload| {
            crate::events::emit(&app, workspace, "terminal", payload)
        });
        self.dispatch(workspace, cwd, request, "user", emit)
    }

    pub fn agent_call(
        &self,
        app: tauri::AppHandle,
        bound_workspace: &str,
        workspace: &str,
        cwd: &Path,
        request: &Value,
    ) -> HostResult<Value> {
        if bound_workspace != workspace {
            return Err(HostError::new(
                "ACCESS_DENIED",
                "Terminal workspace access denied",
            ));
        }
        let emit: Emitter = Arc::new(move |workspace, payload| {
            crate::events::emit(&app, workspace, "terminal", payload)
        });
        self.dispatch_agent(workspace, cwd, request, emit)
    }

    fn dispatch_agent(&self, workspace: &str, cwd: &Path, request: &Value, emit: Emitter) -> HostResult<Value> {
        if !matches!(
            request["action"].as_str(),
            Some("list" | "read" | "write" | "resize" | "create" | "close" | "release")
        ) {
            return Err(HostError::new(
                "ACCESS_DENIED",
                "Terminal ownership can only be transferred by the user",
            ));
        }
        if matches!(request["action"].as_str(), Some("close" | "release")) {
            let session=self.find(workspace,string(request,"id")?)?;
            if session.snapshot().owner != "agent" {
                return Err(HostError::new("TERMINAL_OWNER","Only agent-owned terminals can be closed or released by the agent"));
            }
            if request["action"] == "release" {return Ok(serde_json::to_value(session.owner("user")?)?);}
        }
        self.dispatch(workspace, cwd, request, "agent", emit)
    }

    fn dispatch(
        &self,
        workspace: &str,
        cwd: &Path,
        request: &Value,
        source: &str,
        emit: Emitter,
    ) -> HostResult<Value> {
        if workspace.trim().is_empty() {
            return Err(HostError::new(
                "INVALID_ARGUMENT",
                "Terminal workspace is required",
            ));
        }
        match string(request, "action")? {
            "list" => {
                let sessions = self
                    .state
                    .lock()
                    .unwrap()
                    .sessions
                    .values()
                    .filter_map(|session| {
                        let snapshot = session.snapshot();
                        (snapshot.workspace_id == workspace).then_some(snapshot)
                    })
                    .collect::<Vec<_>>();
                Ok(serde_json::to_value(sessions)?)
            }
            "create" => {
                let options = serde_json::from_value(
                    request
                        .get("options")
                        .filter(|value| !value.is_null())
                        .cloned()
                        .unwrap_or(json!({})),
                )?;
                Ok(serde_json::to_value(
                    self.create_owned(workspace, cwd, options, emit, source)?,
                )?)
            }
            "read" => Ok(self.find(workspace, string(request, "id")?)?.read()),
            "write" => Ok(serde_json::to_value(
                self.find(workspace, string(request, "id")?)?
                    .write(string(request, "data")?, source)?,
            )?),
            "report" => {
                self.find(workspace, string(request, "id")?)?
                    .report(string(request, "data")?)?;
                Ok(Value::Null)
            }
            "resize" => {
                let cols = dimension(request.get("cols").and_then(Value::as_f64), 80, 20, 500)?;
                let rows = dimension(request.get("rows").and_then(Value::as_f64), 24, 5, 300)?;
                if request.get("cols").and_then(Value::as_f64).is_none()
                    || request.get("rows").and_then(Value::as_f64).is_none()
                {
                    return Err(HostError::new(
                        "INVALID_ARGUMENT",
                        "Invalid terminal dimensions",
                    ));
                }
                Ok(serde_json::to_value(
                    self.find(workspace, string(request, "id")?)?
                        .resize(cols, rows)?,
                )?)
            }
            "owner" => {
                let owner = string(request, "owner")?;
                if !matches!(owner, "user" | "agent") {
                    return Err(HostError::new("INVALID_ARGUMENT", "Invalid terminal owner"));
                }
                Ok(serde_json::to_value(
                    self.find(workspace, string(request, "id")?)?.owner(owner)?,
                )?)
            }
            "close" => {
                self.close_one(workspace, string(request, "id")?)?;
                Ok(Value::Null)
            }
            _ => Err(HostError::new(
                "INVALID_ARGUMENT",
                "Unknown terminal action",
            )),
        }
    }

    fn create(
        &self,
        workspace: &str,
        cwd: &Path,
        options: TerminalOptions,
        emit: Emitter,
    ) -> HostResult<TerminalSnapshot> {
        self.create_owned(workspace,cwd,options,emit,"user")
    }

    fn create_owned(&self,workspace:&str,cwd:&Path,options:TerminalOptions,emit:Emitter,owner:&str)->HostResult<TerminalSnapshot> {
        let mut manager = self.state.lock().unwrap();
        if manager.disposed || manager.closed_workspaces.contains(workspace) {
            return Err(HostError::new(
                "TERMINAL_CLOSED",
                "Terminal workspace is closed",
            ));
        }
        if manager.sessions.len() >= MAX_SESSIONS {
            return Err(HostError::new(
                "TERMINAL_LIMIT",
                "Terminal session limit reached (12)",
            ));
        }
        let cwd = dunce::canonicalize(cwd)?;
        if !cwd.is_dir() {
            return Err(HostError::new(
                "INVALID_ARGUMENT",
                "Terminal working directory must be a folder",
            ));
        }
        let cols = dimension(options.cols, 80, 20, 500)?;
        let rows = dimension(options.rows, 24, 5, 300)?;
        let shell = options.shell.unwrap_or_else(|| {
            if cfg!(windows) {
                "powershell.exe".into()
            } else {
                std::env::var("SHELL").unwrap_or_else(|_| "/bin/sh".into())
            }
        });
        let args = options.args.unwrap_or_else(|| {
            if cfg!(windows) {
                vec!["-NoLogo".into(), "-NoProfile".into()]
            } else {
                vec![]
            }
        });
        if shell.is_empty()
            || shell.contains('\0')
            || args.len() > 128
            || args.iter().any(|argument| argument.contains('\0'))
        {
            return Err(HostError::new(
                "INVALID_ARGUMENT",
                "Invalid terminal shell or arguments",
            ));
        }
        let pair = native_pty_system()
            .openpty(PtySize {
                rows,
                cols,
                pixel_width: 0,
                pixel_height: 0,
            })
            .map_err(pty_error)?;
        let mut command = CommandBuilder::new(&shell);
        command.args(&args);
        command.cwd(&cwd);
        command.env("TERM", "xterm-256color");
        command.env("COLORTERM", "truecolor");
        let mut child = pair.slave.spawn_command(command).map_err(pty_error)?;
        drop(pair.slave);
        let pid = match child.process_id() {
            Some(pid) => pid,
            None => {
                let _ = child.kill();
                let _ = child.wait();
                return Err(HostError::new(
                    "TERMINAL_PTY",
                    "Terminal process has no process id",
                ));
            }
        };
        let title = options
            .title
            .filter(|title| !title.trim().is_empty())
            .map(|title| title.trim().chars().take(120).collect())
            .unwrap_or_else(|| {
                Path::new(&shell)
                    .file_name()
                    .unwrap_or_default()
                    .to_string_lossy()
                    .into_owned()
            });
        let snapshot = TerminalSnapshot {
            id: uuid::Uuid::new_v4().to_string(),
            workspace_id: workspace.into(),
            title,
            shell,
            cwd: cwd.to_string_lossy().into_owned(),
            pid,
            cols,
            rows,
            owner: owner.into(),
            status: "running".into(),
            created_at: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap_or_default()
                .as_millis() as u64,
            sequence: 0,
            exit_code: None,
        };
        let session = Session::start(snapshot, pair.master, child, emit)?;
        let snapshot = session.snapshot();
        manager.sessions.insert(snapshot.id.clone(), session);
        Ok(snapshot)
    }

    fn find(&self, workspace: &str, id: &str) -> HostResult<Arc<Session>> {
        let session = self
            .state
            .lock()
            .unwrap()
            .sessions
            .get(id)
            .cloned()
            .ok_or_else(|| HostError::new("TERMINAL_NOT_FOUND", "Terminal not found"))?;
        if session.snapshot().workspace_id != workspace {
            return Err(HostError::new(
                "ACCESS_DENIED",
                "Terminal belongs to another workspace",
            ));
        }
        Ok(session)
    }

    pub(crate) fn close_one(&self, workspace: &str, id: &str) -> HostResult<()> {
        let session = self.find(workspace, id)?;
        session.close()?;
        self.state.lock().unwrap().sessions.remove(id);
        Ok(())
    }

    pub fn snapshots(&self) -> Vec<TerminalSnapshot> {
        self.state
            .lock()
            .unwrap()
            .sessions
            .values()
            .map(|session| session.snapshot())
            .collect()
    }

    pub fn close_workspace(&self, workspace: &str) -> HostResult<()> {
        let sessions = {
            let mut manager = self.state.lock().unwrap();
            manager.closed_workspaces.insert(workspace.into());
            manager
                .sessions
                .values()
                .filter(|session| session.snapshot().workspace_id == workspace)
                .cloned()
                .collect::<Vec<_>>()
        };
        self.close_sessions(sessions)
    }

    pub fn shutdown(&self) -> HostResult<()> {
        let sessions = {
            let mut manager = self.state.lock().unwrap();
            manager.disposed = true;
            manager.sessions.values().cloned().collect::<Vec<_>>()
        };
        self.close_sessions(sessions)
    }

    fn close_sessions(&self, sessions: Vec<Arc<Session>>) -> HostResult<()> {
        let mut first_error = None;
        for session in sessions {
            match session.close() {
                Ok(()) => {
                    self.state
                        .lock()
                        .unwrap()
                        .sessions
                        .remove(&session.snapshot().id);
                }
                Err(error) => {
                    if first_error.is_none() {
                        first_error = Some(error);
                    }
                }
            }
        }
        match first_error {
            Some(error) => Err(error),
            None => Ok(()),
        }
    }
}

impl Drop for TerminalManager {
    fn drop(&mut self) {
        let _ = self.shutdown();
    }
}

fn string<'a>(request: &'a Value, key: &str) -> HostResult<&'a str> {
    request.get(key).and_then(Value::as_str).ok_or_else(|| {
        HostError::new(
            "INVALID_ARGUMENT",
            format!("Terminal {key} must be a string"),
        )
    })
}

fn dimension(value: Option<f64>, default: u16, minimum: u16, maximum: u16) -> HostResult<u16> {
    let value = value.unwrap_or(default as f64);
    if !value.is_finite() {
        return Err(HostError::new(
            "INVALID_ARGUMENT",
            "Invalid terminal dimensions",
        ));
    }
    Ok(value.floor().clamp(minimum as f64, maximum as f64) as u16)
}

#[cfg(test)]
mod tests;
