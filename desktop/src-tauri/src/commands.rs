use crate::{
    error::{HostError, HostResult},
    state::AppState,
};
use serde_json::{json, Value};
use std::sync::atomic::Ordering;
use tauri::Manager;

fn string(args: &[Value], index: usize) -> HostResult<&str> {
    args.get(index).and_then(Value::as_str).ok_or_else(|| {
        HostError::new(
            "INVALID_ARGUMENT",
            format!("Argument {index} must be a string"),
        )
    })
}

#[tauri::command]
pub async fn desktop_call(
    app: tauri::AppHandle,
    webview: tauri::Webview,
    method: String,
    args: Vec<Value>,
) -> HostResult<Value> {
    crate::permissions::require_trusted(&webview)?;
    let method_name = method.clone();
    let result = tauri::async_runtime::spawn_blocking(move || dispatch(&app, &method, &args))
        .await
        .map_err(|error| HostError::new("HOST_WORKER", error.to_string()))?;
    if let Err(ref err) = result {
        crate::logging::error(&format!("desktop_call({method_name}) failed [{}]: {}", err.code, err.message));
    }
    result
}

fn reveal_in_file_manager(target: &std::path::Path) -> HostResult<()> {
    if target.is_dir() {
        open::that(target)?;
        return Ok(());
    }
    #[cfg(target_os = "windows")]
    {
        std::process::Command::new("explorer")
            .arg(format!("/select,{}", target.display()))
            .spawn()?;
        Ok(())
    }
    #[cfg(target_os = "macos")]
    {
        std::process::Command::new("open")
            .arg("-R")
            .arg(target)
            .spawn()?;
        Ok(())
    }
    #[cfg(not(any(target_os = "windows", target_os = "macos")))]
    {
        let parent = target.parent().unwrap_or(target);
        open::that(parent)?;
        Ok(())
    }
}

fn urlencoding(value: &str) -> String {
    url::form_urlencoded::byte_serialize(value.as_bytes()).collect()
}

type ProjectPathPair = (String, String);

fn project_sessions_route(requested: &[ProjectPathPair]) -> String {
    let paths = requested
        .iter()
        .map(|(_, local_path)| local_path.as_str())
        .collect::<Vec<_>>()
        .join(",");
    format!("/v1/project-sessions?paths={}", urlencoding(&paths))
}

fn partition_project_sessions(
    requested: Vec<ProjectPathPair>,
) -> (Vec<ProjectPathPair>, Vec<ProjectPathPair>) {
    requested
        .into_iter()
        .partition(|(_, local_path)| !local_path.contains(','))
}

fn map_project_sessions(requested: &[ProjectPathPair], response: &Value) -> serde_json::Map<String, Value> {
    let projects = response["projects"].as_object();
    let mut sessions = serde_json::Map::new();
    for (path, local_path) in requested {
        let entries = projects
            .and_then(|projects| projects.get(local_path))
            .and_then(Value::as_array)
            .cloned()
            .unwrap_or_default();
        sessions.insert(path.clone(), Value::Array(entries));
    }
    sessions
}

fn send_route_body(route: &mut Value) -> HostResult<&mut serde_json::Map<String, Value>> {
    route
        .as_array_mut()
        .and_then(|items| items.get_mut(2))
        .and_then(Value::as_object_mut)
        .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Invalid send operation route"))
}

fn current_session_route_slot(route: &mut Value) -> HostResult<&mut Value> {
    route
        .as_array_mut()
        .and_then(|items| items.get_mut(1))
        .ok_or_else(|| {
            HostError::new(
                "INVALID_ARGUMENT",
                "Invalid currentSession operation route",
            )
        })
}

