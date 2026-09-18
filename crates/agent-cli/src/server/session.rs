use std::sync::Arc;

use agent_sessions::SessionManager;
use hyper::{Response, StatusCode};
use uuid::Uuid;

use super::api::{err_body, json_response, BoxBody};
use super::state::AppState;

#[allow(clippy::result_large_err)]
fn storage_or_error(state: &Arc<AppState>) -> Result<agent_storage::Storage, Response<BoxBody>> {
    match state.storage() {
        Some(storage) => Ok(storage.clone()),
        None => Err(Response::builder()
            .status(StatusCode::INTERNAL_SERVER_ERROR)
            .body(err_body("storage unavailable"))
            .unwrap()),
    }
}

fn session_json(id: &Uuid, storage: &agent_storage::Storage) -> serde_json::Value {
    match storage.read_session(id) {
        Ok(record) => {
            let title = record
                .metadata
                .get("title")
                .and_then(|v| v.as_str())
                .map(|s| s.to_string())
                .unwrap_or_else(|| format!("Session {}", &id.to_string()[..8]));
            serde_json::json!({
                "id": id,
                "title": title,
                "directory": record.project_path,
                "model": record.model,
                "created": record.created_at,
                "updated": record.updated_at,
            })
        }
        Err(_) => serde_json::json!({
            "id": id,
            "title": format!("Session {}", &id.to_string()[..8]),
            "created": null,
            "updated": null,
        }),
    }
}

pub fn list(state: Arc<AppState>) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let storage = match storage_or_error(&state) {
        Ok(storage) => storage,
        Err(response) => return Ok(response),
    };
    let manager = SessionManager::new(storage.clone(), Uuid::new_v4());
    let ids = manager.list().unwrap_or_default();
    let sessions: Vec<serde_json::Value> = ids
        .into_iter()
        .map(|id| session_json(&id, &storage))
        .collect();
    Ok(json_response(&serde_json::json!({ "data": sessions })))
}

pub async fn create(state: Arc<AppState>) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let storage = match storage_or_error(&state) {
        Ok(storage) => storage,
        Err(response) => return Ok(response),
    };
    let id = Uuid::new_v4();
    let model = state.resolve_model();
    let mut record = agent_storage::SessionRecord::new(
        Some(std::env::current_dir().unwrap_or_default()),
        Some(model.model.clone()),
    );
    record.id = id;
    record.metadata = serde_json::json!({ "title": serde_json::Value::Null });
    let _ = SessionManager::new(storage.clone(), id).save(&record, &[]);
    Ok(json_response(&session_json(&id, &storage)))
}

pub fn get(id: Uuid, state: Arc<AppState>) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let storage = match storage_or_error(&state) {
        Ok(storage) => storage,
        Err(response) => return Ok(response),
    };
    let messages = SessionManager::resume(&storage, id).unwrap_or_default();
    let mut session = session_json(&id, &storage);
    session["messages"] = serde_json::Value::Array(messages.iter().map(map_message).collect());
    Ok(json_response(&session))
}

pub fn rename(
    id: Uuid,
    state: Arc<AppState>,
    title: String,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let storage = match storage_or_error(&state) {
        Ok(storage) => storage,
        Err(response) => return Ok(response),
    };
    match storage.read_session(&id) {
        Ok(mut record) => {
            record.metadata["title"] = serde_json::Value::String(title);
            record.touch();
            let _ = storage.replace_session_meta(&record);
            Ok(json_response(&session_json(&id, &storage)))
        }
        Err(_) => Ok(Response::builder()
            .status(StatusCode::NOT_FOUND)
            .body(err_body("session not found"))
            .unwrap()),
    }
}

pub fn delete(
    id: Uuid,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let storage = match storage_or_error(&state) {
        Ok(storage) => storage,
        Err(response) => return Ok(response),
    };
    let _ = storage.delete_session(&id);
    Ok(json_response(&serde_json::json!({ "deleted": id })))
}

pub fn fork(id: Uuid, state: Arc<AppState>) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let storage = match storage_or_error(&state) {
        Ok(storage) => storage,
        Err(response) => return Ok(response),
    };
    let new_id = Uuid::new_v4();
    match storage.fork_session(&id, &new_id) {
        Ok(_) => Ok(json_response(&session_json(&new_id, &storage))),
        Err(error) => Ok(Response::builder()
            .status(StatusCode::NOT_FOUND)
            .body(err_body(&error.to_string()))
            .unwrap()),
    }
}

pub fn compact(
    id: Uuid,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let storage = match storage_or_error(&state) {
        Ok(storage) => storage,
        Err(response) => return Ok(response),
    };
    let record = match storage.read_session(&id) {
        Ok(record) => record,
        Err(_) => {
            return Ok(Response::builder()
                .status(StatusCode::NOT_FOUND)
                .body(err_body("session not found"))
                .unwrap());
        }
    };
    let messages = SessionManager::resume(&storage, id).unwrap_or_default();
    let compacted = compact_messages(&messages);
    let _ = storage.write_session(&record, &compacted);
    let mut session = session_json(&id, &storage);
    session["messages"] = serde_json::Value::Array(compacted.iter().map(map_message).collect());
    Ok(json_response(&session))
}

fn compact_messages(messages: &[serde_json::Value]) -> Vec<serde_json::Value> {
    if messages.len() <= 8 {
        return messages.to_vec();
    }
    let mut out = Vec::new();
    out.extend(messages.iter().take(2).cloned());
    let summary_lines: Vec<String> = messages[2..messages.len() - 6]
        .iter()
        .map(|m| {
            let role = m.get("role").and_then(|r| r.as_str()).unwrap_or("?");
            let content = m.get("content").and_then(|c| c.as_str()).unwrap_or("");
            let preview: String = content.chars().take(200).collect();
            format!("[{role}]: {preview}")
        })
        .collect();
    out.push(serde_json::json!({
        "role": "system",
        "content": format!("<compaction>\n{}\n</compaction>", summary_lines.join("\n")),
    }));
    out.extend(messages[messages.len() - 6..].iter().cloned());
    out
}

fn map_message(message: &serde_json::Value) -> serde_json::Value {
    let role = message
        .get("role")
        .and_then(|r| r.as_str())
        .unwrap_or("assistant");
    let content = message
        .get("content")
        .map(|c| match c {
            serde_json::Value::String(text) => text.clone(),
            other => other
                .get("text")
                .and_then(|t| t.as_str())
                .unwrap_or_default()
                .to_string(),
        })
        .unwrap_or_default();
    serde_json::json!({
        "id": Uuid::new_v4(),
        "role": role,
        "content": content,
        "tool_calls": message.get("tool_calls").cloned().unwrap_or(serde_json::Value::Null),
        "tool_call_id": message.get("tool_call_id").cloned().unwrap_or(serde_json::Value::Null),
    })
}
