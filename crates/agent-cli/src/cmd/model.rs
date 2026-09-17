use agent_config::{AgentConfig, ModelConfig, ProviderKind};
use dialoguer::{Input, Select};

#[derive(Debug, clap::Subcommand)]
pub enum ModelAction {
    /// Add/configure a model (interactive wizard)
    Add {
        /// Provider/model in format provider/name (e.g. openai/gpt-4)
        spec: Option<String>,
    },
    /// List configured models
    List,
    /// Use a model from a provider (format: provider/name)
    Use {
        /// Provider and model name in format provider/name (e.g. openai/gpt-4)
        spec: String,
    },
}

pub fn list_models(cfg: &AgentConfig) {
    if let Some(mc) = &cfg.model {
        println!("active model:");
        println!("  provider: {}", provider_name(&mc.provider));
        println!("  model: {}", mc.model);
        if let Some(url) = &mc.base_url {
            println!("  base_url: {url}");
        }
        if let Some(env) = &mc.api_key_env {
            println!("  api_key_env: {env}");
        }
        if let Some(temp) = mc.temperature {
            println!("  temperature: {temp}");
        }
        if let Some(tokens) = mc.max_tokens {
            println!("  max_tokens: {tokens}");
        }
    } else {
        println!("(no model configured)");
    }

    if !cfg.providers.is_empty() {
        println!("\nsaved providers:");
        let mut providers: Vec<_> = cfg.providers.iter().collect();
        providers.sort_by_key(|(k, _)| k.as_str());

        for (id, _provider) in providers {
            println!("  [{id}] {}(provider)", id);
        }
    }
}

pub fn add_model(cfg: &mut AgentConfig) -> anyhow::Result<ModelConfig> {
    let spec: String = Input::new()
        .with_prompt("Enter provider/model [e.g. openai/gpt-4 or press Enter for wizard]")
        .allow_empty(true)
        .interact()?;

    if spec.is_empty() {
        return run_wizard(cfg);
    }

    let parts: Vec<&str> = spec.splitn(2, '/').collect();
    if parts.len() != 2 {
        return Err(anyhow::anyhow!(
            "Invalid format. Use 'provider/model' (e.g. openai/gpt-4)"
        ));
    }

    let provider_str = parts[0];
    let model_name = parts[1];

    if model_name.is_empty() {
        return Err(anyhow::anyhow!("Model name cannot be empty"));
    }

    let provider = parse_provider(provider_str)?;

    let base_url = prompt_base_url(provider)?;
    let api_key_env = prompt_api_key_env(provider)?;
    let temperature = prompt_temperature()?;
    let max_tokens = prompt_max_tokens()?;

    let model_config = ModelConfig {
        provider,
        model: model_name.to_string(),
        base_url,
        api_key_env,
        temperature,
        max_tokens,
    };

    cfg.model = Some(model_config.clone());
    Ok(model_config)
}

fn run_wizard(cfg: &mut AgentConfig) -> anyhow::Result<ModelConfig> {
    let provider = prompt_provider_kind()?;
    let model = prompt_model_name(provider)?;
    let base_url = prompt_base_url(provider)?;
    let api_key_env = prompt_api_key_env(provider)?;
    let temperature = prompt_temperature()?;
    let max_tokens = prompt_max_tokens()?;

    let model_config = ModelConfig {
        provider,
        model,
        base_url,
        api_key_env,
        temperature,
        max_tokens,
    };

    cfg.model = Some(model_config.clone());
    Ok(model_config)
}

fn prompt_provider_kind() -> anyhow::Result<ProviderKind> {
    let choices = vec!["openai", "anthropic", "google", "mock", "openai-compatible / custom"];

    let selection = Select::new()
        .with_prompt("Select provider type")
        .default(0)
        .items(&choices)
        .interact()?;

    let kind = match selection {
        0 => ProviderKind::OpenAi,
        1 => ProviderKind::Anthropic,
        2 => ProviderKind::Google,
        3 => ProviderKind::Mock,
        4 => ProviderKind::OpenAiCompatible,
        _ => ProviderKind::OpenAi,
    };

    Ok(kind)
}

fn prompt_model_name(provider: ProviderKind) -> anyhow::Result<String> {
    let default_model = match provider {
        ProviderKind::OpenAi => "gpt-4",
        ProviderKind::Anthropic => "claude-3-5-sonnet-20241022",
        ProviderKind::Google => "gemini-1.5-pro",
        ProviderKind::Mock => "mock-1",
        ProviderKind::OpenAiCompatible => "gpt-4",
        _ => "gpt-4",
    };

    let model = Input::new()
        .with_prompt("Enter model name")
        .default(default_model.to_string())
        .interact()?;

    if model.is_empty() {
        return Err(anyhow::anyhow!("Model name cannot be empty"));
    }

    Ok(model)
}

