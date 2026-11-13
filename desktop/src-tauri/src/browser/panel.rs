use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex,
    },
};
use super::placement::{self, TabPlacement, OFFSCREEN_OFFSET};
use tauri::{
    webview::{NewWindowResponse, PageLoadEvent, PermissionResponse, WebviewBuilder},
    AppHandle, Emitter, LogicalPosition, LogicalSize, Manager, Rect, Webview, WebviewUrl,
};
use url::Url;

#[derive(Clone, Debug, Deserialize, PartialEq)]
pub struct Bounds {
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
}

impl Default for Bounds {
    fn default() -> Self {
        Self {
            x: 0.0,
            y: 0.0,
            width: 1280.0,
            height: 720.0,
        }
    }
}

#[derive(Clone, Serialize)]
pub struct Tab {
    pub id: String,
    pub url: String,
    pub title: String,
    pub loading: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub owner: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub viewport: Option<f64>,
    #[serde(skip)]
    pub history: Vec<String>,
    #[serde(skip)]
    pub index: usize,
    #[serde(skip)]
    pub traversing: bool,
    #[serde(skip)]
    pub load_error: Option<String>,
}

#[derive(Default)]
pub struct Panel {
    pub tabs: Vec<Tab>,
    pub active: Option<String>,
    pub visible: bool,
    pub inspector_open: bool,
    pub bounds: Bounds,
    pub logs: Vec<Value>,
    pub view: Option<String>,
    pub selected: HashMap<String, String>,
    pub agent: HashMap<String, String>,
    pub rendering: HashMap<String, usize>,
}

#[derive(Clone, Default)]
pub struct BrowserState {
    pub inner: Arc<Mutex<Panel>>,
    pub creation: Arc<Mutex<()>>,
    pub applied: Arc<Mutex<HashMap<String, TabPlacement>>>,
    monitoring: Arc<AtomicBool>,
}

impl BrowserState {
    pub fn new() -> Self {
        Self::default()
    }

    fn monitor(&self, app: &AppHandle) {
        if self.monitoring.swap(true, Ordering::Relaxed) {
            return;
        }
        let state = self.clone();
        let app = app.clone();
        std::thread::spawn(move || {
            while app.get_window("main").is_some() {
                let _ = super::refresh(&app, &state);
                std::thread::sleep(std::time::Duration::from_secs(1));
            }
            state.monitoring.store(false, Ordering::Relaxed);
        });
    }

    pub fn state(&self) -> Result<Value, String> {
        let panel = self.inner.lock().map_err(|error| error.to_string())?;
        let current = panel
            .tabs
            .iter()
            .find(|tab| Some(&tab.id) == panel.active.as_ref());
        let offscreen_viewport = current.and_then(|tab| {
            tab.viewport.filter(|width| {
                panel.visible
                    && panel.bounds.width > 0.0
                    && width.round() > panel.bounds.width.round()
            })
        });
        let mut result = json!({
            "url": current.map(|tab| tab.url.as_str()).unwrap_or(""),
            "title": current.map(|tab| tab.title.as_str()).unwrap_or(""),
            "loading": current.map(|tab| tab.loading).unwrap_or(false),
            "canGoBack": current.map(|tab| tab.index > 0).unwrap_or(false),
            "canGoForward": current.map(|tab| tab.index + 1 < tab.history.len()).unwrap_or(false),
            "activeTab": panel.active,
            "tabs": panel.visible_tabs(),
        });
        if let Some(width) = offscreen_viewport {
            result["offscreenViewport"] = json!(width);
        }
        Ok(result)
    }

    pub fn session_state(&self, session: Option<&str>) -> Result<Value, String> {
        let panel = self.inner.lock().map_err(|error| error.to_string())?;
        let active = match session {
            Some(session) => panel.agent_tab(session),
            None => panel.active.clone(),
        };
        Ok(json!({"tabs":panel.session_tabs(session),"activeTab":active}))
    }

