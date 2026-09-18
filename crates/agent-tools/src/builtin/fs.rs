use std::path::{Path, PathBuf};

use anyhow::{bail, Context, Result};
use async_trait::async_trait;
use serde_json::{json, Value};

use agent_filesystem::snapshot;
use agent_filesystem::{
    apply_unified_patch, find_files, list_directory, read_file, write_file, FileEdit,
};

use crate::builtin::paths::{display, resolve_path, truncate};
use crate::executor::{
    FileChange, ToolArtifact, ToolArtifactKind, ToolExecutionContext, ToolExecutor, ToolOutput,
};

fn artifact(path: &Path, kind: ToolArtifactKind) -> ToolArtifact {
    ToolArtifact {
        path: display(path),
        kind,
    }
}

const MAX_OUTPUT: usize = 262_144;
const MAX_READ_LINES: usize = 2000;

fn snapshot_before_change(ctx: &ToolExecutionContext, path: &Path) {
    if path.exists() {
        let _ = snapshot::take_snapshot_turn(&ctx.working_dir, path, ctx.turn_id.as_deref());
    } else {
        let _ = snapshot::record_new_file(&ctx.working_dir, path, ctx.turn_id.as_deref());
    }
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
        Ok(ToolOutput::success(truncate(&out, MAX_OUTPUT))
            .artifact(artifact(&path, ToolArtifactKind::Read)))
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
        .file_change(change)
        .artifact(artifact(
            &path,
            if existed {
                ToolArtifactKind::Modified
            } else {
                ToolArtifactKind::Created
            },
        )))
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
        Ok(ToolOutput::success(truncate(&diff, MAX_OUTPUT))
            .file_change(change)
            .artifact(artifact(&path, ToolArtifactKind::Modified)))
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
        Ok(ToolOutput::success(format!("patched {}", display(&path)))
            .file_change(change)
            .artifact(artifact(&path, ToolArtifactKind::Modified)))
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
        let dry_run = args
            .get("dry_run")
            .and_then(|v| v.as_bool())
            .unwrap_or(false);
        let sections = split_patch_sections(patch);
        if sections.is_empty() {
            bail!("patch has no file headers (+++)");
        }
        let mut changes = Vec::new();
        let mut artifacts = Vec::new();
        let mut summary = Vec::new();
        let mut written: Vec<(PathBuf, String)> = Vec::new();
        let run = async {
            for (target, section) in sections {
                if target.is_empty() || target == "/dev/null" {
                    continue;
                }
                let path = resolve_path(&ctx.working_dir, &target)?;
                let existed = path.exists();
                let original = if existed {
                    if !dry_run {
                        snapshot_before_change(ctx, &path);
                    }
                    tokio::fs::read_to_string(&path)
                        .await
                        .with_context(|| format!("reading {target}"))?
                } else {
                    if !dry_run {
                        let _ = snapshot::record_new_file(
                            &ctx.working_dir,
                            &path,
                            ctx.turn_id.as_deref(),
                        );
                    }
                    String::new()
                };
                let updated = apply_unified_patch(&original, &section)
                    .with_context(|| format!("applying patch to {target}"))?;
                if !dry_run {
                    if let Some(parent) = path.parent() {
                        if !parent.as_os_str().is_empty() {
                            tokio::fs::create_dir_all(parent)
                                .await
                                .with_context(|| format!("creating parent dirs for {target}"))?;
                        }
                    }
                    tokio::fs::write(&path, &updated)
                        .await
                        .with_context(|| format!("writing {target}"))?;
                    written.push((path.clone(), original.clone()));
                }
                changes.push(FileChange {
                    path: display(&path),
                    change: {
                        if dry_run {
                            "preview"
                        } else if existed {
                            "patch"
                        } else {
                            "created"
                        }
                    }
                    .into(),
                    diff: if dry_run {
                        Some(FileEdit::patch(&path, "").diff(&original, &updated))
                    } else {
                        None
                    },
                    additions: None,
                    deletions: None,
                });
                if !dry_run {
                    artifacts.push(artifact(
                        &path,
                        if existed {
                            ToolArtifactKind::Modified
                        } else {
                            ToolArtifactKind::Created
                        },
                    ));
                }
                summary.push(format!(
                    "{} {}",
                    if dry_run {
                        "would patch"
                    } else if existed {
                        "patched"
                    } else {
                        "created"
                    },
                    display(&path)
                ));
            }
            Ok::<(), anyhow::Error>(())
        };
        if let Err(error) = run.await {
            for (revert_path, content) in &written {
                let _ = tokio::fs::write(revert_path, content).await;
            }
            return Err(error);
        }
        if changes.is_empty() {
            bail!("patch produced no changes");
        }
        Ok(ToolOutput::success(summary.join("\n"))
            .file_changes(changes)
            .artifacts(artifacts))
    }
}

