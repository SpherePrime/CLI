 pub mod parse;
 pub mod prompt;
 
 use agent_config::{AgentConfig, ModelConfig};
 use dialoguer::Input;
 
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
         println!("  provider: {}", parse::provider_name(&mc.provider));
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
 
     let model_name = parts[1];
     if model_name.is_empty() {
         return Err(anyhow::anyhow!("Model name cannot be empty"));
     }
 
     let provider = parse::parse_provider(parts[0])?;
     let base_url = prompt::prompt_base_url(provider)?;
     let api_key_env = prompt::prompt_api_key_env(provider)?;
     let temperature = prompt::prompt_temperature()?;
     let max_tokens = prompt::prompt_max_tokens()?;
 
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
     let provider = prompt::prompt_provider_kind()?;
     let model = prompt::prompt_model_name(provider)?;
     let base_url = prompt::prompt_base_url(provider)?;
     let api_key_env = prompt::prompt_api_key_env(provider)?;
     let temperature = prompt::prompt_temperature()?;
     let max_tokens = prompt::prompt_max_tokens()?;
 
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
 
 pub fn use_model(cfg: &mut AgentConfig, spec: &str) -> anyhow::Result<()> {
     let parts: Vec<&str> = spec.splitn(2, '/').collect();
     if parts.len() != 2 {
         return Err(anyhow::anyhow!(
             "Invalid format. Use 'provider/model' (e.g. openai/gpt-4)"
         ));
     }
 
     let model_name = parts[1].to_string();
     if model_name.is_empty() {
         return Err(anyhow::anyhow!("Model name cannot be empty"));
     }
 
     let provider = parse::parse_provider(parts[0])?;
     let (base_url, api_key_env) = parse::resolve_provider_config(cfg, provider);
 
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
