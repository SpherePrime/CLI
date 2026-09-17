 use agent_config::ProviderKind;
 use dialoguer::{Input, Select};
 
 pub fn prompt_provider_kind() -> anyhow::Result<ProviderKind> {
     let choices = vec![
         "openai",
         "anthropic",
         "google",
         "openai-compatible (custom)",
         "mock (for testing)",
     ];
 
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
 
 pub fn prompt_base_url(kind: ProviderKind) -> anyhow::Result<Option<String>> {
     let default_url: Option<String> = match kind {
         ProviderKind::OpenAi => Some("https://api.openai.com/v1".to_string()),
         ProviderKind::Anthropic => Some("https://api.anthropic.com".to_string()),
         ProviderKind::Google => Some("https://generativelanguage.googleapis.com".to_string()),
         ProviderKind::OpenAiCompatible => Some(String::new()),
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
