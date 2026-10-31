use crate::{
    error::{HostError, HostResult},
    events,
    state::AppState,
};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::path::PathBuf;

fn data_directory(state: &AppState, path: &str) -> HostResult<Option<PathBuf>> {
    if !state.isolated {
        return Ok(None);
    }
    let directory = state
        .directory
        .join("workspaces")
        .join(format!("{:x}", Sha256::digest(path.as_bytes())));
    std::fs::create_dir_all(&directory)?;
    Ok(Some(dunce::canonicalize(directory)?))
}

fn workspace_request(path: &str, client: &str, data_directory: Option<PathBuf>) -> Value {
    let mut request = json!({"path":path,"client_id":client,"channels":[]});
    if let Some(directory) = data_directory {
        request["data_dir"] = json!(directory);
    }
    request
}

pub fn sessions(state: &AppState, path: &str) -> HostResult<Value> {
    let path = dunce::canonicalize(path)?.to_string_lossy().into_owned();
    let data_directory = data_directory(state, &path)?;
    let client = uuid::Uuid::new_v4().to_string();
    let engine = state.require_engine()?;
    let workspace = engine.request(
        "POST",
        "/v1/workspaces",
        Some(workspace_request(&path, &client, data_directory)),
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
        Some(workspace_request(&path, &client, data_directory)),
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

fn remove_path(values: &[Value], path: &str) -> Vec<Value> {
    values
        .iter()
        .filter(|value| value.as_str() != Some(path))
        .cloned()
        .collect()
}

fn without_key(values: &Value, key: &str) -> Option<Value> {
    let entries = values.as_object()?;
    if !entries.contains_key(key) {
        return None;
    }
    let mut cleared = serde_json::Map::new();
    cleared.insert(key.to_owned(), Value::Null);
    Some(Value::Object(cleared))
}

pub fn delete_project(app: &tauri::AppHandle, state: &AppState, path: &str) -> HostResult<Value> {
    let result = delete_project_data(state, path, Some(app))?;
    events::emit(app, "", "projectsChanged", json!({}));
    Ok(result)
}

fn delete_project_data(
    state: &AppState,
    path: &str,
    app: Option<&tauri::AppHandle>,
) -> HostResult<Value> {
    let project = find_project(state, path);
    let mut warnings = Vec::new();
    if let Some(project) = project.as_ref() {
        detach_project(state, app, project, &mut warnings);
    }
    if let Err(error) = delete_project_chats(state, path, project.as_ref()) {
        warnings.push(error);
    }
    if let Some(id) = project
        .as_ref()
        .and_then(|project| project["id"].as_str())
        .map(str::to_owned)
    {
        state.projects.lock().unwrap().remove(&id);
    }
    clear_project_preferences(state, path, project.as_ref())?;
    Ok(json!({"warnings": warnings}))
}

fn find_project(state: &AppState, path: &str) -> Option<Value> {
    state
        .projects
        .lock()
        .unwrap()
        .values()
        .find(|project| project["path"].as_str() == Some(path))
        .cloned()
}

// Detaches the open workspace from the desktop: windows, terminals and the
// event stream. Failures are reported as warnings because the user asked for
// the project to be removed and a stuck terminal must not keep it alive.
fn detach_project(
    state: &AppState,
    app: Option<&tauri::AppHandle>,
    project: &Value,
    warnings: &mut Vec<String>,
) {
    let Some(id) = project["id"].as_str() else {
        return;
    };
    if let Some(app) = app {
        state.windows.forget(app, id);
    }
    state.remotes.terminals.close_workspace(id);
    if let Err(error) = state.terminals.close_workspace(id) {
        warnings.push(error.message);
    }
    if let Ok(engine) = state.require_engine() {
        engine.unsubscribe(id);
    }
}

// Deletes every chat of the project. A closed project has no workspace, so a
// temporary one is opened for the operation and retired afterwards. The
// failure text is returned instead of an error so the project entry is still
// removed when the chats cannot be reached (missing folder, locked data
// directory, engine still starting).
fn delete_project_chats(
    state: &AppState,
    path: &str,
    project: Option<&Value>,
) -> Result<(), String> {
    let engine = state.require_engine().map_err(|error| error.message)?;
    let local_path = local_project_path(state, path).map_err(|error| error.message)?;
    let data_directory = data_directory(state, &local_path).map_err(|error| error.message)?;
    let open_workspace = project
        .and_then(|project| project["id"].as_str())
        .filter(|id| !id.is_empty());
    let client = match open_workspace {
        Some(_) => project
            .and_then(|project| project["clientId"].as_str())
            .unwrap_or_default()
            .to_owned(),
        None => uuid::Uuid::new_v4().to_string(),
    };
    let workspace = match open_workspace {
        Some(id) => json!({"id": id}),
        None => engine
            .request(
                "POST",
                "/v1/workspaces",
                Some(workspace_request(&local_path, &client, data_directory)),
            )
            .map_err(|error| error.message)?,
    };
    let workspace_id = workspace["id"]
        .as_str()
        .ok_or_else(|| "Workspace response has no id".to_owned())?
        .to_owned();
    let sessions = engine
        .request("GET", &format!("/v1/workspaces/{workspace_id}/sessions"), None)
        .map_err(|error| error.message)?;
    let mut failures = Vec::new();
    for session in sessions.as_array().map(Vec::as_slice).unwrap_or(&[]) {
        let Some(session_id) = session["id"].as_str() else {
            continue;
        };
        if let Err(error) = engine.request(
            "DELETE",
            &format!("/v1/workspaces/{workspace_id}/sessions/{session_id}"),
            None,
        ) {
            failures.push(error.message);
        }
    }
    if let Err(error) = engine.request("DELETE", &format!("/v1/clients/{client}"), None) {
        failures.push(error.message);
    }
    if failures.is_empty() {
        return Ok(());
    }
    failures.dedup();
    Err(failures.join("; "))
}

fn local_project_path(state: &AppState, path: &str) -> HostResult<String> {
    if path.starts_with("prime-ssh://") {
        let profile = state.remotes.profile(&crate::remote::profile_from_workspace(path)?)?;
        Ok(crate::remote::workspace_directory(state, &profile)?
            .to_string_lossy()
            .into_owned())
    } else {
        Ok(dunce::canonicalize(path)?.to_string_lossy().into_owned())
    }
}

fn clear_project_preferences(
    state: &AppState,
    path: &str,
    project: Option<&Value>,
) -> HostResult<()> {
    let mut settings = state.settings.lock().unwrap();
    let preferences = settings.get();
    let recent = remove_path(
        preferences["recent"].as_array().map(Vec::as_slice).unwrap_or(&[]),
        path,
    );
    let mut patch = json!({"recent": recent});
    if preferences["activeProject"].as_str() == Some(path) {
        patch["activeProject"] = json!("");
    }
    if let Some(cleared) = without_key(&preferences["activeSessions"], path) {
        patch["activeSessions"] = cleared;
    }
    for key in [
        path,
        project
            .and_then(|project| project["id"].as_str())
            .unwrap_or_default(),
    ] {
        if key.is_empty() {
            continue;
        }
        if let Some(cleared) = without_key(&preferences["sessionOrder"], key) {
            patch["sessionOrder"] = cleared;
        }
    }
    settings.update(&patch)?;
    Ok(())
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
    use crate::{engine::Engine, state::AppState};

    fn test_state(directory: &std::path::Path) -> (AppState, std::sync::Arc<Engine>) {
        let state = AppState::new(directory.to_path_buf()).unwrap();
        let engine = Engine::load(directory).unwrap();
        state.engine.lock().unwrap().instance = Some(engine.clone());
        (state, engine)
    }

    fn project_path(root: &std::path::Path) -> String {
        dunce::canonicalize(root)
            .unwrap()
            .to_string_lossy()
            .into_owned()
    }

    fn start_workspace(state: &AppState, engine: &Engine, path: &str) -> (String, String) {
        let client = uuid::Uuid::new_v4().to_string();
        let workspace = engine
            .request(
                "POST",
                "/v1/workspaces",
                Some(workspace_request(
                    path,
                    &client,
                    data_directory(state, path).unwrap(),
                )),
            )
            .unwrap();
        let id = workspace["id"].as_str().unwrap().to_owned();
        engine
            .request(
                "POST",
                &format!("/v1/workspaces/{id}/sessions"),
                Some(json!({"title":"Chat"})),
            )
            .unwrap();
        (id, client)
    }

    fn chat_count(state: &AppState, path: &str) -> usize {
        sessions(state, path)
            .unwrap()
            .as_array()
            .map(Vec::len)
            .unwrap_or(0)
    }

    #[test]
    fn deleting_a_closed_project_removes_its_chats() {
        let directory = tempfile::tempdir().unwrap();
        let root = tempfile::tempdir().unwrap();
        let (state, engine) = test_state(directory.path());
        let path = project_path(root.path());
        let (_, client) = start_workspace(&state, &engine, &path);
        engine
            .request("DELETE", &format!("/v1/clients/{client}"), None)
            .unwrap();
        assert_eq!(chat_count(&state, &path), 1, "chat must exist before delete");
        state
            .settings
            .lock()
            .unwrap()
            .update(&json!({"recent":[path.clone()],"activeProject":path.clone()}))
            .unwrap();
        delete_project_data(&state, &path, None).unwrap();
        assert_eq!(chat_count(&state, &path), 0, "chats must be deleted");
        let preferences = state.settings.lock().unwrap().get();
        assert_eq!(preferences["recent"], json!([]));
        assert_eq!(preferences["activeProject"], json!(""));
    }

    #[test]
    fn deleting_an_open_project_removes_its_chats() {
        let directory = tempfile::tempdir().unwrap();
        let root = tempfile::tempdir().unwrap();
        let (state, engine) = test_state(directory.path());
        let path = project_path(root.path());
        let (id, client) = start_workspace(&state, &engine, &path);
        state.projects.lock().unwrap().insert(
            id.clone(),
            json!({"id":id,"path":path,"clientId":client,"kind":"project","connection":"ready","channels":[]}),
        );
        state
            .settings
            .lock()
            .unwrap()
            .update(&json!({"recent":[path.clone()],"activeSessions":{path.clone():"chat"}}))
            .unwrap();
        delete_project_data(&state, &path, None).unwrap();
        assert_eq!(chat_count(&state, &path), 0, "chats must be deleted");
        assert!(state.projects.lock().unwrap().is_empty());
        let preferences = state.settings.lock().unwrap().get();
        assert_eq!(preferences["recent"], json!([]));
        assert_eq!(preferences["activeSessions"][&path], Value::Null);
    }

    #[test]
    fn deleting_a_missing_project_still_clears_its_entry() {
        let directory = tempfile::tempdir().unwrap();
        let (state, _engine) = test_state(directory.path());
        let path = "C:/prime/missing/project";
        state
            .settings
            .lock()
            .unwrap()
            .update(&json!({"recent":[path],"activeProject":path}))
            .unwrap();
        let result = delete_project_data(&state, path, None).unwrap();
        assert!(
            !result["warnings"].as_array().unwrap().is_empty(),
            "the unreachable chats must be reported"
        );
        let preferences = state.settings.lock().unwrap().get();
        assert_eq!(preferences["recent"], json!([]));
        assert_eq!(preferences["activeProject"], json!(""));
    }

    #[test]
    fn deleting_a_project_clears_only_its_own_preferences() {
        let recent = vec![json!("C:/one"), json!("C:/two"), json!("C:/one")];
        assert_eq!(remove_path(&recent, "C:/one"), vec![json!("C:/two")]);
        assert!(remove_path(&[], "C:/one").is_empty());
        let entries = json!({"C:/one": "session", "C:/two": "session"});
        assert_eq!(
            without_key(&entries, "C:/one"),
            Some(json!({"C:/one": null}))
        );
        assert_eq!(without_key(&entries, "C:/missing"), None);
        assert_eq!(without_key(&Value::Null, "C:/one"), None);
    }

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

pub fn chats_directory(state: &AppState) -> PathBuf {
    if !state.isolated {
        if let Some(legacy) = crate::migration::legacy_directory()
            .map(|directory| directory.join("Chats"))
            .filter(|directory| directory.is_dir())
        {
            return legacy;
        }
    }
    state.directory.join("Chats")
}
