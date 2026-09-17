 use anyhow::{anyhow, Result};
 use async_trait::async_trait;
 use futures::stream::{BoxStream, StreamExt};
 
 use crate::provider::{ModelProvider, ProviderConfig};
 use crate::types::{ModelRequest, ModelResponse, StreamChunk};
 
 pub struct OpenAiCompatibleProvider {
     cfg: ProviderConfig,
 }
 
 impl OpenAiCompatibleProvider {
     pub fn new(pc: ProviderConfig) -> Self {
         Self { cfg: pc }
     }
 
     async fn do_chat(&self, _req: &ModelRequest) -> Result<ModelResponse> {
         if self.cfg.base_url.is_none() {
             return Err(anyhow!(
                 "OpenAI-compatible provider requires base_url in config"
             ));
         }
         Err(anyhow!(
             "OpenAI-compatible provider is a stub; configure a real endpoint"
         ))
     }
 }
 
 #[async_trait]
 impl ModelProvider for OpenAiCompatibleProvider {
     async fn name(&self) -> &'static str {
         "openai-compatible"
     }
 
     async fn list_models(&self) -> Result<Vec<String>> {
         Ok(vec![self.cfg.model.clone()])
     }
 
     async fn chat(&self, req: &ModelRequest) -> Result<ModelResponse> {
         tracing::debug!(model = req.model.as_str(), "compat chat");
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
