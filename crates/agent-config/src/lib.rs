use std::collections::HashMap;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum AgentMode {
    Interactive,
    Readonly,
    Auto,
    Plan,
    Debug,
    Ci,
}

impl Default for AgentMode {
    fn default() -> Self {
        Self::Interactive
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PermissionMode {
    Ask,
    Allow,
    Deny,
}

impl Default for PermissionMode {
    fn default() -> Self {
        Self::Ask
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ProviderKind {
    OpenAi,
    Anthropic,
    Google,
    OpenAiCompatible,
    Custom,
    Mock,
}

impl Default for ProviderKind {
    fn default() -> Self {
        Self::OpenAi
    }
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

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpConfig {
    #[serde(default)]
    pub servers: HashMap<String, McpServerConfig>,
}

impl Default for McpConfig {
    fn default() -> Self {
        Self { servers: HashMap::new() }
    }
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
    pub extra: HashMap<String, serde_json::Value>,
}

#[derive(Debug, Clone)]
pub struct ConfigLoader {
    global_path: Option<PathBuf>,
    project_dir: Option<PathBuf>,
}

impl Default for ConfigLoader {
    fn default() -> Self {
        Self::new()
    }
}

impl ConfigLoader {
    pub fn new() -> Self {
        Self {
            global_path: dirs::home_dir().map(|h| h.join(".agent/config.toml")),
            project_dir: None,
        }
    }

    pub fn with_project(mut self, dir: &Path) -> Self {
        self.project_dir = Some(dir.to_path_buf());
        self
    }

    pub fn load(&self) -> Result<AgentConfig> {
        let mut cfg = AgentConfig::default();

        if let Some(g) = &self.global_path {
            if g.exists() {
                let parsed = Self::parse_toml(g)?;
                cfg = Self::merge(cfg, parsed);
            }
        }

        if let Some(pd) = &self.project_dir {
            let p = pd.join("agent.toml");
            if p.exists() {
                let parsed = Self::parse_toml(&p)?;
                cfg = Self::merge(cfg, parsed);
            }
        }

        Self::apply_env(&mut cfg);
        Ok(cfg)
    }

    fn parse_toml(path: &Path) -> Result<AgentConfig> {
        let raw = std::fs::read_to_string(path)
            .with_context(|| format!("reading config {}", path.display()))?;
        let v: AgentConfig = toml::from_str(&raw).with_context(|| format!("parsing {}", path.display()))?;
        Ok(v)
    }

    pub fn merge(base: AgentConfig, overlay: AgentConfig) -> AgentConfig {
        Self::merge_impl(base, overlay)
    }

    fn merge_impl(base: AgentConfig, overlay: AgentConfig) -> AgentConfig {
        AgentConfig {
            mode: overlay.mode.or(base.mode),
            model: overlay.model.or(base.model),
            limits: overlay.limits,
            permissions: overlay.permissions,
            mcp: overlay.mcp,
            skills_enabled: if overlay.skills_enabled.is_empty() {
                base.skills_enabled
            } else {
                overlay.skills_enabled
            },
            skills_disabled: if overlay.skills_disabled.is_empty() {
                base.skills_disabled
            } else {
                overlay.skills_disabled
            },
            plugins: {
                let mut m = base.plugins;
                for (k, v) in overlay.plugins {
                    m.insert(k, v);
                }
                m
            },
            extra: {
                let mut m = base.extra;
                for (k, v) in overlay.extra {
                    m.insert(k, v);
                }
                m
            },
        }
    }

    fn apply_env(cfg: &mut AgentConfig) {
        if let Ok(key) = std::env::var("AGENT_API_KEY") {
            if cfg.model.is_none() {
                cfg.model = Some(ModelConfig {
                    provider: ProviderKind::OpenAi,
                    model: "gpt-4".into(),
                    base_url: None,
                    api_key_env: Some("AGENT_API_KEY".into()),
                    temperature: None,
                    max_tokens: None,
                });
            }
            let _ = key;
        }
        if let Ok(model) = std::env::var("AGENT_MODEL") {
            if let Some(mc) = &mut cfg.model {
                mc.model = model;
            }
        }
        if let Ok(provider) = std::env::var("AGENT_PROVIDER") {
            if let Some(mc) = &mut cfg.model {
                mc.provider = match provider.as_str() {
                    "anthropic" => ProviderKind::Anthropic,
                    "google" => ProviderKind::Google,
                    "mock" => ProviderKind::Mock,
                    "custom" => ProviderKind::Custom,
                    "openai" => ProviderKind::OpenAi,
                    _ => ProviderKind::OpenAiCompatible,
                };
            }
        }
        if let Ok(limit) = std::env::var("AGENT_MAX_TOOL_CALLS") {
            if let Ok(v) = limit.parse() {
                cfg.limits.max_tool_calls = v;
            }
        }
    }

    pub fn save_global(cfg: &AgentConfig) -> Result<PathBuf> {
        let dir = dirs::home_dir()
            .context("cannot determine home dir")?
            .join(".agent");
        std::fs::create_dir_all(&dir).context("creating .agent dir")?;
        let p = dir.join("config.toml");
        let raw = toml::to_string_pretty(cfg)?;
        std::fs::write(&p, raw)?;
        Ok(p)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn merge_overrides() {
        let base = AgentConfig {
            model: Some(ModelConfig {
                provider: ProviderKind::OpenAi,
                model: "m1".into(),
                base_url: None,
                api_key_env: None,
                temperature: None,
                max_tokens: None,
            }),
            ..Default::default()
        };
        let overlay = AgentConfig {
            model: Some(ModelConfig {
                provider: ProviderKind::Anthropic,
                model: "m2".into(),
                base_url: None,
                api_key_env: None,
                temperature: Some(0.5),
                max_tokens: None,
            }),
            ..Default::default()
        };
        let merged = ConfigLoader::merge(base, overlay);
        let merged_model = merged.model.unwrap();
        assert_eq!(merged_model.provider, ProviderKind::Anthropic);
        assert_eq!(merged_model.model, "m2");
    }
}
