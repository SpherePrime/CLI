use super::*;
use adapter::{Prepared, RunStatus};
use chrono::{TimeZone, Timelike};
use std::{
    process::{Child, Command, Stdio},
    sync::atomic::AtomicI64,
    time::Instant,
};

struct TaskFixture {
    tasks: Mutex<HashMap<String, Child>>,
    output: PathBuf,
    slow: bool,
    state_path: PathBuf,
}

impl TaskFixture {
    fn new(directory: &Path, slow: bool) -> Self {
        Self {
            tasks: Mutex::new(HashMap::new()),
            output: directory.join("result.txt"),
            slow,
            state_path: directory.join("automations.json"),
        }
    }
}

impl AgentAdapter for TaskFixture {
    fn prepare(&self, _rule: &AutomationRule, run: &AutomationRun) -> HostResult<Prepared> {
        let persisted: SavedState = serde_json::from_slice(&std::fs::read(&self.state_path)?)?;
        assert_eq!(
            persisted
                .runs
                .iter()
                .find(|candidate| candidate.id == run.id)
                .unwrap()
                .status,
            "claimed"
        );
        Ok(Prepared {
            workspace_id: "fixture-workspace".into(),
            session_id: uuid::Uuid::new_v4().to_string(),
        })
    }
    fn dispatch(&self, _rule: &AutomationRule, run: &AutomationRun) -> HostResult<()> {
        let mut command = if cfg!(windows) {
            let mut command = Command::new("powershell.exe");
            command.args(["-NoLogo", "-NoProfile", "-NonInteractive", "-Command"]);
            command.arg(format!("Set-Content -LiteralPath '{}' -Value 'automation-task-fixture'; Start-Sleep -Milliseconds {}", self.output.display().to_string().replace('\'', "''"), if self.slow { 60000 } else { 10 }));
            command
        } else {
            let mut command = Command::new("/bin/sh");
            command
                .args([
                    "-c",
                    "printf 'automation-task-fixture' > \"$1\"; sleep \"$2\"",
                    "fixture",
                ])
                .arg(&self.output)
                .arg(if self.slow { "60" } else { "0.01" });
            command
        };
        #[cfg(windows)]
        {
            use std::os::windows::process::CommandExt;
            command.creation_flags(0x08000000);
        }
        let child = command
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()?;
        self.tasks.lock().unwrap().insert(run.id.clone(), child);
        Ok(())
    }
    fn status(&self, run: &AutomationRun) -> HostResult<RunStatus> {
        let mut tasks = self.tasks.lock().unwrap();
        match tasks.get_mut(&run.id) {
            Some(child) => match child.try_wait()? {
                None => Ok(RunStatus {
                    status: "running".into(),
                    error: None,
                }),
                Some(status) => Ok(RunStatus {
                    status: if status.success() {
                        "completed"
                    } else {
                        "failed"
                    }
                    .into(),
                    error: None,
                }),
            },
            None => Ok(RunStatus {
                status: "interrupted".into(),
                error: Some("fixture restart".into()),
            }),
        }
    }
    fn cancel(&self, run: &AutomationRun) -> HostResult<()> {
        if let Some(child) = self.tasks.lock().unwrap().get_mut(&run.id) {
            child.kill()?;
            child.wait()?;
        }
        Ok(())
    }
    fn fingerprint(&self, _root: &str) -> HostResult<String> {
        Ok("fixture-fingerprint".into())
    }
}

impl Drop for TaskFixture {
    fn drop(&mut self) {
        for child in self.tasks.get_mut().unwrap().values_mut() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }
}

fn clocked(directory: &Path) -> (Arc<AutomationManager>, Arc<AtomicI64>) {
    let clock = Arc::new(AtomicI64::new(1790000000000));
    let now = clock.clone();
    (
        AutomationManager::load_with_clock(directory, Arc::new(move || now.load(Ordering::SeqCst)))
            .unwrap(),
        clock,
    )
}

