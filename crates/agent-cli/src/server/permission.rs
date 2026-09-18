use std::sync::Arc;

use agent_config::PermissionMode;
use hyper::body::Incoming;
use hyper::{Request, Response};
use uuid::Uuid;

use super::api::{err_body, json_response, read_json, BoxBody};
use super::state::{permission_mode_name, AppState};

pub async fn get(
    id: Uuid,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let Some(storage) = state.storage() else {
        let response = Response::builder()
            .status(hyper::StatusCode::INTERNAL_SERVER_ERROR)
            .body(err_body("storage unavailable"))
            .unwrap();
        return Ok(response);
    };
    let default_mode = || {
        state
            .config
            .read()
            .unwrap()
            .as_ref()
            .map(|cfg| permission_mode_name(cfg.permissions.mode))
            .unwrap_or("ask")
            .to_string()
    };
    let mode = match storage.read_session(&id) {
        Ok(record) => match record
            .metadata
            .get("perms_mode")
            .and_then(|value| value.as_str())
        {
            Some(name) => serde_json::from_str::<PermissionMode>(name)
                .ok()
                .map(permission_mode_name)
                .map(str::to_string)
                .unwrap_or_else(default_mode),
            None => default_mode(),
        },
        Err(_) => "ask".to_string(),
    };
    Ok(json_response(&serde_json::json!({ "mode": mode })))
}

pub async fn set(
    request: Request<Incoming>,
    id: Uuid,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let body = match read_json::<ModeBody>(request).await {
        Ok(body) => body,
        Err(response) => return Ok(response),
    };
    let Some(mode) = parse_mode(&body.mode) else {
        let response = Response::builder()
            .status(hyper::StatusCode::BAD_REQUEST)
            .body(err_body("mode must be ask | auto_edit | full_access"))
            .unwrap();
        return Ok(response);
    };
    let Some(storage) = state.storage() else {
        let response = Response::builder()
            .status(hyper::StatusCode::INTERNAL_SERVER_ERROR)
            .body(err_body("storage unavailable"))
            .unwrap();
        return Ok(response);
    };
    if let Ok(mut record) = storage.read_session(&id) {
        record.metadata["perms_mode"] = serde_json::Value::String(mode_name(mode).into());
        record.touch();
        let _ = storage.replace_session_meta(&record);
    }
    let applied = state.set_permission_mode(&id, mode);
    Ok(json_response(&serde_json::json!({
        "mode": permission_mode_name(mode),
        "applied": applied
    })))
}

fn parse_mode(value: &str) -> Option<PermissionMode> {
    match value {
        "ask" => Some(PermissionMode::Ask),
        "auto_edit" => Some(PermissionMode::AutoEdit),
        "full_access" | "allow" => Some(PermissionMode::Allow),
        "deny" => Some(PermissionMode::Deny),
        _ => None,
    }
}

fn mode_name(mode: PermissionMode) -> &'static str {
    match mode {
        PermissionMode::Ask => "ask",
        PermissionMode::Allow => "allow",
        PermissionMode::AutoEdit => "auto_edit",
        PermissionMode::Deny => "deny",
    }
}

#[derive(serde::Deserialize)]
struct ModeBody {
    mode: String,
}