fn dispatch(app: &tauri::AppHandle, method: &str, args: &[Value]) -> HostResult<Value> {
    let state = app.state::<AppState>();
    if method == "terminal" {
        if let Some(workspace) = args.first().and_then(Value::as_str) {
            if let Some(profile) = crate::remote::workspace_profile(&state, workspace) {
                return state.remotes.terminals.call(
                    &state.remotes,
                    app.clone(),
                    &profile,
                    workspace,
                    args.get(1).unwrap_or(&Value::Null),
                    "user",
                );
            }
        }
    }
    if matches!(
        method,
        "files"
            | "searchFiles"
            | "readFile"
            | "saveFile"
            | "createEntry"
            | "trashEntry"
            | "changes"
            | "diff"
            | "github"
            | "terminal"
            | "launch"
            | "reveal"
            | "openWith"
    ) {
        if let Some(id) = args.first().and_then(Value::as_str) {
            if crate::remote::workspace_profile(&state, id).is_some() {
                return crate::remote::file_call(&state, id, method, args);
            }
        }
    }
    match method {
        "info" => {
            let engine = state.engine.lock().unwrap();
            Ok(
                json!({"version":env!("CARGO_PKG_VERSION"),"runtime":if engine.starting{"starting"}else if engine.instance.is_some(){"ready"}else{"error"},"runtimeVersion":{"version":"Go ABI 1","transport":"in-process"},"error":engine.error,"packaged":!cfg!(debug_assertions),"terminalCount":state.terminals.count()+state.remotes.terminals.count(),"automationCounts":state.automations.counts(),"update":crate::updater::snapshot(),"host":"tauri","nativePreview":true}),
            )
        }
        "preferences" => {
            let mut settings = state.settings.lock().unwrap();
            match args.first().filter(|value| !value.is_null()) {
                Some(patch) => settings.update(patch),
                None => Ok(settings.get()),
            }
        }
        "speech" => state
            .speech
            .handle(app.clone(), args.first().unwrap_or(&Value::Null)),
        "projects" => Ok(Value::Array(
            state.projects.lock().unwrap().values().cloned().collect(),
        )),
        "projectSessions" => {
            let paths = args
                .first()
                .and_then(Value::as_array)
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Expected project paths"))?;
            if paths.len() > 64 {
                return Err(HostError::new(
                    "INVALID_ARGUMENT",
                    "Maximum 64 project paths",
                ));
            }
            if paths.is_empty() {
                return Ok(json!({}));
            }
            let preferences = state.settings.lock().unwrap().get();
            let recent = preferences["recent"]
                .as_array()
                .cloned()
                .unwrap_or_default();
            let mut requested = Vec::new();
            for path in paths {
                let path = path.as_str().ok_or_else(|| {
                    HostError::new("INVALID_ARGUMENT", "Expected string project path")
                })?;
                if !recent.iter().any(|entry| entry.as_str() == Some(path))
                    && !state
                        .projects
                        .lock()
                        .unwrap()
                        .values()
                        .any(|project| project["path"] == path)
                {
                    return Err(HostError::new(
                        "PATH_DENIED",
                        "Project path has not been approved",
                    ));
                }
                let local_path = if path.starts_with("prime-ssh://") {
                    let profile = state
                        .remotes
                        .profile(&crate::remote::profile_from_workspace(path)?)?;
                    crate::remote::workspace_directory(&state, &profile)?
                        .to_string_lossy()
                        .into_owned()
                } else {
                    path.to_owned()
                };
                requested.push((path.to_owned(), local_path));
            }
            let (batched, individual) = partition_project_sessions(requested);
            let mut sessions = if batched.is_empty() {
                serde_json::Map::new()
            } else {
                let response = state
                    .require_engine()?
                    .request("GET", &project_sessions_route(&batched), None)?;
                map_project_sessions(&batched, &response)
            };
            for (path, local_path) in individual {
                let entries = crate::projects::sessions(&state, &local_path).unwrap_or(json!([]));
                sessions.insert(path, entries);
            }
            Ok(Value::Object(sessions))
        }
        "projectCreationBase" => {
            let documents = app.path().document_dir().map_err(|error| HostError::new("DOCUMENTS_UNAVAILABLE", error.to_string()))?;
            Ok(json!(documents.join("Prime")))
        }
        "projectCreationNextName" => {
            let documents = app.path().document_dir().map_err(|error| HostError::new("DOCUMENTS_UNAVAILABLE", error.to_string()))?;
            let next = crate::project_creation::next_available_name(&documents, string(args, 0)?).map_err(|error| {
                let code = match error.kind() {
                    std::io::ErrorKind::InvalidInput => "INVALID_PROJECT_NAME",
                    std::io::ErrorKind::PermissionDenied => "PROJECT_ACCESS_DENIED",
                    _ => "PROJECT_CREATE_FAILED",
                };
                HostError::new(code, code)
            })?;
            Ok(json!(next))
        }
        "createProject" => {
            let documents = app.path().document_dir().map_err(|error| HostError::new("DOCUMENTS_UNAVAILABLE", error.to_string()))?;
            let path = crate::project_creation::create_directory_auto(&documents, string(args, 0)?).map_err(|error| {
                let code = match error.kind() {
                    std::io::ErrorKind::AlreadyExists => "PROJECT_EXISTS",
                    std::io::ErrorKind::InvalidInput => "INVALID_PROJECT_NAME",
                    std::io::ErrorKind::PermissionDenied => "PROJECT_ACCESS_DENIED",
                    _ => "PROJECT_CREATE_FAILED",
                };
                HostError::new(code, code)
            })?;
            crate::projects::open(app, &state, path, "project")
        }
        "openProject" => {
            let path = match args.first().and_then(Value::as_str) {
                Some(path) => {
                    let recent = state.settings.lock().unwrap().get()["recent"]
                        .as_array()
                        .cloned()
                        .unwrap_or_default();
                    if !recent.iter().any(|entry| entry.as_str() == Some(path)) {
                        return Err(HostError::new(
                            "PATH_DENIED",
                            "Choose the project directory using the folder dialog",
                        ));
                    }
                    if path.starts_with("prime-ssh://") {
                        return crate::remote::open(
                            app,
                            &crate::remote::profile_from_workspace(path)?,
                        );
                    }
                    std::path::PathBuf::from(path)
                }
                None => match rfd::FileDialog::new()
                    .set_title("Открыть проект")
                    .pick_folder()
                {
                    Some(path) => path,
                    None => return Ok(Value::Null),
                },
            };
            crate::projects::open(app, &state, path, "project")
        }
        "openChat" => {
            let path = crate::projects::chats_directory(&state);
            std::fs::create_dir_all(&path)?;
            crate::projects::open(app, &state, path, "chat")
        }
        "openDesigner" => {
            let path = crate::projects::designer_directory(&state);
            std::fs::create_dir_all(&path)?;
            crate::projects::open(app, &state, path, "designer")
        }
        "deleteProject" => crate::projects::delete_project(app, &state, string(args, 0)?),
        "windows" => state
            .windows
            .handle(app, string(args, 0)?, args.get(1).unwrap_or(&Value::Null))
            .map_err(|error| HostError::new("WINDOW_ERROR", error)),
        "appTesting" => state
            .appui
            .handle(app, string(args, 0)?, args.get(1).unwrap_or(&Value::Null))
            .map_err(|error| HostError::new("APP_UI_ERROR", error)),
        "appPick" => state
            .appui
            .pick(app, string(args, 0)?, &json!({"sessionId": string(args, 1)?}))
            .map_err(|error| HostError::new("APP_UI_ERROR", error)),
        "appPickCancel" => state
            .appui
            .pick_cancel()
            .map_err(|error| HostError::new("APP_UI_ERROR", error)),
        "automation" => {
            let id = string(args, 0)?;
            let root = project_path(&state, id)?;
            let mut request = args
                .get(1)
                .cloned()
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing automation request"))?;
            if !request.is_object() {
                return Err(HostError::new(
                    "INVALID_ARGUMENT",
                    "Invalid automation request",
                ));
            }
            request["root"] = json!(root);
            state.automations.handle(app.clone(), &request)
        }
        "team" => {
            let id = string(args, 0)?;
            let root = project_path(&state, id)?;
            let request = args
                .get(1)
                .cloned()
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing team request"))?;
            state.teams.handle(&root, &request)
        }
        "jobs" => crate::jobs::handle(app, &state, args.first().unwrap_or(&Value::Null)),
        "customAgents" => crate::custom_agents::handle(&state, args.first().unwrap_or(&Value::Null)),
        "openRemote" => crate::remote::open(app, string(args, 0)?),
        "remoteKey" => Ok(json!(rfd::FileDialog::new()
            .set_title("Выбрать приватный ключ SSH")
            .pick_file()
            .map(|path| path.to_string_lossy().into_owned()))),
        "remote" => {
            let request = args
                .first()
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing SSH request"))?;
            if matches!(
                request["action"].as_str(),
                Some("disconnect" | "remove" | "save")
            ) {
                let id = request["id"]
                    .as_str()
                    .or_else(|| request["input"]["id"].as_str());
                if let Some(id) = id {
                    let workspaces = state
                        .projects
                        .lock()
                        .unwrap()
                        .values()
                        .filter(|project| project["remote"]["profileId"] == id)
                        .filter_map(|project| project["id"].as_str().map(str::to_owned))
                        .collect::<Vec<_>>();
                    for workspace in workspaces {
                        crate::projects::close(app, &state, &workspace)?;
                    }
                    crate::events::emit(app, "", "projectsChanged", json!({}));
                }
            }
            state.remotes.dispatch(app, request)
        }
        "launch" => state.launches.handle(
            app.clone(),
            &state.terminals,
            string(args, 0)?,
            &state.root(string(args, 0)?)?,
            args.get(1).unwrap_or(&Value::Null),
        ),
        "terminal" => state.terminals.call(
            app.clone(),
            string(args, 0)?,
            &state.root(string(args, 0)?)?,
            args.get(1).unwrap_or(&Value::Null),
        ),
        "call" => {
            let operation = string(args, 1)?;
            if operation == "shell"
                && crate::remote::workspace_profile(&state, string(args, 0)?).is_some()
            {
                return Err(HostError::unsupported(
                    "remote shell operation; use ssh_exec",
                ));
            }
            let mut route = args.get(3).cloned().unwrap_or(Value::Null);
            if operation == "send" {
                let body = send_route_body(&mut route)?;
                let ids = args
                    .get(2)
                    .and_then(|args| args.get("attachmentIds"))
                    .cloned()
                    .unwrap_or(json!([]));
                body.insert(
                    "attachments".into(),
                    state.attachments.lock().unwrap().resolve(&ids)?,
                );
            }
            if operation == "currentSession" {
                let id = string(args, 0)?;
                let route_path = current_session_route_slot(&mut route)?;
                let client = {
                    let projects = state.projects.lock().unwrap();
                    let project = projects
                        .get(id)
                        .ok_or_else(|| HostError::new("PROJECT_NOT_FOUND", "Project is not open"))?;
                    project["clientId"].as_str().unwrap_or("").to_owned()
                };
                *route_path = json!(format!("current-session?client_id={client}"));
            }
            let result = if operation == "shell" {
                let _running = crate::state::ShellGuard::new(&state.running_shells);
                crate::projects::call(&state, string(args, 0)?, &route)?
            } else {
                crate::projects::call(&state, string(args, 0)?, &route)?
            };
            if operation == "currentSession" {
                let id = string(args, 0)?;
                let session = args
                    .get(2)
                    .and_then(|args| args.get("sessionId"))
                    .cloned()
                    .unwrap_or(Value::Null);
                let path = {
                    let mut projects = state.projects.lock().unwrap();
                    let project = projects
                        .get_mut(id)
                        .ok_or_else(|| HostError::new("PROJECT_NOT_FOUND", "Project is not open"))?;
                    project["currentSessionId"] = session.clone();
                    project["path"]
                        .as_str()
                        .map(str::to_owned)
                        .ok_or_else(|| HostError::new("PROJECT_NOT_FOUND", "Project is not open"))?
                };
                state
                    .settings
                    .lock()
                    .unwrap()
                    .update(&json!({"activeSessions":{path:session}}))?;
            }
            if matches!(
                operation,
                "messages" | "messagePage" | "messageSearch" | "agentWorkerMessages"
            ) {
                Ok(result)
            } else {
                Ok(crate::redact::value(result))
            }
        }
        "attachments" => {
            let paths = rfd::FileDialog::new()
                .set_title("Добавить вложения")
                .pick_files()
                .unwrap_or_default();
            state.attachments.lock().unwrap().files(&paths)
        }
        "importImages" => state
            .attachments
            .lock()
            .unwrap()
            .import(args.first().unwrap_or(&Value::Null)),
        "releaseAttachments" => state
            .attachments
            .lock()
            .unwrap()
            .release(args.first().unwrap_or(&Value::Null)),
        "files" => crate::files::list_cached(
            &state.file_cache,
            &state.root(string(args, 0)?)?,
            args.get(1).and_then(Value::as_str).unwrap_or(""),
        ),
        "searchFiles" => {
            let root = state.root(string(args, 0)?)?;
            let query = args.get(1).and_then(Value::as_str).unwrap_or("");
            let limit = args
                .get(2)
                .and_then(Value::as_u64)
                .map(|value| value as usize)
                .unwrap_or(50);
            crate::files::search_cached(&state.file_cache, &root, query, limit)
        }
        "readFile" => crate::files::read(&state.root(string(args, 0)?)?, string(args, 1)?),
        "mediaUrl" => state
            .media
            .resolve(&state, string(args, 0)?, string(args, 1)?),
        "inspectPath" => Ok(crate::media::inspect_local_path(
            &state,
            args.first().and_then(Value::as_str).unwrap_or(""),
            string(args, 1)?,
        )),
        "saveFile" => {
            let _guard = state.file_writes.lock().unwrap();
            let root = state.root(string(args, 0)?)?;
            let result = crate::files::save(
                &root,
                string(args, 1)?,
                string(args, 2)?,
                string(args, 3)?,
            )?;
            state.file_cache.invalidate(&root);
            Ok(result)
        }
        "createEntry" => {
            let root = state.root(string(args, 0)?)?;
            let relative = string(args, 1)?;
            let target = crate::files::scoped_path(&root, relative, true)?;
            match string(args, 2)? {
                "file" => {
                    std::fs::OpenOptions::new()
                        .write(true)
                        .create_new(true)
                        .open(target)?;
                }
                "folder" => std::fs::create_dir(target)?,
                _ => {
                    return Err(HostError::new(
                        "INVALID_ARGUMENT",
                        "Entry kind must be file or folder",
                    ))
                }
            }
            state.file_cache.invalidate(&root);
            Ok(json!({"path":relative}))
        }
        "editorDirty" => {
            state.editor_dirty.store(
                args.first()
                    .and_then(Value::as_bool)
                    .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Expected dirty boolean"))?,
                Ordering::Relaxed,
            );
            Ok(Value::Null)
        }
        "windowDimmed" => Ok(Value::Null),
        "cancelProjectWait" => Ok(Value::Null),
        "sudoPassword" => crate::sudo::handle(args.first().unwrap_or(&Value::Null)),
        "update" => {
            let action = string(args, 0)?;
            let force = args.get(1).and_then(Value::as_bool).unwrap_or(false);
            crate::updater::handle(app, &state, action, force)
        }
        "account" => {
            let action = string(args, 0)?;
            let (http_method, route) = match action {
                "status" => ("GET", "/v1/account"),
                "restore" => ("POST", "/v1/account/restore"),
                "login" => ("POST", "/v1/account/login"),
                "logout" => ("POST", "/v1/account/logout"),
                "limits" => ("GET", "/v1/account/limits"),
                "refreshLimits" => ("POST", "/v1/account/limits/refresh"),
                _ => {
                    return Err(HostError::new(
                        "INVALID_ARGUMENT",
                        "Unknown account action",
                    ))
                }
            };
            state.require_engine()?.request(http_method, route, None)
        }
        "openExternal" => {
            let url = url::Url::parse(string(args, 0)?)
                .map_err(|error| HostError::new("INVALID_URL", error.to_string()))?;
            if !["http", "https", "mailto"].contains(&url.scheme()) {
                return Err(HostError::new(
                    "URL_DENIED",
                    "Unsupported external URL scheme",
                ));
            }
            open::that(url.as_str())?;
            Ok(Value::Null)
        }
        "reveal" => {
            let workspace_id = args.first().and_then(Value::as_str).unwrap_or("");
            let raw_path = args.get(1).and_then(Value::as_str).unwrap_or("");
            let target = crate::media::resolve_reveal_target(&state, workspace_id, raw_path)?;
            reveal_in_file_manager(&target)?;
            Ok(Value::Null)
        }
        "openWith" => {
            let target =
                crate::files::scoped_path(&state.root(string(args, 0)?)?, string(args, 1)?, false)?;
            match string(args, 2)? {
                "default" => open::that(target)?,
                "vscode" => open::that(crate::files::editor_url(&target)?)?,
                _ => return Err(HostError::new("INVALID_ARGUMENT", "Unknown application")),
            }
            Ok(Value::Null)
        }
        "trashEntry" => {
            let root = state.root(string(args, 0)?)?;
            crate::files::trash(&root, string(args, 1)?)?;
            state.file_cache.invalidate(&root);
            Ok(Value::Null)
        }
        "diff" if args.get(3).and_then(Value::as_str) == Some("session") => {
            let id = string(args, 0)?;
            let root = state.root(id)?;
            let session = urlencoding(string(args, 1)?);
            let history = state.require_engine()?.request(
                "GET",
                &format!("/v1/workspaces/{id}/sessions/{session}/history"),
                None,
            )?;
            crate::git::session_diff(&root, string(args, 2)?, &history)
        }
        "changes" | "github" | "diff" => {
            crate::git::dispatch(&state.root(string(args, 0)?)?, method, args)
        }
        method if method.starts_with("browser") => {
            crate::browser::dispatch(app, &state.browser, method, args)
                .map_err(|error| HostError::new("BROWSER_ERROR", error))
        }
        _ => Err(HostError::unsupported(method)),
    }
}

