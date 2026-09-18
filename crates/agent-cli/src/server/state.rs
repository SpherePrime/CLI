use std::collections::HashMap;
use std::sync::atomic::AtomicBool;
use std::sync::{Arc, Mutex, RwLock};

use agent_config::{AgentConfig, ModelConfig, PermissionMode};
use agent_core::{EngineApprover, EngineEvent, EventClock};
use agent_permissions::{PermissionDecision, PermissionEngine};
use tokio::sync::mpsc;

pub struct EngineHandle {
    pub cancel: Arc<AtomicBool>,
    pub approver: Arc<EngineApprover>,
    pub permissions: Arc<Mutex<PermissionEngine>>,
    pub sender: mpsc::Sender<EngineEvent>,
    pub clock: EventClock,
}

pub struct AppState {
    pub config: RwLock<Option<AgentConfig>>,
    pub model: RwLock<Option<ModelConfig>>,
    pub catalog: RwLock<Option<crate::server::catalog::Catalog>>,
    pub storage: Option<agent_storage::Storage>,
    pub workspace: std::path::PathBuf,
    pub token: Option<String>,
    engines: RwLock<HashMap<uuid::Uuid, Arc<EngineHandle>>>,
}

impl AppState {
    pub fn new(
        storage: agent_storage::Storage,
        workspace: std::path::PathBuf,
        token: Option<String>,
    ) -> anyhow::Result<Self> {
        Ok(Self {
            config: RwLock::new(None),
            model: RwLock::new(None),
            catalog: RwLock::new(None),
            storage: Some(storage),
            workspace,
            token,
            engines: RwLock::new(HashMap::new()),
        })
    }

    pub fn storage(&self) -> Option<&agent_storage::Storage> {
        self.storage.as_ref()
    }

    pub fn resolve_model(&self) -> ModelConfig {
        if let Some(model) = self.model.read().unwrap().as_ref() {
            return model.clone();
        }
        if let Some(cfg) = self.config.read().unwrap().as_ref() {
            if let Some(model) = cfg.model.clone() {
                return model;
            }
        }
        ModelConfig {
            provider: agent_config::ProviderKind::Mock,
            model: "mock-1".into(),
            base_url: None,
            api_key_env: None,
            temperature: None,
            max_tokens: None,
        }
    }

    #[allow(clippy::too_many_arguments)]
    pub fn register_engine(
        &self,
        session_id: uuid::Uuid,
        cancel: Arc<AtomicBool>,
        approver: Arc<EngineApprover>,
        permissions: Arc<Mutex<PermissionEngine>>,
        sender: mpsc::Sender<EngineEvent>,
        clock: EventClock,
    ) {
        self.engines.write().unwrap().insert(
            session_id,
            Arc::new(EngineHandle {
                cancel,
                approver,
                permissions,
                sender,
                clock,
            }),
        );
    }

    pub fn unregister_engine(&self, session_id: &uuid::Uuid) {
        self.engines.write().unwrap().remove(session_id);
    }

    pub fn is_session_busy(&self, session_id: &uuid::Uuid) -> bool {
        self.engines.read().unwrap().contains_key(session_id)
    }

    pub fn cancel_session(&self, session_id: &uuid::Uuid) -> bool {
        match self.engines.read().unwrap().get(session_id) {
            Some(handle) => {
                handle
                    .cancel
                    .store(true, std::sync::atomic::Ordering::SeqCst);
                true
            }
            None => false,
        }
    }

    pub fn set_permission_mode(&self, session_id: &uuid::Uuid, mode: PermissionMode) -> bool {
        let handle = self.engines.read().unwrap().get(session_id).cloned();
        let Some(handle) = handle else {
            return false;
        };
        handle.permissions.lock().unwrap().set_mode(mode);
        match mode {
            PermissionMode::Allow => handle.approver.allow_all_pending(PermissionDecision::Allow),
            PermissionMode::AutoEdit => handle.approver.resolve_pending_file_scopes(),
            PermissionMode::Deny => handle.approver.allow_all_pending(PermissionDecision::Deny),
            _ => {}
        }
        let _ = handle.sender.try_send(EngineEvent::PermissionModeChanged {
            meta: handle.clock.meta("permission_mode"),
            mode: permission_mode_name(mode).to_string(),
        });
        true
    }

    pub async fn resolve_permission(
        &self,
        session_id: &uuid::Uuid,
        permission_id: uuid::Uuid,
        decision: PermissionDecision,
        remember: bool,
    ) -> bool {
        let approver = self
            .engines
            .read()
            .unwrap()
            .get(session_id)
            .map(|handle| Arc::clone(&handle.approver));
        match approver {
            Some(approver) => approver.resolve(permission_id, decision, remember).await,
            None => false,
        }
    }
}

pub fn permission_mode_name(mode: PermissionMode) -> &'static str {
    match mode {
        PermissionMode::Ask => "ask",
        PermissionMode::Allow => "full_access",
        PermissionMode::AutoEdit => "auto_edit",
        PermissionMode::Deny => "deny",
    }
}

impl std::fmt::Debug for AppState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("AppState").finish()
    }
}
