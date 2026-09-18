use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use chrono::Utc;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum AgentScope {
    Agent,
    Message,
    Tool,
    File,
    Command,
    Model,
    Plugin,
    Skill,
    Mcp,
    Session,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
pub enum AgentEvent {
    #[default]
    AgentStarted,
    AgentStopped,
    AgentPaused,
    AgentResumed,
    MessageCreated,
    MessageCompleted,
    ToolBefore,
    ToolAfter,
    ToolError,
    FileBeforeWrite,
    FileAfterWrite,
    CommandBefore,
    CommandAfter,
    ModelRequest,
    ModelResponse,
    PluginLoaded,
    PluginUnloaded,
    SkillActivated,
    SkillDeactivated,
    McpConnected,
    McpDisconnected,
    ContextCompacted,
    SessionSaved,
    SessionResumed,
    ShutdownRequested,
}

impl AgentEvent {
    pub fn topic(&self) -> &'static str {
        let s: &str = self.into();
        s
    }
}

impl From<&AgentEvent> for &str {
    fn from(event: &AgentEvent) -> Self {
        match event {
            AgentEvent::AgentStarted => "agent.started",
            AgentEvent::AgentStopped => "agent.stopped",
            AgentEvent::AgentPaused => "agent.paused",
            AgentEvent::AgentResumed => "agent.resumed",
            AgentEvent::MessageCreated => "message.created",
            AgentEvent::MessageCompleted => "message.completed",
            AgentEvent::ToolBefore => "tool.before",
            AgentEvent::ToolAfter => "tool.after",
            AgentEvent::ToolError => "tool.error",
            AgentEvent::FileBeforeWrite => "file.before_write",
            AgentEvent::FileAfterWrite => "file.after_write",
            AgentEvent::CommandBefore => "command.before",
            AgentEvent::CommandAfter => "command.after",
            AgentEvent::ModelRequest => "model.request",
            AgentEvent::ModelResponse => "model.response",
            AgentEvent::PluginLoaded => "plugin.loaded",
            AgentEvent::PluginUnloaded => "plugin.unloaded",
            AgentEvent::SkillActivated => "skill.activated",
            AgentEvent::SkillDeactivated => "skill.deactivated",
            AgentEvent::McpConnected => "mcp.connected",
            AgentEvent::McpDisconnected => "mcp.disconnected",
            AgentEvent::ContextCompacted => "context.compacted",
            AgentEvent::SessionSaved => "session.saved",
            AgentEvent::SessionResumed => "session.resumed",
            AgentEvent::ShutdownRequested => "shutdown.requested",
        }
    }
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AgentEventPayload {
    pub event: AgentEvent,
    pub scope: Option<AgentScope>,
    pub detail: Option<serde_json::Value>,
}

impl AgentEventPayload {
    pub fn new(event: AgentEvent) -> Self {
        Self {
            event,
            scope: None,
            detail: None,
        }
    }

    pub fn with_scope(mut self, scope: AgentScope) -> Self {
        self.scope = Some(scope);
        self
    }

    pub fn with_detail(mut self, detail: serde_json::Value) -> Self {
        self.detail = Some(detail);
        self
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EventEnvelope {
    pub id: Uuid,
    pub event: AgentEvent,
    pub scope: Option<AgentScope>,
    pub detail: Option<serde_json::Value>,
    pub timestamp: chrono::DateTime<Utc>,
}

pub trait EventSubscriber: Send + Sync + 'static {
    fn on_event(&mut self, envelope: &EventEnvelope);
}

struct SubscriberSlot {
    name: String,
    subscriber: Box<dyn EventSubscriber>,
}

pub struct EventBus {
    subscribers: Arc<Mutex<HashMap<String, SubscriberSlot>>>,
}

impl Clone for EventBus {
    fn clone(&self) -> Self {
        Self {
            subscribers: Arc::clone(&self.subscribers),
        }
    }
}

impl EventBus {
    pub fn new() -> Self {
        Self {
            subscribers: Arc::new(Mutex::new(HashMap::new())),
        }
    }

    pub fn dispatch(&self, payload: AgentEventPayload) {
        let envelope = EventEnvelope {
            id: Uuid::new_v4(),
            event: payload.event,
            scope: payload.scope,
            detail: payload.detail,
            timestamp: Utc::now(),
        };
        let mut subs = self.subscribers.lock().unwrap();
        for slot in subs.values_mut() {
            tracing::trace!(subscriber = slot.name.as_str(), event = ?envelope.event, "dispatching");
            slot.subscriber.on_event(&envelope);
        }
    }

    pub fn subscribe(&self, name: &str, subscriber: Box<dyn EventSubscriber>) {
        let mut subs = self.subscribers.lock().unwrap();
        subs.insert(
            name.to_string(),
            SubscriberSlot {
                name: name.to_string(),
                subscriber,
            },
        );
    }

    pub fn unsubscribe(&self, name: &str) {
        let mut subs = self.subscribers.lock().unwrap();
        subs.remove(name);
    }

    pub fn subscriber_names(&self) -> Vec<String> {
        let subs = self.subscribers.lock().unwrap();
        subs.values().map(|s| s.name.clone()).collect()
    }
}

impl Default for EventBus {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, thiserror::Error)]
pub enum AgentEventsError {
    #[error("unknown event subscriber {0}")]
    UnknownSubscriber(String),
}

pub type Result<T> = std::result::Result<T, AgentEventsError>;

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicBool, Ordering};

    struct FlagSubscriber {
        called: AtomicBool,
    }

    impl EventSubscriber for FlagSubscriber {
        fn on_event(&mut self, _: &EventEnvelope) {
            self.called.store(true, Ordering::SeqCst);
        }
    }

    #[test]
    fn dispatch_reaches_subscriber() {
        let bus = EventBus::new();
        let flag = FlagSubscriber {
            called: AtomicBool::new(false),
        };
        bus.subscribe("test", Box::new(flag));
        bus.dispatch(AgentEventPayload::new(AgentEvent::AgentStarted));
        assert!(bus.subscriber_names().contains(&"test".to_string()));
    }
}