#[cfg(test)]
mod project_sessions_tests {
    use super::*;

    fn requested(paths: &[(&str, &str)]) -> Vec<(String, String)> {
        paths
            .iter()
            .map(|(path, local)| (path.to_string(), local.to_string()))
            .collect()
    }

    #[test]
    fn closed_project_sessions_are_requested_in_one_batch() {
        let route = project_sessions_route(&requested(&[
            ("C:/one", "C:/one"),
            ("C:/two three", "C:/two three"),
        ]));
        assert_eq!(
            route,
            "/v1/project-sessions?paths=C%3A%2Fone%2CC%3A%2Ftwo+three"
        );
    }

    #[test]
    fn engine_answer_is_mapped_back_to_the_requested_paths() {
        let asked = requested(&[
            ("C:/one", "C:/one"),
            ("C:/missing", "C:/missing"),
            ("prime-ssh://profile/home", "/home/user/project"),
        ]);
        let response = json!({"projects": {
            "C:/one": [{"id": "chat"}],
            "/home/user/project": [{"id": "remote-chat"}]
        }});
        let sessions = map_project_sessions(&asked, &response);
        assert_eq!(sessions["C:/one"], json!([{"id": "chat"}]));
        assert_eq!(sessions["C:/missing"], json!([]));
        assert_eq!(
            sessions["prime-ssh://profile/home"],
            json!([{"id": "remote-chat"}])
        );
    }

