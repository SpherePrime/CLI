use super::{
    adapter::{TeamPrepared, TeamRun, TeamRunStatus, TeamAdapter},
    manager::TeamManager,
    storage,
    types::*,
};
use crate::error::HostResult;
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    path::Path,
    sync::{Arc, Mutex},
};

struct MockAdapter {
    fail_next: Mutex<bool>,
    dispatch_fail: Mutex<HashMap<String, String>>,
    cancel_fail: Mutex<HashMap<String, String>>,
    statuses: Mutex<HashMap<String, String>>,
    prepared: Mutex<Vec<String>>,
    dispatched: Mutex<Vec<String>>,
}

impl MockAdapter {
    fn new() -> Arc<Self> {
        Arc::new(Self {
            fail_next: Mutex::new(false),
            dispatch_fail: Mutex::new(HashMap::new()),
            cancel_fail: Mutex::new(HashMap::new()),
            statuses: Mutex::new(HashMap::new()),
            prepared: Mutex::new(Vec::new()),
            dispatched: Mutex::new(Vec::new()),
        })
    }

    fn fail_dispatch_next(&self) {
        *self.fail_next.lock().unwrap() = true;
    }

    fn fail_cancel(&self, run_id: &str, message: &str) {
        self.cancel_fail
            .lock()
            .unwrap()
            .insert(run_id.to_owned(), message.to_owned());
    }

    fn complete(&self, run_id: &str) {
        self.statuses
            .lock()
            .unwrap()
            .insert(run_id.to_owned(), "completed".to_owned());
    }

    fn dispatched(&self) -> Vec<String> {
        self.dispatched.lock().unwrap().clone()
    }
}

impl TeamAdapter for MockAdapter {
    fn prepare(&self, root: &str, title: &str) -> HostResult<TeamPrepared> {
        self.prepared
            .lock()
            .unwrap()
            .push(format!("{root}:{title}"));
        Ok(TeamPrepared {
            workspace_id: "workspace".into(),
            session_id: format!("session-{}", self.prepared.lock().unwrap().len()),
        })
    }

    fn dispatch(&self, _root: &str, run_id: &str, _session_id: &str, _prompt: &str) -> HostResult<()> {
        let mut fail_next = self.fail_next.lock().unwrap();
        let failing = *fail_next;
        *fail_next = false;
        drop(fail_next);
        if failing {
            return Err(crate::error::HostError::new("MOCK", "dispatch broke"));
        }
        self.dispatched.lock().unwrap().push(run_id.to_owned());
        if let Some(message) = self.dispatch_fail.lock().unwrap().get(run_id).cloned() {
            return Err(crate::error::HostError::new("MOCK", message));
        }
        Ok(())
    }

    fn status(&self, run: &TeamRun) -> HostResult<TeamRunStatus> {
        let status = self
            .statuses
            .lock()
            .unwrap()
            .get(&run.id)
            .cloned()
            .unwrap_or_else(|| "running".into());
        Ok(TeamRunStatus {
            status,
            error: None,
        })
    }

    fn cancel(&self, run: &TeamRun) -> HostResult<()> {
        if let Some(message) = self.cancel_fail.lock().unwrap().get(&run.id).cloned() {
            return Err(crate::error::HostError::new("MOCK", message));
        }
        self.statuses
            .lock()
            .unwrap()
            .insert(run.id.clone(), "cancelled".to_owned());
        Ok(())
    }
}

fn manager(path: &Path, adapter: Arc<MockAdapter>) -> Arc<TeamManager> {
    TeamManager::load_with_clock(
        path,
        Some(adapter),
        Arc::new(|_root, _payload| {}),
        Arc::new(|| 1_000_000),
    )
    .unwrap()
}

fn input(tasks: usize, concurrency: u32) -> Value {
    json!({
        "title":"Team",
        "goal":"Goal",
        "concurrency":concurrency,
        "tasks":(0..tasks).map(|index| json!({"title":format!("Task {index}"),"instruction":"Do the work"})).collect::<Vec<_>>()
    })
}

#[test]
fn create_respects_concurrency_and_persists_claim_before_dispatch() {
    let directory = tempfile::tempdir().unwrap();
    let adapter = MockAdapter::new();
    let teams = manager(directory.path(), adapter.clone());
    let response = teams
        .handle("/root", &json!({"action":"create","input":input(2,1)}))
        .unwrap();
    assert_eq!(response["teams"].as_array().unwrap().len(), 1);
    let tasks = response["teams"][0]["tasks"].as_array().unwrap();
    assert_eq!(tasks.len(), 2);
    for task in tasks {
        assert_eq!(task["status"], "queued");
    }
    teams.tick().unwrap();
    assert_eq!(adapter.dispatched().len(), 1);
    let response = teams.handle("/root", &json!({"action":"list"})).unwrap();
    let tasks = response["teams"][0]["tasks"].as_array().unwrap();
    assert_eq!(tasks[0]["status"], "running");
    assert_eq!(tasks[1]["status"], "queued");
    let saved: SavedTeams =
        serde_json::from_slice(&std::fs::read(directory.path().join("teams.json")).unwrap())
            .unwrap();
    assert!(saved.teams[0].tasks[0].run_id.is_some());
    assert!(saved.teams[0].tasks[0].session_id.is_some());
}

