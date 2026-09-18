use std::path::Path;
use std::sync::Arc;

use agent_config::PermissionMode;
use hyper::body::Incoming;
use hyper::{Request, Response};
use uuid::Uuid;

use super::api::{err_body, json_response, read_json, BoxBody};
use super::state::{permission_mode_name, AppState};

const PROJECT_MODE_FILE: &str = "permission-mode.json";

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
            .and_then(|value| serde_json::from_value::<PermissionMode>(value.clone()).ok())
        {
            Some(mode) => permission_mode_name(mode).to_string(),
            None => project_mode(&record, state.workspace.as_path())
                .map(|mode| permission_mode_name(mode).to_string())
                .unwrap_or_else(default_mode),
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
            .body(err_body(
                "mode must be ask | auto_edit | full_access | deny",
            ))
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
        if body.remember_project {
            persist_project_mode(&record, state.workspace.as_path(), mode);
        }
    }
    let applied = state.set_permission_mode(&id, mode);
    Ok(json_response(&serde_json::json!({
        "mode": permission_mode_name(mode),
        "applied": applied
    })))
}

pub fn project_mode(
    record: &agent_storage::SessionRecord,
    workspace: &Path,
) -> Option<PermissionMode> {
    let dir = project_dir(record, workspace)?;
    let file = dir.join(".agent").join(PROJECT_MODE_FILE);
    let value = std::fs::read_to_string(file).ok()?;
    let parsed = serde_json::from_str::<serde_json::Value>(&value).ok()?;
    let name = parsed.get("mode").and_then(|v| v.as_str())?;
    parse_mode(name)
}

pub fn persist_project_mode(
    record: &agent_storage::SessionRecord,
    workspace: &Path,
    mode: PermissionMode,
) {
    let Some(dir) = project_dir(record, workspace) else {
        return;
    };
    let agent_dir = dir.join(".agent");
    if std::fs::create_dir_all(&agent_dir).is_err() {
        return;
    }
    let payload = serde_json::json!({ "mode": permission_mode_name(mode) });
    let _ = std::fs::write(
        agent_dir.join(PROJECT_MODE_FILE),
        serde_json::to_string_pretty(&payload).unwrap_or_default(),
    );
}

fn project_dir(
    record: &agent_storage::SessionRecord,
    workspace: &Path,
) -> Option<std::path::PathBuf> {
    record
        .project_path
        .as_deref()
        .map(Path::to_path_buf)
        .or_else(|| Some(workspace.to_path_buf()))
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
    #[serde(default)]
    remember_project: bool,
}

#[cfg(test)]
mod tests {
    use super::*;
    use agent_storage::Storage;
    use http_body_util::BodyExt;

    fn stored_mode(mode: &str) -> String {
        let tmp = tempfile::tempdir().unwrap();
        let storage = Storage::new(tmp.path().to_path_buf());
        let id = Uuid::new_v4();
        let state = AppState::new(storage.clone(), tmp.path().to_path_buf(), None).unwrap();
        let mut record = agent_storage::SessionRecord::new(None, None);
        record.id = id;
        let _ = storage.write_session(&record, &[]);

        let mode = parse_mode(mode).expect("valid mode");
        if let Ok(mut record) = storage.read_session(&id) {
            record.metadata["perms_mode"] = serde_json::Value::String(mode_name(mode).into());
            record.touch();
            let _ = storage.replace_session_meta(&record);
        }
        let _ = state.set_permission_mode(&id, mode);

        let runtime = tokio::runtime::Runtime::new().unwrap();
        let response = runtime.block_on(get(id, Arc::new(state))).unwrap();
        runtime
            .block_on(response.into_body().collect())
            .ok()
            .and_then(|collected| {
                serde_json::from_slice::<serde_json::Value>(&collected.to_bytes()).ok()
            })
            .and_then(|value| value.get("mode").and_then(|m| m.as_str()).map(String::from))
            .expect("mode from response")
    }

    #[test]
    fn permission_mode_round_trip_persists() {
        assert_eq!(stored_mode("ask"), "ask");
        assert_eq!(stored_mode("auto_edit"), "auto_edit");
        assert_eq!(stored_mode("full_access"), "full_access");
        assert_eq!(stored_mode("deny"), "deny");
    }

    #[test]
    fn project_mode_persists_and_reads_back() {
        use agent_storage::SessionRecord;
        let tmp = tempfile::tempdir().unwrap();
        let mut record = SessionRecord::new(Some(tmp.path().to_path_buf()), None);
        record.id = Uuid::new_v4();
        persist_project_mode(&record, tmp.path(), PermissionMode::Allow);
        assert_eq!(
            project_mode(&record, tmp.path()),
            Some(PermissionMode::Allow)
        );
        let file = tmp.path().join(".agent").join("permission-mode.json");
        assert!(file.exists(), "project permission file must be written");
        persist_project_mode(&record, tmp.path(), PermissionMode::AutoEdit);
        assert_eq!(
            project_mode(&record, tmp.path()),
            Some(PermissionMode::AutoEdit)
        );
        let mut ask_record = SessionRecord::new(Some(tmp.path().to_path_buf()), None);
        ask_record.id = Uuid::new_v4();
        persist_project_mode(&ask_record, tmp.path(), PermissionMode::Allow);
        // The helper returns the persisted project file mode.
        assert_eq!(
            project_mode(&ask_record, tmp.path()),
            Some(PermissionMode::Allow)
        );
        // Missing file falls back to None.
        let empty = tempfile::tempdir().unwrap();
        let other_record = SessionRecord::new(Some(empty.path().to_path_buf()), None);
        assert_eq!(project_mode(&other_record, empty.path()), None);
    }
}
