use std::sync::Arc;

use agent_sessions::SessionManager;
use hyper::{Response, StatusCode};
use uuid::Uuid;

use super::api::{json_response, BoxBody};
use super::state::AppState;

pub fn list(state: Arc<AppState>) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let storage = match state.storage() {
        Some(storage) => storage.clone(),
        None => {
            return Ok(Response::builder()
                .status(StatusCode::INTERNAL_SERVER_ERROR)
                .body(super::api::err_body("storage unavailable"))
                .unwrap());
        }
    };

    let manager = SessionManager::new(storage.clone(), Uuid::new_v4());
    let ids = match manager.list() {
        Ok(ids) => ids,
        Err(_) => Vec::new(),
    };

    let sessions: Vec<serde_json::Value> = ids
        .into_iter()
        .filter_map(|id| {
            let record = storage.read_session(&id).ok()?;
            Some(serde_json::json!({
                "id": id,
                "title": format!("Session {}", &id.to_string()[..8]),
                "directory": record.project_path,
                "model": record.model,
                "created": record.created_at,
                "updated": record.updated_at,
            }))
        })
        .collect();

    Ok(json_response(&serde_json::json!({ "data": sessions })))
}

pub async fn create(state: Arc<AppState>) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let storage = match state.storage() {
        Some(storage) => storage.clone(),
        None => {
            return Ok(Response::builder()
                .status(StatusCode::INTERNAL_SERVER_ERROR)
                .body(super::api::err_body("storage unavailable"))
                .unwrap());
        }
    };

    let id = Uuid::new_v4();
    let model = state.resolve_model();
    let record = agent_storage::SessionRecord::new(
        Some(std::env::current_dir().unwrap_or_default()),
        Some(model.model.clone()),
    );

    let _ = SessionManager::new(storage.clone(), id).save(&record, &[]);

    Ok(json_response(&serde_json::json!({ "id": id })))
}

pub fn get(id: Uuid, state: Arc<AppState>) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let storage = match state.storage() {
        Some(storage) => storage.clone(),
        None => {
            return Ok(Response::builder()
                .status(StatusCode::INTERNAL_SERVER_ERROR)
                .body(super::api::err_body("storage unavailable"))
                .unwrap());
        }
    };

    let messages = match SessionManager::resume(&storage, id) {
        Ok(messages) => messages,
        Err(_) => Vec::new(),
    };

    Ok(json_response(&serde_json::json!({
        "session_id": id,
        "messages": messages
    })))
}