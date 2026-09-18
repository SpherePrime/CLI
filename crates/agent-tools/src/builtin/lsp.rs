use std::path::{Path, PathBuf};
use std::time::Duration;

use anyhow::{Context, Result};
use async_trait::async_trait;
use serde_json::{json, Value};

use crate::builtin::paths::resolve_path;
use crate::executor::{ToolExecutionContext, ToolExecutor, ToolOutput};
use crate::lsp::{path_to_uri, LspConnection};

const DIAGNOSTIC_WAIT: Duration = Duration::from_millis(1_500);

struct Server {
    command: String,
    args: Vec<String>,
}

fn server_for(path: &Path) -> Option<Server> {
    let ext = path
        .extension()
        .and_then(|ext| ext.to_str())
        .unwrap_or("")
        .to_ascii_lowercase();
    let (command, args): (&str, &[&str]) = match ext.as_str() {
        "rs" => ("rust-analyzer", &[]),
        "go" => ("gopls", &[]),
        "py" => ("pylsp", &[]),
        "ts" | "tsx" | "js" | "jsx" | "mjs" | "cjs" | "mts" | "cts" => {
            ("typescript-language-server", &["--stdio"])
        }
        "c" | "h" | "cpp" | "hpp" | "cc" | "cxx" => ("clangd", &[]),
        "swift" => ("sourcekit-lsp", &[]),
        "kt" | "kts" => ("kotlin-language-server", &[]),
        "yaml" | "yml" => ("yaml-language-server", &["--stdio"]),
        "json" | "jsonc" => ("vscode-json-language-server", &["--stdio"]),
        "css" | "scss" | "less" => ("vscode-css-language-server", &["--stdio"]),
        _ => return None,
    };
    Some(Server {
        command: command.to_string(),
        args: args.iter().map(|arg| arg.to_string()).collect(),
    })
}

fn binary_available(command: &str) -> bool {
    std::process::Command::new(command)
        .arg("--version")
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .status()
        .is_ok()
}

fn line_character(args: &Value) -> (u32, u32) {
    (
        args.get("line").and_then(Value::as_u64).unwrap_or(0) as u32,
        args.get("character").and_then(Value::as_u64).unwrap_or(0) as u32,
    )
}

fn format_diagnostics(items: &[Value]) -> String {
    if items.is_empty() {
        return "(no diagnostics)".to_string();
    }
    let mut lines = Vec::new();
    for item in items {
        let severity = match item["severity"].as_u64().unwrap_or(1) {
            1 => "error",
            2 => "warning",
            3 => "info",
            _ => "hint",
        };
        let code = item["code"].as_str().unwrap_or("");
        let message = item["message"].as_str().unwrap_or("");
        lines.push(format!(
            "{severity} [{}:{}] {code}: {message}",
            item["range"]["start"]["line"].as_u64().unwrap_or(0),
            item["range"]["start"]["character"].as_u64().unwrap_or(0),
        ));
    }
    lines.join("\n")
}

fn symbol_kind(kind: u64) -> &'static str {
    match kind {
        2 => "module",
        3 => "namespace",
        4 => "package",
        5 => "class",
        6 => "method",
        7 => "property",
        8 => "field",
        9 => "constructor",
        10 => "enum",
        11 => "interface",
        12 => "function",
        13 => "variable",
        14 => "constant",
        15 => "string",
        16 => "number",
        17 => "boolean",
        18 => "array",
        19 => "object",
        20 => "key",
        21 => "null",
        22 => "enum member",
        23 => "struct",
        24 => "event",
        25 => "operator",
        26 => "type parameter",
        _ => "symbol",
    }
}

fn collect_symbols(value: &Value, out: &mut Vec<String>) {
    let Some(symbols) = value.as_array() else {
        return;
    };
    for symbol in symbols {
        let name = symbol["name"].as_str().unwrap_or("?");
        let kind = symbol_kind(symbol["kind"].as_u64().unwrap_or(0));
        let line = symbol["location"]["range"]["start"]["line"]
            .as_u64()
            .or_else(|| symbol["range"]["start"]["line"].as_u64())
            .unwrap_or(0);
        let entry = format!("{name} ({kind}) line {line}");
        out.push(entry);
        if let Some(children) = symbol["children"].as_array() {
            for child in children {
                collect_children(child, out);
            }
        }
    }
}

