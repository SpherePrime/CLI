use super::{
    adapter::{TeamAdapter, TeamRun},
    storage,
    types::*,
};
use crate::{
    error::{HostError, HostResult},
    events,
};
use serde_json::{json, Value};
use std::{
    path::PathBuf,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Condvar, Mutex,
    },
    thread::JoinHandle,
    time::Duration,
};

type Emitter = Arc<dyn Fn(&str, Value) + Send + Sync>;

struct Runtime {
    saved: SavedTeams,
    error: Option<String>,
}

pub struct TeamManager {
    path: PathBuf,
    adapter: Mutex<Option<Arc<dyn TeamAdapter>>>,
    emit: Mutex<Emitter>,
    runtime: Mutex<Runtime>,
    operations: Mutex<()>,
    now: Arc<dyn Fn() -> i64 + Send + Sync>,
    stop: Arc<(Mutex<bool>, Condvar)>,
    stopped: AtomicBool,
    worker: Mutex<Option<JoinHandle<()>>>,
}

impl TeamManager {
    pub fn load(directory: &std::path::Path) -> HostResult<Arc<Self>> {
        Self::load_with_clock(
            directory,
            None,
            Arc::new(|_root, _payload| {}),
            Arc::new(|| chrono::Utc::now().timestamp_millis()),
        )
    }

    pub fn load_with_clock(
        directory: &std::path::Path,
        adapter: Option<Arc<dyn TeamAdapter>>,
        emit: Emitter,
        now: Arc<dyn Fn() -> i64 + Send + Sync>,
    ) -> HostResult<Arc<Self>> {
        std::fs::create_dir_all(directory)?;
        let path = directory.join("teams.json");
        let (mut saved, error) = match storage::load(&path) {
            Ok(saved) => (saved, None),
            Err(load_error) => (SavedTeams::default(), Some(load_error.message)),
        };
        if error.is_none() {
            storage::recover_all(&mut saved);
            let _ = storage::persist(&path, &saved);
        }
        Ok(Arc::new(Self {
            path,
            adapter: Mutex::new(adapter),
            emit: Mutex::new(emit),
            runtime: Mutex::new(Runtime { saved, error }),
            operations: Mutex::new(()),
            now,
            stop: Arc::new((Mutex::new(false), Condvar::new())),
            stopped: AtomicBool::new(false),
            worker: Mutex::new(None),
        }))
    }

    pub fn set_adapter(&self, adapter: Arc<dyn TeamAdapter>) {
        *self.adapter.lock().unwrap() = Some(adapter);
    }

    pub fn require_adapter(&self) -> HostResult<Arc<dyn TeamAdapter>> {
        self.adapter
            .lock()
            .unwrap()
            .clone()
            .ok_or_else(|| HostError::new("TEAM_NOT_READY", "Планировщик ещё загружается"))
    }

    pub fn start(self: &Arc<Self>, app: tauri::AppHandle) {
        {
            let mut slot = self.worker.lock().unwrap();
            if slot.is_some() || self.stopped.load(Ordering::Acquire) {
                return;
            }
            *self.emit.lock().unwrap() = Arc::new(move |root, payload| {
                let mut payload = payload;
                payload["root"] = json!(root);
                events::emit(&app, "", "team", payload);
            });
            let manager = Arc::downgrade(self);
            let stop = self.stop.clone();
            *slot = Some(std::thread::spawn(move || loop {
                if *stop.0.lock().unwrap() {
                    break;
                }
                let Some(manager) = manager.upgrade() else {
                    break;
                };
                if let Err(error) = manager.tick() {
                    eprintln!("Team scheduler: {}", error.message);
                }
                drop(manager);
                let stopped = stop.0.lock().unwrap();
                if *stopped {
                    break;
                }
                let _ = stop.1.wait_timeout(stopped, Duration::from_secs(2));
            }));
        }
    }

    pub fn all(&self) -> Vec<Team> {
        self.runtime.lock().unwrap().saved.teams.clone()
    }

    pub fn active_count(&self) -> usize {
        Self::active_count_of(&self.runtime.lock().unwrap().saved.teams)
    }

    fn active_count_of(teams: &[Team]) -> usize {
        teams
            .iter()
            .map(|team| {
                team.tasks
                    .iter()
                    .filter(|task| {
                        !task.status.finished() && task.status != TeamTaskStatus::Queued
                    })
                    .count()
            })
            .sum()
    }

    pub fn owned_run_ids(&self) -> Vec<String> {
        self.runtime
            .lock()
            .unwrap()
            .saved
            .teams
            .iter()
            .flat_map(|team| team.tasks.iter().filter_map(|task| task.run_id.clone()))
            .collect()
    }

