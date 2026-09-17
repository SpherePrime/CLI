use std::collections::HashMap;

use agent_events::{AgentEvent, AgentEventPayload, AgentScope, EventBus};
use agent_model::ToolSchema;
use anyhow::{anyhow, Context, Result};
use serde::{Deserialize, Serialize};
use tokio::io::{AsyncBufReadExt, AsyncWriteExt};
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
    pub description: String,
    pub input_schema: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpResource {
    pub uri: String,
    pub name: String,
    pub mime_type: String,
    pub content: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpPrompt {
    pub name: String,
    pub description: String,
    pub arguments: HashMap<String, String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpCallResult {
    pub content: Vec<McpContent>,
    pub is_error: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum McpContent {
    Text(String),
    Resource { uri: String, mime_type: String, text: String },
    Unknown(serde_json::Value),
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
    pub fn stdio(name: &str, command: &str, args: Vec<String>, env: HashMap<String, String>) -> Self {
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
                Ok(())
            }
        }
    }

    async fn connect_stdio(&mut self) -> Result<()> {
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

    pub async fn list_tools(&self) -> Result<Vec<McpTool>> {
        Ok(vec![])
    }

    pub async fn call_tool(
        &self,
        _name: &str,
        _args: serde_json::Value,
    ) -> Result<McpCallResult> {
        Ok(McpCallResult {
            content: vec![McpContent::Text("stub".into())],
            is_error: false,
        })
    }

    pub async fn list_resources(&self) -> Result<Vec<McpResource>> {
        Ok(vec![])
    }

    pub async fn list_prompts(&self) -> Result<Vec<McpPrompt>> {
        Ok(vec![])
    }

    pub fn tools_to_schema(&self) -> Vec<ToolSchema> {
        Vec::new()
    }

    pub async fn disconnect(&mut self) {
        if let Some(child_mutex) = self.child.take() {
            if let Some(mut child) = child_mutex.into_inner() {
                let _ = child.kill().await;
            }
        }
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

    pub async fn add_stdio_server(
        &mut self,
        name: &str,
        command: &str,
        args: Vec<String>,
        env: HashMap<String, String>,
    ) -> Result<()> {
        let mut s = McpServer::stdio(name, command, args, env);
        s.connect().await?;
        self.servers.push(s);
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

    pub fn all_tools(&self) -> Vec<McpTool> {
        Vec::new()
    }

    pub fn schema(&self) -> Vec<ToolSchema> {
        let mut out = Vec::new();
        for s in &self.servers {
            out.extend(s.tools_to_schema());
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
        let _ = c
            .add_stdio_server("test-server", "echo", vec!["hello".into()], HashMap::new())
            .await
            .is_err();
        assert_eq!(c.servers.len(), 1);
    }

    #[tokio::test]
    async fn mcp_client_schema() {
        let c = McpClient::new();
        assert!(c.schema().is_empty());
    }
}
