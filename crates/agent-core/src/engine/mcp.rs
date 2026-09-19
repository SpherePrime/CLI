use std::sync::Arc;

use agent_mcp::McpClient;
use agent_permissions::PermissionScope;
use agent_tools::{
    ToolArtifact, ToolArtifactKind, ToolDefinition, ToolExecutionContext, ToolExecutor, ToolOutput,
    ToolRegistry,
};
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
        let artifacts = result
            .content
            .iter()
            .filter_map(|item| {
                if let agent_mcp::McpContent::Resource { uri, .. } = item {
                    Some(ToolArtifact {
                        path: uri.clone(),
                        kind: ToolArtifactKind::Read,
                    })
                } else {
                    None
                }
            })
            .collect();
        if result.is_error {
            let mut out = ToolOutput::failure(text);
            out.artifacts = artifacts;
            Ok(out)
        } else {
            let mut out = ToolOutput::success(text);
            out.artifacts = artifacts;
            Ok(out)
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
        } else if let Some(url) = &server.url {
            let _ = client.add_http_server(name, url);
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
        if let Err(error) = server.list_resources().await {
            tracing::warn!(server = %server_name, error = %error, "failed to list MCP resources");
        } else {
            registry.add(ToolDefinition {
                name: format!("{server_name}_resources"),
                description: format!("List resources exposed by MCP server '{server_name}'"),
                input_schema: serde_json::json!({
                    "type": "object",
                    "properties": {},
                    "additionalProperties": false
                }),
                executor: Arc::new(McpResourceExecutor::new(
                    client.clone(),
                    server_name.clone(),
                )),
                permissions: PermissionScope::Read,
                timeout_secs: 60,
            });
        }
        if let Err(error) = server.list_prompts().await {
            tracing::warn!(server = %server_name, error = %error, "failed to list MCP prompts");
        } else {
            registry.add(ToolDefinition {
                name: format!("{server_name}_prompts"),
                description: format!("List prompts exposed by MCP server '{server_name}'"),
                input_schema: serde_json::json!({
                    "type": "object",
                    "properties": {},
                    "additionalProperties": false
                }),
                executor: Arc::new(McpPromptExecutor::new(client.clone(), server_name.clone())),
                permissions: PermissionScope::Read,
                timeout_secs: 60,
            });
        }
    }
}

pub struct McpResourceExecutor {
    client: Arc<tokio::sync::Mutex<McpClient>>,
    server: String,
}

impl McpResourceExecutor {
    pub fn new(client: Arc<tokio::sync::Mutex<McpClient>>, server: String) -> Self {
        Self { client, server }
    }
}

#[async_trait]
impl ToolExecutor for McpResourceExecutor {
    async fn execute(
        &self,
        _args: serde_json::Value,
        _ctx: &ToolExecutionContext,
    ) -> Result<ToolOutput> {
        let client = self.client.lock().await;
        let server = client
            .server(&self.server)
            .context("mcp server not found")?;
        let resources = server.list_resources().await?;
        let body = resources
            .iter()
            .map(|resource| {
                let mut line = format!(
                    "{} {} ({})",
                    resource.uri, resource.name, resource.mime_type
                );
                if !resource.content.is_empty() {
                    line = format!("{line}: {}", resource.content);
                }
                line
            })
            .collect::<Vec<_>>()
            .join("\n");
        if body.trim().is_empty() {
            Ok(ToolOutput::success("no resources exposed".into()))
        } else {
            Ok(ToolOutput::success(body))
        }
    }
}

pub struct McpPromptExecutor {
    client: Arc<tokio::sync::Mutex<McpClient>>,
    server: String,
}

impl McpPromptExecutor {
    pub fn new(client: Arc<tokio::sync::Mutex<McpClient>>, server: String) -> Self {
        Self { client, server }
    }
}

#[async_trait]
impl ToolExecutor for McpPromptExecutor {
    async fn execute(
        &self,
        _args: serde_json::Value,
        _ctx: &ToolExecutionContext,
    ) -> Result<ToolOutput> {
        let client = self.client.lock().await;
        let server = client
            .server(&self.server)
            .context("mcp server not found")?;
        let prompts = server.list_prompts().await?;
        let body = prompts
            .iter()
            .map(|prompt| {
                let args = prompt
                    .arguments
                    .iter()
                    .map(|(key, value)| format!("{}={}", key, value))
                    .collect::<Vec<_>>()
                    .join(", ");
                if args.is_empty() {
                    prompt.description.clone()
                } else {
                    format!("{}: {}", args, prompt.description)
                }
            })
            .collect::<Vec<_>>()
            .join("\n");
        if body.trim().is_empty() {
            Ok(ToolOutput::success("no prompts exposed".into()))
        } else {
            Ok(ToolOutput::success(body))
        }
    }
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use agent_config::{McpConfig, McpServerConfig};

    use super::build_client;

    fn server(
        name: &str,
        command: Option<String>,
        url: Option<String>,
        enabled: bool,
    ) -> McpServerConfig {
        McpServerConfig {
            name: name.into(),
            command,
            args: vec!["--verbose".into()],
            url,
            env: HashMap::new(),
            enabled,
        }
    }

    #[test]
    fn build_client_registers_http_servers_from_config() {
        let mut servers = HashMap::new();
        servers.insert(
            "stdio-srv".to_string(),
            server("stdio-srv", Some("node".into()), None, true),
        );
        servers.insert(
            "http-srv".to_string(),
            server(
                "http-srv",
                None,
                Some("http://127.0.0.1:3333/mcp".into()),
                true,
            ),
        );
        servers.insert(
            "off".to_string(),
            server("off", Some("node".into()), None, false),
        );
        let config = McpConfig { servers };

        let client = build_client(&config).expect("expected client with enabled servers");
        let client = client.try_lock().expect("client lock");
        let names: Vec<&str> = client.servers.iter().map(|s| s.name.as_str()).collect();

        assert_eq!(names.len(), 2, "expected 2 registered servers");
        assert!(
            names.contains(&"stdio-srv"),
            "expected stdio-srv to be registered: {names:?}"
        );
        assert!(
            names.contains(&"http-srv"),
            "expected http-srv to be registered: {names:?}"
        );
        assert!(
            !names.contains(&"off"),
            "disabled server 'off' must not be registered"
        );
    }
}
