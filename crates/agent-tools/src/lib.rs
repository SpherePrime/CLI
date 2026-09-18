pub mod builtin;
pub mod executor;
pub mod registry;

pub use executor::{
    PermissionApprover, PermissionRequest, ToolDefinition, ToolExecutionContext, ToolExecutor,
    ToolOutput, UserPrompter, UserQuestion,
};
pub use registry::ToolRegistry;

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use agent_config::{PermissionMode, PermissionsConfig};
    use agent_permissions::{PermissionDecision, PermissionEngine, PermissionScope};
    use anyhow::Result;
    use async_trait::async_trait;
    use uuid::Uuid;

    use crate::executor::{
        PermissionApprover, PermissionRequest, ToolDefinition, ToolExecutionContext, ToolExecutor,
        ToolOutput,
    };
    use crate::registry::ToolRegistry;

    struct EchoTool;

    struct FixedApprover(PermissionDecision);

    #[async_trait]
    impl PermissionApprover for FixedApprover {
        async fn approve(&self, _request: PermissionRequest) -> PermissionDecision {
            self.0
        }
    }

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
        let engine = PermissionEngine::new(PermissionsConfig {
            mode: PermissionMode::Allow,
            ..Default::default()
        });
        let ctx =
            ToolExecutionContext::new(Uuid::new_v4(), std::env::current_dir().unwrap(), engine);
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

    fn ask_tool() -> ToolDefinition {
        ToolDefinition {
            name: "echo".into(),
            description: "echo back".into(),
            input_schema: serde_json::json!({ "type": "object" }),
            executor: std::sync::Arc::new(EchoTool),
            permissions: PermissionScope::Write,
            timeout_secs: 10,
        }
    }

    async fn run_with(approver: PermissionDecision) -> ToolOutput {
        let engine = PermissionEngine::new(PermissionsConfig {
            mode: PermissionMode::Ask,
            ..Default::default()
        });
        let ctx =
            ToolExecutionContext::new(Uuid::new_v4(), std::env::current_dir().unwrap(), engine)
                .with_approver(std::sync::Arc::new(FixedApprover(approver)));
        let reg = ToolRegistry::new().register(ask_tool());
        reg.call("echo", serde_json::json!({ "text": "hi" }), &ctx)
            .await
            .unwrap()
    }

    #[tokio::test]
    async fn ask_allowed_executes() {
        let out = run_with(PermissionDecision::Allow).await;
        assert!(out.ok);
    }

    #[tokio::test]
    async fn ask_denied_blocks() {
        let out = run_with(PermissionDecision::Deny).await;
        assert!(!out.ok);
        assert!(out.error.unwrap_or_default().contains("permission denied"));
    }

    #[tokio::test]
    async fn ask_without_approver_fails_closed() {
        let engine = PermissionEngine::new(PermissionsConfig::default());
        let ctx =
            ToolExecutionContext::new(Uuid::new_v4(), std::env::current_dir().unwrap(), engine);
        let reg = ToolRegistry::new().register(ask_tool());
        let out = reg
            .call("echo", serde_json::json!({ "text": "hi" }), &ctx)
            .await
            .unwrap();
        assert!(!out.ok);
    }
}
