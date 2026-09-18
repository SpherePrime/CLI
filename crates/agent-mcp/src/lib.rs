use std::collections::HashMap;

use agent_events::{AgentEvent, AgentEventPayload, AgentScope, EventBus};
use agent_model::ToolSchema;
use anyhow::{anyhow, Context, Result};
use serde::{Deserialize, Serialize};
use tokio::io::{AsyncBufReadExt, AsyncWriteExt, BufReader};
use tokio::process::{Child, Command};
use tokio::sync::Mutex;

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

pub struct McpServer {
    pub name: String,
    transport: McpTransport,
    command: Option<String>,
    args: Vec<String>,
    url: Option<String>,
    env: HashMap<String, String>,
    child: Option<Mutex<Option<Child>>>,
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
        }
    }

    pub async fn connect(&mut self) -> Result<()> {
        match self.transport {
            McpTransport::Stdio => self.connect_stdio().await,
            McpTransport::HttpSse => {
                if self.url.is_none() {
                    return Err(anyhow!("HTTP transport requires url"));
                }
                Err(anyhow!("HTTP SSE transport is not implemented yet"))
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
            McpTransport::HttpSse => Err(anyhow!("HTTP SSE transport is not implemented yet")),
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
        if self.child.is_none() {
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
}
