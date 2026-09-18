use std::path::Path;

use anyhow::{bail, Context, Result};
use async_trait::async_trait;
use serde_json::{json, Value};

use agent_filesystem::snapshot;
use agent_filesystem::{find_files, list_directory, read_file, write_file, FileEdit};

use crate::builtin::paths::{display, resolve_path, truncate};
use crate::executor::{FileChange, ToolExecutionContext, ToolExecutor, ToolOutput};

const MAX_OUTPUT: usize = 262_144;
const MAX_READ_LINES: usize = 2000;

fn snapshot_before_change(ctx: &ToolExecutionContext, path: &Path) {
    let _ = snapshot::take_snapshot_turn(&ctx.working_dir, path, ctx.turn_id.as_deref());
}

fn schema(properties: Value, required: &[&str]) -> Value {
    json!({
        "type": "object",
        "properties": properties,
        "required": required,
        "additionalProperties": false,
    })
}

pub struct ReadFileTool;

#[async_trait]
impl ToolExecutor for ReadFileTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let raw = args
            .get("path")
            .and_then(|v| v.as_str())
            .unwrap_or_default();
        let path = resolve_path(&ctx.working_dir, raw)?;
        let content = read_file(&path)
            .await
            .with_context(|| format!("reading {}", display(&path)))?;
        let offset = args
            .get("offset")
            .and_then(|v| v.as_u64())
            .unwrap_or(1)
            .max(1) as usize;
        let limit = args
            .get("limit")
            .and_then(|v| v.as_u64())
            .unwrap_or(MAX_READ_LINES as u64) as usize;
        let lines: Vec<&str> = content.lines().collect();
        let mut out = String::new();
        for (index, line) in lines.iter().enumerate() {
            let number = index + 1;
            if number < offset {
                continue;
            }
            if number >= offset + limit {
                out.push_str(&format!(
                    "\n...[{} more lines]",
                    lines.len().saturating_sub(number - 1)
                ));
                break;
            }
            out.push_str(&format!("{number}: {line}\n"));
        }
        if out.is_empty() {
            out = "(empty file)".to_string();
        }
        Ok(ToolOutput::success(truncate(&out, MAX_OUTPUT)))
    }
}

pub fn read_file_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "read_file".into(),
        description:
            "Read a UTF-8 text file inside the working directory. Use offset/limit for large files."
                .into(),
        input_schema: schema(
            json!({
                "path": { "type": "string", "description": "Relative path to the file" },
                "offset": { "type": "integer", "description": "1-based first line (default 1)" },
                "limit": { "type": "integer", "description": "Maximum lines to read (default 2000)" }
            }),
            &["path"],
        ),
        executor: std::sync::Arc::new(ReadFileTool),
        permissions: agent_permissions::PermissionScope::Read,
        timeout_secs: 30,
    }
}

pub struct WriteFileTool;

#[async_trait]
impl ToolExecutor for WriteFileTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let raw = args
            .get("path")
            .and_then(|v| v.as_str())
            .unwrap_or_default();
        let path = resolve_path(&ctx.working_dir, raw)?;
        let content = args
            .get("content")
            .and_then(|v| v.as_str())
            .context("content is required")?;
        let existed = path.exists();
        if existed {
            snapshot_before_change(ctx, &path);
        }
        write_file(&path, content).await?;
        let change = FileChange {
            path: display(&path),
            change: if existed {
                "modified".into()
            } else {
                "created".into()
            },
            diff: None,
            additions: Some(content.lines().count()),
            deletions: None,
        };
        Ok(ToolOutput::success(format!(
            "wrote {} ({} bytes)",
            display(&path),
            content.len()
        ))
        .file_change(change))
    }
}

pub fn write_file_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "write_file".into(),
        description: "Create or overwrite a text file inside the working directory.".into(),
        input_schema: schema(
            json!({
                "path": { "type": "string" },
                "content": { "type": "string" }
            }),
            &["path", "content"],
        ),
        executor: std::sync::Arc::new(WriteFileTool),
        permissions: agent_permissions::PermissionScope::Write,
        timeout_secs: 30,
    }
}

pub struct EditFileTool;

#[async_trait]
impl ToolExecutor for EditFileTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let raw = args
            .get("path")
            .and_then(|v| v.as_str())
            .unwrap_or_default();
        let path = resolve_path(&ctx.working_dir, raw)?;
        let old = args
            .get("old")
            .and_then(|v| v.as_str())
            .context("old is required")?;
        let new = args
            .get("new")
            .and_then(|v| v.as_str())
            .context("new is required")?;
        let original = read_file(&path).await?;
        snapshot_before_change(ctx, &path);
        let edit = FileEdit::search_replace(&path, old, new);
        let updated = edit.apply(None).await?;
        let diff = edit.diff(&original, &updated);
        let change = FileChange {
            path: display(&path),
            change: "edit".into(),
            diff: Some(diff.clone()),
            additions: None,
            deletions: None,
        };
        Ok(ToolOutput::success(truncate(&diff, MAX_OUTPUT)).file_change(change))
    }
}