    pub fn set_view(&self, app: &AppHandle, view: Option<String>) -> Result<Value, String> {
        let changed = self
            .inner
            .lock()
            .map_err(|error| error.to_string())?
            .set_view(view);
        if changed {
            self.attach(app)?;
            self.publish(app);
        }
        self.state()
    }

    pub fn agent_target(
        &self,
        app: &AppHandle,
        session: Option<&str>,
        id: Option<&str>,
    ) -> Result<(String, Webview), String> {
        let Some(session) = session else {
            return self.target(app, id);
        };
        if let Some(id) = id {
            self.inner
                .lock()
                .map_err(|error| error.to_string())?
                .accessible(Some(session), id)?;
            return self.target(app, Some(id));
        }
        let current = {
            let _creation = self.creation.lock().map_err(|error| error.to_string())?;
            let existing = self
                .inner
                .lock()
                .map_err(|error| error.to_string())?
                .agent_tab(session);
            match existing {
                Some(id) => id,
                None => {
                    let created = self.open_inner(app, "about:blank", false, Some(session))?;
                    let id = created["id"].as_str().ok_or("Вкладка не создана")?.to_owned();
                    self.bind_agent(session, &id)?;
                    id
                }
            }
        };
        self.target(app, Some(&current))
    }

    pub fn bind_agent(&self, session: &str, id: &str) -> Result<(), String> {
        self.inner
            .lock()
            .map_err(|error| error.to_string())?
            .agent
            .insert(session.to_owned(), id.to_owned());
        Ok(())
    }

    pub fn hold_rendering(&self, id: &str) -> Result<bool, String> {
        let mut panel = self.inner.lock().map_err(|error| error.to_string())?;
        let count = panel.rendering.entry(id.to_owned()).or_insert(0);
        *count += 1;
        Ok(*count == 1)
    }

    pub fn release_rendering(&self, id: &str) -> bool {
        let Ok(mut panel) = self.inner.lock() else {
            return false;
        };
        match panel.rendering.get_mut(id) {
            Some(count) if *count > 1 => {
                *count -= 1;
                false
            }
            Some(_) => {
                panel.rendering.remove(id);
                true
            }
            None => false,
        }
    }

    pub fn set_inspector_open(&self, open: bool) -> Result<(), String> {
        let mut panel = self.inner.lock().map_err(|error| error.to_string())?;
        panel.inspector_open = open;
        Ok(())
    }

    pub fn set_viewport(&self, id: &str, viewport: Option<f64>) -> Result<(), String> {
        let mut panel = self.inner.lock().map_err(|error| error.to_string())?;
        if let Some(tab) = panel.tabs.iter_mut().find(|tab| tab.id == id) {
            tab.viewport = viewport;
        }
        Ok(())
    }

    pub fn is_shown(&self, id: &str) -> Result<bool, String> {
        let panel = self.inner.lock().map_err(|error| error.to_string())?;
        let tab_viewport = panel
            .tabs
            .iter()
            .find(|tab| tab.id == id)
            .and_then(|tab| tab.viewport);
        Ok(panel.visible
            && panel.bounds.width > 0.0
            && panel.bounds.height > 0.0
            && panel.active.as_deref() == Some(id)
            && placement::fits_panel(&panel.bounds, tab_viewport))
    }

    pub fn publish(&self, app: &AppHandle) {
        if let Ok(state) = self.state() {
            let mut payload = state;
            payload["action"] = json!("state");
            let _ = app.emit_to(
                tauri::EventTarget::webview("main"),
                "prime:event",
                json!({"type":"browser", "payload":payload}),
            );
        }
    }

    pub fn reveal_target(&self, session: Option<&str>, tab: Option<&str>) -> (bool, bool) {
        match tab {
            Some(id) => match self.inner.lock() {
                Ok(mut panel) => (panel.show(session, id), panel.inspector_open),
                Err(_) => (false, false),
            },
            None => match self.inner.lock() {
                Ok(panel) => (true, panel.inspector_open),
                Err(_) => (false, false),
            },
        }
    }

