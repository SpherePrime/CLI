mod adapter;
mod monitor;
mod operations;
mod runs;
mod schedule;
mod storage;
mod tick;
mod types;

use crate::error::{HostError, HostResult};
use adapter::{AgentAdapter, Monitors, NativeAdapter};
use serde_json::{json, Value};
use std::{
    collections::{HashMap, HashSet},
    path::{Path, PathBuf},
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Condvar, Mutex,
    },
    thread::JoinHandle,
    time::Duration,
};
use types::*;

pub const MAX_CONCURRENT: usize = 4;
type Emitter = Arc<dyn Fn(Value) + Send + Sync>;

struct Runtime {
    saved: SavedState,
    error: Option<String>,
    fingerprints: HashMap<String, String>,
    rebaseline: HashSet<String>,
    monitor_errors: HashMap<String, String>,
}

pub struct AutomationManager {
    path: PathBuf,
    runtime: Mutex<Runtime>,
    operations: Mutex<()>,
    monitors: Monitors,
    now: Arc<dyn Fn() -> i64 + Send + Sync>,
    stop: Arc<(Mutex<bool>, Condvar)>,
    stopped: AtomicBool,
    worker: Mutex<Option<JoinHandle<()>>>,
}

impl AutomationManager {
    pub fn load(directory: &Path) -> HostResult<Arc<Self>> {
        Self::load_with_clock(
            directory,
            Arc::new(|| chrono::Utc::now().timestamp_millis()),
        )
    }

    fn load_with_clock(
        directory: &Path,
        now: Arc<dyn Fn() -> i64 + Send + Sync>,
    ) -> HostResult<Arc<Self>> {
        std::fs::create_dir_all(directory)?;
        let path = directory.join("automations.json");
        let (mut saved, error) = match storage::load(&path, now()) {
            Ok(saved) => (saved, None),
            Err(error) => (SavedState::default(), Some(error.message)),
        };
        let rebaseline = saved
            .runs
            .iter()
            .filter(|run| run.active())
            .map(|run| run.root.clone())
            .collect();
        if error.is_none() && path.exists() {
            storage::persist(&path, &mut saved)?;
        }
        Ok(Arc::new(Self {
            path,
            runtime: Mutex::new(Runtime {
                saved,
                error,
                fingerprints: HashMap::new(),
                rebaseline,
                monitor_errors: HashMap::new(),
            }),
            operations: Mutex::new(()),
            monitors: Arc::new(Mutex::new(HashMap::new())),
            now,
            stop: Arc::new((Mutex::new(false), Condvar::new())),
            stopped: AtomicBool::new(false),
            worker: Mutex::new(None),
        }))
    }

    pub fn start(self: &Arc<Self>, app: tauri::AppHandle) {
        let mut worker = self.worker.lock().unwrap();
        if worker.is_some() || self.stopped.load(Ordering::Acquire) {
            return;
        }
        let manager = Arc::downgrade(self);
        let stop = self.stop.clone();
        *worker = Some(std::thread::spawn(move || loop {
            if *stop.0.lock().unwrap() {
                break;
            }
            let Some(manager) = manager.upgrade() else {
                break;
            };
            let adapter = NativeAdapter::new(app.clone(), manager.monitors.clone());
            let emit = native_emitter(app.clone());
            if let Err(error) = manager.tick_with(&adapter, &emit) {
                emit(
                    json!({"error":error.message,"activeCount":manager.counts()["activeCount"],"enabledCount":manager.counts()["enabledCount"]}),
                );
            }
            drop(manager);
            let stopped = stop.0.lock().unwrap();
            if *stopped {
                break;
            }
            let _ = stop
                .1
                .wait_timeout(stopped, Duration::from_secs(10))
                .unwrap();
        }));
    }

    pub fn handle(&self, app: tauri::AppHandle, request: &Value) -> HostResult<Value> {
        let adapter = NativeAdapter::new(app.clone(), self.monitors.clone());
        self.handle_with(request, &adapter, &native_emitter(app))
    }

    pub fn event(&self, app: tauri::AppHandle, event: &Value) -> HostResult<()> {
        let adapter = NativeAdapter::new(app.clone(), self.monitors.clone());
        self.event_with(
            serde_json::from_value(event.clone())?,
            &adapter,
            &native_emitter(app),
        )
    }