    pub fn handle(&self, root: &str, request: &Value) -> HostResult<Value> {
        if request["action"].as_str() == Some("list") {
            return Ok(self.snapshot(root));
        }
        let _operation = self.operations.lock().unwrap();
        {
            let runtime = self.runtime.lock().unwrap();
            if let Some(error) = &runtime.error {
                return Err(HostError::new("TEAM_STORAGE", error.clone()));
            }
        }
        let root = self.canonical(root);
        self.writable()?;
        let action = request["action"]
            .as_str()
            .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Invalid team request"))?
            .to_owned();
        match action.as_str() {
            "create" => {
                let input: TeamInput = serde_json::from_value(
                    request
                        .get("input")
                        .cloned()
                        .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing team input"))?,
                )
                .map_err(|_| HostError::new("INVALID_ARGUMENT", "team.invalid"))?;
                let input = validate(&input)
                    .map_err(|message| HostError::new("INVALID_ARGUMENT", message))?;
                let mut runtime = self.runtime.lock().unwrap();
                if runtime.saved.teams.len() >= LIMITS.teams {
                    let removable = runtime
                        .saved
                        .teams
                        .iter()
                        .filter(|team| team.tasks.iter().all(|task| task.status.finished()))
                        .min_by_key(|team| team.created_at)
                        .map(|team| team.id.clone());
                    let Some(removable) = removable else {
                        return Err(HostError::new("TEAM_LIMIT", "team.limit"));
                    };
                    runtime.saved.teams.retain(|team| team.id != removable);
                }
                runtime
                    .saved
                    .teams
                    .push(Self::new_team(&root, &input, (self.now)()));
            }
            "remove" => {
                let team_id = string_arg(request, "teamId")?;
                let mut runtime = self.runtime.lock().unwrap();
                if Self::team(&runtime.saved, &root, team_id)?
                    .tasks
                    .iter()
                    .any(|task| {
                        matches!(
                            task.status,
                            TeamTaskStatus::Starting | TeamTaskStatus::Running
                        )
                    })
                {
                    return Err(HostError::new("TEAM_ACTIVE", "team.active"));
                }
                runtime.saved.teams.retain(|team| team.id != team_id);
            }
            "stopTeam" => {
                let team_id = string_arg(request, "teamId")?;
                let task_ids = {
                    let runtime = self.runtime.lock().unwrap();
                    Self::team(&runtime.saved, &root, team_id)?
                        .tasks
                        .iter()
                        .map(|task| task.id.clone())
                        .collect::<Vec<_>>()
                };
                for task_id in task_ids {
                    self.stop_task(&root, team_id, &task_id)?;
                }
            }
            "stopTask" => {
                self.stop_task(
                    &root,
                    string_arg(request, "teamId")?,
                    string_arg(request, "taskId")?,
                )?;
            }
            "retryTask" => {
                let team_id = string_arg(request, "teamId")?;
                let task_id = string_arg(request, "taskId")?;
                let mut runtime = self.runtime.lock().unwrap();
                let task = Self::task(&mut runtime.saved, &root, team_id, task_id)?;
                if !task.status.finished() {
                    return Err(HostError::new("TEAM_MISSING", "team.missing"));
                }
                task.status = TeamTaskStatus::Queued;
                task.error = None;
                task.run_id = None;
                task.finished_at = None;
            }
            _ => {
                return Err(HostError::new(
                    "INVALID_ARGUMENT",
                    "Unknown team action",
                ))
            }
        }
        self.save(&root)?;
        let snapshot = self.snapshot(&root);
        drop(_operation);
        let _ = self.tick();
        Ok(snapshot)
    }

    fn new_team(root: &str, input: &TeamInput, now: i64) -> Team {
        Team {
            id: uuid::Uuid::new_v4().to_string(),
            root: root.to_owned(),
            title: input.title.clone(),
            goal: input.goal.clone(),
            concurrency: input.concurrency,
            created_at: now,
            tasks: input
                .tasks
                .iter()
                .map(|task| TeamTask {
                    id: uuid::Uuid::new_v4().to_string(),
                    title: task.title.clone(),
                    instruction: task.instruction.clone(),
                    status: TeamTaskStatus::Queued,
                    run_id: None,
                    workspace_id: None,
                    session_id: None,
                    error: None,
                    started_at: None,
                    finished_at: None,
                })
                .collect(),
        }
    }

