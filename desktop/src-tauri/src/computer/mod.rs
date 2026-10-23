#[cfg(windows)]
mod capture;
#[cfg(windows)]
mod input;
pub mod tools;
mod types;
#[cfg(windows)]
mod windows;

use serde_json::{json, Value};
use std::{
    collections::HashMap,
    sync::{
        atomic::{AtomicBool, AtomicU32, Ordering},
        Arc, Mutex,
    },
};
use tauri::{AppHandle, Emitter, Manager};
use types::{AccessMode, Action, Log, Share, WindowInfo, Workspace};

pub struct WindowManager {
    state: Mutex<HashMap<String, Workspace>>,
    actions: Mutex<()>,
    emergency_ready: AtomicBool,
    emergency_thread: AtomicU32,
}

impl WindowManager {
    pub fn new() -> Self {
        Self {
            state: Mutex::new(HashMap::new()),
            actions: Mutex::new(()),
            emergency_ready: AtomicBool::new(false),
            emergency_thread: AtomicU32::new(0),
        }
    }

    pub fn state(&self, workspace: &str) -> Result<Value, String> {
        let states = self.state.lock().map_err(|error| error.to_string())?;
        Ok(match states.get(workspace) {
            Some(state) => {
                json!({"share":state.share,"log":state.logs.iter().rev().collect::<Vec<_>>()})
            }
            None => json!({"log":[]}),
        })
    }

    fn publish(&self, app: &AppHandle, workspace: &str) {
        if let Ok(payload) = self.state(workspace) {
            let _ = app.emit_to(
                tauri::EventTarget::webview("main"),
                "prime:event",
                json!({"workspaceId":workspace,"type":"windows","payload":payload}),
            );
        }
    }

    fn record(
        &self,
        app: &AppHandle,
        workspace: &str,
        title: &str,
        source: &str,
        action: &str,
        result: Result<(), String>,
    ) {
        if let Ok(mut states) = self.state.lock() {
            let state = states.entry(workspace.to_owned()).or_default();
            state.logs.push(Log {
                time: now(),
                title: title.to_owned(),
                source: source.to_owned(),
                ok: result.is_ok(),
                action: action.to_owned(),
                error: result.err(),
            });
            let excess = state.logs.len().saturating_sub(100);
            state.logs.drain(..excess);
        }
        self.publish(app, workspace);
    }

    pub fn forget(&self, app: &AppHandle, workspace: &str) {
        if let Ok(mut states) = self.state.lock() {
            states.remove(workspace);
        }
        self.publish(app, workspace);
    }

    pub fn revoke_all(&self, app: &AppHandle) {
        let workspaces = if let Ok(mut states) = self.state.lock() {
            states
                .iter_mut()
                .map(|(id, state)| {
                    state.share = None;
                    id.clone()
                })
                .collect::<Vec<_>>()
        } else {
            Vec::new()
        };
        for workspace in workspaces {
            self.publish(app, &workspace);
        }
    }

    fn guard(&self, app: &AppHandle, share: &Share) -> Result<WindowInfo, String> {
        {
            let states = self.state.lock().map_err(|error| error.to_string())?;
            let current = states
                .get(&share.workspace_id)
                .and_then(|state| state.share.as_ref())
                .ok_or("Window access was revoked")?;
            if current.generation != share.generation {
                return Err("Window access was replaced or revoked".to_owned());
            }
        }
        let result = verify(
            &share.hwnd,
            share.pid,
            &share.identity,
            share.process_created,
        );
        if result.is_err() {
            if let Ok(mut states) = self.state.lock() {
                if let Some(state) = states.get_mut(&share.workspace_id) {
                    if state
                        .share
                        .as_ref()
                        .is_some_and(|current| current.generation == share.generation)
                    {
                        state.share = None;
                    }
                }
            }
            self.publish(app, &share.workspace_id);
        }
        result
    }

