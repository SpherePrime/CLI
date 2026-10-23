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
    tauri::async_runtime::spawn_blocking(move || dispatch(&app, &method, &args))
        .await
        .map_err(|error| HostError::new("HOST_WORKER", error.to_string()))?
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
                json!({"version":env!("CARGO_PKG_VERSION"),"runtime":if engine.starting{"starting"}else if engine.instance.is_some(){"ready"}else{"error"},"runtimeVersion":"native-abi-1","error":engine.error,"packaged":!cfg!(debug_assertions),"terminalCount":state.terminals.count()+state.remotes.terminals.count(),"automationCounts":state.automations.counts(),"update":{"status":"idle"},"host":"tauri","nativePreview":true}),
            )
        }
        "preferences" => {
            let mut settings = state.settings.lock().unwrap();
            match args.first().filter(|value| !value.is_null()) {
                Some(patch) => settings.update(patch),
                None => Ok(settings.get()),
            }
        }
        "speech" => state.speech.handle(app.clone(), args.first().unwrap_or(&Value::Null)),
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
            let mut sessions = serde_json::Map::new();
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
                    path.into()
                };
                sessions.insert(
                    path.to_owned(),
                    crate::projects::sessions(&state, &local_path)?,
                );
            }
            Ok(Value::Object(sessions))
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
            let path = state.directory.join("Chats");
            std::fs::create_dir_all(&path)?;
            crate::projects::open(app, &state, path, "chat")
        }
        "closeProject" => crate::projects::close(app, &state, string(args, 0)?),
        "windows" => state.windows.handle(app, string(args, 0)?, args.get(1).unwrap_or(&Value::Null))
            .map_err(|error| HostError::new("WINDOW_ERROR", error)),
        "automation" => {
            let id = string(args, 0)?;
            state.root(id)?;
            let root = state
                .projects
                .lock()
                .unwrap()
                .get(id)
                .and_then(|project| project["path"].as_str())
                .unwrap()
                .to_owned();
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
                let ids = args
                    .get(2)
                    .and_then(|args| args.get("attachmentIds"))
                    .cloned()
                    .unwrap_or(json!([]));
                route[2]["attachments"] = state.attachments.lock().unwrap().resolve(&ids)?;
            }
            if operation == "currentSession" {
                let id = string(args, 0)?;
                let mut projects = state.projects.lock().unwrap();
                let project = projects
                    .get_mut(id)
                    .ok_or_else(|| HostError::new("PROJECT_NOT_FOUND", "Project is not open"))?;
                let client = project["clientId"].as_str().unwrap_or("");
                route[1] = json!(format!("current-session?client_id={client}"));
                project["currentSessionId"] = args
                    .get(2)
                    .and_then(|args| args.get("sessionId"))
                    .cloned()
                    .unwrap_or(Value::Null);
            }
            let result = crate::projects::call(&state, string(args, 0)?, &route)?;
            if matches!(operation, "messages" | "messagePage" | "messageSearch") {
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
        "files" => crate::files::list(
            &state.root(string(args, 0)?)?,
            args.get(1).and_then(Value::as_str).unwrap_or(""),
        ),
        "readFile" => crate::files::read(&state.root(string(args, 0)?)?, string(args, 1)?),
        "saveFile" => {
            let _guard = state.file_writes.lock().unwrap();
            crate::files::save(
                &state.root(string(args, 0)?)?,
                string(args, 1)?,
                string(args, 2)?,
                string(args, 3)?,
            )
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
        "windowDimmed" => Err(HostError::unsupported(method)),
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
            let root = state.root(string(args, 0)?)?;
            let target = crate::files::scoped_path(
                &root,
                args.get(1).and_then(Value::as_str).unwrap_or(""),
                false,
            )?;
            open::that(if target.is_dir() {
                target
            } else {
                target.parent().unwrap().to_path_buf()
            })?;
            Ok(Value::Null)
        }
        "openWith" if args.get(2).and_then(Value::as_str) == Some("default") => {
            let target =
                crate::files::scoped_path(&state.root(string(args, 0)?)?, string(args, 1)?, false)?;
            open::that(target)?;
            Ok(Value::Null)
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
