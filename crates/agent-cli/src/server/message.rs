use std::sync::Arc;

use agent_model::{build_from_model_config, ChatMessage, MessageContent, ModelRequest, Role};
use futures::StreamExt;
use http_body_util::{BodyExt, StreamBody};
use hyper::body::{Bytes, Frame, Incoming};
use hyper::{Request, Response, StatusCode};
use tokio_stream::wrappers::ReceiverStream;
use uuid::Uuid;

use super::state::AppState;
use super::api::{BoxBody, MessageBody, read_json};

pub async fn post(
    request: Request<Incoming>,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let body: MessageBody = match read_json(request).await {
        Ok(body) => body,
        Err(response) => return Ok(response),
    };

    let session_id = body.session_id.unwrap_or_else(Uuid::new_v4);
    let model = state.resolve_model();

    let provider = match build_from_model_config(&model) {
        Ok(provider) => provider,
        Err(error) => return Ok(stream_error(&format!("model init error: {error}"))),
    };

    let history = load_history(&state, session_id);
    let provider_request = ModelRequest {
        model: model.model.clone(),
        messages: history,
        tools: None,
        temperature: model.temperature,
        max_tokens: model.max_tokens,
    };

    let (tx, rx) = tokio::sync::mpsc::channel::<Bytes>(128);
    let stream = ReceiverStream::new(rx);

    tokio::spawn(async move {
        emit(&tx, serde_json::json!({ "type": "session.created", "session": { "id": session_id } })).await;

        emit(&tx, serde_json::json!({
            "type": "message.created",
            "message": {
                "id": Uuid::new_v4(),
                "role": "user",
                "content": body.text
            }
        })).await;

        match provider.chat(&provider_request).await {
            Ok(response) => {
                let text = response.content.as_text();
                emit(&tx, serde_json::json!({
                    "type": "message.part.updated",
                    "part": { "type": "text", "text": text }
                })).await;
                emit(&tx, serde_json::json!({
                    "type": "message.updated",
                    "message": {
                        "id": Uuid::new_v4(),
                        "role": "assistant",
                        "content": text
                    }
                })).await;
                emit(&tx, serde_json::json!({
                    "type": "session.updated",
                    "session": { "id": session_id }
                })).await;
            }
            Err(error) => {
                emit(&tx, serde_json::json!({
                    "type": "session.error",
                    "error": error.to_string()
                })).await;
            }
        }

        emit(&tx, serde_json::json!({ "type": "done" })).await;
    });

    let stream_body = StreamBody::new(stream.map(|bytes| {
        Ok::<Frame<Bytes>, std::convert::Infallible>(Frame::data(bytes))
    }));

    let boxed: BoxBody = BoxBody::new(stream_body);

    Ok(Response::builder()
        .status(StatusCode::OK)
        .header(hyper::header::CONTENT_TYPE, "application/x-ndjson")
        .header("Cache-Control", "no-cache")
        .body(boxed)
        .unwrap())
}

async fn emit(tx: &tokio::sync::mpsc::Sender<Bytes>, value: serde_json::Value) {
    let mut bytes = serde_json::to_vec(&value).unwrap_or_default();
    bytes.push(b'\n');
    let _ = tx.send(Bytes::from(bytes)).await;
}

fn load_history(state: &Arc<AppState>, session_id: Uuid) -> Vec<ChatMessage> {
    let mut messages = vec![ChatMessage {
        role: Role::System,
        content: MessageContent::Text("You are a production-grade AI coding agent.".into()),
        tool_calls: None,
        tool_call_id: None,
    }];

    if let Some(storage) = state.storage() {
        match agent_sessions::SessionManager::resume(storage, session_id) {
            Ok(entries) => {
                for entry in entries {
                    let role = match entry.get("role").and_then(|r| r.as_str()) {
                        Some("user") => Role::User,
                        Some("assistant") => Role::Assistant,
                        _ => continue,
                    };
                    let content = entry.get("content").and_then(|c| c.as_str()).unwrap_or_default();
                    messages.push(ChatMessage {
                        role,
                        content: MessageContent::Text(content.to_string()),
                        tool_calls: None,
                        tool_call_id: None,
                    });
                }
            }
            Err(_) => {}
        }
    }

    messages
}

fn stream_error(message: &str) -> Response<BoxBody> {
    let payload = format!(
        "{}\n{}\n",
        serde_json::json!({ "type": "session.error", "error": message }),
        serde_json::json!({ "type": "done" })
    );
    Response::builder()
        .status(StatusCode::OK)
        .header(hyper::header::CONTENT_TYPE, "application/x-ndjson")
        .body(http_body_util::Full::new(Bytes::from(payload))
            .map_err(|never| match never {})
            .boxed())
        .unwrap()
}