fn collect_children(symbol: &Value, out: &mut Vec<String>) {
    let name = symbol["name"].as_str().unwrap_or("?");
    let kind = symbol_kind(symbol["kind"].as_u64().unwrap_or(0));
    let line = symbol["range"]["start"]["line"].as_u64().unwrap_or(0);
    out.push(format!("  {name} ({kind}) line {line}"));
    if let Some(children) = symbol["children"].as_array() {
        for child in children {
            collect_children(child, out);
        }
    }
}

fn format_locations(value: &Value, limit: usize) -> String {
    let locations = value
        .as_array()
        .map(|items| items.iter().collect::<Vec<_>>())
        .unwrap_or_else(|| vec![&value]);
    let locations = locations
        .iter()
        .filter_map(|item| {
            let uri = item
                .get("uri")
                .or_else(|| item.get("targetUri"))
                .and_then(Value::as_str)?;
            let range = item
                .get("range")
                .or_else(|| item.get("targetSelectionRange"));
            let start = range.and_then(|r| r.get("start"))?;
            Some(format!(
                "{}:{}:{}",
                uri,
                start["line"].as_u64().unwrap_or(0),
                start["character"].as_u64().unwrap_or(0)
            ))
        })
        .collect::<Vec<_>>();
    if locations.is_empty() {
        return "(no locations)".to_string();
    }
    let mut body: Vec<String> = Vec::new();
    for (index, location) in locations.iter().enumerate() {
        if index >= limit {
            body.push(format!("... and {} more", locations.len() - limit));
            break;
        }
        body.push(location.clone());
    }
    body.join("\n")
}

pub struct LspTool;

#[async_trait]
impl ToolExecutor for LspTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let action = args
            .get("action")
            .and_then(|v| v.as_str())
            .context("action is required")?;
        let file = args
            .get("file")
            .and_then(|v| v.as_str())
            .context("file is required")?;
        let path = resolve_path(&ctx.working_dir, file)?;

        let Some(server) = server_for(&path) else {
            return Ok(ToolOutput::success(format!(
                "no language server configured for {}",
                path.extension().and_then(|ext| ext.to_str()).unwrap_or("")
            )));
        };
        if !binary_available(&server.command) {
            return Ok(ToolOutput::success(format!(
                "language server `{}` is not installed; install it to enable lsp {}",
                server.command, action
            )));
        }

        let uri = path_to_uri(&path);
        let root_uri = path_to_uri(&ctx.working_dir);
        let mut connection = LspConnection::connect(&server.command, &server.args, &root_uri)
            .await
            .context("connecting to language server")?;

        let result = self
            .run_action(action, &args, &path, &uri, &mut connection)
            .await;
        let outcome = match &result {
            Ok(output) => output.clone(),
            Err(error) => ToolOutput::failure(error.to_string()),
        };
        connection.close().await;
        result.map(|_| outcome)
    }
}

impl LspTool {
    async fn run_action(
        &self,
        action: &str,
        args: &Value,
        path: &Path,
        uri: &str,
        connection: &mut LspConnection,
    ) -> Result<ToolOutput> {
        match action {
            "diagnostics" => {
                let text = std::fs::read_to_string(path)
                    .with_context(|| format!("reading {}", path.display()))?;
                connection.open(uri, &text).await?;
                let items = connection
                    .collect_diagnostics(uri, DIAGNOSTIC_WAIT)
                    .await;
                Ok(ToolOutput::success(format_diagnostics(&items)))
            }
            "document_symbols" => {
                let result = connection
                    .request(
                        "textDocument/documentSymbol",
                        json!({ "textDocument": { "uri": uri } }),
                    )
                    .await?;
                let mut symbols = Vec::new();
                collect_symbols(&result, &mut symbols);
                let body = if symbols.is_empty() {
                    "(no symbols)".to_string()
                } else {
                    symbols.join("\n")
                };
                Ok(ToolOutput::success(body))
            }
            "workspace_symbols" => {
                let query = args.get("query").and_then(Value::as_str).unwrap_or("");
                let result = connection
                    .request(
                        "workspace/symbol",
                        json!({ "query": query }),
                    )
                    .await?;
                let body = if result.as_array().is_some_and(|arr| arr.is_empty()) {
                    "(no symbols)".to_string()
                } else {
                    format_locations(&result, 50)
                };
                Ok(ToolOutput::success(body))
            }
            "definition" => {
                let (line, character) = line_character(args);
                let result = connection
                    .request(
                        "textDocument/definition",
                        json!({ "textDocument": { "uri": uri }, "position": { "line": line, "character": character } }),
                    )
                    .await?;
                Ok(ToolOutput::success(format_locations(&result, 10)))
            }
            "references" => {
                let (line, character) = line_character(args);
                let result = connection
                    .request(
                        "textDocument/references",
                        json!({ "textDocument": { "uri": uri }, "position": { "line": line, "character": character }, "context": { "includeDeclaration": false } }),
                    )
                    .await?;
                Ok(ToolOutput::success(format_locations(&result, 30)))
            }
            "rename" => {
                let new_name = args
                    .get("new_name")
                    .and_then(Value::as_str)
                    .context("new_name is required for rename")?;
                let (line, character) = line_character(args);
                let result = connection
                    .request(
                        "textDocument/rename",
                        json!({ "textDocument": { "uri": uri }, "position": { "line": line, "character": character }, "newName": new_name }),
                    )
                    .await?;
                let edits = apply_workspace_edit(&result)?;
                Ok(ToolOutput::success(format!(
                    "renamed to `{new_name}`: {edits}"
                )))
            }
            _ => Err(anyhow::anyhow!(
                "unknown action `{action}` (diagnostics, document_symbols, workspace_symbols, definition, references, rename)"
            )),
        }
    }
}

