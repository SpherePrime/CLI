use anyhow::{Context, Result};
use async_trait::async_trait;
use serde_json::{json, Value};

use crate::builtin::exec::run_shell_command;
use crate::builtin::paths::resolve_path;
use crate::executor::{ToolExecutionContext, ToolExecutor};

pub struct ShellTool;

#[async_trait]
impl ToolExecutor for ShellTool {
    async fn execute(
        &self,
        args: Value,
        ctx: &ToolExecutionContext,
    ) -> Result<crate::executor::ToolOutput> {
        let command = args
            .get("command")
            .and_then(|v| v.as_str())
            .context("command is required")?;
        let cwd = match args.get("cwd").and_then(|v| v.as_str()) {
            Some(raw) => resolve_path(&ctx.working_dir, raw)?,
            None => ctx.working_dir.clone(),
        };
        let limit = args
            .get("timeout_secs")
            .and_then(|v| v.as_u64())
            .unwrap_or(120);
        run_shell_command(ctx, command, cwd, limit).await
    }
}

pub fn shell_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "shell".into(),
        description: "Run a shell command inside the working directory and return stdout, stderr and exit code.".into(),
        input_schema: json!({
            "type": "object",
            "properties": {
                "command": { "type": "string" },
                "cwd": { "type": "string" },
                "timeout_secs": { "type": "integer" }
            },
            "required": ["command"],
            "additionalProperties": false,
        }),
        executor: std::sync::Arc::new(ShellTool),
        permissions: agent_permissions::PermissionScope::Execute,
        timeout_secs: 180,
    }
}
