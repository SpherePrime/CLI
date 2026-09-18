use anyhow::{bail, Result};
use async_trait::async_trait;
use serde_json::{json, Value};

use agent_git::GitRepo;

use crate::builtin::paths::truncate;
use crate::executor::{ToolExecutionContext, ToolExecutor, ToolOutput};

const MAX_OUTPUT: usize = 262_144;

pub struct GitTool;

#[async_trait]
impl ToolExecutor for GitTool {
    async fn execute(&self, args: Value, _ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let action = args
            .get("action")
            .and_then(|v| v.as_str())
            .unwrap_or("status");
        let repo = GitRepo::new(_ctx.working_dir.clone());
        if !repo.is_repo().await {
            bail!("not inside a git repository");
        }
        match action {
            "status" => {
                let status = repo.status().await?;
                let body = serde_json::to_string_pretty(&status)?;
                Ok(ToolOutput::success(truncate(&body, MAX_OUTPUT)))
            }
            "diff" => {
                let staged = args
                    .get("staged")
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false);
                let diff = repo.diff(staged).await?;
                if diff.trim().is_empty() {
                    Ok(ToolOutput::success("(no changes)".into()))
                } else {
                    Ok(ToolOutput::success(truncate(&diff, MAX_OUTPUT)))
                }
            }
            "log" => {
                let limit = args.get("limit").and_then(|v| v.as_u64()).unwrap_or(10) as usize;
                let entries = repo.log(limit).await?;
                let body = serde_json::to_string_pretty(&entries)?;
                Ok(ToolOutput::success(truncate(&body, MAX_OUTPUT)))
            }
            "branches" => {
                let branches = repo.branches().await?;
                Ok(ToolOutput::success(branches.join("\n")))
            }
            "add" => {
                let paths = string_list(&args, "paths");
                if paths.is_empty() {
                    bail!("paths is required for add");
                }
                let refs: Vec<&str> = paths.iter().map(|s| s.as_str()).collect();
                repo.add(&refs).await?;
                Ok(ToolOutput::success(format!("staged {}", paths.join(", "))))
            }
            "commit" => {
                let message = args
                    .get("message")
                    .and_then(|v| v.as_str())
                    .unwrap_or("update from agent");
                let allow_empty = args
                    .get("allow_empty")
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false);
                repo.commit(message, allow_empty).await?;
                Ok(ToolOutput::success(format!("committed: {message}")))
            }
            "branch" => {
                let name = args
                    .get("name")
                    .and_then(|v| v.as_str())
                    .ok_or_else(|| anyhow::anyhow!("name is required for branch"))?;
                repo.checkout(name).await?;
                Ok(ToolOutput::success(format!("switched to {name}")))
            }
            other => bail!("unsupported git action: {other}"),
        }
    }
}

fn string_list(args: &Value, key: &str) -> Vec<String> {
    args.get(key)
        .and_then(|v| v.as_array())
        .map(|items| {
            items
                .iter()
                .filter_map(|v| v.as_str().map(|s| s.to_string()))
                .collect()
        })
        .unwrap_or_default()
}

pub fn git_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "git".into(),
        description: "Inspect or modify git state. Actions: status, diff, log, branches, add, commit, branch.".into(),
        input_schema: json!({
            "type": "object",
            "properties": {
                "action": {
                    "type": "string",
                    "enum": ["status", "diff", "log", "branches", "add", "commit", "branch"]
                },
                "staged": { "type": "boolean" },
                "limit": { "type": "integer" },
                "paths": { "type": "array", "items": { "type": "string" } },
                "message": { "type": "string" },
                "name": { "type": "string" },
                "allow_empty": { "type": "boolean" }
            },
            "required": ["action"],
            "additionalProperties": false,
        }),
        executor: std::sync::Arc::new(GitTool),
        permissions: agent_permissions::PermissionScope::Execute,
        timeout_secs: 60,
    }
}
