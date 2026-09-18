use std::path::Path;

use anyhow::{bail, Context, Result};
use async_trait::async_trait;
use serde_json::{json, Value};

use agent_filesystem::snapshot;
use agent_filesystem::{find_files, list_directory, read_file, write_file, FileEdit};

use crate::builtin::paths::{display, resolve_path, truncate};
use crate::executor::{ToolExecutionContext, ToolExecutor, ToolOutput};

const MAX_OUTPUT: usize = 262_144;
const MAX_READ_LINES: usize = 2000;

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
        if path.exists() {
            let _ = snapshot::take_snapshot(&ctx.working_dir, &path);
        }
        write_file(&path, content).await?;
        Ok(ToolOutput::success(format!(
            "wrote {} ({} bytes)",
            display(&path),
            content.len()
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
        let _ = snapshot::take_snapshot(&ctx.working_dir, &path);
        let edit = FileEdit::search_replace(&path, old, new);
        let updated = edit.apply(None).await?;
        let diff = edit.diff(&original, &updated);
        Ok(ToolOutput::success(truncate(&diff, MAX_OUTPUT)))
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
        let _ = snapshot::take_snapshot(&ctx.working_dir, &path);
        let edit = FileEdit::patch(&path, patch);
        edit.apply(None).await?;
        Ok(ToolOutput::success(format!("patched {}", display(&path))))
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
            let _ = snapshot::take_snapshot(&ctx.working_dir, &path);
            tokio::fs::remove_file(&path).await?;
        }
        Ok(ToolOutput::success(format!("deleted {}", display(&path))))
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
