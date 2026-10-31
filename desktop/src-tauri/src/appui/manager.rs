use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use base64::Engine;
use serde_json::{json, Value};
use tauri::{AppHandle, Emitter, Manager};

use super::types::Locator;
use super::uia::action::{UiAction, MAX_DEPTH, MAX_NODES};
use super::uia::worker::{UiaRequest, UiaWorker};
use crate::browser::workbench::WorkbenchContext;
use crate::computer::types::{Log, Share};

const SNAPSHOT_TIMEOUT: Duration = Duration::from_secs(20);
const ACTION_TIMEOUT: Duration = Duration::from_secs(15);
const MAX_ACTIONS_PER_MINUTE: usize = 200;
const MAX_LOG_ENTRIES: usize = 100;

#[derive(Clone)]
pub struct Target {
    pub workspace_id: String,
    pub session_id: String,
    pub hwnd: String,
    pub title: String,
    pub pid: u32,
    pub identity: String,
    pub generation: String,
}

pub struct AppUiManager {
    worker: Mutex<Option<Arc<UiaWorker>>>,
    targets: Mutex<HashMap<String, Target>>,
    logs: Mutex<HashMap<String, Vec<Log>>>,
    actions: Mutex<Vec<Instant>>,
}

impl AppUiManager {
    pub fn new() -> Self {
        Self {
            worker: Mutex::new(None),
            targets: Mutex::new(HashMap::new()),
            logs: Mutex::new(HashMap::new()),
            actions: Mutex::new(Vec::new()),
        }
    }

    pub fn targets(&self, workspace: &str) -> Result<Value, String> {
        let windows = crate::computer::windows::list()?;
        let target = self
            .targets
            .lock()
            .map_err(|error| error.to_string())?
            .get(workspace)
            .map(target_json);
        Ok(json!({"windows": windows, "target": target}))
    }

    pub fn state(&self, workspace: &str) -> Result<Value, String> {
        let target = self
            .targets
            .lock()
            .map_err(|error| error.to_string())?
            .get(workspace)
            .map(target_json);
        let log = self
            .logs
            .lock()
            .map_err(|error| error.to_string())?
            .get(workspace)
            .cloned()
            .unwrap_or_default();
        Ok(json!({"target": target, "log": log}))
    }

    pub fn attach(
        &self,
        app: &AppHandle,
        context: &WorkbenchContext,
        args: &Value,
    ) -> Result<Value, String> {
        let hwnd = argument(args, "hwnd")?;
        let identity = argument(args, "identity")?;
        let title = argument(args, "title")?;
        let mode = argument(args, "mode")?;
        if mode != "view" && mode != "control" {
            return Err("Mode must be view or control".to_owned());
        }
        let pid = args
            .get("pid")
            .and_then(Value::as_u64)
            .filter(|pid| *pid > 0 && *pid <= u32::MAX as u64)
            .ok_or("Missing argument: pid")? as u32;
        let state = app.state::<crate::state::AppState>();
        state.windows.tool(
            app,
            context,
            "window_request_access",
            &json!({"hwnd": hwnd, "pid": pid, "identity": identity, "title": title, "mode": mode}),
        )?;
        let share = state.windows.share_for(
            app,
            &context.workspace_id,
            context.session_id.as_deref(),
            mode == "control",
        )?;
        let target = Target {
            workspace_id: context.workspace_id.clone(),
            session_id: share.session_id.clone(),
            hwnd: share.hwnd.clone(),
            title: share.title.clone(),
            pid: share.pid,
            identity: share.identity.clone(),
            generation: share.generation.clone(),
        };
        self.targets
            .lock()
            .map_err(|error| error.to_string())?
            .insert(context.workspace_id.clone(), target.clone());
        self.record(
            app,
            &context.workspace_id,
            "attach",
            &target.title,
            Ok(()),
        );
        Ok(json!({"status": "attached", "mode": mode, "target": target_json(&target)}))
    }

    pub fn detach(&self, app: &AppHandle, context: &WorkbenchContext) -> Result<Value, String> {
        let removed = self
            .targets
            .lock()
            .map_err(|error| error.to_string())?
            .remove(&context.workspace_id);
        if let Some(target) = removed {
            self.record(app, &context.workspace_id, "detach", &target.title, Ok(()));
        }
        Ok(json!({"status": "detached"}))
    }

    pub fn snapshot(&self, app: &AppHandle, context: &WorkbenchContext) -> Result<Value, String> {
        let target = self.target(app, context, false)?;
        let result = self.call(
            UiaRequest::Snapshot {
                hwnd: target.hwnd.clone(),
                max_nodes: MAX_NODES,
                max_depth: MAX_DEPTH,
            },
            SNAPSHOT_TIMEOUT,
        );
        self.record(app, &context.workspace_id, "snapshot", &target.title, result.as_ref().map(|_| ()));
        result
    }

