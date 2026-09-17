use anyhow::{anyhow, Context, Result};

use crate::provider::ProviderConfig;
use crate::types::{
    ChatMessage, ContentPart, MessageContent, ModelRequest, ModelResponse, Role, StopReason,
    ToolCall, Usage,
};

const OPENAI_BASE: &str = "https://api.openai.com/v1";
const ANTHROPIC_BASE: &str = "https://api.anthropic.com";

pub fn openai_tool_schema(t: &crate::types::ToolSchema) -> serde_json::Value {
    serde_json::json!({
        "type": "function",
        "function": {
            "name": t.name,
            "description": t.description,
            "parameters": t.input_schema,
        }
    })
}

fn to_openai_messages(req: &ModelRequest) -> Vec<serde_json::Value> {
    let mut out = Vec::new();
    for msg in &req.messages {
        out.push(openai_message(msg));
    }
    out
}

fn openai_message(msg: &ChatMessage) -> serde_json::Value {
    let mut v = serde_json::json!({
        "role": msg.role,
        "content": msg.content.as_text(),
    });
    if let Some(calls) = &msg.tool_calls {
        let calls: Vec<serde_json::Value> = calls
            .iter()
            .map(|c| {
                serde_json::json!({
                    "id": c.id,
                    "type": "function",
                    "function": {
                        "name": c.name,
                        "arguments": c.args.to_string(),
                    }
                })
            })
            .collect();
        v["tool_calls"] = serde_json::Value::Array(calls);
    }
    if let Some(id) = &msg.tool_call_id {
        v["tool_call_id"] = serde_json::Value::String(id.clone());
    }
    v
}

pub fn parse_openai_response(body: serde_json::Value) -> Result<ModelResponse> {
    let choice = body["choices"].as_array().and_then(|c| c.first())
        .context("empty choices in provider response")?;
    let msg = &choice["message"];

    let text = msg["content"].as_str().unwrap_or_default().to_string();
    let tool_calls: Vec<ToolCall> = msg["tool_calls"]
        .as_array()
        .map(|arr| {
            arr.iter()
                .filter_map(|tc| {
                    let id = tc["id"].as_str()?.to_string();
                    let name = tc["function"]["name"].as_str()?.to_string();
                    let args_str = tc["function"]["arguments"].as_str().unwrap_or("{}");
                    let args = serde_json::from_str(args_str).unwrap_or(serde_json::Value::Null);
                    Some(ToolCall {
                        id,
                        name,
                        args,
                    })
                })
                .collect()
        })
        .unwrap_or_default();

    let content = build_content(&text, &tool_calls);
    let usage = Usage {
        input_tokens: body["usage"]["prompt_tokens"].as_u64().unwrap_or(0),
        output_tokens: body["usage"]["completion_tokens"].as_u64().unwrap_or(0),
    };
    let finish = choice["finish_reason"].as_str().unwrap_or("stop");
    let stop_reason = match finish {
        "tool_calls" => StopReason::ToolUse,
        "length" => StopReason::MaxTokens,
        _ => StopReason::EndTurn,
    };

    Ok(ModelResponse {
        content,
        tool_calls,
        stop_reason,
        usage,
    })
}

fn build_content(text: &str, tool_calls: &[ToolCall]) -> MessageContent {
    if text.is_empty() && tool_calls.is_empty() {
        return MessageContent::Text(String::new());
    }
    if text.is_empty() {
        return MessageContent::Multi {
            parts: tool_calls
                .iter()
                .map(|tc| ContentPart::ToolCall {
                    id: tc.id.clone(),
                    name: tc.name.clone(),
                    args: tc.args.clone(),
                })
                .collect(),
        };
    }
    if tool_calls.is_empty() {
        return MessageContent::Text(text.to_string());
    }
    let mut parts: Vec<ContentPart> = vec![ContentPart::Text {
        text: text.to_string(),
    }];
    parts.extend(
        tool_calls.iter().map(|tc| ContentPart::ToolCall {
            id: tc.id.clone(),
            name: tc.name.clone(),
            args: tc.args.clone(),
        }),
    );
    MessageContent::Multi { parts }
}

