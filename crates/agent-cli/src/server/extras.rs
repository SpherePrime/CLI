use std::sync::Arc;

use agent_plugins::manifest::PluginDescriptor;
use agent_plugins::PluginManager;
use agent_skills::SkillRegistry;
use hyper::body::Incoming;
use hyper::{Request, Response};
use serde::Deserialize;

use super::api::{json_response, read_json, BoxBody};
use super::state::AppState;
use crate::cli::util::{load_config, plugins_dir, save_config, skills_dir};

pub async fn list_skills(
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let config = state.config.read().unwrap().clone().unwrap_or_default();
    let registry = load_skill_registry(&config);
    let data: Vec<serde_json::Value> = registry
        .all()
        .iter()
        .map(|skill| {
            serde_json::json!({
                "name": skill.name,
                "scope": format!("{:?}", skill.scope),
                "enabled": skill.enabled,
                "description": skill
                    .description
                    .lines()
                    .next()
                    .unwrap_or("")
                    .trim(),
            })
        })
        .collect();
    Ok(json_response(&serde_json::json!({ "data": data })))
}

#[derive(Deserialize)]
pub struct ToggleBody {
    pub name: String,
    pub enabled: bool,
}

pub async fn toggle_skill(
    request: Request<Incoming>,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let body = match read_json::<ToggleBody>(request).await {
        Ok(body) => body,
        Err(response) => return Ok(response),
    };
    let registry = load_skill_registry(&state.config.read().unwrap().clone().unwrap_or_default());
    if registry.find(&body.name).is_none() {
        return Ok(json_response(
            &serde_json::json!({ "ok": false, "error": "skill not found" }),
        ));
    }

    let mut config = load_config();
    match body.enabled {
        true => {
            if let Some(index) = config
                .skills_disabled
                .iter()
                .position(|entry| entry == &body.name)
            {
                config.skills_disabled.remove(index);
            }
        }
        false => {
            if !config
                .skills_disabled
                .iter()
                .any(|entry| entry == &body.name)
            {
                config.skills_disabled.push(body.name.clone());
            }
        }
    }
    let _ = save_config(&config);
    *state.config.write().unwrap() = Some(config);
    Ok(json_response(&serde_json::json!({ "ok": true })))
}

pub async fn list_plugins(
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let config = state.config.read().unwrap().clone().unwrap_or_default();
    let enabled = config.plugins.clone();
    let dirs = PluginManager::discover_from(&plugins_dir()).unwrap_or_default();
    let mut data: Vec<serde_json::Value> = Vec::new();
    for dir in dirs {
        let name = dir
            .file_name()
            .and_then(|name| name.to_str())
            .unwrap_or("plugin")
            .to_string();
        let enabled_state = enabled.get(&name).copied().unwrap_or(true);
        let (version, description) = match PluginDescriptor::load(&dir) {
            Ok(descriptor) => (descriptor.version, descriptor.description),
            Err(_) => (String::new(), String::new()),
        };
        data.push(serde_json::json!({
            "name": name,
            "enabled": enabled_state,
            "version": version,
            "description": description,
        }));
    }
    Ok(json_response(&serde_json::json!({ "data": data })))
}

pub async fn toggle_plugin(
    request: Request<Incoming>,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let body = match read_json::<ToggleBody>(request).await {
        Ok(body) => body,
        Err(response) => return Ok(response),
    };
    if !plugins_dir().join(&body.name).is_dir() {
        return Ok(json_response(
            &serde_json::json!({ "ok": false, "error": "plugin not found" }),
        ));
    }
    let mut config = load_config();
    config.plugins.insert(body.name.clone(), body.enabled);
    let _ = save_config(&config);
    *state.config.write().unwrap() = Some(config);
    Ok(json_response(&serde_json::json!({ "ok": true })))
}

fn load_skill_registry(config: &agent_config::AgentConfig) -> SkillRegistry {
    let project = std::env::current_dir()
        .unwrap_or_default()
        .join(".agent")
        .join("skills");
    let mut registry = SkillRegistry::new()
        .with_global(skills_dir())
        .with_project(project);
    let _ = registry.discover_sync();
    for name in &config.skills_disabled {
        let _ = registry.disable(name);
    }
    registry
}
