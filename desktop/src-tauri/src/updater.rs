use crate::{
    error::{HostError, HostResult},
    events,
    state::AppState,
};
use serde_json::{json, Value};
use std::sync::{atomic::{AtomicBool, Ordering}, Mutex};
use tauri_plugin_updater::Update;
use tauri_plugin_updater::UpdaterExt;

static UPDATE_IN_PROGRESS: AtomicBool = AtomicBool::new(false);
static STATUS: Mutex<Option<Value>> = Mutex::new(None);
static DOWNLOADED: Mutex<Option<(Update, Vec<u8>)>> = Mutex::new(None);

pub fn snapshot() -> Value {
    STATUS.lock().unwrap().clone().unwrap_or_else(|| json!({"state":"idle"}))
}

pub fn handle(app: &tauri::AppHandle, state: &AppState, action: &str) -> HostResult<Value> {
    match action {
        "check" => check(app, state),
        "install" => install(app, state),
        _ => Err(HostError::new(
            "INVALID_ARGUMENT",
            "Update action must be check or install",
        )),
    }
}

fn status(app: &tauri::AppHandle, payload: Value) {
    *STATUS.lock().unwrap() = Some(payload.clone());
    events::emit(app, "", "update", payload);
}

fn check(app: &tauri::AppHandle, _state: &AppState) -> HostResult<Value> {
    if UPDATE_IN_PROGRESS.swap(true, Ordering::AcqRel) {
        return Err(HostError::new("UPDATE_BUSY", "Обновление уже выполняется"));
    }
    if DOWNLOADED.lock().unwrap().is_some() {
        UPDATE_IN_PROGRESS.store(false, Ordering::Release);
        return Ok(snapshot());
    }
    let handle = app.clone();
    let result = tauri::async_runtime::block_on(async move {
        let app = handle;
        if app.config().plugins.0.get("updater").and_then(|config| config.get("pubkey")).and_then(Value::as_str).unwrap_or("").is_empty() {
            let unavailable = json!({"state":"unavailable","message":"Signed native update feed is not configured"});
            status(&app, unavailable.clone());
            return Ok(unavailable);
        }
        status(&app, json!({"state":"checking"}));
        let updater = match app.updater() {
            Ok(updater) => updater,
            Err(error) => {
                return Err(HostError::new(
                    "UPDATE_UNAVAILABLE",
                    format!("Update service is unavailable: {error}"),
                ));
            }
        };
        let release = updater
            .check()
            .await
            .map_err(|error| HostError::new("UPDATE_CHECK", error.to_string()))?;
        let Some(release) = release else {
            status(&app, json!({"state":"current"}));
            return Ok(json!({"state":"current"}));
        };
        let version = release.version.clone();
        let version_for_progress = version.clone();
        status(&app, json!({"state":"available","version":version.clone()}));
        let app_for_progress = app.clone();
        let mut last_reported = 0u64;
        let outcome = release
            .download(
                move |chunk, total| {
                    last_reported += chunk as u64;
                    let percent = total
                        .filter(|total| *total > 0)
                        .map(|total| (last_reported.min(total) * 100 / total) as f64)
                        .unwrap_or(0.0);
                    status(
                        &app_for_progress,
                        json!({
                            "state":"downloading",
                            "version":version_for_progress.clone(),
                            "percent":percent,
                            "transferred":last_reported,
                            "total":total,
                        }),
                    );
                },
                || {},
            )
            .await;
        match outcome {
            Ok(bytes) => {
                *DOWNLOADED.lock().unwrap() = Some((release, bytes));
                status(&app, json!({"state":"downloaded","version":version}));
                Ok(json!({"state":"downloaded","version":version}))
            }
            Err(error) => {
                status(&app, json!({"state":"error","message":error.to_string()}));
                Err(HostError::new("UPDATE_INSTALL", error.to_string()))
            }
        }
    });
    if let Err(error) = &result {
        status(app, json!({"state":"error","message":error.message}));
    }
    UPDATE_IN_PROGRESS.store(false, Ordering::Release);
    result
}

fn install(app: &tauri::AppHandle, state: &AppState) -> HostResult<Value> {
    if UPDATE_IN_PROGRESS.swap(true, Ordering::AcqRel) {
        return Err(HostError::new("UPDATE_BUSY", "Update is already running"));
    }
    let result = (|| {
        if DOWNLOADED.lock().unwrap().is_none() {
            return Err(HostError::new("UPDATE_NOT_READY", "Update is not ready to install"));
        }
        require_idle(state)?;
        let (release, bytes) = DOWNLOADED.lock().unwrap().take().unwrap();
        status(app, json!({"state":"installing","version":release.version}));
        state.closing.store(true, Ordering::Release);
        crate::lifecycle::shutdown(app, state);
        if let Err(error) = release.install(&bytes) {
            let message = format!("{error}. Restart the application before continuing");
            status(app, json!({"state":"error","message":message}));
            return Err(HostError::new("UPDATE_INSTALL", message));
        }
        app.restart();
    })();
    UPDATE_IN_PROGRESS.store(false, Ordering::Release);
    result
}

fn require_idle(state: &AppState) -> HostResult<()> {
    if state.editor_dirty.load(Ordering::Acquire)
        || state.terminals.count() > 0
        || state.remotes.terminals.count() > 0
        || state.teams.active_count() > 0
        || state.automations.counts()["activeCount"].as_u64().unwrap_or(0) > 0
    {
        return Err(HostError::new("UPDATE_BUSY", "Finish active tasks, close terminals and save files before updating"));
    }
    let engine = state.require_engine()?;
    let projects = state.projects.lock().unwrap().values().cloned().collect::<Vec<_>>();
    for project in projects {
        let Some(id) = project["id"].as_str() else { continue };
        let sessions = if crate::remote::workspace_profile(state, id).is_some() {
            return Err(HostError::new("UPDATE_BUSY", "Close remote projects before updating"));
        } else {
            engine.request("GET", &format!("/v1/workspaces/{id}/sessions"), None)?
        };
        if sessions.as_array().is_some_and(|items| items.iter().any(|session| session["is_busy"] == true)) {
            return Err(HostError::new("UPDATE_BUSY", "Finish active chats before updating"));
        }
    }
    Ok(())
}
