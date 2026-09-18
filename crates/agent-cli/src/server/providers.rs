use std::collections::HashSet;
use std::sync::Arc;
use std::time::Duration;

use agent_config::{AgentConfig, ModelConfig, ProviderConfig, ProviderKind};
use hyper::body::Incoming;
use hyper::{Request, Response};
use serde::Deserialize;

use super::api::{json_response, read_json, BoxBody};
use super::catalog::{self, Catalog};
use super::state::AppState;

pub async fn list(state: Arc<AppState>) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let cfg = state.config.read().unwrap().clone().unwrap_or_default();
    let catalog = load_catalog(&state).await;
    let current = state.resolve_model();
    let current_id = provider_id_for_model(&current, &catalog);

    let mut connected: HashSet<String> = cfg.providers.keys().cloned().collect();
    connected.insert(current_id.clone());

    let mut entries: Vec<serde_json::Value> = Vec::new();
    let mut seen: HashSet<String> = HashSet::new();

    let mut custom_ids: Vec<String> = cfg
        .providers
        .keys()
        .filter(|id| catalog.provider(id).is_none())
        .cloned()
        .collect();
    custom_ids.sort();

    if catalog.provider(&current_id).is_none() && !cfg.providers.contains_key(&current_id) {
        custom_ids.push(current_id.clone());
        custom_ids.sort();
    }

    for id in &custom_ids {
        if !seen.insert(id.clone()) {
            continue;
        }
        let api = cfg
            .providers
            .get(id)
            .and_then(|provider| provider.base_url.clone())
            .or_else(|| current.base_url.clone().filter(|_| id == &current_id));
        entries.push(serde_json::json!({
            "id": id,
            "name": id,
            "connected": true,
            "custom": true,
            "api": api,
        }));
    }

    for provider in &catalog.providers {
        if !seen.insert(provider.id.clone()) {
            continue;
        }
        entries.push(serde_json::json!({
            "id": provider.id,
            "name": provider.name,
            "connected": connected.contains(&provider.id),
            "custom": false,
            "api": provider.api,
        }));
    }

    entries.push(serde_json::json!({
        "id": "other",
        "name": "Other (custom provider)",
        "connected": false,
        "custom": true,
        "api": null,
    }));

    Ok(json_response(&serde_json::json!({ "data": entries })))
}

pub async fn models(state: Arc<AppState>) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let cfg = state.config.read().unwrap().clone().unwrap_or_default();
    let catalog = load_catalog(&state).await;
    let current = state.resolve_model();
    let current_id = provider_id_for_model(&current, &catalog);

    let mut connected: Vec<String> = cfg.providers.keys().cloned().collect();
    if current.provider != ProviderKind::Mock && !connected.contains(&current_id) {
        connected.push(current_id.clone());
    }
    connected.sort();

    let mut free_models: Vec<(String, String, String, String)> = Vec::new();
    let mut seen_free: HashSet<String> = HashSet::new();
    let mut groups: Vec<serde_json::Value> = Vec::new();

    for id in &connected {
        let meta = provider_meta(id, &cfg, &catalog, &current, &current_id);
        let mut models: Vec<(String, String, bool)> = Vec::new();
        let mut seen: HashSet<String> = HashSet::new();

        if let Some(provider) = catalog.provider(id) {
            for model in &provider.models {
                if seen.insert(model.id.clone()) {
                    models.push((model.id.clone(), model.name.clone(), model.free));
                }
            }
        }

        if let Some(base_url) = &meta.base_url {
            let key = meta
                .api_key_env
                .as_ref()
                .map(|value| std::env::var(value).unwrap_or_else(|_| value.clone()));
            for model_id in fetch_remote_models(base_url, key.as_deref()).await {
                if seen.insert(model_id.clone()) {
                    let name = model_id.clone();
                    models.push((model_id, name, false));
                }
            }
        }

        let mut group_models: Vec<(String, serde_json::Value)> = Vec::new();
        for (model_id, name, free) in models {
            if free {
                let key = format!("{id}/{model_id}");
                if seen_free.insert(key) {
                    free_models.push((name.to_lowercase(), model_id, name, id.clone()));
                }
            } else {
                let value = serde_json::json!({
                    "id": model_id,
                    "name": name,
                    "free": false,
                    "provider": id,
                });
                group_models.push((name.to_lowercase(), value));
            }
        }

        if group_models.is_empty() {
            continue;
        }
        group_models.sort_by(|a, b| a.0.cmp(&b.0));
        let models: Vec<serde_json::Value> =
            group_models.into_iter().map(|entry| entry.1).collect();
        groups.push(serde_json::json!({
            "id": id,
            "name": meta.name,
            "kind": meta.kind,
            "base_url": meta.base_url,
            "connected": true,
            "models": models,
        }));
    }

    free_models.sort_by(|a, b| a.0.cmp(&b.0));
    let mut data: Vec<serde_json::Value> = Vec::new();
    if !free_models.is_empty() {
        let models: Vec<serde_json::Value> = free_models
            .into_iter()
            .map(|(_, model_id, name, provider)| {
                serde_json::json!({
                    "id": model_id,
                    "name": name,
                    "free": true,
                    "provider": provider,
                })
            })
            .collect();
        data.push(serde_json::json!({
            "id": "free",
            "name": "Free",
            "connected": true,
            "models": models,
        }));
    }
    data.extend(groups);

    Ok(json_response(&serde_json::json!({
        "data": data,
        "current": { "provider": current_id, "model": current.model },
        "favorites": cfg.favorites,
    })))
}

