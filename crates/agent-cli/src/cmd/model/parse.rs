 use agent_config::ProviderKind;
 
 pub fn parse_provider(s: &str) -> anyhow::Result<ProviderKind> {
     let provider = match s.to_lowercase().as_str() {
         "openai" => ProviderKind::OpenAi,
         "anthropic" => ProviderKind::Anthropic,
         "google" => ProviderKind::Google,
         "mock" => ProviderKind::Mock,
         "custom" | "openai-compatible" => ProviderKind::OpenAiCompatible,
         _ => {
             return Err(anyhow::anyhow!(
                 "Unknown provider '{s}'. Available: openai, anthropic, google, mock, custom"
             ))
         }
     };
     Ok(provider)
 }
 
 pub fn resolve_provider_config(
     cfg: &agent_config::AgentConfig,
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
         ProviderKind::OpenAiCompatible | ProviderKind::Custom => {
             (None, Some("OPENAI_API_KEY".to_string()))
         }
     }
 }
 
 pub fn provider_name(kind: &ProviderKind) -> &'static str {
     match kind {
         ProviderKind::OpenAi => "openai",
         ProviderKind::Anthropic => "anthropic",
         ProviderKind::Google => "google",
         ProviderKind::OpenAiCompatible => "openai-compatible",
         ProviderKind::Custom => "custom",
         ProviderKind::Mock => "mock",
     }
 }