pub fn edit_file_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "edit_file".into(),
        description: "Replace the first occurrence of `old` with `new` in a file. Returns a diff."
            .into(),
        input_schema: schema(
            json!({
                "path": { "type": "string" },
                "old": { "type": "string" },
                "new": { "type": "string" }
            }),
            &["path", "old", "new"],
        ),
        executor: std::sync::Arc::new(EditFileTool),
        permissions: agent_permissions::PermissionScope::Write,
        timeout_secs: 30,
    }
}

pub struct PatchFileTool;

#[async_trait]
impl ToolExecutor for PatchFileTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let raw = args
            .get("path")
            .and_then(|v| v.as_str())
            .unwrap_or_default();
        let path = resolve_path(&ctx.working_dir, raw)?;
        let patch = args
            .get("patch")
            .and_then(|v| v.as_str())
            .context("patch is required")?;
        snapshot_before_change(ctx, &path);
        let edit = FileEdit::patch(&path, patch);
        edit.apply(None).await?;
        let change = FileChange {
            path: display(&path),
            change: "patch".into(),
            diff: None,
            additions: None,
            deletions: None,
        };
        Ok(ToolOutput::success(format!("patched {}", display(&path))).file_change(change))
    }
}

pub fn patch_file_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "patch_file".into(),
        description: "Apply a unified diff patch to a file inside the working directory.".into(),
        input_schema: schema(
            json!({
                "path": { "type": "string" },
                "patch": { "type": "string" }
            }),
            &["path", "patch"],
        ),
        executor: std::sync::Arc::new(PatchFileTool),
        permissions: agent_permissions::PermissionScope::Write,
        timeout_secs: 30,
    }
}

fn normalize_patch_path(raw: &str) -> String {
    let token = raw.split_whitespace().next().unwrap_or(raw).trim();
    token
        .strip_prefix("b/")
        .or_else(|| token.strip_prefix("a/"))
        .unwrap_or(token)
        .to_string()
}

fn split_patch_sections(patch: &str) -> Vec<(String, String)> {
    let mut sections: Vec<(String, String)> = Vec::new();
    let mut current: Option<(String, Vec<&str>)> = None;
    for line in patch.lines() {
        if let Some(rest) = line.strip_prefix("+++ ") {
            if let Some((path, lines)) = current.take() {
                sections.push((path, lines.join("\n")));
            }
            current = Some((normalize_patch_path(rest), Vec::new()));
        } else if let Some((_, lines)) = current.as_mut() {
            lines.push(line);
        }
    }
    if let Some((path, lines)) = current.take() {
        sections.push((path, lines.join("\n")));
    }
    sections
}

pub struct ApplyPatchTool;

#[async_trait]
impl ToolExecutor for ApplyPatchTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let patch = args
            .get("patch")
            .and_then(|v| v.as_str())
            .context("patch is required")?;
        let sections = split_patch_sections(patch);
        if sections.is_empty() {
            bail!("patch has no file headers (+++)");
        }
        let mut changes = Vec::new();
        let mut summary = Vec::new();
        for (target, section) in sections {
            if target.is_empty() || target == "/dev/null" {
                continue;
            }
            let path = resolve_path(&ctx.working_dir, &target)?;
            let existed = path.exists();
            if existed {
                snapshot_before_change(ctx, &path);
            } else if let Some(parent) = path.parent() {
                tokio::fs::create_dir_all(parent).await?;
                tokio::fs::write(&path, "").await?;
            }
            let edit = FileEdit::patch(&path, &section);
            edit.apply(None)
                .await
                .with_context(|| format!("applying patch to {target}"))?;
            changes.push(FileChange {
                path: display(&path),
                change: if existed { "patch" } else { "created" }.into(),
                diff: None,
                additions: None,
                deletions: None,
            });
            summary.push(format!(
                "{} {}",
                if existed { "patched" } else { "created" },
                display(&path)
            ));
        }
        if changes.is_empty() {
            bail!("patch produced no changes");
        }
        Ok(ToolOutput::success(summary.join("\n")).file_changes(changes))
    }
}

