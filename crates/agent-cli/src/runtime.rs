use std::sync::Arc;

use agent_events::{AgentEvent, AgentEventPayload, AgentScope, EventBus};
use agent_model::{MockProvider, ModelProvider, ToolSchema};
use agent_tools::{ToolDefinition, ToolExecutionContext, ToolOutput, ToolRegistry};
use agent_ui::AgentApp;
use uuid::Uuid;

pub struct AgentRuntime {
    pub session_id: Uuid,
    pub provider: Arc<dyn ModelProvider>,
    pub events: EventBus,
    pub tool_registry: Arc<ToolRegistry>,
    pub tool_ctx: Option<ToolExecutionContext>,
    _app: Option<AgentApp>,
}

impl AgentRuntime {
    pub fn new(session_id: Uuid, provider: Arc<dyn ModelProvider>, app: AgentApp) -> Self {
        let events = EventBus::new();
        events.dispatch(
            AgentEventPayload::new(AgentEvent::AgentStarted)
                .with_scope(AgentScope::Agent)
                .with_detail(serde_json::json!({ "session_id": session_id })),
        );
        Self {
            session_id,
            provider,
            events,
            tool_registry: Arc::new(ToolRegistry::new()),
            tool_ctx: None,
            _app: Some(app),
        }
    }

    pub fn stop(&self) {
        self.events.dispatch(
            AgentEventPayload::new(AgentEvent::AgentStopped).with_scope(AgentScope::Agent),
        );
    }

    pub fn with_tool_ctx(&mut self, ctx: ToolExecutionContext) {
        self.tool_ctx = Some(ctx);
    }

    pub async fn call_tool(
        &self,
        name: &str,
        args: serde_json::Value,
    ) -> anyhow::Result<ToolOutput> {
        let ctx = self
            .tool_ctx
            .as_ref()
            .ok_or_else(|| anyhow::anyhow!("no tool execution context"))?;
        self.tool_registry.call(name, args, ctx).await
    }

    pub fn tool_names(&self) -> Vec<String> {
        self.tool_registry.schemas().into_iter().map(|s| s.name).collect()
    }

    pub fn tool_schemas(&self) -> Vec<ToolSchema> {
        self.tool_registry.schemas()
    }
}

impl Default for AgentRuntime {
    fn default() -> Self {
        Self {
            session_id: Uuid::new_v4(),
            provider: Arc::new(MockProvider::new()),
            events: EventBus::new(),
            tool_registry: Arc::new(ToolRegistry::new()),
            tool_ctx: None,
            _app: None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use agent_config::PermissionsConfig;
    use agent_permissions::PermissionEngine;

    #[tokio::test]
    async fn runtime_with_context() {
        let mut rt: AgentRuntime = Default::default();
        let engine = PermissionEngine::new(PermissionsConfig::default());
        let ctx = ToolExecutionContext::new(
            Uuid::new_v4(),
            std::env::current_dir().unwrap(),
            engine,
        );
        rt.with_tool_ctx(ctx);
        assert!(rt.tool_ctx.is_some());
    }
}