    fn active_session(
        app: &AppHandle,
        workspace: &str,
        session: Option<&str>,
        busy: bool,
    ) -> Result<String, String> {
        let session = session
            .filter(|session| !session.is_empty() && session.len() <= 200)
            .ok_or("The active chat cannot be identified unambiguously")?;
        let state = app.state::<crate::state::AppState>();
        state.root(workspace).map_err(|error| error.message)?;
        let sessions = state
            .require_engine()
            .map_err(|error| error.message)?
            .request("GET", &format!("/v1/workspaces/{workspace}/sessions"), None)
            .map_err(|error| error.message)?;
        if !sessions.as_array().is_some_and(|sessions| {
            sessions
                .iter()
                .any(|item| item["id"] == session && (!busy || item["is_busy"] == true))
        }) {
            return Err(
                "The window is shared with a different chat, or this chat is no longer active"
                    .to_owned(),
            );
        }
        Ok(session.to_owned())
    }

    fn grant(
        &self,
        app: &AppHandle,
        workspace: &str,
        session: String,
        info: WindowInfo,
        mode: AccessMode,
        source: &str,
    ) -> Result<Value, String> {
        if mode == AccessMode::Control && !self.emergency_ready.load(Ordering::Acquire) {
            return Err(
                "Window control is unavailable because Ctrl+Alt+Escape could not be registered"
                    .to_owned(),
            );
        }
        let identity = info
            .identity
            .clone()
            .ok_or("Cannot verify this window identity")?;
        let share = Share {
            workspace_id: workspace.to_owned(),
            session_id: session,
            hwnd: info.hwnd.clone(),
            title: info.title.clone(),
            pid: info.pid,
            identity,
            mode: mode.clone(),
            shared_at: now(),
            process_created: info.process_created,
            generation: uuid::Uuid::new_v4().to_string(),
        };
        verify(
            &share.hwnd,
            share.pid,
            &share.identity,
            share.process_created,
        )?;
        self.state
            .lock()
            .map_err(|error| error.to_string())?
            .entry(workspace.to_owned())
            .or_default()
            .share = Some(share);
        self.record(
            app,
            workspace,
            &info.title,
            source,
            if mode == AccessMode::Control {
                "grant_control"
            } else {
                "grant_view"
            },
            Ok(()),
        );
        self.state(workspace)
    }

    fn access(
        &self,
        app: &AppHandle,
        workspace: &str,
        session: Option<&str>,
        control: bool,
    ) -> Result<Share, String> {
        if control && !self.emergency_ready.load(Ordering::Acquire) {
            return Err(
                "Window control is unavailable because the emergency stop is not active".to_owned(),
            );
        }
        let session = Self::active_session(app, workspace, session, true)?;
        let share = self
            .state
            .lock()
            .map_err(|error| error.to_string())?
            .get(workspace)
            .and_then(|state| state.share.clone())
            .ok_or("The user has not shared a window with this chat. Use window_request_access.")?;
        check_access(&share, &session, control)?;
        self.guard(app, &share)?;
        Ok(share)
    }

