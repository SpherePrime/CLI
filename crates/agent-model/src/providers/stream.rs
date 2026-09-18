use anyhow::{Context, Result};
use futures::stream::BoxStream;
use serde_json::{json, Value};
use tokio::sync::mpsc;

use crate::error::ModelError;
use crate::provider::ProviderConfig;
use crate::providers::http;
use crate::types::{ModelRequest, ModelResponse, Role, StopReason, StreamChunk, ToolCall, Usage};

const OPENAI_BASE: &str = "https://api.openai.com/v1";
const ANTHROPIC_BASE: &str = "https://api.anthropic.com";
const GOOGLE_BASE: &str = "https://generativelanguage.googleapis.com/v1beta";

pub type ChunkResult = std::result::Result<StreamChunk, ModelError>;
pub type ChunkStream = BoxStream<'static, ChunkResult>;

#[derive(Debug, Default, Clone)]
struct PartialToolCall {
    index: usize,
    id: String,
    name: String,
    args: String,
}

fn receiver_stream(rx: mpsc::Receiver<ChunkResult>) -> ChunkStream {
    Box::pin(futures::stream::unfold(rx, |mut rx| async move {
        rx.recv().await.map(|item| (item, rx))
    }))
}

fn status_error(status: reqwest::StatusCode, body: String) -> anyhow::Error {
    let error = match status.as_u16() {
        401 | 403 => ModelError::Auth(body),
        429 => ModelError::RateLimited,
        _ => ModelError::Upstream(format!("{status}: {body}")),
    };
    anyhow::Error::new(error)
}

async fn open_sse(
    url: String,
    headers: Vec<(String, String)>,
    body: &Value,
) -> Result<mpsc::Receiver<Value>> {
    let client = reqwest::Client::builder()
        .build()
        .context("failed to build http client")?;
    let mut request = client.post(&url);
    for (name, value) in &headers {
        request = request.header(name, value);
    }
    let response = request
        .json(body)
        .send()
        .await
        .with_context(|| format!("request to {url} failed"))?;
    let status = response.status();
    if !status.is_success() {
        let text = response.text().await.unwrap_or_default();
        return Err(status_error(status, text));
    }
    let (tx, rx) = mpsc::channel::<Value>(64);
    tokio::spawn(async move {
        let mut response = response;
        let mut buffer = String::new();
        loop {
            match response.chunk().await {
                Ok(Some(bytes)) => {
                    buffer.push_str(&String::from_utf8_lossy(&bytes));
                    while let Some(pos) = buffer.find('\n') {
                        let line = buffer[..pos].trim_end_matches('\r').to_string();
                        buffer.drain(..pos + 1);
                        let Some(data) = line.strip_prefix("data:") else {
                            continue;
                        };
                        let data = data.trim();
                        if data.is_empty() || data == "[DONE]" {
                            continue;
                        }
                        if let Ok(value) = serde_json::from_str::<Value>(data) {
                            if tx.send(value).await.is_err() {
                                return;
                            }
                        }
                    }
                }
                Ok(None) => break,
                Err(error) => {
                    let _ = tx.send(json!({ "_stream_error": error.to_string() })).await;
                    break;
                }
            }
        }
    });
    Ok(rx)
}

fn finish_response(
    text: &str,
    tools: &[PartialToolCall],
    finish: &Option<String>,
    usage: &Usage,
) -> ModelResponse {
    let tool_calls: Vec<ToolCall> = tools
        .iter()
        .filter(|tool| !tool.name.is_empty())
        .map(|tool| ToolCall {
            id: if tool.id.is_empty() {
                format!("call_{}", tool.index)
            } else {
                tool.id.clone()
            },
            name: tool.name.clone(),
            args: serde_json::from_str(&tool.args).unwrap_or(Value::Null),
        })
        .collect();
    let stop_reason = match finish.as_deref() {
        Some("tool_calls") | Some("tool_use") | Some("function_call") => StopReason::ToolUse,
        Some("length") | Some("max_tokens") | Some("MAX_TOKENS") => StopReason::MaxTokens,
        _ if !tool_calls.is_empty() => StopReason::ToolUse,
        _ => StopReason::EndTurn,
    };
    ModelResponse {
        content: http::build_content(text, &tool_calls),
        tool_calls,
        stop_reason,
        usage: usage.clone(),
    }
}