    pub fn reveal(
        &self,
        app: &AppHandle,
        session: Option<&str>,
        tab: Option<&str>,
        agent: bool,
        _width: Option<f64>,
    ) -> bool {
        let (on_screen, inspector_open) = self.reveal_target(session, tab);
        if !on_screen {
            return false;
        }
        if tab.is_some() {
            let _ = self.attach(app);
            self.publish(app);
        }
        if !agent || inspector_open {
            let _ = app.emit_to(
                tauri::EventTarget::webview("main"),
                "prime:event",
                json!({
                    "type": "browser",
                    "payload": {
                        "action": "activate",
                        "agent": agent,
                        "sessionId": session
                    }
                }),
            );
        }
        true
    }

    pub fn target(&self, app: &AppHandle, id: Option<&str>) -> Result<(String, Webview), String> {
        if id.is_none() {
            let _creation = self.creation.lock().map_err(|error| error.to_string())?;
            let (empty, view) = {
                let panel = self.inner.lock().map_err(|error| error.to_string())?;
                (panel.visible_tabs().is_empty(), panel.view.clone())
            };
            if empty {
                self.open_inner(app, "about:blank", true, view.as_deref())?;
            }
        }
        let panel = self.inner.lock().map_err(|error| error.to_string())?;
        let selected = id.or(panel.active.as_deref()).ok_or("Вкладка не найдена")?;
        let tab = panel
            .tabs
            .iter()
            .find(|tab| tab.id == selected)
            .ok_or("Вкладка не найдена")?;
        let view = app.get_webview(&label(&tab.id)).ok_or("Вкладка закрыта")?;
        Ok((tab.id.clone(), view))
    }

    pub fn attach(&self, app: &AppHandle) -> Result<(), String> {
        let mut applied = self.applied.lock().map_err(|error| error.to_string())?;
        let (tabs, active, visible, bounds, rendering) = {
            let panel = self.inner.lock().map_err(|error| error.to_string())?;
            (
                panel.tabs.clone(),
                panel.active.clone(),
                panel.visible,
                panel.bounds.clone(),
                panel.rendering.clone(),
            )
        };
        applied.retain(|id, _| tabs.iter().any(|tab| &tab.id == id));
        for tab in tabs {
            let Some(view) = app.get_webview(&label(&tab.id)) else {
                continue;
            };
            let shown = visible
                && bounds.width > 0.0
                && bounds.height > 0.0
                && active.as_ref() == Some(&tab.id);
            let offscreen = rendering.contains_key(&tab.id)
                && (!shown || !placement::fits_panel(&bounds, tab.viewport));
            let target = placement::compute_placement(shown, offscreen, &bounds, tab.viewport);
            let previous = applied.get(&tab.id).copied();
            if previous == Some(target) {
                continue;
            }
            let rect = Rect {
                position: LogicalPosition::new(target.x, target.y).into(),
                size: LogicalSize::new(target.width, target.height).into(),
            };
            let bounds_changed = previous
                .map(|p| {
                    p.x != target.x
                        || p.y != target.y
                        || p.width != target.width
                        || p.height != target.height
                })
                .unwrap_or(true);
            if !target.visible {
                if previous.map(|p| p.visible).unwrap_or(true) {
                    view.hide().map_err(|error| error.to_string())?;
                }
                if bounds_changed {
                    view.set_bounds(rect).map_err(|error| error.to_string())?;
                }
            } else {
                if bounds_changed {
                    view.set_bounds(rect).map_err(|error| error.to_string())?;
                }
                if !previous.map(|p| p.visible).unwrap_or(false) {
                    view.show().map_err(|error| error.to_string())?;
                }
            }
            applied.insert(tab.id, target);
        }
        Ok(())
    }

    pub fn open(
        &self,
        app: &AppHandle,
        input: &str,
        activate: bool,
        owner: Option<&str>,
    ) -> Result<Value, String> {
        let _creation = self.creation.lock().map_err(|error| error.to_string())?;
        self.open_inner(app, input, activate, owner)
    }