    pub fn handle(
        &self,
        app: &AppHandle,
        workspace: &str,
        request: &Value,
    ) -> Result<Value, String> {
        app.state::<crate::state::AppState>()
            .root(workspace)
            .map_err(|error| error.message)?;
        let action = string(request, "action")?;
        match action {
            "state" => self.state(workspace),
            "list" => Ok(json!(list()?)),
            "revoke" => {
                if let Some(state) = self
                    .state
                    .lock()
                    .map_err(|error| error.to_string())?
                    .get_mut(workspace)
                {
                    state.share = None;
                }
                self.publish(app, workspace);
                self.state(workspace)
            }
            "dismiss" => {
                self.publish(app, workspace);
                self.state(workspace)
            }
            "snapshot" => {
                let captured = snapshot(string(request, "hwnd")?)?;
                self.record(
                    app,
                    workspace,
                    captured["info"]["title"].as_str().unwrap_or(""),
                    "user",
                    "snapshot",
                    Ok(()),
                );
                Ok(captured)
            }
            "share" => {
                let session = Self::active_session(
                    app,
                    workspace,
                    Some(string(request, "sessionId")?),
                    false,
                )?;
                let info = claim(string(request, "hwnd")?)?;
                if request
                    .get("expectedPid")
                    .is_some_and(|pid| pid.as_u64() != Some(info.pid as u64))
                    || request
                        .get("expectedIdentity")
                        .is_some_and(|identity| identity.as_str() != info.identity.as_deref())
                {
                    return Err(
                        "The selected window was closed or replaced. Refresh the window list."
                            .to_owned(),
                    );
                }
                let mode =
                    serde_json::from_value(request.get("mode").cloned().unwrap_or(json!("view")))
                        .map_err(|error| error.to_string())?;
                self.grant(app, workspace, session, info, mode, "user")
            }
            _ => Err("Unknown window request".to_owned()),
        }
    }

    pub fn tool(
        &self,
        app: &AppHandle,
        context: &crate::browser::workbench::WorkbenchContext,
        name: &str,
        args: &Value,
    ) -> Result<Value, String> {
        use base64::Engine;
        let workspace = &context.workspace_id;
        app.state::<crate::state::AppState>()
            .root(workspace)
            .map_err(|error| error.message)?;
        match name {
            "window_list" => Ok(text(json!(list()?))),
            "window_request_access" => {
                let session =
                    Self::active_session(app, workspace, context.session_id.as_deref(), true)?;
                let info = claim(string(args, "hwnd")?)?;
                if args["pid"].as_u64() != Some(info.pid as u64)
                    || args["identity"].as_str() != info.identity.as_deref()
                    || args["title"].as_str() != Some(info.title.as_str())
                {
                    return Err("The selected window was closed or replaced. Refresh window_list and request access again.".to_owned());
                }
                let mode: AccessMode = serde_json::from_value(args["mode"].clone())
                    .map_err(|error| error.to_string())?;
                self.grant(
                    app,
                    workspace,
                    session.clone(),
                    info.clone(),
                    mode.clone(),
                    "agent",
                )?;
                Ok(text(
                    json!({"status":"granted","window":info,"mode":mode,"sessionId":session}),
                ))
            }
            "window_view" => {
                let share = self.access(app, workspace, context.session_id.as_deref(), false)?;
                self.guard(app, &share)?;
                let result = snapshot(&share.hwnd).and_then(|snapshot| {
                    self.guard(app, &share)?;
                    Ok(snapshot)
                });
                self.record(
                    app,
                    workspace,
                    &share.title,
                    "agent",
                    "snapshot",
                    result.as_ref().map(|_| ()).map_err(Clone::clone),
                );
                let captured = result?;
                let data = captured["image"]
                    .as_str()
                    .and_then(|value| value.strip_prefix("data:image/png;base64,"))
                    .ok_or("Invalid window image")?;
                let _ = base64::engine::general_purpose::STANDARD
                    .decode(data)
                    .map_err(|error| error.to_string())?;
                Ok(
                    json!([{"type":"text","text":json!({"window":captured["info"],"imageSize":captured["imageSize"],"mode":share.mode}).to_string()},{"type":"image","data":data,"mimeType":"image/png"}]),
                )
            }
            "window_action" => {
                let action: Action =
                    serde_json::from_value(args.clone()).map_err(|error| error.to_string())?;
                action.validate()?;
                let share = self.access(app, workspace, context.session_id.as_deref(), true)?;
                let _operation = self.actions.lock().map_err(|error| error.to_string())?;
                let result = perform(&share.hwnd, &action, || self.guard(app, &share).map(|_| ()));
                self.record(
                    app,
                    workspace,
                    &share.title,
                    "agent",
                    action.name(),
                    result.clone(),
                );
                result?;
                Ok(text(
                    json!({"status":"ok","action":action.name(),"hwnd":share.hwnd}),
                ))
            }
            _ => Err("Unknown window tool".to_owned()),
        }
    }

