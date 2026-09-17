 use agent_config::ProviderKind;
 use dialoguer::{Input, Select};
 
 pub fn prompt_provider_kind() -> anyhow::Result<ProviderKind> {
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
 
 pub fn prompt_model_name(provider: ProviderKind) -> anyhow::Result<String> {
     let default_model = match provider {
         ProviderKind::OpenAi => "gpt-4",
         ProviderKind::Anthropic => "claude-3-5-sonnet-20241022",
         ProviderKind::Google => "gemini-1.5-pro",
         ProviderKind::Mock => "mock-1",
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
 
 pub fn prompt_base_url(kind: ProviderKind) -> anyhow::Result<Option<String>> {
     let default_url: Option<String> = match kind {
         ProviderKind::OpenAi => Some("https://api.openai.com/v1".to_string()),
         ProviderKind::Anthropic => Some("https://api.anthropic.com".to_string()),
         ProviderKind::Google => Some("https://generativelanguage.googleapis.com".to_string()),
         ProviderKind::OpenAiCompatible => Some(String::new()),
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
 
 pub fn prompt_api_key_env(kind: ProviderKind) -> anyhow::Result<Option<String>> {
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
 
 pub fn prompt_temperature() -> anyhow::Result<Option<f32>> {
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
 
 pub fn prompt_max_tokens() -> anyhow::Result<Option<u32>> {
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
