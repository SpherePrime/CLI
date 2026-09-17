 use std::path::{Path, PathBuf};
 
 use anyhow::{Context, Result};
 
 use crate::types::{AgentConfig, ModelConfig, ProviderKind};
 
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
         let v: AgentConfig = toml::from_str(&raw)
             .with_context(|| format!("parsing {}", path.display()))?;
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
             providers: {
                 let mut m = base.providers;
                 for (k, v) in overlay.providers {
                     m.insert(k, v);
                 }
                 m
             },
             favorites: {
                 let mut list = base.favorites;
                 for key in overlay.favorites {
                     if !list.contains(&key) {
                         list.push(key);
                     }
                 }
                 list
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
         if std::env::var("AGENT_API_KEY").is_ok() {
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
     use crate::types::{ModelConfig, ProviderKind};
 
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