    pub fn start(self: &Arc<Self>, app: &AppHandle) -> Result<(), String> {
        #[cfg(windows)]
        {
            super_start(self, app)
        }
        #[cfg(not(windows))]
        {
            let _ = app;
            Ok(())
        }
    }

    pub fn shutdown(&self, app: &AppHandle) {
        self.emergency_ready.store(false, Ordering::Release);
        self.revoke_all(app);
        #[cfg(windows)]
        {
            let thread = self.emergency_thread.swap(0, Ordering::AcqRel);
            if thread != 0 {
                unsafe {
                    let _ = ::windows::Win32::UI::WindowsAndMessaging::PostThreadMessageW(
                        thread,
                        ::windows::Win32::UI::WindowsAndMessaging::WM_QUIT,
                        ::windows::Win32::Foundation::WPARAM(0),
                        ::windows::Win32::Foundation::LPARAM(0),
                    );
                }
            }
        }
    }
}

fn string<'a>(value: &'a Value, key: &str) -> Result<&'a str, String> {
    value[key]
        .as_str()
        .ok_or_else(|| format!("{key} must be a string"))
}
fn check_access(share: &Share, session: &str, control: bool) -> Result<(), String> {
    if share.session_id != session {
        return Err("The window is shared with a different chat".to_owned());
    }
    if control && share.mode != AccessMode::Control {
        return Err(
            "This window is view-only. Request control through window_request_access.".to_owned(),
        );
    }
    Ok(())
}
fn now() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64
}
fn text(value: Value) -> Value {
    json!([{"type":"text","text":value.to_string()}])
}

fn list() -> Result<Vec<WindowInfo>, String> {
    #[cfg(windows)]
    {
        windows::list()
    }
    #[cfg(not(windows))]
    {
        Err("Desktop window viewing and control are currently supported only on Windows".to_owned())
    }
}
fn claim(hwnd: &str) -> Result<WindowInfo, String> {
    #[cfg(windows)]
    {
        windows::claim(hwnd)
    }
    #[cfg(not(windows))]
    {
        let _ = hwnd;
        Err("Desktop window viewing and control are currently supported only on Windows".to_owned())
    }
}
fn verify(hwnd: &str, pid: u32, identity: &str, created: u64) -> Result<WindowInfo, String> {
    #[cfg(windows)]
    {
        windows::verify(hwnd, pid, identity, created)
    }
    #[cfg(not(windows))]
    {
        let _ = (hwnd, pid, identity, created);
        Err("Desktop window identity checks require Windows".to_owned())
    }
}
fn perform(
    hwnd: &str,
    action: &Action,
    guard: impl Fn() -> Result<(), String>,
) -> Result<(), String> {
    #[cfg(windows)]
    {
        input::perform(hwnd, action, guard)
    }
    #[cfg(not(windows))]
    {
        let _ = (hwnd, action, guard);
        Err("Desktop window control requires Windows".to_owned())
    }
}
fn snapshot(hwnd: &str) -> Result<Value, String> {
    #[cfg(windows)]
    {
        use base64::Engine;
        let captured = capture::capture(hwnd)?;
        windows::verify(
            hwnd,
            captured.info.pid,
            captured
                .info
                .identity
                .as_deref()
                .ok_or("Window identity is unavailable")?,
            captured.info.process_created,
        )?;
        Ok(
            json!({"info":captured.info,"imageSize":{"width":captured.width,"height":captured.height},"image":format!("data:image/png;base64,{}",base64::engine::general_purpose::STANDARD.encode(captured.png))}),
        )
    }
    #[cfg(not(windows))]
    {
        let _ = hwnd;
        Err("Desktop window screenshots require Windows".to_owned())
    }
}