fn emitter() -> Emitter {
    Arc::new(|_| {})
}

fn save(
    manager: &AutomationManager,
    directory: &Path,
    adapter: &dyn AgentAdapter,
    trigger: Value,
) -> Value {
    manager.handle_with(&json!({"action":"save","root":directory,"input":{"title":"Task fixture","prompt":"Do fixture task","enabled":true,"trigger":trigger}}), adapter, &emitter()).unwrap()
}

#[test]
fn local_schedule_handles_dst_gap_fold_and_weekdays() {
    use chrono_tz::America::New_York;
    let spring = New_York
        .with_ymd_and_hms(2026, 3, 8, 0, 0, 0)
        .unwrap()
        .timestamp_millis();
    let next = schedule::next_in_zone(
        &Trigger::Daily {
            time: "02:30".into(),
        },
        spring,
        &New_York,
    )
    .unwrap()
    .unwrap();
    let next = New_York.timestamp_millis_opt(next).unwrap();
    assert_eq!((next.hour(), next.minute()), (3, 30));
    let fall = New_York
        .with_ymd_and_hms(2026, 11, 1, 0, 0, 0)
        .unwrap()
        .timestamp_millis();
    let first = schedule::next_in_zone(
        &Trigger::Daily {
            time: "01:30".into(),
        },
        fall,
        &New_York,
    )
    .unwrap()
    .unwrap();
    assert_eq!(chrono::Utc.timestamp_millis_opt(first).unwrap().hour(), 5);
    let between = first + 30 * 60000;
    let tomorrow = schedule::next_in_zone(
        &Trigger::Daily {
            time: "01:30".into(),
        },
        between,
        &New_York,
    )
    .unwrap()
    .unwrap();
    assert_eq!(tomorrow - first, 25 * 60 * 60000);
    let monday = schedule::next_in_zone(
        &Trigger::Weekly {
            time: "09:00".into(),
            days: vec![1],
        },
        spring,
        &New_York,
    )
    .unwrap()
    .unwrap();
    assert_eq!(
        New_York
            .timestamp_millis_opt(monday)
            .unwrap()
            .format("%a %H:%M")
            .to_string(),
        "Mon 09:00"
    );
    assert_eq!(
        schedule::next_in_zone(&Trigger::Interval { minutes: 1.5 }, spring, &New_York).unwrap(),
        Some(spring + 90000)
    );
}

#[test]
fn actual_task_run_persists_claim_session_history_and_completion() {
    let directory = tempfile::tempdir().unwrap();
    let (manager, _) = clocked(directory.path());
    let adapter = TaskFixture::new(directory.path(), false);
    let saved = save(
        &manager,
        directory.path(),
        &adapter,
        json!({"type":"interval","minutes":60}),
    );
    let id = saved["rules"][0]["id"].clone();
    let response = manager
        .handle_with(
            &json!({"action":"run","root":directory.path(),"id":id}),
            &adapter,
            &emitter(),
        )
        .unwrap();
    assert_eq!(response["runs"][0]["status"], "running");
    assert!(response["runs"][0]["sessionId"].is_string());
    let deadline = Instant::now() + Duration::from_secs(10);
    loop {
        manager.tick_with(&adapter, &emitter()).unwrap();
        if manager.counts()["activeCount"] == 0 {
            break;
        }
        assert!(Instant::now() < deadline, "Automation task did not finish");
        std::thread::sleep(Duration::from_millis(20));
    }
    assert!(std::fs::read_to_string(&adapter.output)
        .unwrap()
        .contains("automation-task-fixture"));
    assert_eq!(manager.activity()[0]["status"], "completed");
    let reopened = AutomationManager::load(directory.path()).unwrap();
    assert_eq!(reopened.activity()[0]["status"], "completed");
    assert_eq!(reopened.saved().rules[0].id, id.as_str().unwrap());
}