fn openai_process(
    value: &Value,
    text: &mut String,
    tools: &mut Vec<PartialToolCall>,
    finish: &mut Option<String>,
    usage: &mut Usage,
    out: &mut Vec<StreamChunk>,
) {
    if let Some(u) = value.get("usage").filter(|v| !v.is_null()) {
        let parsed = Usage {
            input_tokens: u["prompt_tokens"].as_u64().unwrap_or(usage.input_tokens),
            output_tokens: u["completion_tokens"]
                .as_u64()
                .unwrap_or(usage.output_tokens),
        };
        if parsed.input_tokens > 0 || parsed.output_tokens > 0 {
            *usage = parsed.clone();
            out.push(StreamChunk::Usage(parsed));
        }
    }
    let Some(choice) = value["choices"].as_array().and_then(|c| c.first()) else {
        return;
    };
    if let Some(reason) = choice["finish_reason"].as_str() {
        *finish = Some(reason.to_string());
    }
    let delta = &choice["delta"];
    if let Some(content) = delta["content"].as_str() {
        if !content.is_empty() {
            text.push_str(content);
            out.push(StreamChunk::TextDelta(content.to_string()));
        }
    }
    if let Some(reasoning) = delta["reasoning_content"].as_str() {
        if !reasoning.is_empty() {
            out.push(StreamChunk::ReasoningDelta(reasoning.to_string()));
        }
    }
    if let Some(calls) = delta["tool_calls"].as_array() {
        for call in calls {
            let index = call["index"].as_u64().unwrap_or(tools.len() as u64) as usize;
            while tools.len() <= index {
                let next = tools.len();
                tools.push(PartialToolCall {
                    index: next,
                    ..Default::default()
                });
            }
            let slot = &mut tools[index];
            if let Some(id) = call["id"].as_str() {
                if !id.is_empty() {
                    slot.id = id.to_string();
                }
            }
            if let Some(name) = call["function"]["name"].as_str() {
                if !name.is_empty() {
                    slot.name = name.to_string();
                }
            }
            let args_delta = call["function"]["arguments"]
                .as_str()
                .unwrap_or("")
                .to_string();
            if !args_delta.is_empty() {
                slot.args.push_str(&args_delta);
            }
            out.push(StreamChunk::ToolCallDelta {
                index,
                id: if slot.id.is_empty() {
                    None
                } else {
                    Some(slot.id.clone())
                },
                name: if slot.name.is_empty() {
                    None
                } else {
                    Some(slot.name.clone())
                },
                args_delta,
            });
        }
    }
}

fn openai_chunks(mut rx: mpsc::Receiver<Value>) -> ChunkStream {
    let (tx, out) = mpsc::channel::<ChunkResult>(64);
    tokio::spawn(async move {
        let mut text = String::new();
        let mut tools: Vec<PartialToolCall> = Vec::new();
        let mut finish: Option<String> = None;
        let mut usage = Usage::default();
        while let Some(value) = rx.recv().await {
            if let Some(error) = value.get("_stream_error").and_then(|v| v.as_str()) {
                let _ = tx.send(Err(ModelError::Stream(error.to_string()))).await;
                continue;
            }
            let mut chunks = Vec::new();
            openai_process(
                &value,
                &mut text,
                &mut tools,
                &mut finish,
                &mut usage,
                &mut chunks,
            );
            for chunk in chunks {
                if tx.send(Ok(chunk)).await.is_err() {
                    return;
                }
            }
        }
        let response = finish_response(&text, &tools, &finish, &usage);
        let _ = tx.send(Ok(StreamChunk::Done { response })).await;
    });
    receiver_stream(out)
}

pub async fn openai_stream(cfg: &ProviderConfig, req: &ModelRequest) -> Result<ChunkStream> {
    let key = cfg
        .resolved_api_key()
        .context("no API key configured for provider")?;
    let base = cfg
        .base_url
        .as_deref()
        .unwrap_or(OPENAI_BASE)
        .trim_end_matches('/');
    let url = format!("{base}/chat/completions");
    let mut body = json!({
        "model": req.model,
        "messages": http::to_openai_messages(req),
        "stream": true,
        "stream_options": { "include_usage": true },
    });
    if let Some(temp) = req.temperature {
        body["temperature"] = json!(temp);
    }
    if let Some(max) = req.max_tokens {
        body["max_tokens"] = json!(max);
    }
    if let Some(tools) = &req.tools {
        body["tools"] = json!(tools
            .iter()
            .map(http::openai_tool_schema)
            .collect::<Vec<_>>());
    }
    let headers = vec![("Authorization".to_string(), format!("Bearer {key}"))];
    let rx = open_sse(url, headers, &body).await?;
    Ok(openai_chunks(rx))
}