    fn open_inner(
        &self,
        app: &AppHandle,
        input: &str,
        activate: bool,
        owner: Option<&str>,
    ) -> Result<Value, String> {
        let url = normalize_url(input)?;
        let normalized = url.to_string();
        let id = uuid::Uuid::new_v4().to_string();
        {
            let mut panel = self.inner.lock().map_err(|error| error.to_string())?;
            if panel.owned_count(owner) >= super::ownership::TABS_PER_OWNER {
                return Err("Можно открыть не больше 12 вкладок".to_owned());
            }
            let tab = Tab {
                id: id.clone(),
                url: url.to_string(),
                title: String::new(),
                loading: true,
                owner: owner.map(str::to_owned),
                viewport: None,
                history: vec![url.to_string()],
                index: 0,
                traversing: false,
                load_error: None,
            };
            let visible = tab.visible_to(panel.view.as_deref());
            panel.tabs.push(tab);
            if activate {
                panel.show(owner, &id);
            } else if visible && panel.active.is_none() {
                panel.active = Some(id.clone());
            }
        }
        let (initial_width, initial_height) = {
            let panel = self.inner.lock().map_err(|error| error.to_string())?;
            let width = if panel.bounds.width > 0.0 {
                panel.bounds.width.round().max(1.0)
            } else {
                1280.0
            };
            let height = panel.bounds.height.max(720.0).round().max(1.0);
            (width, height)
        };
        let navigation_state = self.clone();
        let navigation_id = id.clone();
        let load_state = self.clone();
        let load_id = id.clone();
        let title_state = self.clone();
        let title_id = id.clone();
        let popup_state = self.clone();
        let popup_app = app.clone();
        let popup_id = id.clone();
        let mut builder = WebviewBuilder::new(label(&id), WebviewUrl::External(url))
            .focused(activate)
            .initialization_script(include_str!("telemetry.js"))
            .disable_drag_drop_handler()
            .on_permission_request(|_, _| PermissionResponse::Deny)
            .on_navigation(move |url| {
                if !allowed(url) {
                    return false;
                }
                if let Ok(mut panel) = navigation_state.inner.lock() {
                    if let Some(tab) = panel.tabs.iter_mut().find(|tab| tab.id == navigation_id) {
                        if !tab.traversing && tab.url != url.as_str() {
                            tab.history.truncate(tab.index + 1);
                            tab.history.push(url.to_string());
                            tab.index = tab.history.len() - 1;
                        }
                        tab.traversing = false;
                        tab.url = url.to_string();
                        tab.loading = true;
                        tab.load_error = None;
                    }
                }
                true
            })
            .on_page_load(move |view, payload| {
                if let Ok(mut panel) = load_state.inner.lock() {
                    if let Some(tab) = panel.tabs.iter_mut().find(|tab| tab.id == load_id) {
                        tab.url = payload.url().to_string();
                        tab.loading = matches!(payload.event(), PageLoadEvent::Started);
                    }
                }
                load_state.publish(view.app_handle());
            })
            .on_document_title_changed(move |view, title| {
                if let Ok(mut panel) = title_state.inner.lock() {
                    if let Some(tab) = panel.tabs.iter_mut().find(|tab| tab.id == title_id) {
                        tab.title = title;
                    }
                }
                title_state.publish(view.app_handle());
            })
            .on_new_window(move |url, _| {
                if allowed(&url) {
                    let state = popup_state.clone();
                    let app = popup_app.clone();
                    let source_id = popup_id.clone();
                    let (active, owner) = state
                        .inner
                        .lock()
                        .ok()
                        .map(|panel| {
                            let owner = panel
                                .tabs
                                .iter()
                                .find(|tab| tab.id == popup_id)
                                .and_then(|tab| tab.owner.clone());
                            (panel.active.as_ref() == Some(&popup_id), owner)
                        })
                        .unwrap_or((false, None));
                    std::thread::spawn(move || {
                        let at_limit = state
                            .inner
                            .lock()
                            .map(|panel| {
                                panel.owned_count(owner.as_deref())
                                    >= super::ownership::TABS_PER_OWNER
                            })
                            .unwrap_or(true);
                        if at_limit {
                            if let Some(source) = app.get_webview(&label(&source_id)) {
                                if let Err(error) = source.navigate(url.clone()) { state.log(&app, json!({"level":"error", "message":error.to_string(), "url":url.as_str()})); }
                            }
                            return;
                        }
                        if let Err(error) = state.open(&app, url.as_str(), active, owner.as_deref()) {
                            state.log(
                                &app,
                                json!({"level":"error", "message":error, "url":url.as_str()}),
                            );
                        }
                    });
                }
                NewWindowResponse::Deny
            });
        #[cfg(not(target_os = "macos"))]
        {
            builder = builder.data_directory(
                app.path()
                    .app_data_dir()
                    .map_err(|error| error.to_string())?
                    .join("browser"),
            );
        }
        #[cfg(target_os = "macos")]
        {
            builder = builder.data_store_identifier(*b"prime-browser-v1");
        }
        let build = (|| {
            let window = app.get_window("main").ok_or("Основное окно недоступно")?;
            let view = window
                .add_child(
                    builder,
                    LogicalPosition::new(-OFFSCREEN_OFFSET, 0.0),
                    LogicalSize::new(initial_width, initial_height),
                )
                .map_err(|error| error.to_string())?;
            super::platform::install_events(&view, self, app, &id)?;
            view.hide().map_err(|error| error.to_string())?;
            {
                let mut applied = self.applied.lock().map_err(|error| error.to_string())?;
                applied.insert(
                    id.clone(),
                    TabPlacement {
                        visible: false,
                        x: -OFFSCREEN_OFFSET,
                        y: 0.0,
                        width: initial_width,
                        height: initial_height,
                    },
                );
            }
            self.attach(app)
        })();
        if let Err(error) = build {
            if let Some(view) = app.get_webview(&label(&id)) {
                let _ = view.close();
            }
            self.remove(&id)?;
            return Err(error);
        }
        self.publish(app);
        self.monitor(app);
        Ok(json!({"id":id,"tabId":id,"url":normalized,"title":"","loading":true}))
    }

