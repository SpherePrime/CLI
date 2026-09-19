use crate::builtin::paths::truncate;
use crate::executor::{ToolExecutionContext, ToolExecutor, ToolOutput};
use agent_git::GitRepo;
use anyhow::{bail, Result};
use async_trait::async_trait;
use serde_json::{json, Value};
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
            "checkout" | "branch" => {
                let name = args
                    .get("name")
                    .and_then(|v| v.as_str())
                    .ok_or_else(|| anyhow::anyhow!("name is required for {action}"))?;
                let create = args
                    .get("create")
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false);
                if create {
                    run_git(&repo, &["checkout", "-b", name]).await?;
                    Ok(ToolOutput::success(format!(
                        "created and switched to {name}"
                    )))
                } else {
                    run_git(&repo, &["checkout", name]).await?;
                    Ok(ToolOutput::success(format!("switched to {name}")))
                }
            }
            "push" => {
                let remote = args
                    .get("remote")
                    .and_then(|v| v.as_str())
                    .unwrap_or("origin");
                let branch = args.get("branch").and_then(|v| v.as_str());
                let set_upstream = args
                    .get("set_upstream")
                    .and_then(|v| v.as_bool())
                    .unwrap_or(true);
                let mut owned: Vec<String> = vec!["push".into()];
                if set_upstream {
                    owned.push("-u".into());
                }
                owned.push(remote.into());
                if let Some(branch) = branch {
                    owned.push(branch.into());
                }
                let out = run_git_owned(&repo, &owned).await?;
                Ok(ToolOutput::success(trim_or_ok(&out, "pushed")))
            }
            "pull" => {
                let remote = args
                    .get("remote")
                    .and_then(|v| v.as_str())
                    .unwrap_or("origin");
                let branch = args.get("branch").and_then(|v| v.as_str());
                let mut owned: Vec<String> = vec!["pull".into(), remote.into()];
                if let Some(branch) = branch {
                    owned.push(branch.into());
                }
                let out = run_git_owned(&repo, &owned).await?;
                Ok(ToolOutput::success(trim_or_ok(&out, "pulled")))
            }
            "fetch" => {
                let out = run_git(&repo, &["fetch", "--all", "--prune"]).await?;
                Ok(ToolOutput::success(trim_or_ok(&out, "fetched")))
            }
            "stash" => {
                let mode = args.get("mode").and_then(|v| v.as_str()).unwrap_or("push");
                let out = match mode {
                    "list" => run_git(&repo, &["stash", "list"]).await?,
                    "pop" => run_git(&repo, &["stash", "pop"]).await?,
                    "drop" => run_git(&repo, &["stash", "drop"]).await?,
                    _ => {
                        let message = args.get("message").and_then(|v| v.as_str());
                        match message {
                            Some(message) => {
                                run_git(&repo, &["stash", "push", "-m", message]).await?
                            }
                            None => run_git(&repo, &["stash", "push"]).await?,
                        }
                    }
                };
                Ok(ToolOutput::success(trim_or_ok(&out, "stashed")))
            }
            "show" => {
                let rev = args.get("rev").and_then(|v| v.as_str()).unwrap_or("HEAD");
                let out = run_git(&repo, &["show", "--stat", rev]).await?;
                Ok(ToolOutput::success(truncate(&out, MAX_OUTPUT)))
            }
            "blame" => {
                let path = args
                    .get("path")
                    .and_then(|v| v.as_str())
                    .ok_or_else(|| anyhow::anyhow!("path is required for blame"))?;
                let out = run_git(&repo, &["blame", "--date=short", path]).await?;
                Ok(ToolOutput::success(truncate(&out, MAX_OUTPUT)))
            }
            "reset" => {
                let mode = args.get("mode").and_then(|v| v.as_str()).unwrap_or("mixed");
                let rev = args.get("rev").and_then(|v| v.as_str()).unwrap_or("HEAD");
                let out = run_git(&repo, &["reset", &format!("--{mode}"), rev]).await?;
                Ok(ToolOutput::success(trim_or_ok(&out, "reset")))
            }
            "remote" => {
                let out = run_git(&repo, &["remote", "-v"]).await?;
                Ok(ToolOutput::success(trim_or_ok(&out, "(no remotes)")))
            }
            "run" => {
                let raw = string_list(&args, "args");
                if raw.is_empty() {
                    bail!("args is required for run");
                }
                let refs: Vec<&str> = raw.iter().map(|s| s.as_str()).collect();
                let out = repo.command(&refs).await?;
                Ok(ToolOutput::success(truncate(&out, MAX_OUTPUT)))
            }
            "worktree" => {
                let path = args
                    .get("path")
                    .and_then(|v| v.as_str())
                    .ok_or_else(|| anyhow::anyhow!("path is required for worktree"))?;
                let branch = args.get("branch").and_then(|v| v.as_str());
                let mut owned: Vec<String> = vec!["worktree".into(), "add".into(), path.into()];
                if let Some(branch) = branch {
                    owned.push(branch.into());
                }
                let out = run_git_owned(&repo, &owned).await?;
                Ok(ToolOutput::success(trim_or_ok(
                    &out,
                    &format!("worktree created at {path}"),
                )))
            }
            "restore" => {
                let paths = string_list(&args, "paths");
                if paths.is_empty() {
                    bail!("paths is required for restore");
                }
                let mut owned: Vec<String> = vec!["restore".into(), "--".into()];
                owned.extend(paths.iter().cloned());
                let out = run_git_owned(&repo, &owned).await?;
                Ok(ToolOutput::success(trim_or_ok(
                    &out,
                    &format!("restored {} from HEAD", paths.join(", ")),
                )))
            }
            other => bail!("unsupported git action: {other}"),
        }
    }
}
async fn run_git(repo: &GitRepo, args: &[&str]) -> Result<String> {
    repo.command(args).await
}
async fn run_git_owned(repo: &GitRepo, args: &[String]) -> Result<String> {
    let refs: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
    repo.command(&refs).await
}
fn trim_or_ok(output: &str, fallback: &str) -> String {
    let trimmed = output.trim();
    if trimmed.is_empty() {
        fallback.to_string()
    } else {
        truncated_owned(trimmed)
    }
}
fn truncated_owned(text: &str) -> String {
    truncate(text, MAX_OUTPUT)
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
        description: "Inspect or modify git state. Actions: status, diff, log, branches, add, commit, checkout, push, pull, fetch, stash, show, blame, reset, remote, worktree, restore, run.".into(),
        input_schema: json!({
            "type": "object",
            "properties": {
                "action": {
                    "type": "string",
                    "enum": ["status", "diff", "log", "branches", "add", "commit", "checkout", "branch", "push", "pull", "fetch", "stash", "show", "blame", "reset", "remote", "worktree", "restore", "run"]
                },
                "staged": { "type": "boolean" },
                "limit": { "type": "integer" },
                "paths": { "type": "array", "items": { "type": "string" } },
                "args": { "type": "array", "items": { "type": "string" } },
                "message": { "type": "string" },
                "name": { "type": "string" },
                "create": { "type": "boolean" },
                "remote": { "type": "string" },
                "branch": { "type": "string" },
                "set_upstream": { "type": "boolean" },
                "mode": { "type": "string" },
                "rev": { "type": "string" },
                "path": { "type": "string" },
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
#[cfg(test)]
mod tests {
    use super::*;
    use agent_config::PermissionsConfig;
    use agent_permissions::PermissionEngine;
    use tempfile::tempdir;
    fn ctx(root: &std::path::Path) -> ToolExecutionContext {
        ToolExecutionContext::new(
            uuid::Uuid::new_v4(),
            root.to_path_buf(),
            PermissionEngine::new(PermissionsConfig::default()),
        )
    }
    async fn init_repo(dir: &std::path::Path) -> GitRepo {
        let repo = GitRepo::new(dir.to_path_buf());
        run_git(&repo, &["init"]).await.unwrap();
        run_git(&repo, &["config", "user.name", "test"])
            .await
            .unwrap();
        run_git(&repo, &["config", "user.email", "test@example.com"])
            .await
            .unwrap();
        repo
    }
    #[tokio::test]
    async fn restore_reverts_local_changes() {
        let dir = tempdir().unwrap();
        let repo = init_repo(dir.path()).await;
        let file = dir.path().join("a.txt");
        std::fs::write(&file, "v1").unwrap();
        run_git(&repo, &["add", "a.txt"]).await.unwrap();
        run_git(&repo, &["commit", "-m", "first"]).await.unwrap();
        std::fs::write(&file, "modified").unwrap();
        let out = GitTool
            .execute(
                json!({ "action": "restore", "paths": ["a.txt"] }),
                &ctx(dir.path()),
            )
            .await
            .unwrap();
        assert!(out.ok, "restore failed: {:?}", out.error);
        assert_eq!(std::fs::read_to_string(&file).unwrap(), "v1");
    }
    #[tokio::test]
    async fn worktree_creates_sibling_with_committed_files() {
        let base = tempdir().unwrap();
        let main = base.path().join("main");
        std::fs::create_dir_all(&main).unwrap();
        let repo = init_repo(&main).await;
        let file = main.join("a.txt");
        std::fs::write(&file, "v1").unwrap();
        run_git(&repo, &["add", "a.txt"]).await.unwrap();
        run_git(&repo, &["commit", "-m", "first"]).await.unwrap();
        let wt = base.path().join("wt");
        let out = GitTool
            .execute(
                json!({ "action": "worktree", "path": wt.display().to_string() }),
                &ctx(&main),
            )
            .await
            .unwrap();
        assert!(out.ok, "worktree failed: {:?}", out.error);
        assert!(
            wt.join("a.txt").is_file(),
            "committed file must exist in worktree"
        );
    }
}