fn anthropic_process(
    value: &Value,
    text: &mut String,
    tools: &mut Vec<PartialToolCall>,
    current: &mut Option<usize>,
    finish: &mut Option<String>,
    usage: &mut Usage,
    out: &mut Vec<StreamChunk>,
) {
    match value["type"].as_str() {
        Some("message_start") => {
            usage.input_tokens = value["message"]["usage"]["input_tokens"]
                .as_u64()
                .unwrap_or(usage.input_tokens);
            out.push(StreamChunk::Usage(usage.clone()));
        }
        Some("content_block_start") => {
            let block = &value["content_block"];
            if block["type"].as_str() == Some("tool_use") {
                let index = tools.len();
                tools.push(PartialToolCall {
                    index,
                    id: block["id"].as_str().unwrap_or("").to_string(),
                    name: block["name"].as_str().unwrap_or("").to_string(),
                    args: String::new(),
                });
                *current = Some(index);
                out.push(StreamChunk::ToolCallDelta {
                    index,
                    id: Some(block["id"].as_str().unwrap_or("").to_string()),
                    name: Some(block["name"].as_str().unwrap_or("").to_string()),
                    args_delta: String::new(),
                });
            }
        }
        Some("content_block_delta") => {
            let delta = &value["delta"];
            match delta["type"].as_str() {
                Some("text_delta") => {
                    if let Some(chunk) = delta["text"].as_str() {
                        if !chunk.is_empty() {
                            text.push_str(chunk);
                            out.push(StreamChunk::TextDelta(chunk.to_string()));
                        }
                    }
                }
                Some("input_json_delta") => {
                    if let (Some(index), Some(chunk)) =
                        (current.as_ref(), delta["partial_json"].as_str())
                    {
                        if let Some(slot) = tools.get_mut(*index) {
                            slot.args.push_str(chunk);
                        }
                        out.push(StreamChunk::ToolCallDelta {
                            index: *index,
                            id: None,
                            name: None,
                            args_delta: chunk.to_string(),
                        });
                    }
                }
                Some("thinking_delta") => {
                    if let Some(chunk) = delta["thinking"].as_str() {
                        if !chunk.is_empty() {
                            out.push(StreamChunk::ReasoningDelta(chunk.to_string()));
                        }
                    }
                }
                _ => {}
            }
        }
        Some("content_block_stop") => {
            *current = None;
        }
        Some("message_delta") => {
            if let Some(reason) = value["delta"]["stop_reason"].as_str() {
                *finish = Some(reason.to_string());
            }
            if let Some(output) = value["usage"]["output_tokens"].as_u64() {
                usage.output_tokens = output;
                out.push(StreamChunk::Usage(usage.clone()));
            }
        }
        _ => {}
    }
}

fn anthropic_chunks(mut rx: mpsc::Receiver<Value>) -> ChunkStream {
    let (tx, out) = mpsc::channel::<ChunkResult>(64);
    tokio::spawn(async move {
        let mut text = String::new();
        let mut tools: Vec<PartialToolCall> = Vec::new();
        let mut current: Option<usize> = None;
        let mut finish: Option<String> = None;
        let mut usage = Usage::default();
        while let Some(value) = rx.recv().await {
            if let Some(error) = value.get("_stream_error").and_then(|v| v.as_str()) {
                let _ = tx.send(Err(ModelError::Stream(error.to_string()))).await;
                continue;
            }
            let mut chunks = Vec::new();
            anthropic_process(
                &value,
                &mut text,
                &mut tools,
                &mut current,
                &mut finish,
                &mut usage,
                &mut chunks,
            );
            for chunk in chunks {
                if tx.send(Ok(chunk)).await.is_err() {
                    return;
                }
            }
        }
        let response = finish_response(&text, &tools, &finish, &usage);
        let _ = tx.send(Ok(StreamChunk::Done { response })).await;
    });
    receiver_stream(out)
}

