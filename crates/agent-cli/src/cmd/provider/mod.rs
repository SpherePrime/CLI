 pub mod prompt;
 
 use agent_config::{AgentConfig, ModelConfig, ProviderConfig, ProviderKind};
 use dialoguer::Input;
 
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
         println!("  kind: {}", provider_kind_name(&provider.kind));
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
         return Err(anyhow::anyhow!("Provider '{provider_id}' already exists"));
     }
 
     let kind = prompt::prompt_provider_kind()?;
     let base_url = prompt::prompt_base_url(kind)?;
     let api_key_env = prompt::prompt_api_key_env(kind)?;
 
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
 
 pub fn use_provider(cfg: &mut AgentConfig, id: &str) -> anyhow::Result<()> {
     if !cfg.providers.contains_key(id) {
         return Err(anyhow::anyhow!("Provider '{id}' not found"));
     }
 
     let provider = cfg.providers.get(id).cloned().unwrap();
 
     cfg.model = Some(ModelConfig {
         provider: provider.kind,
         model: "gpt-4".to_string(),
         base_url: provider.base_url,
         api_key_env: provider.api_key_env,
         temperature: provider.temperature,
         max_tokens: provider.max_tokens,
     });
 
     Ok(())
 }
 
 pub fn provider_kind_name(kind: &ProviderKind) -> &'static str {
     match kind {
         ProviderKind::OpenAi => "openai",
         ProviderKind::Anthropic => "anthropic",
         ProviderKind::Google => "google",
         ProviderKind::OpenAiCompatible => "openai-compatible",
         ProviderKind::Custom => "custom",
         ProviderKind::Mock => "mock",
     }
 }
