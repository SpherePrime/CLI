use std::collections::HashMap;
use std::ops::RangeInclusive;

use anyhow::{Context, Result};
use async_trait::async_trait;
use serde_json::{json, Value};

use agent_filesystem::read_file;

use crate::builtin::paths::{display, resolve_path, truncate};
use crate::executor::{
    ToolArtifact, ToolArtifactKind, ToolExecutionContext, ToolExecutor, ToolOutput,
};

const MAX_OUTPUT: usize = 262_144;
const MAX_FILES: usize = 50;

fn schema(properties: Value, required: &[&str]) -> Value {
    json!({
        "type": "object",
        "properties": properties,
        "required": required,
        "additionalProperties": false,
    })
}

pub struct ReadManyFilesTool;

fn parse_ranges(
    args: &Value,
    working_dir: &std::path::Path,
) -> Result<HashMap<String, RangeInclusive<usize>>> {
    let mut ranges = HashMap::new();
    let Some(list) = args.get("ranges").and_then(|v| v.as_array()) else {
        return Ok(ranges);
    };
    for item in list {
        let Some(path) = item.get("path").and_then(|v| v.as_str()) else {
            continue;
        };
        let resolved = resolve_path(working_dir, path)?;
        let key = display(&resolved);
        if let (Some(start), Some(end)) = (
            item.get("start").and_then(|v| v.as_u64()),
            item.get("end").and_then(|v| v.as_u64()),
        ) {
            if start >= 1 && end >= start {
                ranges.insert(key, start as usize..=end as usize);
            }
        }
    }
    Ok(ranges)
}

fn slice_lines(content: &str, range: &RangeInclusive<usize>) -> String {
    let lines: Vec<&str> = content.split('\n').collect();
    let start = (*range.start()).saturating_sub(1);
    let end = (*range.end()).min(lines.len());
    if start >= end {
        return String::new();
    }
    lines[start..end].join("\n")
}

#[async_trait]
impl ToolExecutor for ReadManyFilesTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let paths = args
            .get("paths")
            .and_then(|v| v.as_array())
            .context("paths is required")?;
        if paths.is_empty() {
            return Ok(ToolOutput::success("(no paths given)".into()));
        }
        let ranges = parse_ranges(&args, &ctx.working_dir)?;
        let mut out = String::new();
        let mut read = 0usize;
        let mut skipped: Vec<String> = Vec::new();
        let mut artifacts = Vec::new();
        for raw in paths.iter().take(MAX_FILES) {
            let Some(raw) = raw.as_str() else {
                continue;
            };
            let path = resolve_path(&ctx.working_dir, raw)?;
            let key = display(&path);
            match read_file(&path).await {
                Ok(content) => {
                    match ranges.get(&key) {
                        Some(range) => {
                            out.push_str(&format!(
                                "==> {key}:lines {}-{} <==\n",
                                range.start(),
                                range.end()
                            ));
                            out.push_str(&slice_lines(&content, range));
                            out.push('\n');
                        }
                        None => {
                            out.push_str(&format!("==> {key} <==\n"));
                            out.push_str(&content);
                            if !content.ends_with('\n') {
                                out.push('\n');
                            }
                        }
                    }
                    out.push('\n');
                    read += 1;
                    artifacts.push(ToolArtifact {
                        path: key,
                        kind: ToolArtifactKind::Read,
                    });
                }
                Err(error) => skipped.push(format!("{key}: {error}")),
            }
        }
        if paths.len() > MAX_FILES {
            out.push_str(&format!(
                "...[{} more paths omitted]\n",
                paths.len() - MAX_FILES
            ));
        }
        if !skipped.is_empty() {
            out.push_str("\n-- skipped --\n");
            out.push_str(&skipped.join("\n"));
        }
        let summary = format!("read {read} file(s)");
        Ok(ToolOutput::success(truncate(&out, MAX_OUTPUT))
            .summary(summary)
            .artifacts(artifacts))
    }
}

pub fn read_many_files_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "read_many_files".into(),
        description: "Read several files at once, optionally restricted to 1-based line ranges. Each file is prefixed with a '==> path <==' header.".into(),
        input_schema: schema(
            json!({
                "paths": {
                    "type": "array",
                    "items": { "type": "string" },
                    "description": "Relative file paths"
                },
                "ranges": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "properties": {
                            "path": { "type": "string" },
                            "start": { "type": "integer", "minimum": 1 },
                            "end": { "type": "integer", "minimum": 1 }
                        },
                        "required": ["path", "start", "end"]
                    },
                    "description": "Optional 1-based line ranges per path"
                }
            }),
            &["paths"],
        ),
        executor: std::sync::Arc::new(ReadManyFilesTool),
        permissions: agent_permissions::PermissionScope::Read,
        timeout_secs: 60,
    }
}

pub struct ViewImageTool;

