pub mod executor;
pub mod registry;

pub use executor::{
    ToolDefinition, ToolExecutionContext, ToolExecutor, ToolOutput,
};
pub use registry::ToolRegistry;

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use agent_config::PermissionsConfig;
    use agent_permissions::{PermissionEngine, PermissionScope};
    use anyhow::Result;
    use async_trait::async_trait;
    use uuid::Uuid;

    use crate::executor::{ToolDefinition, ToolExecutionContext, ToolExecutor, ToolOutput};
    use crate::registry::ToolRegistry;

    struct EchoTool;

    #[async_trait]
    impl ToolExecutor for EchoTool {
        async fn execute(
            &self,
            args: serde_json::Value,
            _ctx: &ToolExecutionContext,
        ) -> Result<ToolOutput> {
            let text = args
                .get("text")
                .and_then(|v| v.as_str())
                .unwrap_or_default()
                .to_string();
            Ok(ToolOutput::success(format!("echo {text}")))
        }
    }

    #[tokio::test]
    async fn registry_call() {
        let engine = PermissionEngine::new(PermissionsConfig::default());
        let ctx = ToolExecutionContext::new(
            Uuid::new_v4(),
            std::env::current_dir().unwrap(),
            engine,
        );
        let def = ToolDefinition {
            name: "echo".into(),
            description: "echo back".into(),
            input_schema: serde_json::json!({
                "type": "object",
                "properties": { "text": { "type": "string" } }
            }),
            executor: Arc::new(EchoTool),
            permissions: PermissionScope::Read,
            timeout_secs: 10,
        };
        let reg = ToolRegistry::new().register(def);
        let out = reg
            .call("echo", serde_json::json!({ "text": "hi" }), &ctx)
            .await
            .unwrap();
        assert!(out.ok);
        assert!(out.content.contains("hi"));
    }
}