fn apply_text_edits(path: &Path, edits: &[Value]) -> Result<()> {
    let mut text =
        std::fs::read_to_string(path).with_context(|| format!("reading {}", path.display()))?;
    let mut offset_edits = Vec::new();
    for edit in edits {
        let Some(start_line) = edit["range"]["start"]["line"].as_u64() else {
            continue;
        };
        let Some(start_char) = edit["range"]["start"]["character"].as_u64() else {
            continue;
        };
        let Some(end_line) = edit["range"]["end"]["line"].as_u64() else {
            continue;
        };
        let Some(end_char) = edit["range"]["end"]["character"].as_u64() else {
            continue;
        };
        let Some(new_text) = edit["newText"].as_str() else {
            continue;
        };
        let start = locate_char(&text, start_line, start_char)?;
        let end = locate_char(&text, end_line, end_char)?;
        offset_edits.push((start, end, new_text.to_string()));
    }
    offset_edits.sort_by_key(|(_, end, _)| *end);
    let mut applied = 0;
    for (start, end, new_text) in offset_edits {
        if start > text.len() || end > text.len() || start > end {
            anyhow::bail!("invalid edit range in {path:?}");
        }
        text.replace_range(start..end, &new_text);
        applied += 1;
    }
    if applied > 0 {
        std::fs::write(path, text).with_context(|| format!("writing {}", path.display()))?;
    }
    Ok(())
}

fn locate_char(text: &str, line: u64, character: u64) -> Result<usize> {
    let mut current_line = 0u64;
    for (index, byte) in text.bytes().enumerate() {
        if current_line == line && byte == b'\n' {
            return Err(anyhow::anyhow!("line {line} is empty"));
        }
        if current_line == line {
            let line_text = text[index..].split('\n').next().unwrap_or("");
            let byte_offset = line_text
                .char_indices()
                .nth(character as usize)
                .map(|(offset, _)| offset)
                .unwrap_or(line_text.len());
            return Ok(index + byte_offset);
        }
        if byte == b'\n' {
            current_line += 1;
        }
    }
    Err(anyhow::anyhow!("line {line} not found"))
}

fn apply_workspace_edit(edit: &Value) -> Result<usize> {
    let mut applied = 0;
    if let Some(changes) = edit.get("changes").and_then(Value::as_object) {
        for (uri, text_edits) in changes {
            let path = uri_to_path(uri)?;
            let edits = text_edits.as_array().cloned().unwrap_or_default();
            apply_text_edits(&path, &edits)?;
            applied += edits.len();
        }
    }
    if let Some(document_changes) = edit.get("documentChanges").and_then(Value::as_array) {
        for change in document_changes {
            if let Some(uri) = change["textDocument"]["uri"].as_str() {
                let path = uri_to_path(uri)?;
                let edits = change
                    .get("edits")
                    .and_then(Value::as_array)
                    .cloned()
                    .unwrap_or_default();
                apply_text_edits(&path, &edits)?;
                applied += edits.len();
            }
        }
    }
    Ok(applied)
}