#[test]
fn cancellation_and_concurrency_are_scoped_and_recoverable() {
    let directory = tempfile::tempdir().unwrap();
    let (manager, _) = clocked(directory.path());
    let adapter = TaskFixture::new(directory.path(), true);
    for _ in 0..MAX_CONCURRENT {
        let saved = save(
            &manager,
            directory.path(),
            &adapter,
            json!({"type":"interval","minutes":60}),
        );
        let id = saved["rules"].as_array().unwrap().last().unwrap()["id"].clone();
        manager
            .handle_with(
                &json!({"action":"run","root":directory.path(),"id":id}),
                &adapter,
                &emitter(),
            )
            .unwrap();
    }
    let fifth = save(
        &manager,
        directory.path(),
        &adapter,
        json!({"type":"interval","minutes":60}),
    );
    let id = fifth["rules"].as_array().unwrap().last().unwrap()["id"].clone();
    assert!(manager
        .handle_with(
            &json!({"action":"run","root":directory.path(),"id":id}),
            &adapter,
            &emitter()
        )
        .is_err());
    assert_eq!(manager.counts()["activeCount"], MAX_CONCURRENT);
    let run_id = fifth["runs"][0]["id"].clone();
    let other = tempfile::tempdir().unwrap();
    assert!(manager
        .handle_with(
            &json!({"action":"cancel","root":other.path(),"runId":run_id}),
            &adapter,
            &emitter()
        )
        .is_err());
    let cancelled = manager
        .handle_with(
            &json!({"action":"cancel","root":directory.path(),"runId":run_id}),
            &adapter,
            &emitter(),
        )
        .unwrap();
    assert_eq!(cancelled["runs"][0]["status"], "cancelled");
    let reopened = AutomationManager::load(directory.path()).unwrap();
    let empty_adapter = TaskFixture::new(directory.path(), false);
    reopened.tick_with(&empty_adapter, &emitter()).unwrap();
    assert_eq!(reopened.counts()["activeCount"], 0);
    assert!(reopened
        .saved()
        .runs
        .iter()
        .any(|run| run.status == "interrupted"));
}

#[test]
fn cooldown_dedup_and_owned_run_events_prevent_loops() {
    let directory = tempfile::tempdir().unwrap();
    let (manager, clock) = clocked(directory.path());
    let adapter = TaskFixture::new(directory.path(), true);
    save(
        &manager,
        directory.path(),
        &adapter,
        json!({"type":"runFailed","cooldownMinutes":1}),
    );
    let root = manager
        .canonical(directory.path().to_str().unwrap())
        .unwrap();
    let event = AutomationEvent {
        root,
        kind: "runFailed".into(),
        source_run_id: Some("external-run".into()),
        fingerprint: None,
    };
    manager
        .event_with(event.clone(), &adapter, &emitter())
        .unwrap();
    assert_eq!(manager.counts()["activeCount"], 0);
    clock.fetch_add(60001, Ordering::SeqCst);
    manager
        .event_with(event.clone(), &adapter, &emitter())
        .unwrap();
    assert_eq!(manager.counts()["activeCount"], 1);
    let own = manager.saved().runs[0].id.clone();
    let mut own_event = event.clone();
    own_event.source_run_id = Some(own);
    clock.fetch_add(60001, Ordering::SeqCst);
    manager.event_with(own_event, &adapter, &emitter()).unwrap();
    manager.event_with(event, &adapter, &emitter()).unwrap();
    assert_eq!(manager.saved().runs.len(), 1);
}

#[test]
fn corrupt_storage_is_visible_and_not_overwritten() {
    let directory = tempfile::tempdir().unwrap();
    let path = directory.path().join("automations.json");
    std::fs::write(&path, "{broken}").unwrap();
    let (manager, _) = clocked(directory.path());
    let adapter = TaskFixture::new(directory.path(), false);
    let response = manager
        .handle_with(
            &json!({"action":"list","root":directory.path()}),
            &adapter,
            &emitter(),
        )
        .unwrap();
    assert_eq!(response["error"], "automation.corrupt");
    assert!(manager
        .handle_with(
            &json!({"action":"save","root":directory.path(),"input":{}}),
            &adapter,
            &emitter()
        )
        .is_err());
    assert_eq!(std::fs::read_to_string(path).unwrap(), "{broken}");
}

