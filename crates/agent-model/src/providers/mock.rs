use async_trait::async_trait;
use futures::stream::{BoxStream, StreamExt};

use crate::provider::{ModelProvider, ProviderConfig};
 use crate::types::{
     ChatMessage, MessageContent, ModelRequest, ModelResponse, Role, StreamChunk, StopReason,
     Usage,
 };
 
 pub struct MockProvider {
     model: String,
 }
 
 impl MockProvider {
     pub fn new() -> Self {
         Self {
             model: "mock-1".into(),
         }
     }
 
     pub fn with_model(mut self, model: &str) -> Self {
         self.model = model.to_string();
         self
     }
 }
 
 impl Default for MockProvider {
     fn default() -> Self {
         Self::new()
     }
 }
 
 #[async_trait]
 impl ModelProvider for MockProvider {
     async fn name(&self) -> &'static str {
         "mock"
     }
 
     async fn list_models(&self) -> anyhow::Result<Vec<String>> {
         Ok(vec![self.model.clone()])
     }
 
     async fn chat(&self, req: &ModelRequest) -> anyhow::Result<ModelResponse> {
         let last_user = req
             .messages
             .iter()
             .rfind(|m| m.role == Role::User)
             .map(|m| m.content.as_text().to_string())
             .unwrap_or_default();
         Ok(ModelResponse {
             content: MessageContent::Text(format!("[mock] echo: {last_user}")),
             tool_calls: vec![],
             stop_reason: StopReason::EndTurn,
             usage: Usage {
                 input_tokens: last_user.len() as u64 / 4,
                 output_tokens: 4,
             },
         })
     }
 
     async fn chat_stream(
         &self,
         req: &ModelRequest,
     ) -> anyhow::Result<BoxStream<'static, std::result::Result<StreamChunk, crate::error::ModelError>>>
     {
         let resp = self.chat(req).await?;
         Ok(futures::stream::iter(vec![Ok(StreamChunk::Done {
             response: resp,
         })])
         .boxed())
     }
 }