async fn http_post(
    url: &str,
    headers: Vec<(String, String)>,
    body: &serde_json::Value,
) -> Result<serde_json::Value> {
    let client = reqwest::Client::new();
    let mut req = client.post(url);
    for (name, value) in headers {
        req = req.header(name, value);
    }
    let resp = req
        .json(body)
        .send()
        .await
        .with_context(|| format!("request to {url} failed"))?;
    let status = resp.status();
    let text = resp.text().await.context("failed to read response body")?;
    if !status.is_success() {
        return Err(anyhow!("provider returned {status}: {text}"));
    }
    serde_json::from_str(&text).with_context(|| "provider returned invalid JSON")
}

pub async fn post_chat_completion(
    cfg: &ProviderConfig,
    req: &ModelRequest,
) -> Result<ModelResponse> {
    let key = cfg
        .resolved_api_key()
        .context("no API key configured for provider")?;
    let base = cfg
        .base_url
        .as_deref()
        .unwrap_or(OPENAI_BASE)
        .trim_end_matches('/');
    let url = format!("{base}/chat/completions");

    let mut body = serde_json::json!({
        "model": req.model,
        "messages": to_openai_messages(req),
    });
    if let Some(temp) = req.temperature {
        body["temperature"] = serde_json::json!(temp);
    }
    if let Some(max) = req.max_tokens {
        body["max_tokens"] = serde_json::json!(max);
    }
    if let Some(tools) = &req.tools {
        body["tools"] = serde_json::json!(
            tools.iter().map(openai_tool_schema).collect::<Vec<_>>()
        );
    }

    let headers = vec![(
        "Authorization".to_string(),
        format!("Bearer {key}"),
    )];
    let json = http_post(&url, headers, &body).await?;
    parse_openai_response(json)
}

fn to_anthropic_messages(
    req: &ModelRequest,
) -> (String, Vec<serde_json::Value>) {
    let mut system: Vec<String> = Vec::new();
    let mut messages: Vec<serde_json::Value> = Vec::new();
    for msg in &req.messages {
        match msg.role {
            Role::System => {
                system.push(msg.content.as_text());
            }
            Role::Tool => {
                let block = serde_json::json!({
                    "type": "tool_result",
                    "tool_use_id": msg.tool_call_id.clone().unwrap_or_default(),
                    "content": msg.content.as_text(),
                });
                messages.push(serde_json::json!({ "role": "user", "content": [block] }));
            }
            _ => {
                let text = msg.content.as_text();
                let has_text = !text.is_empty();
                let mut parts: Vec<serde_json::Value> = Vec::new();
                if has_text {
                    parts.push(serde_json::json!({ "type": "text", "text": text }));
                }
                if let Some(calls) = &msg.tool_calls {
                    for c in calls {
                        parts.push(serde_json::json!({
                            "type": "tool_use",
                            "id": c.id,
                            "name": c.name,
                            "input": c.args,
                        }));
                    }
                }
                let content = if parts.is_empty() {
                    serde_json::Value::String(text)
                } else {
                    serde_json::Value::Array(parts)
                };
                let role = match msg.role {
                    Role::User => "user",
                    Role::Assistant => "assistant",
                    _ => "user",
                };
                messages.push(serde_json::json!({ "role": role, "content": content }));
            }
        }
    }
    (system.join("\n"), messages)
}

pub fn parse_anthropic_response(body: serde_json::Value) -> Result<ModelResponse> {
    let mut text = String::new();
    let mut tool_calls: Vec<ToolCall> = Vec::new();
    for block in body["content"].as_array().cloned().unwrap_or_default() {
        match block.get("type").and_then(|t| t.as_str()) {
            Some("text") => {
                if let Some(t) = block.get("text").and_then(|v| v.as_str()) {
                    text.push_str(t);
                }
            }
            Some("tool_use") => {
                tool_calls.push(ToolCall {
                    id: block
                        .get("id")
                        .and_then(|v| v.as_str())
                        .unwrap_or("")
                        .to_string(),
                    name: block
                        .get("name")
                        .and_then(|v| v.as_str())
                        .unwrap_or("")
                        .to_string(),
                    args: block
                        .get("input")
                        .cloned()
                        .unwrap_or(serde_json::Value::Null),
                });
            }
            _ => {}
        }
    }

    let usage = Usage {
        input_tokens: body["usage"]["input_tokens"].as_u64().unwrap_or(0),
        output_tokens: body["usage"]["output_tokens"].as_u64().unwrap_or(0),
    };
    let stop_reason = match body["stop_reason"].as_str().unwrap_or("end_turn") {
        "tool_use" => StopReason::ToolUse,
        "max_tokens" => StopReason::MaxTokens,
        _ => StopReason::EndTurn,
    };

    Ok(ModelResponse {
        content: build_content(&text, &tool_calls),
        tool_calls,
        stop_reason,
        usage,
    })
}

