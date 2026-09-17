use agent_config::ModelConfig;
use anyhow::{anyhow, Result};
use async_trait::async_trait;
use futures::stream::BoxStream;

use crate::error::ModelError;
use crate::types::{ModelRequest, ModelResponse, StreamChunk};

#[derive(Clone, Default)]
pub struct ProviderConfig {
    pub kind: agent_config::ProviderKind,
    pub model: String,
    pub base_url: Option<String>,
    pub api_key: Option<String>,
}

impl ProviderConfig {
    pub fn resolved_api_key(&self) -> Option<String> {
        self.api_key
            .clone()
            .filter(|k| !k.is_empty())
    }
}

#[async_trait]
pub trait ModelProvider: Send + Sync + 'static {
    async fn name(&self) -> &'static str;
    async fn list_models(&self) -> Result<Vec<String>>;
    async fn chat(&self, request: &ModelRequest) -> Result<ModelResponse>;
    async fn chat_stream(
        &self,
        request: &ModelRequest,
    ) -> Result<BoxStream<'static, std::result::Result<StreamChunk, ModelError>>>;
}
 
pub fn build_from_model_config(mc: &ModelConfig) -> Result<Box<dyn ModelProvider>> {
    let api_key = mc
        .api_key_env
        .as_ref()
        .map(|value| std::env::var(value).unwrap_or_else(|_| value.clone()))
        .filter(|key| !key.is_empty());
     let pc = ProviderConfig {
         kind: mc.provider,
         model: mc.model.clone(),
         base_url: mc.base_url.clone(),
         api_key,
     };
     match mc.provider {
         agent_config::ProviderKind::OpenAi => Ok(Box::new(
             crate::providers::openai::OpenAiProvider::new(pc),
         )),
         agent_config::ProviderKind::Anthropic => Ok(Box::new(
             crate::providers::anthropic::AnthropicProvider::new(pc),
         )),
         agent_config::ProviderKind::Google => Ok(Box::new(
             crate::providers::compat::OpenAiCompatibleProvider::new(pc),
         )),
         agent_config::ProviderKind::OpenAiCompatible => Ok(Box::new(
             crate::providers::compat::OpenAiCompatibleProvider::new(pc),
         )),
         agent_config::ProviderKind::Custom => Ok(Box::new(
             crate::providers::compat::OpenAiCompatibleProvider::new(pc),
         )),
         agent_config::ProviderKind::Mock => Ok(Box::new(
             crate::providers::mock::MockProvider::new(),
         )),
     }
 }