    pub fn act(
        &self,
        app: &AppHandle,
        context: &WorkbenchContext,
        locator: Locator,
        action: UiAction,
    ) -> Result<Value, String> {
        let target = self.target(app, context, true)?;
        self.budget()?;
        let name = action.name();
        let result = self.call(
            UiaRequest::Act {
                hwnd: target.hwnd.clone(),
                locator,
                action,
            },
            ACTION_TIMEOUT,
        );
        self.record(app, &context.workspace_id, name, &target.title, result.as_ref().map(|_| ()));
        result
    }

    pub fn type_text(
        &self,
        app: &AppHandle,
        context: &WorkbenchContext,
        locator: Locator,
        text: String,
    ) -> Result<Value, String> {
        let target = self.target(app, context, true)?;
        self.budget()?;
        let result = self.call(
            UiaRequest::Act {
                hwnd: target.hwnd.clone(),
                locator: locator.clone(),
                action: UiAction::SetValue(text.clone()),
            },
            ACTION_TIMEOUT,
        );
        let result = match result {
            Ok(value) => Ok(value),
            Err(_) => {
                self.call(
                    UiaRequest::Act {
                        hwnd: target.hwnd.clone(),
                        locator,
                        action: UiAction::Focus,
                    },
                    ACTION_TIMEOUT,
                )?;
                crate::computer::input::perform(
                    &target.hwnd,
                    &crate::computer::types::Action::Type { text },
                    || Ok(()),
                )?;
                Ok(json!({"status": "ok", "action": "type", "confidence": "medium"}))
            }
        };
        self.record(app, &context.workspace_id, "type", &target.title, result.as_ref().map(|_| ()));
        result
    }

    pub fn press_key(
        &self,
        app: &AppHandle,
        context: &WorkbenchContext,
        key: String,
    ) -> Result<Value, String> {
        let target = self.target(app, context, true)?;
        self.budget()?;
        let result = crate::computer::input::perform(
            &target.hwnd,
            &crate::computer::types::Action::PressKey { key },
            || Ok(()),
        )
        .map(|_| json!({"status": "ok", "action": "press_key"}));
        self.record(app, &context.workspace_id, "press_key", &target.title, result.as_ref().map(|_| ()));
        result
    }

    pub fn element_png(
        &self,
        app: &AppHandle,
        context: &WorkbenchContext,
        locator: Locator,
    ) -> Result<Value, String> {
        let target = self.target(app, context, false)?;
        let value = self.call(
            UiaRequest::ElementPng {
                hwnd: target.hwnd.clone(),
                locator,
            },
            SNAPSHOT_TIMEOUT,
        )?;
        let encoded = value["png"]
            .as_str()
            .ok_or("The element screenshot is unavailable")?;
        let bytes = base64::engine::general_purpose::STANDARD
            .decode(encoded)
            .map_err(|error| error.to_string())?;
        let stored = app
            .state::<crate::state::AppState>()
            .attachments
            .lock()
            .map_err(|error| error.to_string())?
            .image("element.png", &bytes)
            .map_err(|error| error.message)?;
        Ok(json!({
            "attachmentId": stored[0]["id"].clone(),
            "image": format!("data:image/png;base64,{encoded}"),
        }))
    }

    fn target(
        &self,
        app: &AppHandle,
        context: &WorkbenchContext,
        control: bool,
    ) -> Result<Target, String> {
        let target = self
            .targets
            .lock()
            .map_err(|error| error.to_string())?
            .get(&context.workspace_id)
            .cloned()
            .ok_or("No application is attached. Use app_attach.")?;
        let share = app
            .state::<crate::state::AppState>()
            .windows
            .share_for(
                app,
                &context.workspace_id,
                context.session_id.as_deref(),
                control,
            )?;
        check_target(&target, &share)?;
        Ok(target)
    }

    fn budget(&self) -> Result<(), String> {
        let mut actions = self.actions.lock().map_err(|error| error.to_string())?;
        let now = Instant::now();
        actions.retain(|time| now.duration_since(*time) < Duration::from_secs(60));
        if actions.len() >= MAX_ACTIONS_PER_MINUTE {
            return Err("Too many application actions in the last minute".to_owned());
        }
        actions.push(now);
        Ok(())
    }

    fn worker(&self) -> Result<Arc<UiaWorker>, String> {
        let mut slot = self.worker.lock().map_err(|error| error.to_string())?;
        if let Some(worker) = slot.as_ref() {
            return Ok(worker.clone());
        }
        let worker = Arc::new(UiaWorker::start()?);
        *slot = Some(worker.clone());
        Ok(worker)
    }