    fn stop_task(&self, root: &str, team_id: &str, task_id: &str) -> HostResult<()> {
        let target: StopTarget = {
            let runtime = self.runtime.lock().unwrap();
            let task = Self::task_read(&runtime.saved, root, team_id, task_id)?;
            match task.status {
                TeamTaskStatus::Queued => StopTarget::Queued,
                TeamTaskStatus::Starting | TeamTaskStatus::Running => match &task.run_id {
                    Some(run_id) => StopTarget::Run(TeamRun {
                        id: run_id.clone(),
                        root: root.to_owned(),
                        title: task.title.clone(),
                        workspace_id: task.workspace_id.clone(),
                        session_id: task.session_id.clone(),
                        created_at: task.started_at.unwrap_or(0),
                    }),
                    None => StopTarget::Idle,
                },
                _ => StopTarget::Idle,
            }
        };
        match target {
            StopTarget::Queued => self.mark(root, team_id, task_id, |task| {
                task.status = TeamTaskStatus::Cancelled;
                task.finished_at = Some((self.now)());
            }),
            StopTarget::Run(run) => match self.require_adapter()?.cancel(&run) {
                Ok(()) => self.mark(root, team_id, task_id, |task| {
                    task.status = TeamTaskStatus::Cancelled;
                    task.finished_at = Some((self.now)());
                }),
                Err(error) => self.mark(root, team_id, task_id, |task| {
                    task.error = Some(error.message);
                }),
            },
            StopTarget::Idle => Ok(()),
        }
    }

    fn mark(
        &self,
        root: &str,
        team_id: &str,
        task_id: &str,
        change: impl FnOnce(&mut TeamTask),
    ) -> HostResult<()> {
        {
            let mut runtime = self.runtime.lock().unwrap();
            let task = Self::task(&mut runtime.saved, root, team_id, task_id)?;
            change(task);
        }
        self.save(root)
    }

    pub(crate) fn tick(&self) -> HostResult<()> {
        let _operation = self.operations.lock().unwrap();
        if self.writable().is_err() {
            return Ok(());
        }
        {
            let runtime = self.runtime.lock().unwrap();
            if runtime.error.is_some() {
                return Ok(());
            }
        }
        let mut changed: Vec<String> = Vec::new();
        {
            let mut runtime = self.runtime.lock().unwrap();
            for team_index in 0..runtime.saved.teams.len() {
                let root = runtime.saved.teams[team_index].root.clone();
                for task in runtime.saved.teams[team_index]
                    .tasks
                    .iter()
                    .filter(|task| task.status == TeamTaskStatus::Running)
                    .cloned()
                    .collect::<Vec<_>>()
                {
                    let Some(run_id) = task.run_id.clone() else {
                        continue;
                    };
                    let run = TeamRun {
                        id: run_id,
                        root: root.clone(),
                        title: task.title.clone(),
                        workspace_id: task.workspace_id.clone(),
                        session_id: task.session_id.clone(),
                        created_at: task.started_at.unwrap_or(0),
                    };
                    if let Ok(result) = self.require_adapter()?.status(&run) {
                        if result.status == "running" {
                            continue;
                        }
                        let team = &mut runtime.saved.teams[team_index];
                        let task = team
                            .tasks
                            .iter_mut()
                            .find(|task| task.run_id.as_deref() == Some(run.id.as_str()));
                        let Some(task) = task else { continue };
                        task.status = parse_status(&result.status);
                        if let Some(error) = result.error {
                            task.error = Some(error);
                        }
                        task.finished_at = Some((self.now)());
                        changed.push(root.clone());
                    }
                }
                let team = &runtime.saved.teams[team_index];
                let active = team
                    .tasks
                    .iter()
                    .filter(|task| {
                        matches!(task.status, TeamTaskStatus::Starting | TeamTaskStatus::Running)
                    })
                    .count();
                let queued: Vec<String> = team
                    .tasks
                    .iter()
                    .filter(|task| task.status == TeamTaskStatus::Queued)
                    .take((team.concurrency as usize).saturating_sub(active))
                    .map(|task| task.id.clone())
                    .collect();
                for task_id in queued {
                    changed.push(root.clone());
                    let run_id = uuid::Uuid::new_v4().to_string();
                    {
                        let team = &mut runtime.saved.teams[team_index];
                        let task =
                            team.tasks.iter_mut().find(|task| task.id == task_id).unwrap();
                        task.status = TeamTaskStatus::Starting;
                        task.run_id = Some(run_id.clone());
                        task.started_at = Some((self.now)());
                    }
                    storage::persist(&self.path, &runtime.saved)?;
                    let (title, prompt) = {
                        let team = runtime.saved.teams[team_index].clone();
                        let task = team.tasks.iter().find(|task| task.id == task_id).unwrap();
                        (
                            format!("{}: {}", team.title, task.title),
                            team_prompt(&team, task),
                        )
                    };
                    let outcome = (|| -> HostResult<()> {
                        let adapter = self.require_adapter()?;
                        let prepared = adapter.prepare(&root, &title)?;
                        let team = &mut runtime.saved.teams[team_index];
                        let task = team.tasks.iter_mut().find(|task| task.id == task_id).unwrap();
                        task.workspace_id = Some(prepared.workspace_id.clone());
                        task.session_id = Some(prepared.session_id.clone());
                        adapter.dispatch(&root, &run_id, &prepared.session_id, &prompt)
                    })();
                    let team = &mut runtime.saved.teams[team_index];
                    let task = team.tasks.iter_mut().find(|task| task.id == task_id).unwrap();
                    match outcome {
                        Ok(()) => task.status = TeamTaskStatus::Running,
                        Err(error) => {
                            task.status = TeamTaskStatus::Failed;
                            task.error = Some(error.message);
                            task.finished_at = Some((self.now)());
                        }
                    }
                }
            }
        }
        if !changed.is_empty() {
            self.save_roots(&changed)?;
        }
        Ok(())
    }

