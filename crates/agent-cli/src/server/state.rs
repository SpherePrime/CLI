use std::sync::RwLock;

use agent_config::{AgentConfig, ModelConfig};
use tokio::sync::mpsc;

#[derive(Debug, Clone)]
pub enum SessionMessage {
    User(String),
    Assistant(String),
    Error(String),
}

#[derive(Debug, Clone)]
pub struct SessionChannel {
    pub session_id: uuid::Uuid,
    pub tx: mpsc::Sender<serde_json::Value>,
}

pub struct AppState {
    pub config: RwLock<Option<AgentConfig>>,
    pub model: RwLock<Option<ModelConfig>>,
    pub sessions: RwLock<SessionRegistry>,
    pub catalog: RwLock<Option<crate::server::catalog::Catalog>>,
    pub storage: Option<agent_storage::Storage>,
}

#[derive(Debug, Default)]
pub struct SessionRegistry {
    pub channels: Vec<SessionChannel>,
}

impl AppState {
    pub fn new(storage: agent_storage::Storage) -> anyhow::Result<Self> {
        Ok(Self {
            config: RwLock::new(None),
            model: RwLock::new(None),
            sessions: RwLock::new(SessionRegistry::default()),
            catalog: RwLock::new(None),
            storage: Some(storage),
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

    pub fn subscribe(&self, session_id: uuid::Uuid, tx: mpsc::Sender<serde_json::Value>) {
        let mut registry = self.sessions.write().unwrap();
        registry
            .channels
            .retain(|channel| !channel.tx.is_closed() || channel.session_id != session_id);
        registry.channels.push(SessionChannel { session_id, tx });
    }
}

impl std::fmt::Debug for AppState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("AppState").finish()
    }
}