use std::sync::Arc;

use anyhow::{Context, Result};
use async_trait::async_trait;
use serde_json::{json, Value};

use agent_terminal::{ExecOutcome, ExecRequest, ShellRunner};

use crate::builtin::paths::{display, resolve_path, truncate};
use crate::executor::{ToolExecutionContext, ToolExecutor, ToolOutput};

const MAX_OUTPUT: usize = 262_144;

pub struct ShellTool;

#[async_trait]
impl ToolExecutor for ShellTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let command = args
            .get("command")
            .and_then(|v| v.as_str())
            .context("command is required")?;
        let cwd = match args.get("cwd").and_then(|v| v.as_str()) {
            Some(raw) => resolve_path(&ctx.working_dir, raw)?,
            None => ctx.working_dir.clone(),
        };
        let timeout = args.get("timeout_secs").and_then(|v| v.as_u64());
        let (shell, flag) = if cfg!(windows) {
            ("cmd", "/C")
        } else {
            ("sh", "-c")
        };
        let limit = timeout.unwrap_or(120);
        let request = ExecRequest {
            command: shell.to_string(),
            args: vec![flag.to_string(), command.to_string()],
            cwd: Some(cwd.clone()),
            env: Vec::new(),
            timeout_secs: Some(limit),
        };
        let runner = ShellRunner::new();
        let outcome = runner
            .run_with_cancel(&request, limit, Arc::clone(&ctx.cancel))
            .await
            .with_context(|| format!("running command in {}", display(&cwd)))?;
        let output = match outcome {
            ExecOutcome::Completed(output) => output,
            ExecOutcome::TimedOut => {
                return Ok(ToolOutput::failure(format!(
                    "command timed out after {limit}s"
                )));
            }
            ExecOutcome::Cancelled => {
                return Ok(ToolOutput::failure("command cancelled".into()));
            }
        };
        let mut body = String::new();
        if !output.stdout.trim().is_empty() {
            body.push_str(output.stdout.trim_end());
        }
        if !output.stderr.trim().is_empty() {
            if !body.is_empty() {
                body.push('\n');
            }
            body.push_str("stderr:\n");
            body.push_str(output.stderr.trim_end());
        }
        if body.is_empty() {
            body = "(no output)".to_string();
        }
        let report = format!("exit_code: {}\n{}", output.exit_code, body);
        let truncated = report.len() > MAX_OUTPUT;
        let content = truncate(&report, MAX_OUTPUT);
        if output.exit_code == 0 {
            Ok(ToolOutput::success(content)
                .exit_code(output.exit_code)
                .truncated(truncated))
        } else {
            Ok(
                ToolOutput::failure(format!("command exited with code {}", output.exit_code))
                    .content(content)
                    .exit_code(output.exit_code)
                    .truncated(truncated),
            )
        }
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
