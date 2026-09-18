use std::collections::HashMap;
use std::path::{Path, PathBuf};

use agent_events::{AgentEvent, AgentEventPayload, AgentScope, EventBus};
use agent_tools::ToolRegistry;
use anyhow::{Context, Result};
use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PluginPhase {
    Discovered,
    Loaded,
    Initialized,
    Active,
    Inactive,
    Unloaded,
}

pub struct PluginManifest {
    pub id: Uuid,
    pub name: String,
    pub version: String,
    pub path: PathBuf,
    pub enabled: bool,
    pub phase: PluginPhase,
}

pub struct PluginContext {
    pub registry: ToolRegistry,
    pub events: EventBus,
    pub env: HashMap<String, String>,
}

impl PluginContext {
    pub fn new(registry: ToolRegistry, events: EventBus) -> Self {
        Self {
            registry,
            events,
            env: HashMap::new(),
        }
    }
}

#[async_trait]
pub trait Plugin: Send + Sync {
    fn manifest(&self) -> &PluginManifest;
    async fn initialize(&mut self, ctx: &mut PluginContext) -> Result<()>;
    async fn activate(&mut self, ctx: &mut PluginContext) -> Result<()>;
    async fn deactivate(&mut self, ctx: &mut PluginContext) -> Result<()>;
    async fn unload(&mut self, ctx: &mut PluginContext) -> Result<()>;
}

pub struct PluginManager {
    pub plugins: Vec<Box<dyn Plugin>>,
    pub manifest_by_name: HashMap<String, Uuid>,
    events: EventBus,
}

impl PluginManager {
    pub fn new() -> Self {
        Self {
            plugins: Vec::new(),
            manifest_by_name: HashMap::new(),
            events: EventBus::new(),
        }
    }

    pub fn with_events(mut self, events: EventBus) -> Self {
        self.events = events;
        self
    }

    pub fn register(&mut self, plugin: Box<dyn Plugin>) {
        let id = plugin.manifest().id;
        let name = plugin.manifest().name.clone();
        self.manifest_by_name.insert(name.clone(), id);
        self.events.dispatch(
            AgentEventPayload::new(AgentEvent::PluginLoaded)
                .with_scope(AgentScope::Plugin)
                .with_detail(serde_json::json!({ "name": name, "id": id })),
        );
        self.plugins.push(plugin);
    }

    pub async fn load_all(&mut self, registry: &ToolRegistry) -> Result<()> {
        for p in &mut self.plugins {
            let mut ctx = PluginContext::new(registry.clone(), self.events.clone());
            p.initialize(&mut ctx).await?;
            p.activate(&mut ctx).await?;
        }
        Ok(())
    }

    pub async fn unload_all(&mut self, registry: &ToolRegistry) {
        for p in &mut self.plugins {
            let mut ctx = PluginContext::new(registry.clone(), self.events.clone());
            let _ = p.deactivate(&mut ctx).await;
            let _ = p.unload(&mut ctx).await;
            self.events.dispatch(
                AgentEventPayload::new(AgentEvent::PluginUnloaded)
                    .with_scope(AgentScope::Plugin)
                    .with_detail(serde_json::json!({ "name": p.manifest().name })),
            );
        }
    }

    pub fn find(&self, name: &str) -> Option<&dyn Plugin> {
        self.plugins
            .iter()
            .find(|p| p.manifest().name == name)
            .map(|p| &**p)
    }

    pub fn manifest(&self, name: &str) -> Option<&PluginManifest> {
        self.plugins
            .iter()
            .find(|p| p.manifest().name == name)
            .map(|p| p.manifest())
    }

    pub fn discover_from(dir: &Path) -> Result<Vec<PathBuf>> {
        if !dir.exists() {
            return Ok(vec![]);
        }
        let mut out = Vec::new();
        for entry in std::fs::read_dir(dir)
            .with_context(|| format!("reading plugin dir {}", dir.display()))?
        {
            let p = entry?.path();
            if p.is_dir() {
                out.push(p);
            }
        }
        Ok(out)
    }
}

impl Default for PluginManager {
    fn default() -> Self {
        Self::new()
    }
}

pub struct SamplePlugin {
    manifest: PluginManifest,
}

impl SamplePlugin {
    pub fn new(name: &str, version: &str, path: PathBuf) -> Self {
        Self {
            manifest: PluginManifest {
                id: Uuid::new_v4(),
                name: name.to_string(),
                version: version.to_string(),
                path,
                enabled: true,
                phase: PluginPhase::Discovered,
            },
        }
    }
}

#[async_trait]
impl Plugin for SamplePlugin {
    fn manifest(&self) -> &PluginManifest {
        &self.manifest
    }

    async fn initialize(&mut self, _ctx: &mut PluginContext) -> Result<()> {
        self.manifest.phase = PluginPhase::Initialized;
        Ok(())
    }

    async fn activate(&mut self, _ctx: &mut PluginContext) -> Result<()> {
        self.manifest.phase = PluginPhase::Active;
        Ok(())
    }

    async fn deactivate(&mut self, _ctx: &mut PluginContext) -> Result<()> {
        self.manifest.phase = PluginPhase::Inactive;
        Ok(())
    }

    async fn unload(&mut self, _ctx: &mut PluginContext) -> Result<()> {
        self.manifest.phase = PluginPhase::Unloaded;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn plugin_lifecycle() {
        let mut pm = PluginManager::new();
        let p = Box::new(SamplePlugin::new(
            "test-plugin",
            "0.1.0",
            PathBuf::from("/tmp"),
        )) as Box<dyn Plugin>;
        pm.register(p);
        let _ = pm.load_all(&ToolRegistry::new()).await;
        assert_eq!(pm.plugins.len(), 1);
    }
}