    pub fn remove(&self, id: &str) -> Result<(), String> {
        {
            let mut panel = self.inner.lock().map_err(|error| error.to_string())?;
            let index = panel
                .tabs
                .iter()
                .position(|tab| tab.id == id)
                .ok_or("Вкладка не найдена")?;
            panel.tabs.remove(index);
            panel.forget(id, index);
        }
        super::evaluate::forget_health(id);
        if let Ok(mut applied) = self.applied.lock() {
            applied.remove(id);
        }
        Ok(())
    }

    pub fn navigate(
        &self,
        app: &AppHandle,
        value: &str,
        tab: Option<&str>,
    ) -> Result<Value, String> {
        let url = normalize_url(value)?;
        let (id, view) = self.target(app, tab)?;
        view.navigate(url).map_err(|error| error.to_string())?;
        if let Some(tab) = self
            .inner
            .lock()
            .map_err(|error| error.to_string())?
            .tabs
            .iter_mut()
            .find(|tab| tab.id == id)
        {
            tab.loading = true;
            tab.load_error = None;
        }
        self.state()
    }

    pub fn command(
        &self,
        app: &AppHandle,
        command: &str,
        id: Option<&str>,
    ) -> Result<Value, String> {
        let (id, view) = self.target(app, id)?;
        match command {
            "fit" => {
                self.set_viewport(&id, None)?;
                self.attach(app)?;
            }
            "back" | "forward" => {
                #[cfg(target_os = "windows")]
                {
                    let history =
                        super::platform::protocol(&view, "Page.getNavigationHistory", json!({}))?;
                    let current = history["currentIndex"]
                        .as_i64()
                        .ok_or("Invalid native browser history")?;
                    let next = current + if command == "back" { -1 } else { 1 };
                    if let Some(entry) = history["entries"].as_array().and_then(|entries| {
                        if next >= 0 {
                            entries.get(next as usize)
                        } else {
                            None
                        }
                    }) {
                        super::platform::protocol(
                            &view,
                            "Page.navigateToHistoryEntry",
                            json!({"entryId":entry["id"]}),
                        )?;
                    }
                }
                #[cfg(target_os = "linux")]
                {
                    super::platform_linux::navigate_history(&view, command)?;
                }
                #[cfg(not(any(target_os = "windows", target_os = "linux")))]
                {
                    let mut panel = self.inner.lock().map_err(|error| error.to_string())?;
                    let tab = panel
                        .tabs
                        .iter_mut()
                        .find(|tab| tab.id == id)
                        .ok_or("Вкладка не найдена")?;
                    let direction = if command == "back" { -1 } else { 1 };
                    let next = tab.index as isize + direction;
                    if next >= 0 && (next as usize) < tab.history.len() {
                        view.eval(format!("history.go({direction})"))
                            .map_err(|error| error.to_string())?;
                        tab.index = next as usize;
                        tab.traversing = true;
                    }
                }
            }
            "reload" => {
                let loading = self
                    .inner
                    .lock()
                    .map_err(|error| error.to_string())?
                    .tabs
                    .iter()
                    .find(|tab| tab.id == id)
                    .map(|tab| tab.loading)
                    .unwrap_or(false);
                if loading {
                    view.eval("window.stop()")
                        .map_err(|error| error.to_string())?;
                } else {
                    view.reload().map_err(|error| error.to_string())?;
                }
            }
            _ => return Err("Неизвестная команда браузера".to_owned()),
        }
        self.publish(app);
        self.state()
    }