    pub fn counts(&self) -> Value {
        let runtime = self.runtime.lock().unwrap();
        counts(&runtime.saved)
    }

    pub fn activity(&self) -> Value {
        let runtime = self.runtime.lock().unwrap();
        json!(runtime
            .saved
            .runs
            .iter()
            .skip(runtime.saved.runs.len().saturating_sub(100))
            .collect::<Vec<_>>())
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
        let _operation = self.operations.lock().unwrap();
        self.monitors.lock().unwrap().clear();
    }

    fn writable(&self) -> HostResult<()> {
        if self.stopped.load(Ordering::Acquire) {
            return Err(HostError::new(
                "AUTOMATION_STOPPED",
                "Automation scheduler is stopped",
            ));
        }
        if let Some(error) = &self.runtime.lock().unwrap().error {
            return Err(HostError::new("AUTOMATION_STORAGE", error));
        }
        Ok(())
    }

    fn canonical(&self, root: &str) -> HostResult<String> {
        if root.starts_with("prime-ssh://") {
            return Ok(root.into());
        }
        match dunce::canonicalize(root) {
            Ok(path) => Ok(identity(&path.to_string_lossy())),
            Err(error) => {
                let root = identity(root);
                let runtime = self.runtime.lock().unwrap();
                if matches!(
                    error.kind(),
                    std::io::ErrorKind::NotFound | std::io::ErrorKind::NotADirectory
                ) && (runtime.saved.rules.iter().any(|rule| rule.root == root)
                    || runtime.saved.runs.iter().any(|run| run.root == root))
                {
                    Ok(root)
                } else {
                    Err(error.into())
                }
            }
        }
    }

    fn saved(&self) -> SavedState {
        self.runtime.lock().unwrap().saved.clone()
    }

    fn persist(&self, mut next: SavedState, emit: &Emitter) -> HostResult<()> {
        storage::persist(&self.path, &mut next)?;
        let roots: HashSet<_> = next
            .rules
            .iter()
            .map(|rule| rule.root.clone())
            .chain(next.runs.iter().map(|run| run.root.clone()))
            .collect();
        let count = counts(&next);
        {
            let mut runtime = self.runtime.lock().unwrap();
            runtime.monitor_errors.retain(|root, _| {
                next.rules.iter().any(|rule| {
                    &rule.root == root
                        && rule.enabled
                        && matches!(rule.trigger, Trigger::FilesChanged { .. })
                })
            });
            runtime.saved = next;
        }
        for root in roots {
            emit(
                json!({"root":root,"activeCount":count["activeCount"],"enabledCount":count["enabledCount"]}),
            );
        }
        Ok(())
    }

    fn snapshot(&self, root: &str) -> Value {
        let runtime = self.runtime.lock().unwrap();
        let mut snapshot = json!({"root":root,"rules":runtime.saved.rules.iter().filter(|rule| rule.root == root).collect::<Vec<_>>(),"runs":runtime.saved.runs.iter().rev().filter(|run| run.root == root).collect::<Vec<_>>()});
        if root.starts_with("prime-ssh://") {
            snapshot["error"] = json!("Native remote automation is not implemented yet");
        } else if let Some(error) = runtime
            .error
            .as_ref()
            .or_else(|| runtime.monitor_errors.get(root))
        {
            snapshot["error"] = json!(error);
        }
        snapshot
    }
}

impl Drop for AutomationManager {
    fn drop(&mut self) {
        self.shutdown();
    }
}

pub fn identity(root: &str) -> String {
    if cfg!(windows) && !root.starts_with("prime-ssh://") {
        root.to_lowercase()
    } else {
        root.into()
    }
}
fn counts(state: &SavedState) -> Value {
    json!({"activeCount":state.runs.iter().filter(|run| run.active()).count(),"enabledCount":state.rules.iter().filter(|rule| rule.enabled).count()})
}
fn native_emitter(app: tauri::AppHandle) -> Emitter {
    Arc::new(move |payload| crate::events::emit(&app, "", "automation", payload))
}
fn string<'a>(value: &'a Value, key: &str) -> HostResult<&'a str> {
    value
        .get(key)
        .and_then(Value::as_str)
        .ok_or_else(|| schedule::invalid())
}

#[cfg(test)]
mod tests;
