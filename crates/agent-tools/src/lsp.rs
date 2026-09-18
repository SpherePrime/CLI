use std::process::Stdio;

use anyhow::{Context, Result};
use serde_json::{json, Value};
use tokio::io::{AsyncBufRead, AsyncReadExt, AsyncWriteExt, BufReader};
use tokio::process::{Child, ChildStdin, Command as TokioCommand};
use tokio::sync::mpsc;

pub struct LspConnection {
    child: Child,
    stdin: Option<ChildStdin>,
    incoming: mpsc::UnboundedReceiver<Value>,
    next_id: u64,
}

impl Drop for LspConnection {
    fn drop(&mut self) {
        let _ = self.child.start_kill();
    }
}

impl LspConnection {
    pub async fn connect(server: &str, args: &[String], root_uri: &str) -> Result<Self> {
        let mut child = TokioCommand::new(server)
            .args(args)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()
            .with_context(|| format!("spawning language server `{server}`"))?;

        let stdout = child
            .stdout
            .take()
            .context("language server stdout missing")?;
        let stdin = child.stdin.take();

        let (tx, rx) = mpsc::unbounded_channel();
        tokio::spawn(async move {
            let mut reader = BufReader::new(stdout);
            while let Ok(Some(frame)) = read_frame(&mut reader).await {
                if tx.send(frame).is_err() {
                    break;
                }
            }
        });

        let mut connection = Self {
            child,
            stdin,
            incoming: rx,
            next_id: 1,
        };
        connection
            .request(
                "initialize",
                json!({
                    "processId": null,
                    "rootUri": root_uri,
                    "capabilities": {
                        "textDocument": {
                            "definition": { "linkSupport": false },
                            "references": {},
                            "documentSymbol": { "hierarchicalDocumentSymbolSupport": true },
                            "rename": { "prepareSupport": false }
                        },
                        "workspace": { "workspaceFolders": true }
                    }
                }),
            )
            .await
            .context("initialize request failed")?;
        connection.notify("initialized", json!({})).await?;
        Ok(connection)
    }

    pub async fn open(&mut self, uri: &str, text: &str) -> Result<()> {
        self.notify(
            "textDocument/didOpen",
            json!({
                "textDocument": { "uri": uri, "languageId": "plaintext", "version": 1, "text": text }
            }),
        )
        .await
    }

    pub async fn request(&mut self, method: &str, params: Value) -> Result<Value> {
        let id = self.next_id;
        self.next_id += 1;
        self.write_frame(
            &json!({ "jsonrpc": "2.0", "id": id, "method": method, "params": params }),
        )
        .await?;
        loop {
            let frame = self
                .incoming
                .recv()
                .await
                .context("language server closed the connection while waiting for a reply")?;
            if frame.get("id").and_then(Value::as_u64) == Some(id) {
                if let Some(error) = frame.get("error") {
                    let message = error["message"].as_str().unwrap_or("lsp error");
                    anyhow::bail!("lsp request {method} failed: {message}");
                }
                return Ok(frame.get("result").cloned().unwrap_or(Value::Null));
            }
        }
    }

    pub async fn notify(&mut self, method: &str, params: Value) -> Result<()> {
        self.write_frame(&json!({ "jsonrpc": "2.0", "method": method, "params": params }))
            .await
    }

    pub async fn collect_diagnostics(
        &mut self,
        uri: &str,
        timeout: std::time::Duration,
    ) -> Vec<Value> {
        let deadline = tokio::time::Instant::now() + timeout;
        let mut diagnostics = Vec::new();
        while tokio::time::Instant::now() < deadline {
            let remaining = deadline.saturating_duration_since(tokio::time::Instant::now());
            match tokio::time::timeout(remaining, self.incoming.recv()).await {
                Ok(Some(frame)) => {
                    if frame.get("method").and_then(Value::as_str)
                        == Some("textDocument/publishDiagnostics")
                        && frame["params"]["uri"].as_str() == Some(uri)
                    {
                        if let Some(items) = frame["params"]["diagnostics"].as_array() {
                            diagnostics.extend(items.iter().cloned());
                        }
                    }
                }
                Ok(None) | Err(_) => break,
            }
        }
        diagnostics
    }

