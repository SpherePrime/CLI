use anyhow::{Context, Result};
use async_trait::async_trait;
use serde_json::{json, Value};
use uuid::Uuid;

use crate::builtin::paths::resolve_path;
use crate::executor::{ToolExecutionContext, ToolExecutor, ToolOutput};
use crate::processes::ProcessManager;

const MAX_OUTPUT_LINES: usize = 5_000;

pub struct ProcessTool;

#[async_trait]
impl ToolExecutor for ProcessTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let action = args
            .get("action")
            .and_then(|v| v.as_str())
            .context("action is required")?;

        let manager = ProcessManager::global();

        match action {
            "start" => {
                let command = args
                    .get("command")
                    .and_then(|v| v.as_str())
                    .context("command is required")?;
                let cwd = match args.get("cwd").and_then(|v| v.as_str()) {
                    Some(raw) => resolve_path(&ctx.working_dir, raw)?,
                    None => ctx.working_dir.clone(),
                };
                let process = manager
                    .start(ctx.session_id, command, &cwd)
                    .with_context(|| format!("starting `{command}`"))?;
                Ok(ToolOutput::success(
                    json!({
                        "id": process.id,
                        "pid": process.pid,
                        "command": process.command,
                        "session": process.session,
                    })
                    .to_string(),
                ))
            }
            "output" => {
                let id = parse_id(&args)?;
                let process = manager.get(&id).context("unknown process id")?;
                let limit = args
                    .get("lines")
                    .and_then(|v| v.as_u64())
                    .unwrap_or(MAX_OUTPUT_LINES as u64) as usize;
                let lines = process.output(limit.clamp(1, MAX_OUTPUT_LINES));
                if lines.is_empty() {
                    Ok(ToolOutput::success("(no output yet)".into()).truncated(false))
                } else {
                    let body = lines.join("\n");
                    Ok(ToolOutput::success(body).truncated(lines.len() > limit))
                }
            }
            "write_stdin" => {
                let id = parse_id(&args)?;
                let process = manager.get(&id).context("unknown process id")?;
                let data = args
                    .get("data")
                    .and_then(|v| v.as_str())
                    .context("data is required")?;
                process.write_stdin(data)?;
                Ok(ToolOutput::success("stdin written".into()))
            }
            "status" => {
                let id = parse_id(&args)?;
                let process = manager.get(&id).context("unknown process id")?;
                let exit_code = process.try_wait();
                Ok(ToolOutput::success(
                    json!({
                        "id": process.id,
                        "pid": process.pid,
                        "command": process.command,
                        "running": exit_code.is_none(),
                        "exit_code": exit_code,
                        "output_lines": process.output(0).len(),
                    })
                    .to_string(),
                ))
            }
            "stop" => {
                let id = parse_id(&args)?;
                let stopped = manager.stop(&id)?;
                if stopped {
                    Ok(ToolOutput::success("process stopped".into()))
                } else {
                    Ok(ToolOutput::success("process already exited".into()))
                }
            }
            _ => Err(anyhow::anyhow!(
                "unknown action `{action}` (start, output, write_stdin, status, stop)"
            )),
        }
    }
}

fn parse_id(args: &Value) -> Result<Uuid> {
    args.get("id")
        .and_then(|v| v.as_str())
        .and_then(|raw| Uuid::parse_str(raw).ok())
        .context("id must be a valid process uuid")
}

fn schema() -> Value {
    json!({
        "type": "object",
        "properties": {
            "action": { "type": "string", "enum": ["start", "output", "write_stdin", "status", "stop"] },
            "command": { "type": "string" },
            "cwd": { "type": "string" },
            "id": { "type": "string" },
            "data": { "type": "string" },
            "lines": { "type": "integer" }
        },
        "required": ["action"],
        "additionalProperties": false,
    })
}

pub fn process_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "process".into(),
        description: "Manage long-running processes bound to the session (dev servers, watchers). Actions: start (spawn a command in the background), output (read the buffered stdout/stderr since start), write_stdin (send a line to the process stdin), status (pid, running, exit code), stop (terminate the process tree). Processes are killed automatically when the turn is cancelled or the session exits.".into(),
        input_schema: schema(),
        executor: std::sync::Arc::new(ProcessTool),
        permissions: agent_permissions::PermissionScope::Execute,
        timeout_secs: 30,
    }
}