    fn call(&self, request: UiaRequest, timeout: Duration) -> Result<Value, String> {
        let worker = self.worker()?;
        match worker.call(request, timeout) {
            Ok(value) => Ok(value),
            Err(error) => {
                if error.contains("did not respond within") {
                    if let Ok(mut slot) = self.worker.lock() {
                        *slot = None;
                    }
                }
                Err(error)
            }
        }
    }

    fn record(
        &self,
        app: &AppHandle,
        workspace: &str,
        action: &str,
        title: &str,
        result: Result<(), &String>,
    ) {
        let entry = Log {
            time: now(),
            title: title.to_owned(),
            source: "appui".to_owned(),
            ok: result.is_ok(),
            action: action.to_owned(),
            error: result.err().cloned(),
        };
        if let Ok(mut logs) = self.logs.lock() {
            let entries = logs.entry(workspace.to_owned()).or_default();
            entries.push(entry);
            if entries.len() > MAX_LOG_ENTRIES {
                entries.remove(0);
            }
        }
        if let Ok(payload) = self.state(workspace) {
            let _ = app.emit_to(
                tauri::EventTarget::webview("main"),
                "prime:event",
                json!({"workspaceId": workspace, "type": "appui", "payload": payload}),
            );
        }
    }
}

impl Default for AppUiManager {
    fn default() -> Self {
        Self::new()
    }
}

fn check_target(target: &Target, share: &Share) -> Result<(), String> {
    if share.hwnd != target.hwnd || share.generation != target.generation {
        return Err("The shared window changed. Attach the application again.".to_owned());
    }
    if share.session_id != target.session_id {
        return Err("The shared window belongs to another chat".to_owned());
    }
    if share.identity != target.identity || share.pid != target.pid {
        return Err("The application was replaced. Attach it again.".to_owned());
    }
    Ok(())
}

fn target_json(target: &Target) -> Value {
    json!({
        "hwnd": target.hwnd,
        "title": target.title,
        "pid": target.pid,
        "identity": target.identity,
        "workspaceId": target.workspace_id,
    })
}

fn argument(args: &Value, name: &str) -> Result<String, String> {
    args.get(name)
        .and_then(Value::as_str)
        .filter(|value| !value.is_empty())
        .map(str::to_owned)
        .ok_or_else(|| format!("Missing argument: {name}"))
}

fn now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|value| value.as_secs())
        .unwrap_or(0)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::computer::types::AccessMode;

    fn target() -> Target {
        Target {
            workspace_id: "workspace".to_owned(),
            session_id: "session".to_owned(),
            hwnd: "42".to_owned(),
            title: "Prime app UI fixture".to_owned(),
            pid: 7,
            identity: "identity".to_owned(),
            generation: "generation".to_owned(),
        }
    }

    fn share() -> Share {
        Share {
            workspace_id: "workspace".to_owned(),
            session_id: "session".to_owned(),
            hwnd: "42".to_owned(),
            title: "Prime app UI fixture".to_owned(),
            pid: 7,
            identity: "identity".to_owned(),
            mode: AccessMode::Control,
            shared_at: 0,
            process_created: 0,
            generation: "generation".to_owned(),
        }
    }

    #[test]
    fn a_matching_share_is_accepted() {
        assert!(check_target(&target(), &share()).is_ok());
    }

    #[test]
    fn a_replaced_window_is_rejected() {
        let mut replaced = share();
        replaced.generation = "other".to_owned();
        assert!(check_target(&target(), &replaced).is_err());
        let mut other_window = share();
        other_window.hwnd = "43".to_owned();
        assert!(check_target(&target(), &other_window).is_err());
    }

    #[test]
    fn another_chat_cannot_reuse_the_attachment() {
        let mut foreign = share();
        foreign.session_id = "other".to_owned();
        assert!(check_target(&target(), &foreign).is_err());
    }

    #[test]
    fn a_replaced_process_is_rejected() {
        let mut replaced = share();
        replaced.identity = "other".to_owned();
        assert!(check_target(&target(), &replaced).is_err());
        let mut other_process = share();
        other_process.pid = 8;
        assert!(check_target(&target(), &other_process).is_err());
    }

    #[test]
    fn the_action_budget_is_enforced_per_minute() {
        let manager = AppUiManager::new();
        for _ in 0..MAX_ACTIONS_PER_MINUTE {
            assert!(manager.budget().is_ok());
        }
        assert!(manager.budget().is_err());
    }

    #[test]
    fn actions_without_an_attachment_are_rejected() {
        let manager = AppUiManager::new();
        assert!(manager
            .targets
            .lock()
            .unwrap()
            .get("workspace")
            .is_none());
        assert!(manager.state("workspace").unwrap()["target"].is_null());
    }
}