fn uri_to_path(uri: &str) -> Result<PathBuf> {
    let path = uri
        .strip_prefix("file://")
        .context("expected a file:// uri")?;
    let path = path.strip_prefix('/').unwrap_or(path);
    let path = path.replace('/', path_separator_str());
    Ok(PathBuf::from(percent_decode(&path)))
}

fn path_separator_str() -> &'static str {
    if cfg!(windows) {
        "\\"
    } else {
        "/"
    }
}

fn percent_decode(input: &str) -> String {
    let bytes = input.as_bytes();
    let mut out = Vec::with_capacity(bytes.len());
    let mut index = 0;
    while index < bytes.len() {
        if bytes[index] == b'%' && index + 2 < bytes.len() {
            if let Ok(byte) = u8::from_str_radix(&input[index + 1..index + 3], 16) {
                out.push(byte);
                index += 3;
                continue;
            }
        }
        out.push(bytes[index]);
        index += 1;
    }
    String::from_utf8_lossy(&out).into_owned()
}

pub fn lsp_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "lsp".into(),
        description: "Language server integration (read-only). Actions: diagnostics (syntax/semantic problems), document_symbols (symbols in a file), workspace_symbols (search symbols by query), definition (definition location for a symbol at line/character), references (usages of a symbol at line/character). Gracefully reports when the language server is not installed.".into(),
        input_schema: json!({
            "type": "object",
            "properties": {
                "action": { "type": "string", "enum": ["diagnostics", "document_symbols", "workspace_symbols", "definition", "references"] },
                "file": { "type": "string" },
                "query": { "type": "string" },
                "line": { "type": "integer" },
                "character": { "type": "integer" }
            },
            "required": ["action", "file"],
            "additionalProperties": false,
        }),
        executor: std::sync::Arc::new(LspTool),
        permissions: agent_permissions::PermissionScope::Read,
        timeout_secs: 30,
    }
}

pub struct RenameTool;

#[async_trait]
impl ToolExecutor for RenameTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let mut rename_args = args.clone();
        rename_args["action"] = json!("rename");
        LspTool.execute(rename_args, ctx).await
    }
}

pub fn lsp_rename_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "lsp_rename".into(),
        description: "Rename a symbol across the project via the language server. Finds the symbol at file/line/character and applies the workspace edits on disk. Requires write permission. Gracefully reports when the language server is not installed.".into(),
        input_schema: json!({
            "type": "object",
            "properties": {
                "file": { "type": "string" },
                "line": { "type": "integer" },
                "character": { "type": "integer" },
                "new_name": { "type": "string" }
            },
            "required": ["file", "new_name"],
            "additionalProperties": false,
        }),
        executor: std::sync::Arc::new(RenameTool),
        permissions: agent_permissions::PermissionScope::Write,
        timeout_secs: 30,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detects_server_by_extension() {
        assert_eq!(
            server_for(Path::new("src/main.rs")).unwrap().command,
            "rust-analyzer"
        );
        assert_eq!(
            server_for(Path::new("app.tsx")).unwrap().command,
            "typescript-language-server"
        );
        assert!(server_for(Path::new("README.md")).is_none());
    }

    #[test]
    fn decodes_uris() {
        let uri = "file:///C:/work/My%20App/src/edit.rs";
        #[cfg(windows)]
        assert_eq!(
            uri_to_path(uri).unwrap(),
            Path::new("C:\\work\\My App\\src\\edit.rs")
        );
        #[cfg(not(windows))]
        assert_eq!(
            uri_to_path(uri).unwrap(),
            Path::new("C:/work/My App/src/edit.rs")
        );
    }

    #[test]
    fn locates_char_positions() {
        let text = "aa\nbb\ncc";
        assert_eq!(locate_char(text, 1, 1).unwrap(), 4);
        assert!(locate_char(text, 5, 0).is_err());
    }

    #[test]
    fn applies_range_edits() {
        let path = std::env::temp_dir().join("lsp_edit_test.txt");
        std::fs::write(&path, "hello world").unwrap();
        let edit = json!({
            "range": {
                "start": { "line": 0, "character": 0 },
                "end": { "line": 0, "character": 5 }
            },
            "newText": "hola"
        });
        apply_text_edits(&path, std::slice::from_ref(&edit)).unwrap();
        let result = std::fs::read_to_string(&path).unwrap();
        std::fs::remove_file(&path).ok();
        assert_eq!(result, "hola world");
    }
}
