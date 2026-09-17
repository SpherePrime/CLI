use agent_config::{AgentConfig, ProviderConfig, ProviderKind};
use dialoguer::{Input, Select};

#[derive(Debug, clap::Subcommand)]
pub enum ProviderAction {
    /// Add a new provider
    Add {
        /// Provider ID (name)
        id: Option<String>,
    },
    /// List all providers
    List,
    /// Set the active provider
    Use {
        /// Provider ID to use
        id: String,
    },
}

pub fn list_providers(cfg: &AgentConfig) {
    if cfg.providers.is_empty() {
        println!("(no providers configured)");
        return;
    }

    let mut providers: Vec<_> = cfg.providers.iter().collect();
    providers.sort_by_key(|(k, _)| k.as_str());

    for (id, provider) in providers {
        println!("{id}");
        println!("  kind: {}", provider_name(&provider.kind));
        if let Some(url) = &provider.base_url {
            println!("  base_url: {url}");
        }
        if let Some(env) = &provider.api_key_env {
            println!("  api_key_env: {env}");
        }
    }
}

pub fn add_provider(cfg: &mut AgentConfig, id: Option<String>) -> anyhow::Result<String> {
    let provider_id = if let Some(id) = id {
        id
    } else {
        Input::new()
            .with_prompt("Enter provider ID")
            .default("my-openai".to_string())
            .interact()?
    };

    if provider_id.is_empty() {
        return Err(anyhow::anyhow!("Provider ID cannot be empty"));
    }

    if cfg.providers.contains_key(&provider_id) {
        return Err(anyhow::anyhow!("Provider '{}' already exists", provider_id));
    }

    let kind = prompt_provider_kind()?;
    let base_url = prompt_base_url(kind)?;
    let api_key_env = prompt_api_key_env(kind)?;

    let provider = ProviderConfig {
        kind,
        base_url,
        api_key_env,
        temperature: None,
        max_tokens: None,
    };

    cfg.providers.insert(provider_id.clone(), provider);
    Ok(provider_id)
}

fn prompt_provider_kind() -> anyhow::Result<ProviderKind> {
    let choices = vec!["openai", "anthropic", "google", "openai-compatible (custom)", "mock (for testing)"];
    
    let selection = Select::new()
        .with_prompt("Select provider type")
        .default(0)
        .items(&choices)
        .interact()?;

    let kind = match selection {
        0 => ProviderKind::OpenAi,
        1 => ProviderKind::Anthropic,
        2 => ProviderKind::Google,
        3 => ProviderKind::OpenAiCompatible,
        4 => ProviderKind::Mock,
        _ => ProviderKind::OpenAi,
    };

    Ok(kind)
}

fn prompt_base_url(kind: ProviderKind) -> anyhow::Result<Option<String>> {
    let default_url: Option<String> = match kind {
        ProviderKind::OpenAi => Some("https://api.openai.com/v1".to_string()),
        ProviderKind::Anthropic => Some("https://api.anthropic.com".to_string()),
        ProviderKind::Google => Some("https://generativelanguage.googleapis.com".to_string()),
        ProviderKind::OpenAiCompatible => Some("".to_string()),
        ProviderKind::Mock => return Ok(None),
        ProviderKind::Custom => None,
    };

    let prompt_text = if let Some(ref url) = &default_url {
        if url.is_empty() {
            "Enter base URL (press Enter to skip): ".to_string()
        } else {
            format!("Enter base URL [{}] (press Enter to skip): ", url)
        }
    } else {
        "Enter base URL (press Enter to skip): ".to_string()
    };

    let default_value = default_url.unwrap_or_default();
    let url = Input::new()
        .with_prompt(&prompt_text)
        .default(default_value)
        .allow_empty(true)
        .interact()?;

    if url.is_empty() {
        Ok(None)
    } else if url.starts_with("http://") || url.starts_with("https://") {
        Ok(Some(url))
    } else {
        Err(anyhow::anyhow!("Invalid URL format. Must start with http:// or https://"))
    }
}

fn prompt_api_key_env(kind: ProviderKind) -> anyhow::Result<Option<String>> {
    let default_env = match kind {
        ProviderKind::OpenAi => "OPENAI_API_KEY",
        ProviderKind::Anthropic => "ANTHROPIC_API_KEY",
        ProviderKind::Google => "GOOGLE_API_KEY",
        ProviderKind::OpenAiCompatible => "OPENAI_API_KEY",
        ProviderKind::Mock => return Ok(None),
        ProviderKind::Custom => "OPENAI_API_KEY",
    };

    let env = Input::new()
        .with_prompt("Enter API key env var name")
        .default(default_env.to_string())
        .allow_empty(true)
        .interact()?;

    if env.is_empty() {
        Ok(None)
    } else {
        Ok(Some(env))
    }
}

pub fn use_provider(cfg: &mut AgentConfig, id: &str) -> anyhow::Result<()> {
    if !cfg.providers.contains_key(id) {
        return Err(anyhow::anyhow!("Provider '{}' not found", id));
    }

    let provider = cfg.providers.get(id).cloned().unwrap();

    cfg.model = Some(agent_config::ModelConfig {
        provider: provider.kind,
        model: "gpt-4".to_string(),
        base_url: provider.base_url,
        api_key_env: provider.api_key_env,
        temperature: provider.temperature,
        max_tokens: provider.max_tokens,
    });

    Ok(())
}

fn provider_name(kind: &ProviderKind) -> &'static str {
    match kind {
        ProviderKind::OpenAi => "openai",
        ProviderKind::Anthropic => "anthropic",
        ProviderKind::Google => "google",
        ProviderKind::OpenAiCompatible => "openai-compatible",
        ProviderKind::Custom => "custom",
        ProviderKind::Mock => "mock",
    }
}