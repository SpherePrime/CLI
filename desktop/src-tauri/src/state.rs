use crate::{engine::Engine, error::HostResult, settings::Settings};
use serde_json::Value;
use std::{
    collections::HashMap,
    path::PathBuf,
    sync::{atomic::AtomicBool, Arc, Mutex},
};

pub struct AppState {
    pub directory: PathBuf,
    pub isolated: bool,
    pub settings: Mutex<Settings>,
    pub projects: Mutex<HashMap<String, Value>>,
    pub engine: Mutex<EngineStatus>,
    pub editor_dirty: AtomicBool,
    pub closing: AtomicBool,
    pub close_ready: AtomicBool,
    pub file_writes: Mutex<()>,
    pub attachments: Mutex<crate::attachments::Attachments>,
    pub browser: crate::browser::BrowserState,
    pub terminals: Arc<crate::terminal::TerminalManager>,
    pub remotes: Arc<crate::remote::RemoteManager>,
    pub launches: crate::terminal::launch::LaunchManager,
    pub automations: Arc<crate::automations::AutomationManager>,
    pub teams: Arc<crate::teams::TeamManager>,
    pub jobs: std::sync::Mutex<crate::jobs::JobCenter>,
    pub windows: Arc<crate::computer::WindowManager>,
    pub speech: Arc<crate::speech::SpeechService>,
    pub project_open: Mutex<()>,
}

pub struct EngineStatus {
    pub instance: Option<Arc<Engine>>,
    pub error: Option<String>,
    pub starting: bool,
}

impl AppState {
    pub fn new(directory: PathBuf) -> HostResult<Self> {
        let settings = Settings::load(&directory)?;
        let remotes = Arc::new(crate::remote::RemoteManager::load(&directory)?);
        let launches = crate::terminal::launch::LaunchManager::load(&directory)?;
        let automations = crate::automations::AutomationManager::load(&directory)?;
        let teams = crate::teams::TeamManager::load(&directory)?;
        let attachments = crate::attachments::Attachments::new(directory.join("attachments"))?;
        Ok(Self {
            isolated: isolated_profile(),
            directory,
            settings: Mutex::new(settings),
            projects: Mutex::new(HashMap::new()),
            engine: Mutex::new(EngineStatus {
                instance: None,
                error: None,
                starting: true,
            }),
            editor_dirty: AtomicBool::new(false),
            closing: AtomicBool::new(false),
            close_ready: AtomicBool::new(false),
            file_writes: Mutex::new(()),
            attachments: Mutex::new(attachments),
            browser: crate::browser::BrowserState::new(),
            terminals: Arc::new(crate::terminal::TerminalManager::new()),
            remotes,
            launches,
            automations,
            teams,
            jobs: std::sync::Mutex::new(crate::jobs::JobCenter::default()),
            windows: Arc::new(crate::computer::WindowManager::new()),
            speech: crate::speech::SpeechService::new(),
            project_open: Mutex::new(()),
        })
    }

    pub fn require_engine(&self) -> HostResult<Arc<Engine>> {
        if self.closing.load(std::sync::atomic::Ordering::Acquire) {
            return Err(crate::error::HostError::new(
                "APP_CLOSING",
                "Application is shutting down",
            ));
        }
        let status = self.engine.lock().unwrap();
        status.instance.clone().ok_or_else(|| {
            crate::error::HostError::new(
                if status.starting {
                    "ENGINE_STARTING"
                } else {
                    "ENGINE_UNAVAILABLE"
                },
                status.error.clone().unwrap_or_else(|| {
                    if status.starting {
                        "Native engine is starting".into()
                    } else {
                        "Native engine unavailable".into()
                    }
                }),
            )
        })
    }

    pub fn root(&self, id: &str) -> HostResult<PathBuf> {
        self.projects
            .lock()
            .unwrap()
            .get(id)
            .and_then(|project| {
                project
                    .get("localPath")
                    .unwrap_or(&project["path"])
                    .as_str()
            })
            .map(PathBuf::from)
            .ok_or_else(|| crate::error::HostError::new("PROJECT_NOT_FOUND", "Project is not open"))
    }
}

pub fn isolated_profile() -> bool {
    cfg!(test) || (cfg!(debug_assertions) && std::env::var_os("PRIME_NATIVE_TEST_DATA").is_some())
}
