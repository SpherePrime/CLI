use std::collections::HashMap;
use std::time::Duration;

use serde::Deserialize;

const MODELS_DEV_URL: &str = "https://models.dev/api.json";

#[derive(Clone, Debug)]
pub struct CatalogModel {
    pub id: String,
    pub name: String,
    pub free: bool,
}

#[derive(Clone, Debug, Default)]
pub struct CatalogProvider {
    pub id: String,
    pub name: String,
    pub api: Option<String>,
    pub env: Vec<String>,
    pub npm: Option<String>,
    pub models: Vec<CatalogModel>,
}

#[derive(Clone, Debug, Default)]
pub struct Catalog {
    pub providers: Vec<CatalogProvider>,
}

impl Catalog {
    pub fn provider(&self, id: &str) -> Option<&CatalogProvider> {
        self.providers.iter().find(|provider| provider.id == id)
    }
}

#[derive(Deserialize)]
struct RawProvider {
    #[serde(default)]
    id: String,
    #[serde(default)]
    name: String,
    #[serde(default)]
    api: Option<String>,
    #[serde(default)]
    env: Vec<String>,
    #[serde(default)]
    npm: Option<String>,
    #[serde(default)]
    models: HashMap<String, RawModel>,
}

#[derive(Deserialize)]
struct RawModel {
    #[serde(default)]
    id: String,
    #[serde(default)]
    name: String,
    #[serde(default)]
    cost: Option<RawCost>,
}

#[derive(Deserialize)]
struct RawCost {
    #[serde(default)]
    input: f64,
    #[serde(default)]
    output: f64,
}

pub async fn fetch() -> Option<Catalog> {
    let client = match reqwest::Client::builder()
        .timeout(Duration::from_secs(20))
        .build()
    {
        Ok(client) => client,
        Err(_) => return None,
    };

    let response = client.get(MODELS_DEV_URL).send().await.ok()?;
    if !response.status().is_success() {
        return None;
    }
    let raw: HashMap<String, RawProvider> = response.json().await.ok()?;

    let mut providers: Vec<CatalogProvider> = raw
        .into_values()
        .filter_map(|provider| {
            let id = if provider.id.is_empty() {
                return None;
            } else {
                provider.id
            };
            let mut models: Vec<CatalogModel> = provider
                .models
                .into_values()
                .filter(|model| !model.id.is_empty())
                .map(|model| {
                    let free = model
                        .cost
                        .map(|cost| cost.input == 0.0 && cost.output == 0.0)
                        .unwrap_or(false);
                    let name = if model.name.is_empty() {
                        model.id.clone()
                    } else {
                        model.name
                    };
                    CatalogModel {
                        id: model.id,
                        name,
                        free,
                    }
                })
                .collect();
            if models.is_empty() {
                return None;
            }
            models.sort_by(|a, b| a.name.to_lowercase().cmp(&b.name.to_lowercase()));
            Some(CatalogProvider {
                id,
                name: if provider.name.is_empty() {
                    String::from("Unknown")
                } else {
                    provider.name
                },
                api: provider.api,
                env: provider.env,
                npm: provider.npm,
                models,
            })
        })
        .collect();

    providers.sort_by(|a, b| a.name.to_lowercase().cmp(&b.name.to_lowercase()));
    Some(Catalog { providers })
}

pub fn provider_kind(npm: Option<&str>, id: &str) -> agent_config::ProviderKind {
    let npm = npm.unwrap_or_default().to_lowercase();
    if npm.contains("anthropic") || id.contains("anthropic") {
        return agent_config::ProviderKind::Anthropic;
    }
    if npm.contains("google") || id.contains("google") {
        return agent_config::ProviderKind::Google;
    }
    if id == "openai" {
        return agent_config::ProviderKind::OpenAi;
    }
    if id == "mock" {
        return agent_config::ProviderKind::Mock;
    }
    agent_config::ProviderKind::OpenAiCompatible
}