    #[test]
    fn an_engine_error_payload_yields_empty_lists() {
        let asked = requested(&[("C:/one", "C:/one")]);
        let sessions = map_project_sessions(&asked, &Value::Null);
        assert_eq!(sessions["C:/one"], json!([]));
    }

    #[test]
    fn comma_paths_are_partitioned_out_of_batch_requests() {
        let asked = requested(&[
            ("C:/one", "C:/one"),
            ("C:/two,three", "C:/two,three"),
            ("prime-ssh://profile/a,b", "/local/dir,comma"),
            ("prime-ssh://profile/c,d", "/local/dir-clean"),
        ]);
        let (batched, individual) = partition_project_sessions(asked);
        assert_eq!(
            batched,
            requested(&[
                ("C:/one", "C:/one"),
                ("prime-ssh://profile/c,d", "/local/dir-clean"),
            ])
        );
        assert_eq!(
            individual,
            requested(&[
                ("C:/two,three", "C:/two,three"),
                ("prime-ssh://profile/a,b", "/local/dir,comma"),
            ])
        );
    }

    #[test]
    fn route_validation_rejects_malformed_send_and_current_session_routes() {
        for invalid in [
            Value::Null,
            json!({}),
            json!([]),
            json!(["POST"]),
            json!(["POST", "agent"]),
            json!(["POST", "agent", "not-an-object"]),
        ] {
            let mut route = invalid;
            assert_eq!(send_route_body(&mut route).unwrap_err().code, "INVALID_ARGUMENT");
        }
        let mut valid_send = json!(["POST", "agent", {"prompt":"hi"}]);
        send_route_body(&mut valid_send)
            .unwrap()
            .insert("attachments".into(), json!(["a1"]));
        assert_eq!(valid_send[2]["attachments"], json!(["a1"]));

        for invalid in [Value::Null, json!({}), json!([]), json!(["PUT"])] {
            let mut route = invalid;
            assert_eq!(
                current_session_route_slot(&mut route).unwrap_err().code,
                "INVALID_ARGUMENT"
            );
        }
        let mut valid_current = json!(["PUT", "current-session", {"session_id":"s1"}]);
        *current_session_route_slot(&mut valid_current).unwrap() =
            json!("current-session?client_id=c1");
        assert_eq!(valid_current[1], "current-session?client_id=c1");
    }
}

