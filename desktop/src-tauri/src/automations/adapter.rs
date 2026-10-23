use super::{monitor::FileMonitor, types::*};
use crate::error::{HostError, HostResult};
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    path::PathBuf,
    sync::{Arc, Mutex},
    time::{Duration, Instant},
};
use tauri::Manager;

pub struct Prepared {
    pub workspace_id: String,
    pub session_id: String,
}
pub struct RunStatus {
    pub status: String,
    pub error: Option<String>,
}

pub trait AgentAdapter {
    fn prepare(&self, rule: &AutomationRule, run: &AutomationRun) -> HostResult<Prepared>;
    fn dispatch(&self, rule: &AutomationRule, run: &AutomationRun) -> HostResult<()>;
    fn status(&self, run: &AutomationRun) -> HostResult<RunStatus>;
    fn cancel(&self, run: &AutomationRun) -> HostResult<()>;
    fn fingerprint(&self, root: &str) -> HostResult<String>;
}

pub type Monitors = Arc<Mutex<HashMap<String, FileMonitor>>>;

pub struct NativeAdapter {
    app: tauri::AppHandle,
    monitors: Monitors,
}

impl NativeAdapter {
    pub fn new(app: tauri::AppHandle, monitors: Monitors) -> Self {
        Self { app, monitors }
    }

    pub(crate) fn project(&self, root: &str) -> HostResult<Value> {
        if root.starts_with("prime-ssh://") {
            return Err(HostError::unsupported("remote automation"));
        }
        let state = self.app.state::<crate::state::AppState>();
        let existing = state
            .projects
            .lock()
            .unwrap()
            .values()
            .find(|project| {
                project["path"]
                    .as_str()
                    .is_some_and(|path| super::identity(path) == super::identity(root))
            })
            .cloned();
        if let Some(project) = existing {
            return Ok(project);
        }
        let project = crate::projects::open(&self.app, &state, PathBuf::from(root), "project")?;
        crate::events::emit(&self.app, "", "projectsChanged", json!({}));
        Ok(project)
    }

    pub(crate) fn request(
        &self,
        root: &str,
        method: &str,
        suffix: &str,
        body: Option<Value>,
    ) -> HostResult<Value> {
        let project = self.project(root)?;
        let id = project["id"]
            .as_str()
            .ok_or_else(|| HostError::new("ENGINE_RESPONSE", "Workspace response has no id"))?;
        self.app
            .state::<crate::state::AppState>()
            .require_engine()?
            .request(method, &format!("/v1/workspaces/{id}/{suffix}"), body)
    }
}

impl AgentAdapter for NativeAdapter {
    fn prepare(&self, rule: &AutomationRule, _run: &AutomationRun) -> HostResult<Prepared> {
        let project = self.project(&rule.root)?;
        self.request(
            &rule.root,
            "POST",
            "agent/init",
            Some(json!({"interactive":true})),
        )?;
        let session = self.request(
            &rule.root,
            "POST",
            "sessions",
            Some(json!({"title":rule.title})),
        )?;
        Ok(Prepared {
            workspace_id: project["id"]
                .as_str()
                .ok_or_else(|| HostError::new("ENGINE_RESPONSE", "Workspace response has no id"))?
                .into(),
            session_id: session["id"]
                .as_str()
                .ok_or_else(|| HostError::new("ENGINE_RESPONSE", "Session response has no id"))?
                .into(),
        })
    }

    fn dispatch(&self, rule: &AutomationRule, run: &AutomationRun) -> HostResult<()> {
        self.request(&rule.root, "POST", "agent", Some(json!({"session_id":run.session_id,"run_id":run.id,"prompt":rule.prompt,"attachments":[]})))?;
        Ok(())
    }

    fn status(&self, run: &AutomationRun) -> HostResult<RunStatus> {
        let record = match self.request(&run.root, "GET", &format!("runs/{}", run.id), None) {
            Ok(record) => record,
            Err(error)
                if error.message.starts_with("Engine status 404:")
                    || (error.code == "IO_ERROR" && !std::path::Path::new(&run.root).exists()) =>
            {
                return Ok(RunStatus {
                    status: "interrupted".into(),
                    error: Some("Запуск не найден после восстановления".into()),
                })
            }
            Err(error) => return Err(error),
        };
        let status = match record["status"].as_str().unwrap_or("") {
            "accepted" | "queued" | "running" => "running",
            "completed" => "completed",
            "failed" => "failed",
            "cancelled" => "cancelled",
            "interrupted" => "interrupted",
            _ => {
                return Err(HostError::new(
                    "ENGINE_RESPONSE",
                    "Unknown automation run status",
                ))
            }
        };
        Ok(RunStatus {
            status: status.into(),
            error: record["error"]
                .as_str()
                .filter(|error| !error.is_empty())
                .map(str::to_owned),
        })
    }

    fn cancel(&self, run: &AutomationRun) -> HostResult<()> {
        let session = run
            .session_id
            .as_ref()
            .ok_or_else(|| HostError::new("AUTOMATION_STATE", "Automation run has no session"))?;
        self.request(
            &run.root,
            "POST",
            &format!("agent/sessions/{session}/cancel"),
            None,
        )?;
        let deadline = Instant::now() + Duration::from_secs(15);
        loop {
            if !matches!(self.status(run)?.status.as_str(), "claimed" | "running") {
                return Ok(());
            }
            if Instant::now() >= deadline {
                return Err(HostError::new(
                    "AUTOMATION_TIMEOUT",
                    "Prime is still stopping the automation run",
                ));
            }
            std::thread::sleep(Duration::from_millis(100));
        }
    }

    fn fingerprint(&self, root: &str) -> HostResult<String> {
        if root.starts_with("prime-ssh://") {
            return Err(HostError::unsupported("remote automation file monitor"));
        }
        let mut monitors = self.monitors.lock().unwrap();
        if !monitors.contains_key(root) {
            monitors.insert(root.into(), FileMonitor::new(std::path::Path::new(root))?);
        }
        monitors.get(root).unwrap().fingerprint()
    }
}