pub async fn anthropic_stream(cfg: &ProviderConfig, req: &ModelRequest) -> Result<ChunkStream> {
    let key = cfg
        .resolved_api_key()
        .context("no API key configured for provider")?;
    let base = cfg
        .base_url
        .as_deref()
        .unwrap_or(ANTHROPIC_BASE)
        .trim_end_matches('/');
    let url = format!("{base}/v1/messages");
    let (system, messages) = http::to_anthropic_messages(req);
    let mut body = json!({
        "model": req.model,
        "messages": messages,
        "max_tokens": req.max_tokens.unwrap_or(4096),
        "stream": true,
    });
    if !system.is_empty() {
        body["system"] = json!(system);
    }
    if let Some(temp) = req.temperature {
        body["temperature"] = json!(temp);
    }
    if let Some(tools) = &req.tools {
        body["tools"] = json!(tools
            .iter()
            .map(|t| json!({
                "name": t.name,
                "description": t.description,
                "input_schema": t.input_schema,
            }))
            .collect::<Vec<_>>());
    }
    let headers = vec![
        ("x-api-key".to_string(), key),
        ("anthropic-version".to_string(), "2023-06-01".to_string()),
    ];
    let rx = open_sse(url, headers, &body).await?;
    Ok(anthropic_chunks(rx))
}

pub(crate) fn to_google_request(req: &ModelRequest) -> (Option<Value>, Vec<Value>, Option<Value>) {
    let names: std::collections::HashMap<String, String> = req
        .messages
        .iter()
        .filter_map(|m| m.tool_calls.as_ref().map(|calls| (m, calls)))
        .flat_map(|(_, calls)| calls.iter())
        .map(|c| (c.id.clone(), c.name.clone()))
        .collect();
    let mut system = String::new();
    let mut contents: Vec<Value> = Vec::new();
    for msg in &req.messages {
        match msg.role {
            Role::System => {
                if !system.is_empty() {
                    system.push('\n');
                }
                system.push_str(&msg.content.as_text());
            }
            Role::Tool => {
                let name = msg
                    .tool_call_id
                    .as_ref()
                    .and_then(|id| names.get(id))
                    .cloned()
                    .unwrap_or_default();
                let response: Value = serde_json::from_str(&msg.content.as_text())
                    .unwrap_or_else(|_| json!({ "result": msg.content.as_text() }));
                contents.push(json!({
                    "role": "user",
                    "parts": [{ "functionResponse": { "name": name, "response": response } }],
                }));
            }
            Role::User | Role::Assistant => {
                let role = if msg.role == Role::Assistant {
                    "model"
                } else {
                    "user"
                };
                let mut parts: Vec<Value> = Vec::new();
                let text = msg.content.as_text();
                if !text.is_empty() {
                    parts.push(json!({ "text": text }));
                }
                if let Some(calls) = &msg.tool_calls {
                    for call in calls {
                        parts.push(json!({
                            "functionCall": { "name": call.name, "args": call.args },
                        }));
                    }
                }
                if !parts.is_empty() {
                    contents.push(json!({ "role": role, "parts": parts }));
                }
            }
        }
    }
    let system_instruction = if system.is_empty() {
        None
    } else {
        Some(json!({ "parts": [{ "text": system }] }))
    };
    let tools = req.tools.as_ref().map(|tools| {
        json!({
            "functionDeclarations": tools
                .iter()
                .map(|t| json!({
                    "name": t.name,
                    "description": t.description,
                    "parameters": t.input_schema,
                }))
                .collect::<Vec<_>>(),
        })
    });
    (system_instruction, contents, tools)
}