#[test]
fn interval_tick_claims_once_and_shutdown_blocks_dispatch() {
    let directory = tempfile::tempdir().unwrap();
    let (manager, clock) = clocked(directory.path());
    let adapter = TaskFixture::new(directory.path(), true);
    save(
        &manager,
        directory.path(),
        &adapter,
        json!({"type":"interval","minutes":1}),
    );
    manager.tick_with(&adapter, &emitter()).unwrap();
    assert!(manager.saved().runs.is_empty());
    clock.fetch_add(60001, Ordering::SeqCst);
    manager.tick_with(&adapter, &emitter()).unwrap();
    assert_eq!(manager.saved().runs.len(), 1);
    assert_eq!(manager.saved().runs[0].trigger, "interval");
    assert!(manager.saved().rules[0].next_at.unwrap() > clock.load(Ordering::SeqCst));
    clock.fetch_add(60001, Ordering::SeqCst);
    manager.tick_with(&adapter, &emitter()).unwrap();
    assert_eq!(manager.saved().runs.len(), 1);
    manager.shutdown();
    assert!(manager.tick_with(&adapter, &emitter()).is_err());
}

#[test]
fn unsent_claim_is_interrupted_during_startup_recovery() {
    let directory = tempfile::tempdir().unwrap();
    let (manager, clock) = clocked(directory.path());
    let adapter = TaskFixture::new(directory.path(), false);
    save(
        &manager,
        directory.path(),
        &adapter,
        json!({"type":"daily","time":"09:30"}),
    );
    let mut state = manager.saved();
    let rule = &state.rules[0];
    state.runs.push(AutomationRun {
        id: "unsent-run".into(),
        rule_id: rule.id.clone(),
        root: rule.root.clone(),
        title: rule.title.clone(),
        trigger: "manual".into(),
        status: "claimed".into(),
        created_at: clock.load(Ordering::SeqCst),
        finished_at: None,
        workspace_id: None,
        session_id: None,
        error: None,
    });
    storage::persist(&manager.path, &mut state).unwrap();
    let recovered = AutomationManager::load(directory.path()).unwrap();
    assert_eq!(recovered.saved().runs[0].status, "interrupted");
    assert_eq!(
        recovered.saved().runs[0].error.as_deref(),
        Some("automation.interrupted")
    );
    assert!(recovered.saved().runs[0].finished_at.is_some());
}

#[test]
fn native_monitor_detects_changes_and_ignores_generated_directories() {
    let directory = tempfile::tempdir().unwrap();
    std::fs::write(directory.path().join(".gitignore"), "ignored.txt\n").unwrap();
    let monitor = monitor::FileMonitor::new(directory.path()).unwrap();
    let initial = monitor.fingerprint().unwrap();
    std::fs::write(directory.path().join("source.txt"), "change").unwrap();
    let deadline = Instant::now() + Duration::from_secs(5);
    while monitor.fingerprint().unwrap() == initial {
        assert!(
            Instant::now() < deadline,
            "Native filesystem event was not received"
        );
        std::thread::sleep(Duration::from_millis(20));
    }
    std::thread::sleep(Duration::from_millis(100));
    let baseline = monitor.fingerprint().unwrap();
    std::fs::write(directory.path().join("ignored.txt"), "ignored").unwrap();
    std::fs::create_dir(directory.path().join("node_modules")).unwrap();
    std::fs::write(
        directory.path().join("node_modules/dependency.txt"),
        "ignored",
    )
    .unwrap();
    std::thread::sleep(Duration::from_millis(150));
    assert_eq!(monitor.fingerprint().unwrap(), baseline);
}
