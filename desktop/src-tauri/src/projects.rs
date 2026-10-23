use crate::{
    error::{HostError, HostResult},
    events,
    state::AppState,
};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::path::PathBuf;

fn data_directory(state: &AppState, path: &str) -> HostResult<PathBuf> {
    let directory = state
        .directory
        .join("workspaces")
        .join(format!("{:x}", Sha256::digest(path.as_bytes())));
    std::fs::create_dir_all(&directory)?;
    Ok(dunce::canonicalize(directory)?)
}

pub fn sessions(state: &AppState, path: &str) -> HostResult<Value> {
    let path = dunce::canonicalize(path)?.to_string_lossy().into_owned();
    let data_directory = data_directory(state, &path)?;
    let client = uuid::Uuid::new_v4().to_string();
    let engine = state.require_engine()?;
    let workspace = engine.request(
        "POST",
        "/v1/workspaces",
        Some(json!({"path":path,"client_id":client,"data_dir":data_directory,"channels":[]})),
    )?;
    let result = workspace["id"]
        .as_str()
        .ok_or_else(|| HostError::new("ENGINE_RESPONSE", "Workspace response has no id"))
        .and_then(|id| engine.request("GET", &format!("/v1/workspaces/{id}/sessions"), None));
    let cleanup = engine.request("DELETE", &format!("/v1/clients/{client}"), None);
    let result = result?;
    cleanup?;
    Ok(result)
}

pub fn open(
    app: &tauri::AppHandle,
    state: &AppState,
    path: PathBuf,
    kind: &str,
) -> HostResult<Value> {
    let _opening = state.project_open.lock().unwrap();
    let path = dunce::canonicalize(path)?;
    if !path.is_dir() {
        return Err(HostError::new(
            "INVALID_PROJECT",
            "Select a project directory",
        ));
    }
    let path = path.to_string_lossy().into_owned();
    if let Some(project) = state
        .projects
        .lock()
        .unwrap()
        .values()
        .find(|project| project["path"] == path)
        .cloned()
    {
        return Ok(project);
    }
    let client = uuid::Uuid::new_v4().to_string();
    let data_directory = data_directory(state, &path)?;
    let engine = state.require_engine()?;
    let workspace = engine.request(
        "POST",
        "/v1/workspaces",
        Some(json!({"path":path,"client_id":client,"channels":[],"data_dir":data_directory})),
    )?;
    let id = workspace["id"]
        .as_str()
        .ok_or_else(|| HostError::new("ENGINE_RESPONSE", "Workspace response has no id"))?;
    let project = json!({"id":id,"path":path,"clientId":client,"kind":kind,"connection":"ready","channels":[]});
    if let Err(error) = engine.subscribe(app.clone(), id.into(), &client) {
        let _ = engine.request("DELETE", &format!("/v1/clients/{client}"), None);
        return Err(error);
    }
    state
        .projects
        .lock()
        .unwrap()
        .insert(id.into(), project.clone());
    let mut settings = state.settings.lock().unwrap();
    let mut recent = settings.get()["recent"]
        .as_array()
        .cloned()
        .unwrap_or_default();
    recent.retain(|item| item.as_str() != Some(&path));
    if kind == "project" {
        recent.insert(0, json!(path));
        recent.truncate(64);
    }
    settings.update(&json!({"recent":recent,"activeProject":path}))?;
    events::emit(app, id, "connection", json!("ready"));
    Ok(project)
}

pub fn close(app: &tauri::AppHandle, state: &AppState, id: &str) -> HostResult<Value> {
    state.root(id)?;
    state.windows.forget(app, id);
    state.remotes.terminals.close_workspace(id);
    state.terminals.close_workspace(id)?;
    let engine = state.require_engine()?;
    let client = state
        .projects
        .lock()
        .unwrap()
        .get(id)
        .and_then(|project| project["clientId"].as_str())
        .unwrap_or("")
        .to_string();
    engine.request("DELETE", &format!("/v1/clients/{client}"), None)?;
    engine.unsubscribe(id);
    state.projects.lock().unwrap().remove(id);
    Ok(Value::Null)
}

pub fn call(state: &AppState, id: &str, route: &Value) -> HostResult<Value> {
    state.root(id)?;
    let route = route
        .as_array()
        .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing operation route"))?;
    let method = route.first().and_then(Value::as_str).unwrap_or("");
    let path = route.get(1).and_then(Value::as_str).unwrap_or("");
    let target = operation_target(id, method, path)?;
    state.require_engine()?.request(method, &target, route.get(2).cloned())
}

fn operation_target(id: &str, method: &str, path: &str) -> HostResult<String> {
    let pathname = path.split('?').next().unwrap_or("");
    if !["GET", "POST", "PUT", "DELETE"].contains(&method)
        || path.is_empty()
        || path.starts_with('/')
        || pathname.contains("..")
        || path.contains('#')
        || path.contains('\\')
    {
        return Err(HostError::new(
            "INVALID_ROUTE",
            "Invalid workspace operation route",
        ));
    }
    let url = url::Url::parse(&format!("http://engine/v1/workspaces/{id}/{path}"))
        .map_err(|error| HostError::new("INVALID_ROUTE", error.to_string()))?;
    if !url.path().starts_with(&format!("/v1/workspaces/{id}/")) {
        return Err(HostError::new(
            "INVALID_ROUTE",
            "Operation leaves workspace scope",
        ));
    }
    Ok(match url.query() {
        Some(query) => format!("{}?{query}", url.path()),
        None => url.path().into(),
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn search_query_dots_are_data_and_paths_remain_scoped() {
        for query in ["...", "..%2Fsrc"] {
            let path = format!("sessions/chat/messages?search={query}");
            assert_eq!(operation_target("workspace", "GET", &path).unwrap(), format!("/v1/workspaces/workspace/{path}"));
        }
        for path in ["../another/config", "%2e%2e/config", "/v1/workspaces/another/config", "sessions/../../another/config"] {
            assert!(operation_target("workspace", "GET", path).is_err(), "{path}");
        }
    }
}