#[derive(Deserialize)]
struct SelectModelBody {
    provider: String,
    model: String,
}

pub async fn select_model(
    request: Request<Incoming>,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let body = match read_json::<SelectModelBody>(request).await {
        Ok(body) => body,
        Err(response) => return Ok(response),
    };

    let mut cfg = state.config.read().unwrap().clone().unwrap_or_default();
    let catalog = load_catalog(&state).await;
    let current = state.resolve_model();
    let current_id = provider_id_for_model(&current, &catalog);
    let meta = provider_meta(&body.provider, &cfg, &catalog, &current, &current_id);

    let model = ModelConfig {
        provider: meta.kind,
        model: body.model.clone(),
        base_url: meta.base_url.clone(),
        api_key_env: meta.api_key_env.clone(),
        temperature: None,
        max_tokens: None,
    };

    cfg.model = Some(model.clone());
    *state.model.write().unwrap() = Some(model.clone());
    let _ = agent_config::ConfigLoader::save_global(&cfg);
    *state.config.write().unwrap() = Some(cfg);

    Ok(json_response(&serde_json::json!({
        "model": {
            "provider": model.provider,
            "model": model.model,
            "base_url": model.base_url,
        }
    })))
}

#[derive(Deserialize)]
struct ConnectBody {
    id: String,
    #[serde(default)]
    api_key: Option<String>,
    #[serde(default)]
    base_url: Option<String>,
}

pub async fn connect(
    request: Request<Incoming>,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let body = match read_json::<ConnectBody>(request).await {
        Ok(body) => body,
        Err(response) => return Ok(response),
    };

    let catalog = load_catalog(&state).await;
    let provider = catalog.provider(&body.id);
    let kind = provider
        .map(|entry| catalog::provider_kind(entry.npm.as_deref(), &entry.id))
        .unwrap_or(ProviderKind::OpenAiCompatible);
    let base_url = body
        .base_url
        .clone()
        .filter(|value| !value.is_empty())
        .or_else(|| provider.and_then(|entry| entry.api.clone()));
    let api_key = body.api_key.clone().filter(|value| !value.is_empty());

    let mut cfg = state.config.read().unwrap().clone().unwrap_or_default();
    cfg.providers.insert(
        body.id.clone(),
        ProviderConfig {
            kind,
            base_url,
            api_key_env: api_key,
            temperature: None,
            max_tokens: None,
        },
    );
    let _ = agent_config::ConfigLoader::save_global(&cfg);
    *state.config.write().unwrap() = Some(cfg);

    Ok(json_response(
        &serde_json::json!({ "ok": true, "id": body.id }),
    ))
}

#[derive(Deserialize)]
struct FavoriteBody {
    provider: String,
    model: String,
}

pub async fn favorite(
    request: Request<Incoming>,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let body = match read_json::<FavoriteBody>(request).await {
        Ok(body) => body,
        Err(response) => return Ok(response),
    };

    let key = format!("{}/{}", body.provider, body.model);
    let mut cfg = state.config.read().unwrap().clone().unwrap_or_default();
    match cfg.favorites.iter().position(|entry| entry == &key) {
        Some(index) => {
            cfg.favorites.remove(index);
        }
        None => cfg.favorites.push(key),
    }
    let _ = agent_config::ConfigLoader::save_global(&cfg);
    let favorites = cfg.favorites.clone();
    *state.config.write().unwrap() = Some(cfg);

    Ok(json_response(
        &serde_json::json!({ "favorites": favorites }),
    ))
}