pub async fn post_anthropic_messages(
    cfg: &ProviderConfig,
    req: &ModelRequest,
) -> Result<ModelResponse> {
    let key = cfg
        .resolved_api_key()
        .context("no API key configured for provider")?;
    let base = cfg
        .base_url
        .as_deref()
        .unwrap_or(ANTHROPIC_BASE)
        .trim_end_matches('/');
    let url = format!("{base}/v1/messages");

    let (system, messages) = to_anthropic_messages(req);
    let mut body = serde_json::json!({
        "model": req.model,
        "messages": messages,
        "max_tokens": req.max_tokens.unwrap_or(4096),
    });
    if !system.is_empty() {
        body["system"] = serde_json::json!(system);
    }
    if let Some(temp) = req.temperature {
        body["temperature"] = serde_json::json!(temp);
    }
    if let Some(tools) = &req.tools {
        let arr: Vec<serde_json::Value> = tools
            .iter()
            .map(|t| {
                serde_json::json!({
                    "name": t.name,
                    "description": t.description,
                    "input_schema": t.input_schema,
                })
            })
            .collect();
        body["tools"] = serde_json::Value::Array(arr);
    }

    let headers = vec![
        ("x-api-key".to_string(), key),
        ("anthropic-version".to_string(), "2023-06-01".to_string()),
    ];
    let json = http_post(&url, headers, &body).await?;
    parse_anthropic_response(json)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_openai_text_response() {
        let json = serde_json::json!({
            "choices": [{
                "message": { "role": "assistant", "content": "hi" },
                "finish_reason": "stop"
            }],
            "usage": { "prompt_tokens": 3, "completion_tokens": 1 }
        });
        let resp = parse_openai_response(json).unwrap();
        assert_eq!(resp.content.as_text(), "hi");
        assert_eq!(resp.stop_reason, StopReason::EndTurn);
        assert_eq!(resp.usage.input_tokens, 3);
    }

    #[test]
    fn parses_openai_tool_response() {
        let json = serde_json::json!({
            "choices": [{
                "message": {
                    "role": "assistant",
                    "content": null,
                    "tool_calls": [{
                        "id": "tc_1",
                        "function": { "name": "do_thing", "arguments": "{\"a\": 1}" }
                    }]
                },
                "finish_reason": "tool_calls"
            }]
        });
        let resp = parse_openai_response(json).unwrap();
        assert_eq!(resp.stop_reason, StopReason::ToolUse);
        assert_eq!(resp.tool_calls[0].name, "do_thing");
        assert_eq!(resp.content.as_text(), "");
    }

    #[test]
    fn builds_openai_request_with_tools() {
        let req = ModelRequest {
            model: "m".into(),
            messages: vec![ChatMessage {
                role: Role::User,
                content: MessageContent::Text("hello".into()),
                tool_calls: None,
                tool_call_id: None,
            }],
            tools: Some(vec![crate::types::ToolSchema {
                name: "t".into(),
                description: "d".into(),
                input_schema: serde_json::json!({ "type": "object" }),
            }]),
            temperature: Some(0.1),
            max_tokens: Some(5),
        };
        let msgs = to_openai_messages(&req);
        assert_eq!(msgs[0]["role"], "user");
        assert_eq!(msgs[0]["content"], "hello");
        let _ = openai_tool_schema(&crate::types::ToolSchema {
            name: "t".into(),
            description: "d".into(),
            input_schema: serde_json::json!(null),
        });
    }

    #[test]
    fn parses_anthropic_response() {
        let json = serde_json::json!({
            "content": [
                { "type": "text", "text": "ok" },
                { "type": "tool_use", "id": "t1", "name": "f", "input": { "x": 1 } }
            ],
            "stop_reason": "tool_use",
            "usage": { "input_tokens": 2, "output_tokens": 4 }
        });
        let resp = parse_anthropic_response(json).unwrap();
        assert!(resp.content.as_text().contains("ok"));
        assert_eq!(resp.tool_calls.len(), 1);
        assert_eq!(resp.stop_reason, StopReason::ToolUse);
    }
}
