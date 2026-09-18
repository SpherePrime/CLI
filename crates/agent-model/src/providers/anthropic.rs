use anyhow::Result;
use async_trait::async_trait;
use futures::stream::BoxStream;
use tokio_util::sync::CancellationToken;

use crate::provider::{ModelProvider, ProviderConfig};
use crate::providers::{http, stream};
use crate::types::{ModelRequest, ModelResponse, StreamChunk};

pub struct AnthropicProvider {
    cfg: ProviderConfig,
}

impl AnthropicProvider {
    pub fn new(pc: ProviderConfig) -> Self {
        Self { cfg: pc }
    }

    fn with_env_key(&self) -> ProviderConfig {
        let mut cfg = self.cfg.clone();
        if cfg.resolved_api_key().is_none() {
            cfg.api_key = std::env::var("ANTHROPIC_API_KEY").ok();
        }
        cfg
    }

    async fn do_chat(&self, req: &ModelRequest) -> Result<ModelResponse> {
        http::post_anthropic_messages(&self.with_env_key(), req).await
    }
}

#[async_trait]
impl ModelProvider for AnthropicProvider {
    async fn name(&self) -> &'static str {
        "anthropic"
    }

    async fn list_models(&self) -> Result<Vec<String>> {
        Ok(vec![self.cfg.model.clone()])
    }

    async fn chat(&self, req: &ModelRequest) -> Result<ModelResponse> {
        tracing::debug!(model = req.model.as_str(), "anthropic chat");
        self.do_chat(req).await
    }

    async fn chat_stream(
        &self,
        req: &ModelRequest,
        cancel: CancellationToken,
    ) -> Result<BoxStream<'static, std::result::Result<StreamChunk, crate::error::ModelError>>>
    {
        stream::anthropic_stream(&self.with_env_key(), req, cancel).await
    }
}
