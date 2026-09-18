use std::path::PathBuf;

pub mod engine;

pub use engine::{AgentEngine, EngineApprover, EngineEvent};

use agent_config::{AgentConfig, ConfigLoader};
use agent_events::{AgentEvent, AgentEventPayload, AgentScope, EventBus};
use agent_storage::Storage;
use chrono::Utc;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentState {
    pub session_id: Uuid,
    pub started_at: chrono::DateTime<Utc>,
    pub model: Option<String>,
    pub project: Option<PathBuf>,
    pub config: AgentConfig,
}

pub struct Core {
    pub state: AgentState,
    pub events: EventBus,
    pub storage: Option<Storage>,
}

impl Core {
    pub fn new(config: AgentConfig) -> Self {
        let session_id = Uuid::new_v4();
        let storage = Storage::global().ok();
        let state = AgentState {
            session_id,
            started_at: Utc::now(),
            model: config.model.as_ref().map(|m| m.model.clone()),
            project: None,
            config,
        };
        let events = EventBus::new();
        events.dispatch(
            AgentEventPayload::new(AgentEvent::AgentStarted)
                .with_scope(AgentScope::Agent)
                .with_detail(serde_json::json!({ "session_id": session_id })),
        );
        Self {
            state,
            events,
            storage,
        }
    }

    pub fn with_project(mut self, path: &std::path::Path) -> Self {
        self.state.project = Some(path.to_path_buf());
        self
    }

    pub fn stop(&self) {
        self.events.dispatch(
            AgentEventPayload::new(AgentEvent::AgentStopped).with_scope(AgentScope::Agent),
        );
    }

    pub fn session_id(&self) -> Uuid {
        self.state.session_id
    }

    pub fn storage(&self) -> Option<&Storage> {
        self.storage.as_ref()
    }

    pub fn config(&self) -> &AgentConfig {
        &self.state.config
    }

    pub fn events(&self) -> &EventBus {
        &self.events
    }

    pub fn loader(&self) -> ConfigLoader {
        let mut l = ConfigLoader::new();
        if let Some(p) = &self.state.project {
            l = l.with_project(p);
        }
        l
    }

    pub async fn reload_config(&mut self) -> anyhow::Result<()> {
        let loader = self.loader();
        self.state.config = loader.load()?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use agent_config::AgentConfig;

    #[tokio::test]
    async fn agent_start_stop() {
        let c = Core::new(AgentConfig::default());
        let _ = c.session_id();
        c.stop();
    }
}