    pub async fn close(&mut self) {
        let request = json!({ "jsonrpc": "2.0", "id": self.next_id, "method": "shutdown", "params": Value::Null });
        self.next_id += 1;
        let _ = self.write_frame(&request).await;
        let _ =
            tokio::time::timeout(std::time::Duration::from_millis(200), self.incoming.recv()).await;
        let _ = self.notify("exit", Value::Null).await;
    }

    async fn write_frame(&mut self, value: &Value) -> Result<()> {
        let Some(mut stdin) = self.stdin.take() else {
            anyhow::bail!("language server stdin is closed");
        };
        let body = serde_json::to_vec(value).unwrap_or_default();
        let header = format!("Content-Length: {}\r\n\r\n", body.len());
        stdin
            .write_all(header.as_bytes())
            .await
            .context("writing lsp header")?;
        stdin.write_all(&body).await.context("writing lsp body")?;
        stdin.flush().await.context("flushing lsp stdin")?;
        self.stdin = Some(stdin);
        Ok(())
    }
}

async fn read_line(reader: &mut (impl AsyncBufRead + Unpin)) -> std::io::Result<Option<String>> {
    let mut buf = Vec::new();
    let mut byte = [0u8; 1];
    loop {
        let read = reader.read(&mut byte).await?;
        if read == 0 {
            if buf.is_empty() {
                return Ok(None);
            }
            return Ok(Some(String::from_utf8_lossy(&buf).into_owned()));
        }
        if byte[0] == b'\n' {
            if buf.last() == Some(&b'\r') {
                buf.pop();
            }
            return Ok(Some(String::from_utf8_lossy(&buf).into_owned()));
        }
        buf.push(byte[0]);
    }
}

async fn read_frame(reader: &mut (impl AsyncBufRead + Unpin)) -> Result<Option<Value>> {
    let mut length: Option<usize> = None;
    loop {
        let line = read_line(reader).await.context("reading lsp header")?;
        let Some(line) = line else {
            return Ok(None);
        };
        let line = line.trim_end();
        if line.is_empty() {
            break;
        }
        if let Some(rest) = line.strip_prefix("Content-Length:") {
            length = rest.trim().parse::<usize>().ok();
        }
    }
    let Some(length) = length else {
        return Ok(None);
    };
    let mut body = vec![0u8; length];
    reader
        .read_exact(&mut body)
        .await
        .context("reading lsp body")?;
    serde_json::from_slice(&body)
        .map(Some)
        .with_context(|| "invalid lsp json frame")
}

pub fn path_to_uri(path: &std::path::Path) -> String {
    let absolute = std::fs::canonicalize(path).unwrap_or_else(|_| path.to_path_buf());
    let raw = absolute.to_string_lossy().replace('\\', "/");
    let encoded = raw
        .split('/')
        .map(|part| {
            part.chars()
                .map(|c| {
                    if c.is_ascii_alphanumeric() || matches!(c, '-' | '_' | '.' | '~' | ':') {
                        c.to_string()
                    } else {
                        format!("%{:02X}", c as u32)
                    }
                })
                .collect::<String>()
        })
        .collect::<Vec<_>>()
        .join("/");
    format!("file://{}", encoded)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn encodes_file_uris() {
        let uri = path_to_uri(std::path::Path::new("C:/work/My App/src/main.rs"));
        assert!(uri.starts_with("file://"));
        assert!(uri.contains("main.rs"));
        assert!(uri.contains("%20"));
    }

    #[tokio::test]
    async fn graceful_fallback_for_missing_server() {
        let result = LspConnection::connect(
            "definitely-not-a-real-language-server-xyz",
            &[],
            "file:///tmp",
        )
        .await;
        assert!(result.is_err());
        assert!(result
            .err()
            .unwrap()
            .to_string()
            .contains("definitely-not-a-real-language-server-xyz"));
    }
}