fn prompt_base_url(kind: ProviderKind) -> anyhow::Result<Option<String>> {
    let default_url: Option<String> = match kind {
        ProviderKind::OpenAi => Some("https://api.openai.com/v1".to_string()),
        ProviderKind::Anthropic => Some("https://api.anthropic.com".to_string()),
        ProviderKind::Google => Some("https://generativelanguage.googleapis.com".to_string()),
        ProviderKind::OpenAiCompatible => Some("".to_string()),
        ProviderKind::Mock => return Ok(None),
        _ => None,
    };

    let prompt_text = if let Some(ref url) = &default_url {
        if url.is_empty() {
            "Enter base URL (press Enter to use default): ".to_string()
        } else {
            format!("Enter base URL [{}] (press Enter to use default): ", url)
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
        Err(anyhow::anyhow!(
            "Invalid URL format. Must start with http:// or https://"
        ))
    }
}

fn prompt_api_key_env(kind: ProviderKind) -> anyhow::Result<Option<String>> {
    let default_env = match kind {
        ProviderKind::OpenAi => "OPENAI_API_KEY",
        ProviderKind::Anthropic => "ANTHROPIC_API_KEY",
        ProviderKind::Google => "GOOGLE_API_KEY",
        ProviderKind::OpenAiCompatible => "OPENAI_API_KEY",
        ProviderKind::Mock => return Ok(None),
        _ => "OPENAI_API_KEY",
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

fn prompt_temperature() -> anyhow::Result<Option<f32>> {
    let temp: String = Input::new()
        .with_prompt("Enter temperature (0.0-2.0, press Enter for default)")
        .allow_empty(true)
        .interact()?;

    if temp.is_empty() {
        Ok(None)
    } else {
        let parsed: f32 = temp
            .parse()
            .map_err(|_| anyhow::anyhow!("Invalid temperature value"))?;
        if parsed < 0.0 || parsed > 2.0 {
            return Err(anyhow::anyhow!("Temperature must be between 0.0 and 2.0"));
        }
        Ok(Some(parsed))
    }
}

fn prompt_max_tokens() -> anyhow::Result<Option<u32>> {
    let tokens: String = Input::new()
        .with_prompt("Enter max tokens (press Enter for no limit)")
        .allow_empty(true)
        .interact()?;

    if tokens.is_empty() {
        Ok(None)
    } else {
        let parsed: u32 = tokens
            .parse()
            .map_err(|_| anyhow::anyhow!("Invalid max tokens value"))?;
        Ok(Some(parsed))
    }
}

fn parse_provider(s: &str) -> anyhow::Result<ProviderKind> {
    let provider = match s.to_lowercase().as_str() {
        "openai" => ProviderKind::OpenAi,
        "anthropic" => ProviderKind::Anthropic,
        "google" => ProviderKind::Google,
        "mock" => ProviderKind::Mock,
        "custom" | "openai-compatible" => ProviderKind::OpenAiCompatible,
        _ => {
            return Err(anyhow::anyhow!(
                "Unknown provider '{}'. Available: openai, anthropic, google, mock, custom",
                s
            ))
        }
    };
    Ok(provider)
}

pub fn use_model(cfg: &mut AgentConfig, spec: &str) -> anyhow::Result<()> {
    let parts: Vec<&str> = spec.splitn(2, '/').collect();
    if parts.len() != 2 {
        return Err(anyhow::anyhow!(
            "Invalid format. Use 'provider/model' (e.g. openai/gpt-4)"
        ));
    }

    let provider_str = parts[0];
    let model_name = parts[1].to_string();

    if model_name.is_empty() {
        return Err(anyhow::anyhow!("Model name cannot be empty"));
    }

    let provider = parse_provider(provider_str)?;

    let (base_url, api_key_env) = resolve_provider_config(cfg, provider);

    cfg.model = Some(ModelConfig {
        provider,
        model: model_name,
        base_url,
        api_key_env,
        temperature: None,
        max_tokens: None,
    });

    Ok(())
}

fn resolve_provider_config(
    cfg: &AgentConfig,
    provider: ProviderKind,
) -> (Option<String>, Option<String>) {
    for (_id, pconf) in &cfg.providers {
        if pconf.kind == provider {
            return (pconf.base_url.clone(), pconf.api_key_env.clone());
        }
    }

    match provider {
        ProviderKind::OpenAi => (None, Some("OPENAI_API_KEY".to_string())),
        ProviderKind::Anthropic => (None, Some("ANTHROPIC_API_KEY".to_string())),
        ProviderKind::Google => (None, Some("GOOGLE_API_KEY".to_string())),
        ProviderKind::Mock => (None, None),
        ProviderKind::OpenAiCompatible | ProviderKind::Custom => (None, Some("OPENAI_API_KEY".to_string())),
    }
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