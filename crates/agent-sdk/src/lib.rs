pub use agent_config::AgentConfig;
pub use agent_events::{AgentEvent, AgentEventPayload, EventBus, EventSubscriber};
pub use agent_model::{ModelProvider, MockProvider};
pub use agent_plugins::{Plugin, PluginManager, PluginManifest, PluginContext};
pub use agent_skills::SkillRegistry;
pub use agent_tools::{ToolDefinition, ToolExecutor, ToolRegistry, ToolOutput};

pub fn create_plugin_template(name: &str) -> std::path::PathBuf {
    let p = std::path::PathBuf::from(name.replace('-', "_"));
    let _ = std::fs::create_dir_all(&p);
    let manifest = r#"
{
  "name": "REPLACE_ME",
  "version": "0.1.0",
  "author": "",
  "description": "",
  "min_agent_version": "0.1.0"
}
"#;
    let _ = std::fs::write(p.join("manifest.json"), manifest);
    p
}