fn google_process(
    value: &Value,
    text: &mut String,
    tools: &mut Vec<PartialToolCall>,
    finish: &mut Option<String>,
    usage: &mut Usage,
    out: &mut Vec<StreamChunk>,
) {
    if let Some(meta) = value.get("usageMetadata") {
        usage.input_tokens = meta["promptTokenCount"]
            .as_u64()
            .unwrap_or(usage.input_tokens);
        usage.output_tokens = meta["candidatesTokenCount"]
            .as_u64()
            .unwrap_or(usage.output_tokens);
    }
    let Some(candidate) = value["candidates"].as_array().and_then(|c| c.first()) else {
        return;
    };
    if let Some(reason) = candidate["finishReason"].as_str() {
        *finish = Some(reason.to_string());
    }
    let Some(parts) = candidate["content"]["parts"].as_array() else {
        return;
    };
    for part in parts {
        if let Some(chunk) = part["text"].as_str() {
            if !chunk.is_empty() {
                if part["thought"].as_bool().unwrap_or(false) {
                    out.push(StreamChunk::ReasoningDelta(chunk.to_string()));
                } else {
                    text.push_str(chunk);
                    out.push(StreamChunk::TextDelta(chunk.to_string()));
                }
            }
        }
        if let Some(call) = part.get("functionCall") {
            let index = tools.len();
            let args = call["args"].clone();
            let args_delta = args.to_string();
            let name = call["name"].as_str().unwrap_or("").to_string();
            tools.push(PartialToolCall {
                index,
                id: format!("call_{index}"),
                name: name.clone(),
                args: args_delta.clone(),
            });
            out.push(StreamChunk::ToolCallDelta {
                index,
                id: Some(format!("call_{index}")),
                name: Some(name),
                args_delta,
            });
        }
    }
}

fn google_chunks(mut rx: mpsc::Receiver<Value>) -> ChunkStream {
    let (tx, out) = mpsc::channel::<ChunkResult>(64);
    tokio::spawn(async move {
        let mut text = String::new();
        let mut tools: Vec<PartialToolCall> = Vec::new();
        let mut finish: Option<String> = None;
        let mut usage = Usage::default();
        while let Some(value) = rx.recv().await {
            if let Some(error) = value.get("_stream_error").and_then(|v| v.as_str()) {
                let _ = tx.send(Err(ModelError::Stream(error.to_string()))).await;
                continue;
            }
            let mut chunks = Vec::new();
            google_process(
                &value,
                &mut text,
                &mut tools,
                &mut finish,
                &mut usage,
                &mut chunks,
            );
            for chunk in chunks {
                if tx.send(Ok(chunk)).await.is_err() {
                    return;
                }
            }
        }
        let response = finish_response(&text, &tools, &finish, &usage);
        let _ = tx.send(Ok(StreamChunk::Done { response })).await;
    });
    receiver_stream(out)
}

