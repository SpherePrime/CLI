use std::fmt;

use agent_config::{ModelConfig, ProviderKind};
use anyhow::{anyhow, Result};
use async_trait::async_trait;
use futures::stream::{BoxStream, StreamExt};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatMessage {
    pub role: Role,
    pub content: MessageContent,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_calls: Option<Vec<ToolCall>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_call_id: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum Role {
    System,
    User,
    Assistant,
    Tool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum MessageContent {
    Text(String),
    Multi { parts: Vec<ContentPart> },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum ContentPart {
    Text { text: String },
    ToolCall {
        id: String,
        name: String,
        args: serde_json::Value,
    },
}

impl MessageContent {
    pub fn as_text(&self) -> String {
        match self {
            MessageContent::Text(s) => s.clone(),
            MessageContent::Multi { parts } => parts
                .iter()
                .filter_map(|p| match p {
                    ContentPart::Text { text } => Some(text.as_str()),
                    ContentPart::ToolCall { .. } => None,
                })
                .collect::<Vec<_>>()
                .join(""),
        }
    }

    pub fn role_name(role: &Role) -> &'static str {
        match role {
            Role::System => "system",
            Role::User => "user",
            Role::Assistant => "assistant",
            Role::Tool => "tool",
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolCall {
    pub id: String,
    pub name: String,
    pub args: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelRequest {
    pub model: String,
    pub messages: Vec<ChatMessage>,
    pub tools: Option<Vec<ToolSchema>>,
    pub temperature: Option<f32>,
    pub max_tokens: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolSchema {
    pub name: String,
    pub description: String,
    pub input_schema: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelResponse {
    pub content: MessageContent,
    pub tool_calls: Vec<ToolCall>,
    pub stop_reason: StopReason,
    pub usage: Usage,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum StopReason {
    EndTurn,
    ToolUse,
    MaxTokens,
    Error,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Usage {
    pub input_tokens: u64,
    pub output_tokens: u64,
}

impl Default for Usage {
    fn default() -> Self {
        Self {
            input_tokens: 0,
            output_tokens: 0,
        }
    }
}

impl fmt::Display for Usage {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "in={} out={}",
            self.input_tokens, self.output_tokens
        )
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum StreamChunk {
    TextDelta(String),
    ToolCallDelta {
        id: String,
        name: String,
        partial_args: serde_json::Value,
    },
    Done { response: ModelResponse },
}

#[derive(Debug, thiserror::Error)]
pub enum ModelError {
    #[error("auth failed for {0}")]
    Auth(String),
    #[error("rate limited")]
    RateLimited,
    #[error("upstream error: {0}")]
    Upstream(String),
    #[error("invalid request: {0}")]
    InvalidRequest(String),
    #[error("network error: {0}")]
    Network(#[source] std::io::Error),
}

impl From<std::io::Error> for ModelError {
    fn from(e: std::io::Error) -> Self {
        Self::Network(e)
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

pub struct ProviderConfig {
    pub kind: ProviderKind,
    pub model: String,
    pub base_url: Option<String>,
    pub api_key: Option<String>,
}

pub fn build_from_model_config(mc: &ModelConfig) -> Result<Box<dyn ModelProvider>> {
    let api_key = mc
        .api_key_env
        .as_ref()
        .and_then(|n| std::env::var(n).ok());
    let pc = ProviderConfig {
        kind: mc.provider,
        model: mc.model.clone(),
        base_url: mc.base_url.clone(),
        api_key,
    };
    match mc.provider {
        ProviderKind::OpenAi => Ok(Box::new(OpenAiProvider::new(pc))),
        ProviderKind::Anthropic => Ok(Box::new(AnthropicProvider::new(pc))),
        ProviderKind::Google => Ok(Box::new(OpenAiCompatibleProvider::new(pc))),
        ProviderKind::OpenAiCompatible => Ok(Box::new(OpenAiCompatibleProvider::new(pc))),
        ProviderKind::Custom => Ok(Box::new(OpenAiCompatibleProvider::new(pc))),
        ProviderKind::Mock => Ok(Box::new(MockProvider::new())),
    }
}

pub struct OpenAiProvider {
    cfg: ProviderConfig,
}

impl OpenAiProvider {
    pub fn new(pc: ProviderConfig) -> Self {
        Self { cfg: pc }
    }

    async fn do_chat(&self, req: &ModelRequest) -> Result<ModelResponse> {
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
    ) -> Result<BoxStream<'static, std::result::Result<StreamChunk, ModelError>>> {
        let resp = self.do_chat(req).await?;
        Ok(futures::stream::iter(vec![Ok(StreamChunk::Done {
            response: resp,
        })])
        .boxed())
    }
}

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
    ) -> Result<BoxStream<'static, std::result::Result<StreamChunk, ModelError>>> {
        let resp = self.do_chat(req).await?;
        Ok(futures::stream::iter(vec![Ok(StreamChunk::Done {
            response: resp,
        })])
        .boxed())
    }
}

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
    ) -> Result<BoxStream<'static, std::result::Result<StreamChunk, ModelError>>> {
        let resp = self.do_chat(req).await?;
        Ok(futures::stream::iter(vec![Ok(StreamChunk::Done {
            response: resp,
        })])
        .boxed())
    }
}

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

    async fn list_models(&self) -> Result<Vec<String>> {
        Ok(vec![self.model.clone()])
    }

    async fn chat(&self, req: &ModelRequest) -> Result<ModelResponse> {
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
    ) -> Result<BoxStream<'static, std::result::Result<StreamChunk, ModelError>>> {
        let resp = self.chat(req).await?;
        Ok(futures::stream::iter(vec![Ok(StreamChunk::Done {
            response: resp,
        })])
        .boxed())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn mock_provider_echo() {
        let p = MockProvider::new();
        let resp = p
            .chat(&ModelRequest {
                model: "mock".into(),
                messages: vec![ChatMessage {
                    role: Role::User,
                    content: MessageContent::Text("hello".into()),
                    tool_calls: None,
                    tool_call_id: None,
                }],
                tools: None,
                temperature: None,
                max_tokens: None,
            })
            .await
            .unwrap();
        assert!(resp.content.as_text().contains("hello"));
        assert_eq!(resp.stop_reason, StopReason::EndTurn);
    }

    #[tokio::test]
    async fn build_mock_provider() {
        let mc = ModelConfig {
            provider: ProviderKind::Mock,
            model: "mock".into(),
            base_url: None,
            api_key_env: None,
            temperature: None,
            max_tokens: None,
        };
        let p = build_from_model_config(&mc).unwrap();
        let name = p.name().await;
        assert_eq!(name, "mock");
    }
}
