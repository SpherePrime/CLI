use std::collections::HashMap;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize, Default)]
pub enum AgentMode {
    #[default]
    Interactive,
    Readonly,
    Auto,
    Plan,
    Debug,
    Ci,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum PermissionMode {
    #[default]
    Ask,
    Allow,
    AutoEdit,
    Deny,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "lowercase")]
pub enum ProviderKind {
    #[default]
    OpenAi,
    Anthropic,
    Google,
    OpenAiCompatible,
    Custom,
    Mock,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelConfig {
    pub provider: ProviderKind,
    pub model: String,
    pub base_url: Option<String>,
    pub api_key_env: Option<String>,
    pub temperature: Option<f32>,
    pub max_tokens: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResourceLimits {
    pub max_tool_calls: usize,
    pub max_execution_time_secs: u64,
    pub max_parallel_tools: usize,
    pub max_output_bytes: usize,
    pub max_context_messages: usize,
}

impl Default for ResourceLimits {
    fn default() -> Self {
        Self {
            max_tool_calls: 50,
            max_execution_time_secs: 3600,
            max_parallel_tools: 4,
            max_output_bytes: 1_048_576,
            max_context_messages: 200,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpServerConfig {
    pub name: String,
    pub command: Option<String>,
    pub args: Vec<String>,
    pub url: Option<String>,
    pub env: HashMap<String, String>,
    pub enabled: bool,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct McpConfig {
    #[serde(default)]
    pub servers: HashMap<String, McpServerConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PermissionsConfig {
    #[serde(default)]
    pub mode: PermissionMode,
    #[serde(default)]
    pub allowed_tools: Vec<String>,
    #[serde(default)]
    pub denied_tools: Vec<String>,
    #[serde(default)]
    pub allowed_paths: Vec<String>,
    #[serde(default)]
    pub denied_paths: Vec<String>,
    #[serde(default)]
    pub network_allowed_domains: Vec<String>,
    #[serde(default)]
    pub secret_redaction: bool,
}

impl Default for PermissionsConfig {
    fn default() -> Self {
        Self {
            mode: PermissionMode::Ask,
            allowed_tools: vec![],
            denied_tools: vec![],
            allowed_paths: vec![],
            denied_paths: vec![],
            network_allowed_domains: vec![],
            secret_redaction: true,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProviderConfig {
    pub kind: ProviderKind,
    pub base_url: Option<String>,
    pub api_key_env: Option<String>,
    pub temperature: Option<f32>,
    pub max_tokens: Option<u32>,
}

impl Default for ProviderConfig {
    fn default() -> Self {
        Self {
            kind: ProviderKind::OpenAi,
            base_url: None,
            api_key_env: None,
            temperature: None,
            max_tokens: None,
        }
    }
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AgentConfig {
    pub mode: Option<AgentMode>,
    pub model: Option<ModelConfig>,
    #[serde(default)]
    pub limits: ResourceLimits,
    #[serde(default)]
    pub permissions: PermissionsConfig,
    #[serde(default)]
    pub mcp: McpConfig,
    #[serde(default)]
    pub skills_enabled: Vec<String>,
    #[serde(default)]
    pub skills_disabled: Vec<String>,
    #[serde(default)]
    pub plugins: HashMap<String, bool>,
    #[serde(default)]
    pub providers: HashMap<String, ProviderConfig>,
    #[serde(default)]
    pub favorites: Vec<String>,
    #[serde(default)]
    pub extra: HashMap<String, serde_json::Value>,
}