#[test]
fn status_polling_completes_tasks_and_starts_next() {
    let directory = tempfile::tempdir().unwrap();
    let adapter = MockAdapter::new();
    let teams = manager(directory.path(), adapter.clone());
    teams
        .handle("/root", &json!({"action":"create","input":input(2,1)}))
        .unwrap();
    let first_run = adapter.dispatched()[0].clone();
    adapter.complete(&first_run);
    teams.tick().unwrap();
    let response = teams.handle("/root", &json!({"action":"list"})).unwrap();
    let tasks = response["teams"][0]["tasks"].as_array().unwrap();
    assert_eq!(tasks[0]["status"], "completed");
    assert_eq!(tasks[1]["status"], "running");
    assert_eq!(adapter.dispatched().len(), 2);
}

#[test]
fn dispatch_failure_marks_task_failed_and_retry_queues_again() {
    let directory = tempfile::tempdir().unwrap();
    let adapter = MockAdapter::new();
    let teams = manager(directory.path(), adapter.clone());
    adapter.fail_dispatch_next();
    teams
        .handle("/root", &json!({"action":"create","input":input(1,1)}))
        .unwrap();
    let response = teams.handle("/root", &json!({"action":"list"})).unwrap();
    let task = &response["teams"][0]["tasks"][0];
    assert_eq!(task["status"], "failed");
    assert_eq!(task["error"], "dispatch broke");
    let team_id = response["teams"][0]["id"].as_str().unwrap();
    let task_id = task["id"].as_str().unwrap();
    let response = teams
        .handle(
            "/root",
            &json!({"action":"retryTask","teamId":team_id,"taskId":task_id}),
        )
        .unwrap();
    assert_eq!(response["teams"][0]["tasks"][0]["status"], "queued");
}

#[test]
fn stop_task_cancels_run_and_cancel_failure_stays_active() {
    let directory = tempfile::tempdir().unwrap();
    let adapter = MockAdapter::new();
    let teams = manager(directory.path(), adapter.clone());
    let response = teams
        .handle("/root", &json!({"action":"create","input":input(2,2)}))
        .unwrap();
    let team_id = response["teams"][0]["id"].as_str().unwrap().to_owned();
    let task_id = response["teams"][0]["tasks"][0]["id"].as_str().unwrap().to_owned();
    let response = teams
        .handle(
            "/root",
            &json!({"action":"stopTask","teamId":team_id,"taskId":task_id}),
        )
        .unwrap();
    assert_eq!(response["teams"][0]["tasks"][0]["status"], "cancelled");
    assert_eq!(teams.active_count(), 1);

    let adapter2 = MockAdapter::new();
    let teams2 = manager(&directory.path().join("second"), adapter2.clone());
    let response = teams2
        .handle("/root", &json!({"action":"create","input":input(1,1)}))
        .unwrap();
    let run_id = adapter2.dispatched()[0].clone();
    adapter2.fail_cancel(&run_id, "cancel refused");
    let team_id = response["teams"][0]["id"].as_str().unwrap().to_owned();
    let task_id = response["teams"][0]["tasks"][0]["id"].as_str().unwrap().to_owned();
    let response = teams2
        .handle(
            "/root",
            &json!({"action":"stopTask","teamId":team_id,"taskId":task_id}),
        )
        .unwrap();
    let task = &response["teams"][0]["tasks"][0];
    assert_eq!(task["status"], "running");
    assert_eq!(task["error"], "cancel refused");
}

#[test]
fn remove_rejects_active_team_and_limits_are_enforced() {
    let directory = tempfile::tempdir().unwrap();
    let adapter = MockAdapter::new();
    let teams = manager(directory.path(), adapter);
    let response = teams
        .handle("/root", &json!({"action":"create","input":input(1,1)}))
        .unwrap();
    let team_id = response["teams"][0]["id"].as_str().unwrap().to_owned();
    let error = teams
        .handle("/root", &json!({"action":"remove","teamId":team_id}))
        .unwrap_err();
    assert_eq!(error.code, "TEAM_ACTIVE");
    assert!(super::types::validate(&serde_json::from_value(json!({
        "title":"","goal":"","concurrency":0,
        "tasks":[{"title":"t","instruction":"i"}]
    })).unwrap()).is_err());
    assert!(super::types::validate(&serde_json::from_value(json!({
        "title":"ok","goal":"","concurrency":1,
        "tasks":(0..21).map(|index| json!({"title":format!("t{index}"),"instruction":"i"})).collect::<Vec<_>>()
    })).unwrap()).is_err());
}

#[test]
fn storage_round_trip_keeps_original_schema() {
    let directory = tempfile::tempdir().unwrap();
    let path = directory.path().join("teams.json");
    let saved = SavedTeams {
        version: 1,
        teams: vec![Team {
            id: "team".into(),
            root: "root".into(),
            title: "Team".into(),
            goal: "Goal".into(),
            concurrency: 3,
            created_at: 42,
            tasks: vec![TeamTask {
                id: "task".into(),
                title: "Task".into(),
                instruction: "Work".into(),
                status: TeamTaskStatus::Running,
                run_id: Some("run".into()),
                workspace_id: Some("ws".into()),
                session_id: Some("session".into()),
                error: Some("boom".into()),
                started_at: Some(1),
                finished_at: None,
            }],
        }],
    };
    storage::persist(&path, &saved).unwrap();
    let loaded = storage::load(&path).unwrap();
    assert_eq!(loaded.teams.len(), 1);
    assert_eq!(loaded.teams[0].tasks[0].run_id.as_deref(), Some("run"));
    std::fs::write(&path, b"{\"version\":2}").unwrap();
    assert!(storage::load(&path).is_err());
}