#[async_trait]
impl ToolExecutor for ViewImageTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let raw = args
            .get("path")
            .and_then(|v| v.as_str())
            .context("path is required")?;
        let path = resolve_path(&ctx.working_dir, raw)?;
        if !path.is_file() {
            anyhow::bail!("not a file: {}", display(&path));
        }
        let mime = image_mime(&path).with_context(|| {
            format!(
                "unsupported image type: {}",
                path.extension()
                    .map(|e| e.to_string_lossy().to_string())
                    .unwrap_or_default()
            )
        })?;
        let size = std::fs::metadata(&path).map(|m| m.len()).unwrap_or(0);
        let value = json!({
            "path": display(&path),
            "mime": mime,
            "bytes": size,
        });
        Ok(ToolOutput::success(value.to_string())
            .summary(format!("view {mime} image ({size} bytes)")))
    }
}

fn image_mime(path: &std::path::Path) -> Option<&'static str> {
    let ext = path
        .extension()
        .map(|value| value.to_string_lossy().to_ascii_lowercase())?;
    match ext.as_str() {
        "png" => Some("image/png"),
        "jpg" | "jpeg" => Some("image/jpeg"),
        "gif" => Some("image/gif"),
        "webp" => Some("image/webp"),
        "bmp" => Some("image/bmp"),
        "svg" => Some("image/svg+xml"),
        _ => None,
    }
}

pub fn view_image_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "view_image".into(),
        description:
            "Inspect an image file (path, mime type and size) inside the working directory.".into(),
        input_schema: schema(json!({ "path": { "type": "string" } }), &["path"]),
        executor: std::sync::Arc::new(ViewImageTool),
        permissions: agent_permissions::PermissionScope::Read,
        timeout_secs: 30,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use agent_config::PermissionsConfig;
    use agent_permissions::PermissionEngine;

    fn ctx(root: &std::path::Path) -> ToolExecutionContext {
        ToolExecutionContext::new(
            uuid::Uuid::new_v4(),
            root.to_path_buf(),
            PermissionEngine::new(PermissionsConfig::default()),
        )
    }

    #[tokio::test]
    async fn reads_multiple_files_with_headers() {
        let dir = std::env::temp_dir().join(format!("agent-inspect-{}", uuid::Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        std::fs::write(dir.join("a.txt"), "alpha").unwrap();
        std::fs::write(dir.join("b.txt"), "beta").unwrap();

        let output = ReadManyFilesTool
            .execute(json!({ "paths": ["a.txt", "b.txt"] }), &ctx(&dir))
            .await
            .unwrap();
        assert!(output.content.contains("==> "));
        assert!(output.content.contains("alpha"));
        assert!(output.content.contains("beta"));
        assert_eq!(output.artifacts.len(), 2);
        assert!(output
            .artifacts
            .iter()
            .all(|a| { a.path.ends_with("a.txt") || a.path.ends_with("b.txt") }));
    }

    #[tokio::test]
    async fn read_many_files_honors_line_ranges() {
        let dir = std::env::temp_dir().join(format!("agent-inspect-{}", uuid::Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        std::fs::write(dir.join("a.txt"), "one\ntwo\nthree\nfour").unwrap();

        let output = ReadManyFilesTool
            .execute(
                json!({
                    "paths": ["a.txt"],
                    "ranges": [{ "path": "a.txt", "start": 2, "end": 3 }]
                }),
                &ctx(&dir),
            )
            .await
            .unwrap();
        assert!(output.content.contains(":lines 2-3 <=="));
        assert!(output.content.contains("two"));
        assert!(output.content.contains("three"));
        assert!(!output.content.contains("one"));
        assert!(!output.content.contains("four"));
    }

    #[tokio::test]
    async fn read_many_files_omits_ranges_for_other_files() {
        let dir = std::env::temp_dir().join(format!("agent-inspect-{}", uuid::Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        std::fs::write(dir.join("a.txt"), "one\ntwo\nthree\nfour").unwrap();
        std::fs::write(dir.join("b.txt"), "full").unwrap();

        let output = ReadManyFilesTool
            .execute(
                json!({
                    "paths": ["a.txt", "b.txt"],
                    "ranges": [{ "path": "a.txt", "start": 2, "end": 3 }]
                }),
                &ctx(&dir),
            )
            .await
            .unwrap();
        assert!(output.content.contains(":lines 2-3 <=="));
        assert!(output.content.contains("==> "));
        assert!(output.content.contains("full"));
    }

    #[tokio::test]
    async fn view_image_reports_mime() {
        let dir = std::env::temp_dir().join(format!("agent-image-{}", uuid::Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        std::fs::write(dir.join("pic.png"), b"fake").unwrap();
        let output = ViewImageTool
            .execute(json!({ "path": "pic.png" }), &ctx(&dir))
            .await
            .unwrap();
        assert!(output.content.contains("image/png"));
    }
}