#[cfg(test)]
mod manifest_tests {
    use serde_json::Value;
    use std::collections::BTreeSet;

    const MANIFEST: &str = include_str!("../../src/shared/hostMethods.json");
    const SOURCE: &str = include_str!("commands.rs");

    const BROWSER_PREFIX_METHODS: [&str; 12] = [
        "browserShow",
        "browserHide",
        "browserTab",
        "browserState",
        "browserNavigate",
        "browserCommand",
        "browserOpenExternal",
        "browserPickCancel",
        "browserPick",
        "browserLogs",
        "browserView",
        "browserPreview",
    ];

    const EXPLICIT_UNSUPPORTED: [&str; 2] = ["windowDimmed", "update"];

    #[test]
    fn every_host_method_has_a_native_dispatch_arm() {
        let manifest: BTreeSet<String> = serde_json::from_str::<Value>(MANIFEST)
            .unwrap()
            .as_object()
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let browser_arm = SOURCE.contains("method.starts_with(\"browser\")");
        assert!(browser_arm, "browser prefix dispatch arm is missing");
        let mut unmatched = Vec::new();
        for method in &manifest {
            if BROWSER_PREFIX_METHODS.contains(&method.as_str()) {
                continue;
            }
            if !SOURCE.contains(&format!("\"{method}\"")) {
                unmatched.push(method.clone());
            }
        }
        assert!(
            unmatched.is_empty(),
            "methods without a native dispatch arm: {unmatched:?}"
        );
        for method in EXPLICIT_UNSUPPORTED {
            assert!(manifest.contains(method), "{method} left the manifest");
        }
    }

    #[test]
    fn manifest_matches_preload_contract_shape() {
        let manifest: Value = serde_json::from_str(MANIFEST).unwrap();
        let object = manifest.as_object().unwrap();
        assert!(object.values().all(|value| value == &Value::Bool(true)));
        assert!(object.contains_key("call") && object.contains_key("info"));
    }
}

fn project_path(state: &AppState, id: &str) -> HostResult<String> {
    state
        .projects
        .lock()
        .unwrap()
        .get(id)
        .and_then(|project| project["path"].as_str())
        .map(str::to_owned)
        .ok_or_else(|| HostError::new("PROJECT_NOT_FOUND", "Project is not open"))
}
