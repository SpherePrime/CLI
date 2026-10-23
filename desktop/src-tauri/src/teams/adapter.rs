use crate::error::{HostError, HostResult};
use serde_json::{json, Value};

pub struct TeamPrepared {
    pub workspace_id: String,
    pub session_id: String,
}

pub struct TeamRunStatus {
    pub status: String,
    pub error: Option<String>,
}

pub struct TeamRun {
    pub id: String,
    pub root: String,
    pub title: String,
    pub workspace_id: Option<String>,
    pub session_id: Option<String>,
    pub created_at: i64,
}

pub trait TeamAdapter: Send + Sync {
    fn prepare(&self, root: &str, title: &str) -> HostResult<TeamPrepared>;
    fn dispatch(&self, root: &str, run_id: &str, session_id: &str, prompt: &str) -> HostResult<()>;
    fn status(&self, run: &TeamRun) -> HostResult<TeamRunStatus>;
    fn cancel(&self, run: &TeamRun) -> HostResult<()>;
}

pub struct NativeTeamAdapter {
    app: tauri::AppHandle,
}

impl NativeTeamAdapter {
    pub fn new(app: tauri::AppHandle) -> Self {
        Self { app }
    }

    fn request(
        &self,
        root: &str,
        method: &str,
        suffix: &str,
        body: Option<Value>,
    ) -> HostResult<Value> {
        let adapter = crate::automations::adapter::NativeAdapter::new(
            self.app.clone(),
            std::sync::Arc::new(std::sync::Mutex::new(std::collections::HashMap::new())),
        );
        adapter.request(root, method, suffix, body)
    }
}

impl TeamAdapter for NativeTeamAdapter {
    fn prepare(&self, root: &str, title: &str) -> HostResult<TeamPrepared> {
        let project = {
            let adapter = crate::automations::adapter::NativeAdapter::new(
                self.app.clone(),
                std::sync::Arc::new(std::sync::Mutex::new(std::collections::HashMap::new())),
            );
            adapter.project(root)?
        };
        self.request(root, "POST", "agent/init", Some(json!({"interactive":true})))?;
        let session = self.request(root, "POST", "sessions", Some(json!({"title":title})))?;
        Ok(TeamPrepared {
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

    fn dispatch(&self, root: &str, run_id: &str, session_id: &str, prompt: &str) -> HostResult<()> {
        self.request(
            root,
            "POST",
            "agent",
            Some(json!({"session_id":session_id,"run_id":run_id,"prompt":prompt,"attachments":[]})),
        )?;
        Ok(())
    }

    fn status(&self, run: &TeamRun) -> HostResult<TeamRunStatus> {
        let record = match self.request(&run.root, "GET", &format!("runs/{}", run.id), None) {
            Ok(record) => record,
            Err(error)
                if error.message.starts_with("Engine status 404:")
                    || (error.code == "IO_ERROR" && !std::path::Path::new(&run.root).exists()) =>
            {
                return Ok(TeamRunStatus {
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
                    "Unknown team run status",
                ))
            }
        };
        Ok(TeamRunStatus {
            status: status.into(),
            error: record["error"]
                .as_str()
                .filter(|error| !error.is_empty())
                .map(str::to_owned),
        })
    }

    fn cancel(&self, run: &TeamRun) -> HostResult<()> {
        let session = run
            .session_id
            .as_ref()
            .ok_or_else(|| HostError::new("TEAM_STATE", "Team task has no session"))?;
        self.request(
            &run.root,
            "POST",
            &format!("agent/sessions/{session}/cancel"),
            None,
        )?;
        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(15);
        loop {
            if !matches!(self.status(run)?.status.as_str(), "running") {
                return Ok(());
            }
            if std::time::Instant::now() >= deadline {
                return Err(HostError::new(
                    "TEAM_TIMEOUT",
                    "Prime is still stopping the team task",
                ));
            }
            std::thread::sleep(std::time::Duration::from_millis(100));
        }
    }
}