pub fn apply_patch_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "apply_patch".into(),
        description:
            "Apply a multi-file unified diff patch. Each file section starts with --- a/path and +++ b/path followed by @@ hunks; new files use --- /dev/null."
                .into(),
        input_schema: schema(
            json!({
                "patch": { "type": "string", "description": "Unified diff covering one or more files" }
            }),
            &["patch"],
        ),
        executor: std::sync::Arc::new(ApplyPatchTool),
        permissions: agent_permissions::PermissionScope::Write,
        timeout_secs: 60,
    }
}

pub struct DeletePathTool;

#[async_trait]
impl ToolExecutor for DeletePathTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let raw = args
            .get("path")
            .and_then(|v| v.as_str())
            .unwrap_or_default();
        let path = resolve_path(&ctx.working_dir, raw)?;
        if path == ctx.working_dir {
            bail!("refusing to delete the working directory");
        }
        let recursive = args
            .get("recursive")
            .and_then(|v| v.as_bool())
            .unwrap_or(false);
        if path.is_dir() {
            if !recursive {
                bail!("path is a directory; pass recursive=true to delete it");
            }
            tokio::fs::remove_dir_all(&path).await?;
        } else {
            snapshot_before_change(ctx, &path);
            tokio::fs::remove_file(&path).await?;
        }
        let change = FileChange {
            path: display(&path),
            change: "delete".into(),
            diff: None,
            additions: None,
            deletions: None,
        };
        Ok(ToolOutput::success(format!("deleted {}", display(&path))).file_change(change))
    }
}

pub fn delete_path_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "delete_path".into(),
        description:
            "Delete a file (or directory when recursive is true) inside the working directory."
                .into(),
        input_schema: schema(
            json!({
                "path": { "type": "string" },
                "recursive": { "type": "boolean" }
            }),
            &["path"],
        ),
        executor: std::sync::Arc::new(DeletePathTool),
        permissions: agent_permissions::PermissionScope::Delete,
        timeout_secs: 30,
    }
}

pub struct ListDirectoryTool;

#[async_trait]
impl ToolExecutor for ListDirectoryTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let raw = args.get("path").and_then(|v| v.as_str()).unwrap_or(".");
        let path = resolve_path(&ctx.working_dir, raw)?;
        let mut entries = list_directory(&path).await?;
        entries.sort();
        if entries.is_empty() {
            return Ok(ToolOutput::success("(empty directory)".into()));
        }
        let mut out = String::new();
        for name in entries {
            let child = path.join(&name);
            let kind = if child.is_dir() { "dir " } else { "file" };
            out.push_str(&format!("{kind}  {name}\n"));
        }
        Ok(ToolOutput::success(truncate(&out, MAX_OUTPUT)))
    }
}

pub fn list_directory_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "list_directory".into(),
        description: "List entries of a directory inside the working directory.".into(),
        input_schema: schema(
            json!({ "path": { "type": "string", "description": "Relative directory (default .)" } }),
            &[],
        ),
        executor: std::sync::Arc::new(ListDirectoryTool),
        permissions: agent_permissions::PermissionScope::Read,
        timeout_secs: 30,
    }
}

pub struct GlobTool;

#[async_trait]
impl ToolExecutor for GlobTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let pattern = args
            .get("pattern")
            .and_then(|v| v.as_str())
            .context("pattern is required")?;
        let raw = args.get("path").and_then(|v| v.as_str()).unwrap_or(".");
        let root = resolve_path(&ctx.working_dir, raw)?;
        let matches = find_files(&root, pattern).await?;
        if matches.is_empty() {
            return Ok(ToolOutput::success("(no matches)".into()));
        }
        let mut out = matches
            .iter()
            .take(500)
            .map(|p| display(p))
            .collect::<Vec<_>>()
            .join("\n");
        if matches.len() > 500 {
            out.push_str(&format!("\n...[{} more matches]", matches.len() - 500));
        }
        Ok(ToolOutput::success(truncate(&out, MAX_OUTPUT)))
    }
}

pub fn glob_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "glob".into(),
        description: "Find files by glob pattern (e.g. **/*.rs) inside the working directory."
            .into(),
        input_schema: schema(
            json!({
                "pattern": { "type": "string" },
                "path": { "type": "string", "description": "Relative root (default .)" }
            }),
            &["pattern"],
        ),
        executor: std::sync::Arc::new(GlobTool),
        permissions: agent_permissions::PermissionScope::Read,
        timeout_secs: 30,
    }
}

pub struct SearchTool;