pub async fn google_stream(cfg: &ProviderConfig, req: &ModelRequest) -> Result<ChunkStream> {
    let key = cfg
        .resolved_api_key()
        .context("no API key configured for provider")?;
    let base = cfg
        .base_url
        .as_deref()
        .unwrap_or(GOOGLE_BASE)
        .trim_end_matches('/');
    let url = format!(
        "{base}/models/{}:streamGenerateContent?alt=sse&key={key}",
        req.model
    );
    let (system, contents, tools) = to_google_request(req);
    let mut body = json!({ "contents": contents });
    if let Some(system) = system {
        body["systemInstruction"] = system;
    }
    if let Some(tools) = tools {
        body["tools"] = json!([tools]);
    }
    let mut generation_config = json!({});
    if let Some(temp) = req.temperature {
        generation_config["temperature"] = json!(temp);
    }
    if let Some(max) = req.max_tokens {
        generation_config["maxOutputTokens"] = json!(max);
    }
    if generation_config
        .as_object()
        .map(|o| !o.is_empty())
        .unwrap_or(false)
    {
        body["generationConfig"] = generation_config;
    }
    let rx = open_sse(url, Vec::new(), &body).await?;
    Ok(google_chunks(rx))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::{ChatMessage, MessageContent, ToolSchema};

    #[test]
    fn openai_accumulates_text_and_tools() {
        let mut text = String::new();
        let mut tools = Vec::new();
        let mut finish = None;
        let mut usage = Usage::default();
        let mut chunks = Vec::new();
        openai_process(
            &json!({ "choices": [{ "delta": { "content": "he" } }] }),
            &mut text,
            &mut tools,
            &mut finish,
            &mut usage,
            &mut chunks,
        );
        openai_process(
            &json!({ "choices": [{ "delta": { "content": "llo" } }] }),
            &mut text,
            &mut tools,
            &mut finish,
            &mut usage,
            &mut chunks,
        );
        openai_process(
            &json!({ "choices": [{ "delta": { "tool_calls": [{ "index": 0, "id": "a", "function": { "name": "t", "arguments": "{\"x\":" } }] }, "finish_reason": "tool_calls" }] }),
            &mut text,
            &mut tools,
            &mut finish,
            &mut usage,
            &mut chunks,
        );
        assert_eq!(text, "hello");
        assert_eq!(tools.len(), 1);
        let response = finish_response(&text, &tools, &finish, &usage);
        assert_eq!(response.stop_reason, StopReason::ToolUse);
        assert_eq!(response.content.as_text(), "hello");
    }

    #[test]
    fn openai_forwards_reasoning_deltas() {
        let mut text = String::new();
        let mut tools = Vec::new();
        let mut finish = None;
        let mut usage = Usage::default();
        let mut chunks = Vec::new();
        openai_process(
            &json!({ "choices": [{ "delta": { "reasoning_content": "think" } }] }),
            &mut text,
            &mut tools,
            &mut finish,
            &mut usage,
            &mut chunks,
        );
        openai_process(
            &json!({ "choices": [{ "delta": { "reasoning_content": "ing" } }] }),
            &mut text,
            &mut tools,
            &mut finish,
            &mut usage,
            &mut chunks,
        );
        openai_process(
            &json!({ "choices": [{ "delta": { "content": "answer" } }] }),
            &mut text,
            &mut tools,
            &mut finish,
            &mut usage,
            &mut chunks,
        );
        let reasoning: String = chunks
            .iter()
            .filter_map(|chunk| match chunk {
                StreamChunk::ReasoningDelta(part) => Some(part.as_str()),
                _ => None,
            })
            .collect();
        assert_eq!(reasoning, "thinking");
        assert_eq!(text, "answer");
    }

    #[test]
    fn anthropic_forwards_thinking_deltas() {
        let mut text = String::new();
        let mut tools = Vec::new();
        let mut current = None;
        let mut finish = None;
        let mut usage = Usage::default();
        let mut chunks = Vec::new();
        anthropic_process(
            &json!({
                "type": "content_block_delta",
                "delta": { "type": "thinking_delta", "thinking": "step1" }
            }),
            &mut text,
            &mut tools,
            &mut current,
            &mut finish,
            &mut usage,
            &mut chunks,
        );
        anthropic_process(
            &json!({
                "type": "content_block_delta",
                "delta": { "type": "text_delta", "text": "hello" }
            }),
            &mut text,
            &mut tools,
            &mut current,
            &mut finish,
            &mut usage,
            &mut chunks,
        );
        assert!(matches!(
            chunks.first(),
            Some(StreamChunk::ReasoningDelta(part)) if part == "step1"
        ));
        assert!(matches!(
            chunks.get(1),
            Some(StreamChunk::TextDelta(part)) if part == "hello"
        ));
        assert_eq!(text, "hello");
    }

    #[test]
    fn google_forwards_thought_parts() {
        let mut text = String::new();
        let mut tools = Vec::new();
        let mut finish = None;
        let mut usage = Usage::default();
        let mut chunks = Vec::new();
        google_process(
            &json!({ "candidates": [{ "content": { "parts": [{ "text": "internal", "thought": true }, { "text": "output" }] } }] }),
            &mut text,
            &mut tools,
            &mut finish,
            &mut usage,
            &mut chunks,
        );
        assert!(matches!(
            chunks.first(),
            Some(StreamChunk::ReasoningDelta(part)) if part == "internal"
        ));
        assert!(matches!(
            chunks.get(1),
            Some(StreamChunk::TextDelta(part)) if part == "output"
        ));
        assert_eq!(text, "output");
    }

    #[test]
    fn google_request_builds_contents_and_tools() {
        let req = ModelRequest {
            model: "gemini".into(),
            messages: vec![
                ChatMessage {
                    role: Role::System,
                    content: MessageContent::Text("be nice".into()),
                    tool_calls: None,
                    tool_call_id: None,
                },
                ChatMessage {
                    role: Role::User,
                    content: MessageContent::Text("hi".into()),
                    tool_calls: None,
                    tool_call_id: None,
                },
            ],
            tools: Some(vec![ToolSchema {
                name: "read".into(),
                description: "Read a file".into(),
                input_schema: json!({ "type": "object" }),
            }]),
            temperature: None,
            max_tokens: None,
        };
        let (system, contents, tools) = to_google_request(&req);
        assert!(system.is_some());
        assert_eq!(contents.len(), 1);
        assert!(tools.is_some());
    }
}
