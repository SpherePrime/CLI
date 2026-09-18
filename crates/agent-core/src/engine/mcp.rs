use std::sync::Arc;

use agent_mcp::McpClient;
use agent_permissions::PermissionScope;
use agent_tools::{ToolDefinition, ToolExecutionContext, ToolExecutor, ToolOutput, ToolRegistry};
use anyhow::{Context, Result};
use async_trait::async_trait;

pub struct McpToolExecutor {
    client: Arc<tokio::sync::Mutex<McpClient>>,
    server: String,
    tool: String,
}

impl McpToolExecutor {
    pub fn new(client: Arc<tokio::sync::Mutex<McpClient>>, server: String, tool: String) -> Self {
        Self {
            client,
            server,
            tool,
        }
    }
}

#[async_trait]
impl ToolExecutor for McpToolExecutor {
    async fn execute(
        &self,
        args: serde_json::Value,
        _ctx: &ToolExecutionContext,
    ) -> Result<ToolOutput> {
        let client = self.client.lock().await;
        let server = client
            .server(&self.server)
            .context("mcp server not found")?;
        let result = server.call_tool(&self.tool, args).await?;
        let text = result
            .content
            .iter()
            .map(|content| content.as_text())
            .collect::<Vec<_>>()
            .join("\n");
        if result.is_error {
            Ok(ToolOutput::failure(text))
        } else {
            Ok(ToolOutput::success(text))
        }
    }
}

pub fn build_client(
    config: &agent_config::McpConfig,
) -> Option<Arc<tokio::sync::Mutex<McpClient>>> {
    let mut client = McpClient::new();
    for (name, server) in config.servers.iter().filter(|(_, server)| server.enabled) {
        if let Some(command) = &server.command {
            let _ = client.add_stdio_server(name, command, server.args.clone(), server.env.clone());
        }
    }
    if client.servers.is_empty() {
        None
    } else {
        Some(Arc::new(tokio::sync::Mutex::new(client)))
    }
}

pub async fn register_server_tools(
    registry: &mut ToolRegistry,
    client: Arc<tokio::sync::Mutex<McpClient>>,
) {
    let mut client_guard = client.lock().await;
    for server in &mut client_guard.servers {
        let server_name = server.name.clone();
        if let Err(error) = server.connect().await {
            tracing::warn!(server = %server_name, error = %error, "failed to connect to MCP server");
            continue;
        }
        let tools = match server.list_tools().await {
            Ok(tools) => tools,
            Err(error) => {
                tracing::warn!(server = %server_name, error = %error, "failed to list MCP tools");
                continue;
            }
        };
        for tool in tools {
            let prefixed = format!("{server_name}_{}", tool.name);
            tracing::info!(tool = %prefixed, "registering MCP tool");
            registry.add(ToolDefinition {
                name: prefixed,
                description: tool.description,
                input_schema: tool.input_schema,
                executor: Arc::new(McpToolExecutor::new(
                    client.clone(),
                    server_name.clone(),
                    tool.name,
                )),
                permissions: PermissionScope::Write,
                timeout_secs: 60,
            });
        }
    }
}
