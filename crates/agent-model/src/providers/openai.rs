 use anyhow::{anyhow, Result};
 use async_trait::async_trait;
 use futures::stream::{BoxStream, StreamExt};
 
 use crate::provider::{ModelProvider, ProviderConfig};
 use crate::types::{ModelRequest, ModelResponse, StreamChunk};
 
 pub struct OpenAiProvider {
     cfg: ProviderConfig,
 }
 
 impl OpenAiProvider {
     pub fn new(pc: ProviderConfig) -> Self {
         Self { cfg: pc }
     }
 
     async fn do_chat(&self, _req: &ModelRequest) -> Result<ModelResponse> {
         if self.cfg.api_key.is_none() {
             let _ = std::env::var("OPENAI_API_KEY");
         }
         Err(anyhow!(
             "OpenAI provider requires a real API key and base URL; this is a stub in this build"
         ))
     }
 }
 
 #[async_trait]
 impl ModelProvider for OpenAiProvider {
     async fn name(&self) -> &'static str {
         "openai"
     }
 
     async fn list_models(&self) -> Result<Vec<String>> {
         Ok(vec![self.cfg.model.clone()])
     }
 
     async fn chat(&self, req: &ModelRequest) -> Result<ModelResponse> {
         tracing::debug!(model = req.model.as_str(), "openai chat");
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
