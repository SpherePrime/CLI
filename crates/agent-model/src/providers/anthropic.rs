 use anyhow::{anyhow, Result};
 use async_trait::async_trait;
 use futures::stream::{BoxStream, StreamExt};
 
 use crate::provider::{ModelProvider, ProviderConfig};
 use crate::types::{ModelRequest, ModelResponse, StreamChunk};
 
 pub struct AnthropicProvider {
     cfg: ProviderConfig,
 }
 
 impl AnthropicProvider {
     pub fn new(pc: ProviderConfig) -> Self {
         Self { cfg: pc }
     }
 
     async fn do_chat(&self, _req: &ModelRequest) -> Result<ModelResponse> {
         Err(anyhow!(
             "Anthropic provider requires a real API key; this is a stub in this build"
         ))
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
     ) -> Result<BoxStream<'static, std::result::Result<StreamChunk, crate::error::ModelError>>> {
         let resp = self.do_chat(req).await?;
         Ok(futures::stream::iter(vec![Ok(StreamChunk::Done {
             response: resp,
         })])
         .boxed())
     }
 }