#[cfg(windows)]
fn super_start(manager: &Arc<WindowManager>, app: &AppHandle) -> Result<(), String> {
    use ::windows::Win32::{
        System::Threading::GetCurrentThreadId,
        UI::{
            Input::KeyboardAndMouse::{
                RegisterHotKey, UnregisterHotKey, MOD_ALT, MOD_CONTROL, MOD_NOREPEAT, VK_ESCAPE,
            },
            WindowsAndMessaging::{GetMessageW, MSG, WM_HOTKEY},
        },
    };
    if manager.emergency_thread.load(Ordering::Acquire) != 0 {
        return Ok(());
    }
    let manager = manager.clone();
    let app = app.clone();
    let (sender, receiver) = std::sync::mpsc::channel();
    std::thread::spawn(move || unsafe {
        if let Err(error) = RegisterHotKey(
            None,
            0x5052,
            MOD_CONTROL | MOD_ALT | MOD_NOREPEAT,
            VK_ESCAPE.0 as u32,
        ) {
            let _ = sender.send(Err(error.to_string()));
            return;
        }
        manager
            .emergency_thread
            .store(GetCurrentThreadId(), Ordering::Release);
        manager.emergency_ready.store(true, Ordering::Release);
        let _ = sender.send(Ok(()));
        let mut message = MSG::default();
        while GetMessageW(&mut message, None, 0, 0).0 > 0 {
            if message.message == WM_HOTKEY && message.wParam.0 == 0x5052 {
                manager.revoke_all(&app);
            }
        }
        let _ = UnregisterHotKey(None, 0x5052);
        manager.emergency_ready.store(false, Ordering::Release);
        manager.revoke_all(&app);
        manager.emergency_thread.store(0, Ordering::Release);
    });
    receiver
        .recv_timeout(std::time::Duration::from_secs(5))
        .map_err(|_| "Emergency stop registration timed out".to_owned())?
}

#[cfg(test)]
mod tests {
    use super::*;
    fn share(mode: AccessMode) -> Share {
        Share {
            workspace_id: "workspace".to_owned(),
            session_id: "approved-chat".to_owned(),
            hwnd: "123".to_owned(),
            title: "Fixture".to_owned(),
            pid: 42,
            identity: "random".to_owned(),
            mode,
            shared_at: 0,
            process_created: 100,
            generation: "grant-generation".to_owned(),
        }
    }
    #[test]
    fn chat_and_mode_cannot_be_upgraded_by_another_call() {
        let view = share(AccessMode::View);
        assert!(check_access(&view, "approved-chat", false).is_ok());
        assert!(check_access(&view, "approved-chat", true).is_err());
        assert!(check_access(&view, "other-chat", false).is_err());
        let control = share(AccessMode::Control);
        assert!(check_access(&control, "approved-chat", true).is_ok());
        assert!(check_access(&control, "other-chat", true).is_err());
    }
    #[test]
    fn desktop_wide_shortcuts_and_invalid_coordinates_are_rejected() {
        for key in [
            "ALT+TAB",
            "ALT+ESC",
            "CTRL+ESCAPE",
            "CTRL+ALT+DELETE",
            "WIN+R",
        ] {
            assert!(types::key_codes(key).is_err(), "{key}");
        }
        assert_eq!(types::key_codes("CTRL+A").unwrap(), vec![17, 65]);
        assert!(Action::Click {
            x: -1,
            y: 0,
            button: None,
            count: None
        }
        .validate()
        .is_err());
        assert!(Action::Scroll {
            x: 0,
            y: 0,
            delta: 0
        }
        .validate()
        .is_err());
        assert!(Action::Type {
            text: "x".repeat(1001)
        }
        .validate()
        .is_err());
    }
    #[test]
    fn action_schema_rejects_scope_or_privilege_overrides() {
        assert!(
            serde_json::from_value::<Action>(json!({"action":"focus","hwnd":"another"})).is_err()
        );
        assert!(serde_json::from_value::<Action>(
            json!({"action":"type","text":"hello","mode":"control"})
        )
        .is_err());
    }
}