    fn save(&self, root: &str) -> HostResult<()> {
        self.save_roots(std::slice::from_ref(&root.to_owned()))
    }

    fn save_roots(&self, roots: &[String]) -> HostResult<()> {
        let (saved, active) = {
            let runtime = self.runtime.lock().unwrap();
            (
                runtime.saved.clone(),
                Self::active_count_of(&runtime.saved.teams),
            )
        };
        storage::persist(&self.path, &saved)?;
        let emit = self.emit.lock().unwrap().clone();
        for root in roots {
            emit(root, json!({"root":root,"active":active}));
        }
        Ok(())
    }

    fn snapshot(&self, root: &str) -> Value {
        let runtime = self.runtime.lock().unwrap();
        let mut teams: Vec<&Team> = runtime
            .saved
            .teams
            .iter()
            .filter(|team| team.root == root)
            .collect();
        teams.sort_by(|a, b| b.created_at.cmp(&a.created_at));
        let mut value = json!({"root":root,"teams":teams});
        if let Some(error) = &runtime.error {
            value["error"] = json!(error);
        }
        value
    }

    fn writable(&self) -> HostResult<()> {
        if self.stopped.load(Ordering::Acquire) {
            return Err(HostError::new("TEAM_STOPPED", "Team scheduler is stopped"));
        }
        Ok(())
    }

    fn canonical(&self, root: &str) -> String {
        dunce::canonicalize(root)
            .map(|path| path.to_string_lossy().into_owned())
            .unwrap_or_else(|_| root.to_owned())
    }

    fn team<'a>(saved: &'a SavedTeams, root: &str, id: &str) -> HostResult<&'a Team> {
        saved
            .teams
            .iter()
            .find(|team| team.id == id && team.root == root)
            .ok_or_else(|| HostError::new("TEAM_MISSING", "team.missing"))
    }

    fn task_read<'a>(
        saved: &'a SavedTeams,
        root: &str,
        team_id: &str,
        task_id: &str,
    ) -> HostResult<&'a TeamTask> {
        Self::team(saved, root, team_id)?
            .tasks
            .iter()
            .find(|task| task.id == task_id)
            .ok_or_else(|| HostError::new("TEAM_MISSING", "team.missing"))
    }

    fn task<'a>(
        saved: &'a mut SavedTeams,
        root: &str,
        team_id: &str,
        task_id: &str,
    ) -> HostResult<&'a mut TeamTask> {
        let team = saved
            .teams
            .iter_mut()
            .find(|team| team.id == team_id && team.root == root)
            .ok_or_else(|| HostError::new("TEAM_MISSING", "team.missing"))?;
        team.tasks
            .iter_mut()
            .find(|task| task.id == task_id)
            .ok_or_else(|| HostError::new("TEAM_MISSING", "team.missing"))
    }

    pub fn stop(&self) {
        self.stopped.store(true, Ordering::Release);
        *self.stop.0.lock().unwrap() = true;
        self.stop.1.notify_all();
    }

    pub fn shutdown(&self) {
        self.stop();
        if let Some(worker) = self.worker.lock().unwrap().take() {
            if worker.thread().id() != std::thread::current().id() {
                let _ = worker.join();
            }
        }
    }
}

enum StopTarget {
    Queued,
    Run(TeamRun),
    Idle,
}

fn parse_status(status: &str) -> TeamTaskStatus {
    match status {
        "queued" => TeamTaskStatus::Queued,
        "starting" => TeamTaskStatus::Starting,
        "completed" => TeamTaskStatus::Completed,
        "failed" => TeamTaskStatus::Failed,
        "cancelled" => TeamTaskStatus::Cancelled,
        "interrupted" => TeamTaskStatus::Interrupted,
        _ => TeamTaskStatus::Running,
    }
}

fn string_arg<'a>(request: &'a Value, field: &str) -> HostResult<&'a str> {
    request[field]
        .as_str()
        .filter(|value| !value.is_empty() && value.len() <= 300)
        .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Invalid team identifier"))
}