    pub fn log(&self, app: &AppHandle, mut entry: Value) {
        entry["time"] = json!(std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_millis() as u64);
        if let Ok(mut panel) = self.inner.lock() {
            panel.logs.push(entry);
            let excess = panel.logs.len().saturating_sub(200);
            panel.logs.drain(..excess);
        }
        let _ = app.emit_to(
            tauri::EventTarget::webview("main"),
            "prime:event",
            json!({"type":"browser","payload":{"action":"log"}}),
        );
    }
}

pub fn label(id: &str) -> String {
    format!("browser-{id}")
}
pub fn allowed(url: &Url) -> bool {
    matches!(url.scheme(), "http" | "https" | "file" | "about")
}

pub fn normalize_url(value: &str) -> Result<Url, String> {
    let value = value.trim();
    if value.len() > 8192 {
        return Err("Адрес слишком длинный".to_owned());
    }
    if value.is_empty() {
        return Url::parse("about:blank").map_err(|error| error.to_string());
    }
    let bytes = value.as_bytes();
    if value.starts_with('/')
        || (bytes.len() > 2
            && bytes[0].is_ascii_alphabetic()
            && bytes[1] == b':'
            && matches!(bytes[2], b'/' | b'\\'))
    {
        return Url::from_file_path(std::path::Path::new(value))
            .map_err(|_| "Некорректный путь файла".to_owned());
    }
    let localhost = ["localhost", "127.0.0.1", "[::1]"].iter().any(|host| {
        value == *host
            || value
                .strip_prefix(host)
                .map(|suffix| suffix.starts_with(':') || suffix.starts_with('/'))
                .unwrap_or(false)
    });
    let url = if localhost {
        Url::parse(&format!("http://{value}"))
    } else if let Ok(url) = Url::parse(value) {
        if !allowed(&url) {
            return Err(format!("Протокол {} не поддерживается", url.scheme()));
        }
        return Ok(url);
    } else if value.contains('.') && !value.chars().any(char::is_whitespace) {
        Url::parse(&format!("https://{value}"))
    } else {
        let mut url = Url::parse("https://duckduckgo.com/").map_err(|error| error.to_string())?;
        url.query_pairs_mut().append_pair("q", value);
        return Ok(url);
    };
    url.map_err(|error| error.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rendering_holds_are_counted_per_tab() {
        let state = BrowserState::new();
        assert!(state.hold_rendering("agent").unwrap());
        assert!(!state.hold_rendering("agent").unwrap());
        assert!(!state.release_rendering("agent"));
        assert!(state.release_rendering("agent"));
        assert!(!state.release_rendering("agent"));
        assert!(state.inner.lock().unwrap().rendering.is_empty());
    }

    #[test]
    fn only_the_active_tab_of_a_visible_panel_is_shown() {
        let state = BrowserState::new();
        {
            let mut panel = state.inner.lock().unwrap();
            panel.visible = true;
            panel.active = Some("user".to_owned());
        }
        assert!(state.is_shown("user").unwrap());
        assert!(!state.is_shown("agent").unwrap());
    }

    #[test]
    fn state_emits_offscreen_viewport_only_when_visible_and_exceeding_bounds() {
        let state = BrowserState::new();
        {
            let mut panel = state.inner.lock().unwrap();
            panel.tabs.push(Tab {
                id: "tab1".to_owned(),
                url: "https://example.com".to_owned(),
                title: "Example".to_owned(),
                loading: false,
                owner: None,
                viewport: Some(1280.0),
                history: vec!["https://example.com".to_owned()],
                index: 0,
                traversing: false,
                load_error: None,
            });
            panel.active = Some("tab1".to_owned());
            panel.bounds = Bounds {
                x: 0.0,
                y: 0.0,
                width: 800.0,
                height: 600.0,
            };
            panel.visible = false;
        }
        let hidden_state = state.state().unwrap();
        assert_eq!(hidden_state.get("offscreenViewport"), None);

        state.inner.lock().unwrap().visible = true;
        let visible_state = state.state().unwrap();
        assert_eq!(visible_state["offscreenViewport"], json!(1280.0));

        state.inner.lock().unwrap().bounds.width = 1400.0;
        let wider_bounds_state = state.state().unwrap();
        assert_eq!(wider_bounds_state.get("offscreenViewport"), None);

        state.inner.lock().unwrap().bounds.width = 800.0;
        state.set_viewport("tab1", None).unwrap();
        let reset_viewport_state = state.state().unwrap();
        assert_eq!(reset_viewport_state.get("offscreenViewport"), None);
    }

    #[test]
    fn is_shown_returns_false_when_viewport_exceeds_bounds() {
        let state = BrowserState::new();
        {
            let mut panel = state.inner.lock().unwrap();
            panel.tabs.push(Tab {
                id: "tab1".to_owned(),
                url: "https://example.com".to_owned(),
                title: "Example".to_owned(),
                loading: false,
                owner: None,
                viewport: Some(1200.0),
                history: vec!["https://example.com".to_owned()],
                index: 0,
                traversing: false,
                load_error: None,
            });
            panel.active = Some("tab1".to_owned());
            panel.bounds = Bounds {
                x: 0.0,
                y: 0.0,
                width: 800.0,
                height: 600.0,
            };
            panel.visible = true;
        }
        assert!(!state.is_shown("tab1").unwrap());

        state.set_viewport("tab1", Some(700.0)).unwrap();
        assert!(state.is_shown("tab1").unwrap());

        state.set_viewport("tab1", None).unwrap();
        assert!(state.is_shown("tab1").unwrap());
    }

    #[test]
    fn reveal_updates_active_and_selected_when_inspector_closed() {
        let state = BrowserState::new();
        assert!(!state.inner.lock().unwrap().inspector_open);
        state.set_inspector_open(true).unwrap();
        assert!(state.inner.lock().unwrap().inspector_open);
        state.set_inspector_open(false).unwrap();
        assert!(!state.inner.lock().unwrap().inspector_open);

        {
            let mut panel = state.inner.lock().unwrap();
            panel.view = Some("chat-a".to_owned());
            panel.tabs = vec![
                Tab {
                    id: "tab-a".to_owned(),
                    url: "about:blank".to_owned(),
                    title: String::new(),
                    loading: false,
                    owner: Some("chat-a".to_owned()),
                    viewport: None,
                    history: vec!["about:blank".to_owned()],
                    index: 0,
                    traversing: false,
                    load_error: None,
                },
                Tab {
                    id: "tab-b".to_owned(),
                    url: "about:blank".to_owned(),
                    title: String::new(),
                    loading: false,
                    owner: Some("chat-b".to_owned()),
                    viewport: None,
                    history: vec!["about:blank".to_owned()],
                    index: 0,
                    traversing: false,
                    load_error: None,
                },
            ];
        }

        let (on_screen_a, open_a) = state.reveal_target(Some("chat-a"), Some("tab-a"));
        assert!(on_screen_a);
        assert!(!open_a);
        assert_eq!(state.inner.lock().unwrap().active.as_deref(), Some("tab-a"));

        let (on_screen_b, open_b) = state.reveal_target(Some("chat-b"), Some("tab-b"));
        assert!(!on_screen_b);
        assert!(!open_b);
        assert_eq!(
            state
                .inner
                .lock()
                .unwrap()
                .selected
                .get("chat-b")
                .map(String::as_str),
            Some("tab-b")
        );
        assert_eq!(state.inner.lock().unwrap().active.as_deref(), Some("tab-a"));
    }

    #[test]
    fn normalizes_browser_addresses_and_searches() {
        assert_eq!(normalize_url("  ").unwrap().as_str(), "about:blank");
        assert_eq!(
            normalize_url("example.com").unwrap().as_str(),
            "https://example.com/"
        );
        assert_eq!(
            normalize_url("localhost:3000/app").unwrap().as_str(),
            "http://localhost:3000/app"
        );
        assert_eq!(
            normalize_url("127.0.0.1:5173").unwrap().as_str(),
            "http://127.0.0.1:5173/"
        );
        assert_eq!(
            normalize_url("[::1]:8000").unwrap().as_str(),
            "http://[::1]:8000/"
        );
        assert_eq!(
            normalize_url("hello & world")
                .unwrap()
                .query_pairs()
                .next()
                .unwrap()
                .1,
            "hello & world"
        );
    }

    #[test]
    fn rejects_privileged_and_executable_schemes() {
        for address in [
            "javascript:alert(1)",
            "data:text/html,test",
            "tauri://localhost",
            "ipc://localhost",
            "ftp://example.com",
            "mailto:test@example.com",
        ] {
            assert!(normalize_url(address).is_err(), "{address}");
        }
        assert!(normalize_url(&"x".repeat(8193)).is_err());
    }

    #[test]
    fn tab_close_preserves_nearest_selection() {
        let state = BrowserState::new();
        {
            let mut panel = state.inner.lock().unwrap();
            for id in ["first", "second", "third"] {
                panel.tabs.push(Tab {
                    id: id.to_owned(),
                    url: "about:blank".to_owned(),
                    title: String::new(),
                    loading: false,
                    owner: None,
                    viewport: None,
                    history: vec!["about:blank".to_owned()],
                    index: 0,
                    traversing: false,
                    load_error: None,
                });
            }
            panel.active = Some("second".to_owned());
        }
        state.remove("second").unwrap();
        assert_eq!(state.state().unwrap()["activeTab"], "third");
        state.remove("third").unwrap();
        assert_eq!(state.state().unwrap()["activeTab"], "first");
        state.remove("first").unwrap();
        let empty = state.state().unwrap();
        assert_eq!(empty["tabs"], json!([]));
        assert_eq!(empty["canGoBack"], false);
        assert!(state.remove("first").is_err());
    }

    #[cfg(target_os = "windows")]
    #[test]
    fn normalizes_windows_file_paths() {
        assert_eq!(
            normalize_url(r"C:\page with spaces\index.html")
                .unwrap()
                .as_str(),
            "file:///C:/page%20with%20spaces/index.html"
        );
    }
}
