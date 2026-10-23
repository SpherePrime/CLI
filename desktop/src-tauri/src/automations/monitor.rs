use crate::error::{HostError, HostResult};
use ignore::gitignore::{Gitignore, GitignoreBuilder};
use notify::{RecommendedWatcher, RecursiveMode, Watcher};
use std::{
    path::{Path, PathBuf},
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc, Mutex,
    },
};

pub struct FileMonitor {
    _watcher: RecommendedWatcher,
    revision: Arc<AtomicU64>,
    error: Arc<Mutex<Option<String>>>,
    identity: String,
}

impl FileMonitor {
    pub fn new(root: &Path) -> HostResult<Self> {
        let root = root.to_path_buf();
        let revision = Arc::new(AtomicU64::new(0));
        let error = Arc::new(Mutex::new(None));
        let next_revision = revision.clone();
        let next_error = error.clone();
        let rules = Arc::new(Mutex::new(load_ignores(&root)?));
        let watched = root.clone();
        let mut watcher =
            notify::recommended_watcher(move |event: notify::Result<notify::Event>| match event {
                Ok(event) => {
                    if event.kind.is_access() {
                        return;
                    }
                    let reload = event.paths.iter().any(|path| {
                        matches!(
                            path.file_name().and_then(|name| name.to_str()),
                            Some(".gitignore" | ".primeignore")
                        )
                    });
                    if reload {
                        match load_ignores(&watched) {
                            Ok(next) => {
                                *rules.lock().unwrap() = next;
                                *next_error.lock().unwrap() = None;
                            }
                            Err(error) => {
                                *next_error.lock().unwrap() = Some(error.message);
                            }
                        }
                        next_revision.fetch_add(1, Ordering::Relaxed);
                    } else if event.paths.is_empty()
                        || event
                            .paths
                            .iter()
                            .any(|path| !ignored(&watched, path, &rules.lock().unwrap()))
                    {
                        next_revision.fetch_add(1, Ordering::Relaxed);
                    }
                }
                Err(error) => {
                    *next_error.lock().unwrap() = Some(error.to_string());
                }
            })
            .map_err(monitor_error)?;
        watcher
            .watch(&root, RecursiveMode::Recursive)
            .map_err(monitor_error)?;
        Ok(Self {
            _watcher: watcher,
            revision,
            error,
            identity: uuid::Uuid::new_v4().to_string(),
        })
    }

    pub fn fingerprint(&self) -> HostResult<String> {
        if let Some(error) = self.error.lock().unwrap().clone() {
            return Err(HostError::new("AUTOMATION_MONITOR", error));
        }
        Ok(format!(
            "{}:{}",
            self.identity,
            self.revision.load(Ordering::Relaxed)
        ))
    }
}

fn ignored(root: &Path, path: &Path, ignores: &Gitignore) -> bool {
    let relative = path.strip_prefix(root).unwrap_or(path);
    if relative.components().any(|component| {
        matches!(
            component.as_os_str().to_str(),
            Some(".git" | ".prime" | "node_modules" | ".superpowers")
        )
    }) {
        return true;
    }
    ignores
        .matched_path_or_any_parents(path, path.is_dir())
        .is_ignore()
}

fn load_ignores(root: &Path) -> HostResult<Gitignore> {
    let mut builder = GitignoreBuilder::new(root);
    for filename in [".gitignore", ".primeignore"] {
        let path: PathBuf = root.join(filename);
        if path.exists() {
            if let Some(error) = builder.add(path) {
                return Err(monitor_error(error));
            }
        }
    }
    builder.build().map_err(monitor_error)
}

fn monitor_error(error: impl std::fmt::Display) -> HostError {
    HostError::new("AUTOMATION_MONITOR", error.to_string())
}