#[async_trait]
impl ToolExecutor for SearchTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let query = args
            .get("query")
            .and_then(|v| v.as_str())
            .context("query is required")?;
        let raw = args.get("path").and_then(|v| v.as_str()).unwrap_or(".");
        let root = resolve_path(&ctx.working_dir, raw)?;
        let regex = regex::Regex::new(query).context("invalid regex query")?;
        let glob_filter = args.get("glob").and_then(|v| v.as_str());
        let matcher = match glob_filter {
            Some(pattern) => Some(glob::Pattern::new(pattern).context("invalid glob filter")?),
            None => None,
        };
        let mut matches: Vec<String> = Vec::new();
        for entry in walkdir::WalkDir::new(&root).into_iter().flatten() {
            if matches.len() >= 200 {
                break;
            }
            if !entry.file_type().is_file() {
                continue;
            }
            let path = entry.path();
            if is_ignored(path) {
                continue;
            }
            if let Some(matcher) = &matcher {
                if !matcher.matches(path.to_str().unwrap_or_default()) {
                    continue;
                }
            }
            let Ok(meta) = entry.metadata() else { continue };
            if meta.len() > 1_000_000 {
                continue;
            }
            let Ok(content) = std::fs::read_to_string(path) else {
                continue;
            };
            for (index, line) in content.lines().enumerate() {
                if regex.is_match(line) {
                    matches.push(format!("{}:{}: {}", display(path), index + 1, line.trim()));
                    if matches.len() >= 200 {
                        break;
                    }
                }
            }
        }
        if matches.is_empty() {
            return Ok(ToolOutput::success("(no matches)".into()));
        }
        Ok(ToolOutput::success(truncate(
            &matches.join("\n"),
            MAX_OUTPUT,
        )))
    }
}

fn is_ignored(path: &Path) -> bool {
    path.components().any(|component| {
        let name = component.as_os_str().to_string_lossy();
        matches!(
            name.as_ref(),
            ".git" | "node_modules" | "target" | "dist" | ".next" | ".cache"
        )
    })
}

pub fn search_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "search".into(),
        description: "Search file contents by regular expression inside the working directory."
            .into(),
        input_schema: schema(
            json!({
                "query": { "type": "string" },
                "path": { "type": "string", "description": "Relative root (default .)" },
                "glob": { "type": "string", "description": "Optional glob filter" }
            }),
            &["query"],
        ),
        executor: std::sync::Arc::new(SearchTool),
        permissions: agent_permissions::PermissionScope::Read,
        timeout_secs: 60,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use agent_config::PermissionsConfig;
    use agent_permissions::PermissionEngine;

    fn test_root() -> std::path::PathBuf {
        let dir = std::env::temp_dir().join(format!("agent-fs-test-{}", uuid::Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        dir
    }

    fn ctx(root: &Path) -> ToolExecutionContext {
        ToolExecutionContext::new(
            uuid::Uuid::new_v4(),
            root.to_path_buf(),
            PermissionEngine::new(PermissionsConfig::default()),
        )
    }

    #[tokio::test]
    async fn write_file_reports_file_change() {
        let tmp = test_root();
        let output = WriteFileTool
            .execute(
                json!({ "path": "a.txt", "content": "line1\nline2" }),
                &ctx(&tmp),
            )
            .await
            .unwrap();
        assert_eq!(output.file_changes.len(), 1);
        let change = &output.file_changes[0];
        assert_eq!(change.change, "created");
        assert_eq!(change.additions, Some(2));
    }

    #[tokio::test]
    async fn edit_file_reports_diff() {
        let tmp = test_root();
        std::fs::write(tmp.join("a.txt"), "hello world").unwrap();
        let output = EditFileTool
            .execute(
                json!({ "path": "a.txt", "old": "hello", "new": "goodbye" }),
                &ctx(&tmp),
            )
            .await
            .unwrap();
        assert_eq!(output.file_changes.len(), 1);
        let change = &output.file_changes[0];
        assert_eq!(change.change, "edit");
        assert!(change.diff.as_deref().unwrap_or("").contains("goodbye"));
    }

    #[tokio::test]
    async fn apply_patch_updates_multiple_files_and_creates_new() {
        let tmp = test_root();
        std::fs::write(tmp.join("a.txt"), "one\ntwo\n").unwrap();
        let patch = "--- a/a.txt\n+++ b/a.txt\n@@ -1,2 +1,2 @@\n one\n-two\n+TWO\n--- /dev/null\n+++ b/b.txt\n@@ -0,0 +1,2 @@\n+hello\n+world";
        let output = ApplyPatchTool
            .execute(json!({ "patch": patch }), &ctx(&tmp))
            .await
            .unwrap();
        assert_eq!(output.file_changes.len(), 2);
        assert_eq!(
            std::fs::read_to_string(tmp.join("a.txt")).unwrap(),
            "one\nTWO\n"
        );
        assert_eq!(
            std::fs::read_to_string(tmp.join("b.txt")).unwrap(),
            "hello\nworld"
        );
    }
}