pub fn apply_patch_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "apply_patch".into(),
        description:
            "Apply a multi-file unified diff patch. Each file section starts with --- a/path and +++ b/path followed by @@ hunks; new files use --- /dev/null. Set dry_run=true to preview diffs without writing."
                .into(),
        input_schema: schema(
            json!({
                "patch": { "type": "string", "description": "Unified diff covering one or more files" },
                "dry_run": { "type": "boolean", "description": "Preview diffs without changing files (default false)" }
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
        Ok(ToolOutput::success(format!("deleted {}", display(&path)))
            .file_change(change)
            .artifact(artifact(&path, ToolArtifactKind::Deleted)))
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
        assert_eq!(output.artifacts.len(), 1);
        assert!(output.artifacts[0].path.ends_with("a.txt"));
        assert_eq!(output.artifacts[0].kind, ToolArtifactKind::Created);
    }

    #[tokio::test]
    async fn edit_file_reports_diff_and_artifact() {
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
        assert_eq!(output.artifacts[0].kind, ToolArtifactKind::Modified);
    }

    #[tokio::test]
    async fn delete_path_reports_deleted_artifact() {
        let tmp = test_root();
        std::fs::write(tmp.join("gone.txt"), "bye").unwrap();
        let output = DeletePathTool
            .execute(json!({ "path": "gone.txt" }), &ctx(&tmp))
            .await
            .unwrap();
        assert_eq!(output.artifacts[0].kind, ToolArtifactKind::Deleted);
        assert!(!tmp.join("gone.txt").exists());
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

    #[tokio::test]
    async fn apply_patch_dry_run_does_not_modify_files() {
        let tmp = test_root();
        std::fs::write(tmp.join("a.txt"), "one\n").unwrap();
        let patch = "--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-one\n+ONE";
        let output = ApplyPatchTool
            .execute(json!({ "patch": patch, "dry_run": true }), &ctx(&tmp))
            .await
            .unwrap();
        assert!(output.content.contains("would patch"));
        assert_eq!(output.file_changes.len(), 1);
        assert_eq!(output.file_changes[0].change, "preview");
        assert!(output.file_changes[0]
            .diff
            .as_deref()
            .unwrap_or("")
            .contains("ONE"));
        assert_eq!(std::fs::read_to_string(tmp.join("a.txt")).unwrap(), "one\n");
    }

    #[tokio::test]
    async fn apply_patch_rolls_back_earlier_files_on_error() {
        let tmp = test_root();
        std::fs::write(tmp.join("a.txt"), "one\n").unwrap();
        std::fs::write(tmp.join("b.txt"), "keep\n").unwrap();
        let patch = "\
--- a/a.txt
+++ b/a.txt
@@ -1 +1 @@
-one
+ONE-A
--- a/b.txt
+++ b/b.txt
@@ -1,2 +1,2 @@
-keep
-missing
+ONE-B";
        let result = ApplyPatchTool
            .execute(json!({ "patch": patch }), &ctx(&tmp))
            .await;
        assert!(result.is_err(), "second hunk must fail to apply");
        assert_eq!(
            std::fs::read_to_string(tmp.join("a.txt")).unwrap(),
            "one\n",
            "a.txt must be rolled back to its original content"
        );
        assert_eq!(
            std::fs::read_to_string(tmp.join("b.txt")).unwrap(),
            "keep\n",
            "b.txt must be untouched"
        );
    }
}
