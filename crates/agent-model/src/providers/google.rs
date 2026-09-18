use anyhow::{Context, Result};
use async_trait::async_trait;
use futures::stream::BoxStream;
use serde_json::{json, Value};
use tokio_util::sync::CancellationToken;

use crate::provider::{ModelProvider, ProviderConfig};
use crate::providers::{http, stream};
use crate::types::{ModelRequest, ModelResponse, StopReason, StreamChunk, ToolCall, Usage};

const GOOGLE_BASE: &str = "https://generativelanguage.googleapis.com/v1beta";

pub struct GoogleProvider {
    cfg: ProviderConfig,
}

impl GoogleProvider {
    pub fn new(pc: ProviderConfig) -> Self {
        Self { cfg: pc }
    }

    fn with_env_key(&self) -> ProviderConfig {
        let mut cfg = self.cfg.clone();
        if cfg.resolved_api_key().is_none() {
            cfg.api_key = std::env::var("GEMINI_API_KEY")
                .or_else(|_| std::env::var("GOOGLE_API_KEY"))
                .ok();
        }
        cfg
    }

    fn build_body(&self, req: &ModelRequest) -> Value {
        let (system, contents, tools) = stream::to_google_request(req);
        let mut body = json!({ "contents": contents });
        if let Some(system) = system {
            body["systemInstruction"] = system;
        }
        if let Some(tools) = tools {
            body["tools"] = json!([tools]);
        }
        let mut config = json!({});
        if let Some(temp) = req.temperature {
            config["temperature"] = json!(temp);
        }
        if let Some(max) = req.max_tokens {
            config["maxOutputTokens"] = json!(max);
        }
        if config.as_object().map(|o| !o.is_empty()).unwrap_or(false) {
            body["generationConfig"] = config;
        }
        body
    }
}

pub fn parse_google_response(body: Value) -> Result<ModelResponse> {
    let candidate = body["candidates"]
        .as_array()
        .and_then(|c| c.first())
        .context("empty candidates in provider response")?;
    let mut text = String::new();
    let mut tool_calls: Vec<ToolCall> = Vec::new();
    if let Some(parts) = candidate["content"]["parts"].as_array() {
        for part in parts {
            if let Some(chunk) = part["text"].as_str() {
                text.push_str(chunk);
            }
            if let Some(call) = part.get("functionCall") {
                let index = tool_calls.len();
                tool_calls.push(ToolCall {
                    id: format!("call_{index}"),
                    name: call["name"].as_str().unwrap_or("").to_string(),
                    args: call["args"].clone(),
                });
            }
        }
    }
    let usage = Usage {
        input_tokens: body["usageMetadata"]["promptTokenCount"]
            .as_u64()
            .unwrap_or(0),
        output_tokens: body["usageMetadata"]["candidatesTokenCount"]
            .as_u64()
            .unwrap_or(0),
    };
    let stop_reason = match candidate["finishReason"].as_str().unwrap_or("STOP") {
        "MAX_TOKENS" => StopReason::MaxTokens,
        _ if !tool_calls.is_empty() => StopReason::ToolUse,
        _ => StopReason::EndTurn,
    };
    Ok(ModelResponse {
        content: http::build_content(&text, &tool_calls),
        tool_calls,
        stop_reason,
        usage,
    })
}

#[async_trait]
impl ModelProvider for GoogleProvider {
    async fn name(&self) -> &'static str {
        "google"
    }

    async fn list_models(&self) -> Result<Vec<String>> {
        Ok(vec![self.cfg.model.clone()])
    }

    async fn chat(&self, req: &ModelRequest) -> Result<ModelResponse> {
        let cfg = self.with_env_key();
        let key = cfg
            .resolved_api_key()
            .context("no API key configured for provider")?;
        let base = cfg
            .base_url
            .as_deref()
            .unwrap_or(GOOGLE_BASE)
            .trim_end_matches('/');
        let url = format!("{base}/models/{}:generateContent?key={key}", req.model);
        let client = reqwest::Client::new();
        let response = client
            .post(&url)
            .json(&self.build_body(req))
            .send()
            .await
            .with_context(|| format!("request to {url} failed"))?;
        let status = response.status();
        let text = response
            .text()
            .await
            .context("failed to read response body")?;
        if !status.is_success() {
            anyhow::bail!("provider returned {status}: {text}");
        }
        let value: Value = serde_json::from_str(&text).context("provider returned invalid JSON")?;
        parse_google_response(value)
    }

    async fn chat_stream(
        &self,
        req: &ModelRequest,
        cancel: CancellationToken,
    ) -> Result<BoxStream<'static, std::result::Result<StreamChunk, crate::error::ModelError>>>
    {
        stream::google_stream(&self.with_env_key(), req, cancel).await
    }
}