struct ProviderMeta {
    name: String,
    kind: ProviderKind,
    base_url: Option<String>,
    api_key_env: Option<String>,
}

fn provider_meta(
    id: &str,
    cfg: &AgentConfig,
    catalog: &Catalog,
    current: &ModelConfig,
    current_id: &str,
) -> ProviderMeta {
    let entry = catalog.provider(id);
    if let Some(provider) = cfg.providers.get(id) {
        return ProviderMeta {
            name: entry
                .map(|value| value.name.clone())
                .unwrap_or_else(|| id.to_string()),
            kind: provider.kind,
            base_url: provider
                .base_url
                .clone()
                .or_else(|| entry.and_then(|value| value.api.clone())),
            api_key_env: provider.api_key_env.clone(),
        };
    }
    if id == current_id {
        return ProviderMeta {
            name: entry
                .map(|value| value.name.clone())
                .unwrap_or_else(|| id.to_string()),
            kind: current.provider,
            base_url: current
                .base_url
                .clone()
                .or_else(|| entry.and_then(|value| value.api.clone())),
            api_key_env: current.api_key_env.clone(),
        };
    }
    ProviderMeta {
        name: entry
            .map(|value| value.name.clone())
            .unwrap_or_else(|| id.to_string()),
        kind: entry
            .map(|value| catalog::provider_kind(value.npm.as_deref(), &value.id))
            .unwrap_or(ProviderKind::OpenAiCompatible),
        base_url: entry.and_then(|value| value.api.clone()),
        api_key_env: entry.and_then(|value| value.env.first().cloned()),
    }
}

fn provider_id_for_model(model: &ModelConfig, catalog: &Catalog) -> String {
    if let Some(base_url) = &model.base_url {
        if let Some(host) = host_of(base_url) {
            if let Some(provider) = catalog.providers.iter().find(|provider| {
                provider.api.as_deref().and_then(host_of).as_deref() == Some(host.as_str())
            }) {
                return provider.id.clone();
            }
            let stripped = host.trim_start_matches("api.").trim_start_matches("www.");
            if let Some(first) = stripped.split('.').next() {
                if !first.is_empty() {
                    return first.to_string();
                }
            }
        }
    }
    match model.provider {
        ProviderKind::OpenAi => "openai".into(),
        ProviderKind::Anthropic => "anthropic".into(),
        ProviderKind::Google => "google".into(),
        ProviderKind::Mock => "mock".into(),
        _ => "custom".into(),
    }
}

fn host_of(url: &str) -> Option<String> {
    let rest = url.split("://").nth(1).unwrap_or(url);
    let authority = rest.split('/').next()?;
    let host = authority.split('@').next_back()?.split(':').next()?;
    if host.is_empty() {
        None
    } else {
        Some(host.to_lowercase())
    }
}

async fn load_catalog(state: &AppState) -> Catalog {
    let cached = state.catalog.read().unwrap().clone();
    if let Some(catalog) = cached {
        return catalog;
    }
    match catalog::fetch().await {
        Some(catalog) => {
            *state.catalog.write().unwrap() = Some(catalog.clone());
            catalog
        }
        None => Catalog::default(),
    }
}

async fn fetch_remote_models(base_url: &str, api_key: Option<&str>) -> Vec<String> {
    let client = match reqwest::Client::builder()
        .timeout(Duration::from_secs(10))
        .build()
    {
        Ok(client) => client,
        Err(_) => return Vec::new(),
    };
    let url = format!("{}/models", base_url.trim_end_matches('/'));
    let mut request = client.get(&url);
    if let Some(key) = api_key {
        request = request.header("Authorization", format!("Bearer {key}"));
    }
    let response = match request.send().await {
        Ok(response) => response,
        Err(_) => return Vec::new(),
    };
    if !response.status().is_success() {
        return Vec::new();
    }
    let json: serde_json::Value = match response.json().await {
        Ok(value) => value,
        Err(_) => return Vec::new(),
    };
    let mut ids = Vec::new();
    if let Some(items) = json.get("data").and_then(|value| value.as_array()) {
        for item in items {
            if let Some(id) = item.get("id").and_then(|value| value.as_str()) {
                ids.push(id.to_string());
            }
        }
    }
    ids
}
