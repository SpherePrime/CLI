use std::collections::HashMap;
use std::sync::atomic::{AtomicU64, Ordering as AtomicOrdering};
use std::sync::Arc;

use agent_events::{AgentEvent, AgentEventPayload, AgentScope, EventBus};
use agent_model::ToolSchema;
use anyhow::{anyhow, Context, Result};
use serde::{Deserialize, Serialize};
use tokio::io::{AsyncBufReadExt, AsyncWriteExt, BufReader};
use tokio::process::{Child, Command};
use tokio::sync::mpsc;
use tokio::sync::Mutex;
use tokio_util::sync::CancellationToken;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum McpTransport {
    Stdio,
    HttpSse,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpTool {
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default, rename = "inputSchema")]
    pub input_schema: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpResource {
    pub uri: String,
    pub name: String,
    #[serde(default)]
    pub mime_type: String,
    #[serde(default)]
    pub content: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpPrompt {
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub arguments: HashMap<String, String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpCallResult {
    pub content: Vec<McpContent>,
    #[serde(default)]
    pub is_error: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum McpContent {
    Text(String),
    Resource {
        uri: String,
        mime_type: String,
        text: String,
    },
    Unknown(serde_json::Value),
}

impl McpContent {
    fn from_value(value: &serde_json::Value) -> Self {
        let kind = value.get("type").and_then(|t| t.as_str()).unwrap_or("");
        match kind {
            "text" => McpContent::Text(
                value
                    .get("text")
                    .and_then(|t| t.as_str())
                    .unwrap_or_default()
                    .to_string(),
            ),
            "resource" => McpContent::Resource {
                uri: value
                    .get("uri")
                    .and_then(|u| u.as_str())
                    .unwrap_or_default()
                    .to_string(),
                mime_type: value
                    .get("mimeType")
                    .and_then(|m| m.as_str())
                    .unwrap_or_default()
                    .to_string(),
                text: value
                    .get("text")
                    .and_then(|t| t.as_str())
                    .unwrap_or_default()
                    .to_string(),
            },
            _ => McpContent::Unknown(value.clone()),
        }
    }

    pub fn as_text(&self) -> String {
        match self {
            McpContent::Text(text) => text.clone(),
            McpContent::Resource { text, .. } => text.clone(),
            McpContent::Unknown(value) => value.to_string(),
        }
    }
}

struct HttpState {
    client: reqwest::Client,
    base_url: String,
    session_id: tokio::sync::Mutex<Option<String>>,
    next_id: AtomicU64,
    pending: Arc<tokio::sync::Mutex<HashMap<u64, mpsc::UnboundedSender<serde_json::Value>>>>,
    stream_handle: tokio::sync::Mutex<Option<Arc<CancellationToken>>>,
}

pub struct McpServer {
    pub name: String,
    transport: McpTransport,
    command: Option<String>,
    args: Vec<String>,
    url: Option<String>,
    env: HashMap<String, String>,
    child: Option<Mutex<Option<Child>>>,
    http: Option<HttpState>,
}

impl McpServer {
    pub fn stdio(
        name: &str,
        command: &str,
        args: Vec<String>,
        env: HashMap<String, String>,
    ) -> Self {
        Self {
            name: name.to_string(),
            transport: McpTransport::Stdio,
            command: Some(command.to_string()),
            args,
            url: None,
            env,
            child: None,
            http: None,
        }
    }

    pub fn http(name: &str, url: &str) -> Self {
        Self {
            name: name.to_string(),
            transport: McpTransport::HttpSse,
            command: None,
            args: vec![],
            url: Some(url.to_string()),
            env: HashMap::new(),
            child: None,
            http: None,
        }
    }

    pub async fn connect(&mut self) -> Result<()> {
        match self.transport {
            McpTransport::Stdio => self.connect_stdio().await,
            McpTransport::HttpSse => {
                let url = self
                    .url
                    .clone()
                    .ok_or_else(|| anyhow!("HTTP transport requires url"))?;
                self.connect_http(&url).await
            }
        }
    }

    async fn connect_stdio(&mut self) -> Result<()> {
        if self.child.is_some() {
            return Ok(());
        }
        let cmd = self
            .command
            .clone()
            .ok_or_else(|| anyhow!("stdio transport requires command"))?;
        let mut c = Command::new(cmd.clone());
        c.args(&self.args);
        for (k, v) in &self.env {
            c.env(k, v);
        }
        c.stdin(std::process::Stdio::piped());
        c.stdout(std::process::Stdio::piped());
        c.stderr(std::process::Stdio::piped());
        let child = c
            .spawn()
            .with_context(|| format!("spawning MCP server {cmd}"))?;
        self.child = Some(Mutex::new(Some(child)));
        Ok(())
    }

    async fn rpc_call(&self, method: &str, params: serde_json::Value) -> Result<serde_json::Value> {
        match self.transport {
            McpTransport::HttpSse => self.rpc_http(method, params).await,
            McpTransport::Stdio => {
                let mut guard = self
                    .child
                    .as_ref()
                    .context("MCP server is not connected")?
                    .lock()
                    .await;
                let mut option = guard.as_mut();
                let child = option.as_mut().context("MCP server process is gone")?;
                let stdin = child
                    .stdin
                    .as_mut()
                    .context("MCP server stdin unavailable")?;
                let stdout = child
                    .stdout
                    .as_mut()
                    .context("MCP server stdout unavailable")?;

                let request = serde_json::json!({
                    "jsonrpc": "2.0",
                    "id": 1,
                    "method": method,
                    "params": params,
                });
                let mut payload = serde_json::to_vec(&request)?;
                payload.push(b'\n');

                stdin.write_all(&payload).await?;
                stdin.flush().await?;

                let mut reader = BufReader::new(stdout);
                let mut line = String::new();
                reader.read_line(&mut line).await?;
                let value: serde_json::Value = serde_json::from_str(line.trim())
                    .with_context(|| format!("invalid json-rpc response: {line}"))?;
                if let Some(error) = value.get("error").filter(|e| !e.is_null()) {
                    return Err(anyhow!("MCP error: {error}"));
                }
                value
                    .get("result")
                    .cloned()
                    .filter(|r| !r.is_null())
                    .context("MCP response missing result")
            }
        }
    }

    async fn connect_http(&mut self, url: &str) -> Result<()> {
        if self.http.is_some() {
            return Ok(());
        }
        let client = reqwest::Client::builder()
            .connect_timeout(std::time::Duration::from_secs(10))
            .build()
            .context("building http client")?;
        let (session_id, _) = self
            .http_post(
                &client,
                url,
                None,
                "initialize",
                &self.initialize_params(),
                0,
            )
            .await?;
        let _ = self
            .http_post(
                &client,
                url,
                session_id.as_deref(),
                "notifications/initialized",
                &serde_json::json!({}),
                1,
            )
            .await;
        self.http = Some(HttpState {
            client,
            base_url: url.to_string(),
            session_id: Mutex::new(session_id.clone()),
            next_id: AtomicU64::new(2),
            pending: Arc::new(Mutex::new(HashMap::new())),
            stream_handle: Mutex::new(None),
        });
        if session_id.is_some() {
            self.start_http_event_stream().await;
        }
        Ok(())
    }

    async fn start_http_event_stream(&mut self) {
        let Some(state) = self.http.as_ref() else {
            return;
        };
        let Some(session_id) = state.session_id.lock().await.clone() else {
            return;
        };
        let cancel = CancellationToken::new();
        let handle = Arc::new(cancel.clone());
        *state.stream_handle.lock().await = Some(handle);
        let pending = Arc::clone(&state.pending);
        let client = state.client.clone();
        let base_url = state.base_url.clone();
        let session_id_header = session_id.clone();

        tokio::spawn(async move {
            loop {
                let response = match client
                    .get(&base_url)
                    .header("Accept", "text/event-stream")
                    .header("Mcp-Session-Id", &session_id_header)
                    .send()
                    .await
                {
                    Ok(response) => response,
                    Err(_) => return,
                };
                if !response.status().is_success() {
                    return;
                }
                let mut stream = response.bytes_stream();
                use futures::StreamExt;
                let mut buffer = String::new();
                while let Some(chunk) = stream.next().await {
                    if cancel.is_cancelled() {
                        return;
                    }
                    let chunk = match chunk {
                        Ok(chunk) => chunk,
                        Err(_) => return,
                    };
                    buffer.push_str(&String::from_utf8_lossy(&chunk));
                    while let Some(pos) = buffer.find('\n') {
                        let line = buffer[..pos].trim_end_matches('\r').to_string();
                        buffer.drain(..pos + 1);
                        let Some(data) = line.strip_prefix("data:") else {
                            continue;
                        };
                        let data = data.trim();
                        if data.is_empty() {
                            continue;
                        }
                        let Ok(value) = serde_json::from_str::<serde_json::Value>(data) else {
                            continue;
                        };
                        if let Some(id) = value.get("id").and_then(|id| id.as_u64()) {
                            let mut pending_guard = pending.lock().await;
                            if let Some(tx) = pending_guard.remove(&id) {
                                let _ = tx.send(value);
                                continue;
                            }
                        }
                    }
                }
                if cancel.is_cancelled() {
                    return;
                }
                tokio::time::sleep(std::time::Duration::from_secs(2)).await;
            }
        });
    }

    fn initialize_params(&self) -> serde_json::Value {
        serde_json::json!({
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": { "name": "agent", "version": env!("CARGO_PKG_VERSION") }
        })
    }

    async fn http_post(
        &self,
        client: &reqwest::Client,
        url: &str,
        session_id: Option<&str>,
        method: &str,
        params: &serde_json::Value,
        id: u64,
    ) -> Result<(Option<String>, serde_json::Value)> {
        let request = serde_json::json!({
            "jsonrpc": "2.0",
            "id": id,
            "method": method,
            "params": params,
        });
        let mut request_builder = client
            .post(url)
            .header("Accept", "application/json, text/event-stream")
            .header("Content-Type", "application/json")
            .body(serde_json::to_vec(&request)?);
        if let Some(session) = session_id {
            request_builder = request_builder.header("Mcp-Session-Id", session);
        }
        let response = request_builder
            .send()
            .await
            .with_context(|| format!("POST {url}"))?;
        let new_session = response
            .headers()
            .get("mcp-session-id")
            .and_then(|value| value.to_str().ok())
            .map(|value| value.to_string());
        let content_type = response
            .headers()
            .get("content-type")
            .and_then(|value| value.to_str().ok())
            .map(|value| value.to_string());
        let body = response
            .text()
            .await
            .context("reading MCP http response body")?;
        let result = parse_http_response(&body, content_type.as_deref(), id)?;
        Ok((new_session, result))
    }

    async fn rpc_http(&self, method: &str, params: serde_json::Value) -> Result<serde_json::Value> {
        let state = self.http.as_ref().context("MCP server is not connected")?;
        let current = state.session_id.lock().await.clone();
        let id = state.next_id.fetch_add(1, AtomicOrdering::SeqCst);
        let has_event_stream = state.stream_handle.lock().await.is_some();
        if has_event_stream && !method.starts_with("notifications/") {
            let (tx, mut rx) = mpsc::unbounded_channel::<serde_json::Value>();
            state.pending.lock().await.insert(id, tx);
            let timeout_secs: u64 = 120;
            let result =
                tokio::time::timeout(std::time::Duration::from_secs(timeout_secs), async {
                    let body = self
                        .http_post(
                            &state.client,
                            &state.base_url,
                            current.as_deref(),
                            method,
                            &params,
                            id,
                        )
                        .await
                        .ok()
                        .map(|(_, value)| value);
                    if let Some(value) = body {
                        state.pending.lock().await.remove(&id);
                        return Ok(value);
                    }
                    match rx.recv().await {
                        Some(value) => {
                            state.pending.lock().await.remove(&id);
                            Ok(value)
                        }
                        None => Err(anyhow!(
                            "MCP http event stream closed before response for {method}"
                        )),
                    }
                })
                .await
                .map_err(|_| {
                    anyhow!("MCP http request {method} timed out after {timeout_secs}s")
                })??;
            return Ok(result);
        }
        let (session_id, result) = self
            .http_post(
                &state.client,
                &state.base_url,
                current.as_deref(),
                method,
                &params,
                id,
            )
            .await?;
        if let Some(new_session) = session_id {
            *state.session_id.lock().await = Some(new_session);
        }
        Ok(result)
    }

    pub async fn list_tools(&self) -> Result<Vec<McpTool>> {
        let result = self.rpc_call("tools/list", serde_json::json!({})).await?;
        let tools = result.get("tools").cloned().unwrap_or_default();
        parse_tools(tools)
    }

    pub async fn call_tool(&self, name: &str, args: serde_json::Value) -> Result<McpCallResult> {
        let result = self
            .rpc_call(
                "tools/call",
                serde_json::json!({ "name": name, "arguments": args }),
            )
            .await?;
        let mut content = Vec::new();
        if let Some(items) = result.get("content").and_then(|c| c.as_array()) {
            content = items.iter().map(McpContent::from_value).collect();
        }
        Ok(McpCallResult {
            content,
            is_error: result
                .get("isError")
                .and_then(|v| v.as_bool())
                .unwrap_or(false),
        })
    }

    pub async fn list_resources(&self) -> Result<Vec<McpResource>> {
        let result = self
            .rpc_call("resources/list", serde_json::json!({}))
            .await?;
        let resources = result.get("resources").cloned().unwrap_or_default();
        parse_resources(resources)
    }

    pub async fn list_prompts(&self) -> Result<Vec<McpPrompt>> {
        let result = self.rpc_call("prompts/list", serde_json::json!({})).await?;
        let prompts = result.get("prompts").cloned().unwrap_or_default();
        parse_prompts(prompts)
    }

    pub async fn tools_to_schema(&self) -> Vec<ToolSchema> {
        if self.child.is_none() && self.http.is_none() {
            return Vec::new();
        }
        self.list_tools()
            .await
            .unwrap_or_default()
            .into_iter()
            .map(tool_schema)
            .collect()
    }

    pub async fn disconnect(&mut self) {
        if let Some(child_mutex) = self.child.take() {
            if let Some(mut child) = child_mutex.into_inner() {
                let _ = child.kill().await;
                let _ = child.wait().await;
            }
        }
        if let Some(state) = self.http.take() {
            if let Some(cancel) = state.stream_handle.lock().await.take() {
                cancel.cancel();
            }
        }
    }
}

fn parse_tools(value: serde_json::Value) -> Result<Vec<McpTool>> {
    serde_json::from_value(value).context("failed to parse tools/list result")
}

fn parse_resources(value: serde_json::Value) -> Result<Vec<McpResource>> {
    serde_json::from_value(value).context("failed to parse resources/list result")
}

fn parse_prompts(value: serde_json::Value) -> Result<Vec<McpPrompt>> {
    serde_json::from_value(value).context("failed to parse prompts/list result")
}

fn parse_http_response(
    body: &str,
    content_type: Option<&str>,
    wanted_id: u64,
) -> Result<serde_json::Value> {
    let is_sse = content_type
        .map(|ct| ct.contains("text/event-stream"))
        .unwrap_or(false);
    if !is_sse {
        return parse_http_json(body);
    }
    let mut last_result: Option<serde_json::Value> = None;
    for block in body.split("\r\n\r\n") {
        for data in block.lines().filter_map(|line| line.strip_prefix("data: ")) {
            let data = data.strip_prefix(' ').unwrap_or(data);
            let Ok(value) = serde_json::from_str::<serde_json::Value>(data) else {
                continue;
            };
            if let Some(error) = value.get("error").filter(|e| !e.is_null()) {
                return Err(anyhow!("MCP error: {error}"));
            }
            if value.get("method").is_some() {
                continue;
            }
            if value.get("id").and_then(|id| id.as_u64()) != Some(wanted_id) {
                continue;
            }
            if value.get("result").is_some() {
                last_result = value.get("result").cloned();
            }
        }
    }
    last_result.context("MCP http response missing a matching result in the event stream")
}

fn parse_http_json(body: &str) -> Result<serde_json::Value> {
    let value: serde_json::Value =
        serde_json::from_str(body).with_context(|| format!("invalid json body: {body}"))?;
    if let Some(error) = value.get("error").filter(|e| !e.is_null()) {
        anyhow::bail!("MCP error: {error}");
    }
    if let Some(result) = value.get("result") {
        return Ok(result.clone());
    }
    anyhow::bail!("MCP http response is missing result: {value}")
}

fn tool_schema(tool: McpTool) -> ToolSchema {
    ToolSchema {
        name: tool.name,
        description: tool.description,
        input_schema: tool.input_schema,
    }
}

pub struct McpClient {
    pub servers: Vec<McpServer>,
    events: EventBus,
}

impl McpClient {
    pub fn new() -> Self {
        Self {
            servers: Vec::new(),
            events: EventBus::new(),
        }
    }

    pub fn with_events(mut self, events: EventBus) -> Self {
        self.events = events;
        self
    }

    pub fn add_stdio_server(
        &mut self,
        name: &str,
        command: &str,
        args: Vec<String>,
        env: HashMap<String, String>,
    ) -> Result<()> {
        self.servers
            .push(McpServer::stdio(name, command, args, env));
        Ok(())
    }

    pub fn add_http_server(&mut self, name: &str, url: &str) -> Result<()> {
        self.servers.push(McpServer::http(name, url));
        Ok(())
    }

    pub async fn connect(&mut self, name: &str) -> Result<()> {
        let server = self
            .server_mut(name)
            .ok_or_else(|| anyhow!("unknown MCP server {name}"))?;
        server.connect().await?;
        self.events.dispatch(
            AgentEventPayload::new(AgentEvent::McpConnected)
                .with_scope(AgentScope::Mcp)
                .with_detail(serde_json::json!({ "server": name })),
        );
        Ok(())
    }

    pub async fn connect_all(&mut self) -> Result<()> {
        for s in &mut self.servers {
            s.connect().await?;
            self.events.dispatch(
                AgentEventPayload::new(AgentEvent::McpConnected)
                    .with_scope(AgentScope::Mcp)
                    .with_detail(serde_json::json!({ "server": s.name })),
            );
        }
        Ok(())
    }

    pub async fn disconnect_all(&mut self) {
        for s in &mut self.servers {
            s.disconnect().await;
            self.events.dispatch(
                AgentEventPayload::new(AgentEvent::McpDisconnected)
                    .with_scope(AgentScope::Mcp)
                    .with_detail(serde_json::json!({ "server": s.name })),
            );
        }
    }

    pub fn server(&self, name: &str) -> Option<&McpServer> {
        self.servers.iter().find(|s| s.name == name)
    }

    pub fn server_mut(&mut self, name: &str) -> Option<&mut McpServer> {
        self.servers.iter_mut().find(|s| s.name == name)
    }

    pub async fn all_tools(&self) -> Vec<McpTool> {
        let mut out = Vec::new();
        for s in &self.servers {
            if let Ok(tools) = s.list_tools().await {
                out.extend(tools);
            }
        }
        out
    }

    pub async fn schema(&self) -> Vec<ToolSchema> {
        let mut out = Vec::new();
        for s in &self.servers {
            out.extend(s.tools_to_schema().await);
        }
        out
    }
}

impl Default for McpClient {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait::async_trait]
pub trait McpTransporter: Send + Sync + 'static {
    async fn name(&self) -> &str;
    async fn discover(&self) -> Result<Vec<McpTool>>;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn mcp_client_add() {
        let mut c = McpClient::new();
        c.add_stdio_server("test-server", "echo", vec!["hello".into()], HashMap::new())
            .unwrap();
        assert_eq!(c.servers.len(), 1);
        assert_eq!(
            c.server("test-server").map(|s| s.name.as_str()),
            Some("test-server")
        );
    }

    #[tokio::test]
    async fn mcp_client_schema() {
        let c = McpClient::new();
        assert!(c.schema().await.is_empty());
    }

    #[tokio::test]
    async fn mcp_content_text() {
        assert_eq!(McpContent::Text("hi".into()).as_text(), "hi");
        assert!(crate::McpContent::Resource {
            uri: "u".into(),
            mime_type: "t".into(),
            text: "x".into()
        }
        .as_text()
        .eq("x"));
    }

    #[test]
    fn parses_sse_aggregated_response() {
        let body = concat!(
            "event: message\r\n",
            "data: {\"jsonrpc\":\"2.0\",\"id\":7,\"result\":{\"tools\":[{\"name\":\"echo\"}]}}\r\n",
            "\r\n\r\n",
        );
        let result = parse_http_response(body, Some("text/event-stream"), 7).unwrap();
        assert_eq!(result["tools"][0]["name"].as_str(), Some("echo"));
    }

    #[test]
    fn sse_ignores_unrelated_ids() {
        let body = "data: {\"jsonrpc\":\"2.0\",\"id\":3,\"result\":4}\r\n\r\n\
                    data: {\"jsonrpc\":\"2.0\",\"id\":9,\"result\":5}\r\n\r\n";
        assert!(parse_http_response(body, Some("text/event-stream"), 9)
            .unwrap()
            .eq(&serde_json::json!(5)));
    }

    #[tokio::test]
    async fn http_transport_roundtrip() {
        use tokio::io::{AsyncBufReadExt, AsyncWriteExt};
        use tokio::net::TcpListener;

        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = listener.local_addr().unwrap();

        tokio::spawn(async move {
            loop {
                let (socket, _) = listener.accept().await.unwrap();
                tokio::spawn(async move {
                    let (mut reader, mut writer) = socket.into_split();
                    loop {
                        let mut lines = BufReader::new(&mut reader);
                        let first = lines.read_line(&mut String::new()).await.unwrap_or(0);
                        if first == 0 {
                            return;
                        }
                        let mut length = 0usize;
                        loop {
                            let mut line = String::new();
                            if lines.read_line(&mut line).await.unwrap() == 0 {
                                return;
                            }
                            if line.trim().is_empty() {
                                break;
                            }
                            let lower = line.trim_end().to_ascii_lowercase();
                            if let Some(rest) = lower.strip_prefix("content-length:") {
                                length = rest.trim().parse().unwrap_or(0);
                            }
                        }
                        use tokio::io::AsyncReadExt;
                        let body = if length > 0 {
                            let mut body = vec![0u8; length];
                            lines.read_exact(&mut body).await.unwrap();
                            body
                        } else {
                            Vec::new()
                        };
                        let request: serde_json::Value =
                            serde_json::from_slice(&body).unwrap_or_default();
                        let method = request["method"].as_str().unwrap_or("");
                        let id = request["id"].as_u64().unwrap_or(0);

                        let response = if method == "initialize" {
                            serde_json::json!({
                                "jsonrpc": "2.0",
                                "id": id,
                                "result": {
                                    "protocolVersion": "2024-11-05",
                                    "capabilities": {},
                                    "serverInfo": { "name": "test-server", "version": "1.0" }
                                }
                            })
                        } else if method == "tools/list" {
                            serde_json::json!({
                                "jsonrpc": "2.0",
                                "id": id,
                                "result": {
                                    "tools": [
                                        { "name": "fetch", "description": "fetch a url", "inputSchema": { "type": "object" } }
                                    ]
                                }
                            })
                        } else {
                            serde_json::json!({
                                "jsonrpc": "2.0",
                                "id": id,
                                "result": serde_json::Value::Null
                            })
                        };
                        let payload = serde_json::to_vec(&response).unwrap();
                        let session_header = if method == "initialize" {
                            "Mcp-Session-Id: abc-123\r\n"
                        } else {
                            ""
                        };
                        let frame = format!(
                            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n{session_header}Content-Length: {}\r\n\r\n",
                            payload.len()
                        );
                        writer.write_all(frame.as_bytes()).await.unwrap();
                        writer.write_all(&payload).await.unwrap();
                        writer.flush().await.unwrap();
                    }
                });
            }
        });

        let mut server = McpServer::http("remote", &format!("http://{address}"));
        server.connect().await.unwrap();
        let tools = server.list_tools().await.unwrap();
        assert_eq!(tools.len(), 1);
        assert_eq!(tools[0].name, "fetch");
        server.disconnect().await;
    }
}
