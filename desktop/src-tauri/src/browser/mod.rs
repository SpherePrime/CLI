mod evaluate;
pub mod mcp;
mod panel;
mod platform;
#[cfg(target_os = "linux")]
mod platform_linux;
pub mod tools;
pub mod workbench;
mod workbench_launch;
mod workbench_git;
mod workbench_automation;

use base64::Engine;
pub use panel::BrowserState;
use serde_json::{json, Value};
use std::time::Duration;
use tauri::{AppHandle, Manager};

fn argument(args: &[Value], index: usize) -> Result<&str, String> {
    args.get(index)
        .and_then(Value::as_str)
        .ok_or_else(|| format!("Argument {index} must be a string"))
}

pub fn dispatch(
    app: &AppHandle,
    state: &BrowserState,
    method: &str,
    args: &[Value],
) -> Result<Value, String> {
    match method {
        "browserState" => {
            refresh(app, state)?;
            state.state()
        }
        "browserShow" => {
            state.target(app, None)?;
            let bounds: panel::Bounds =
                serde_json::from_value(args.first().cloned().ok_or("Browser bounds are required")?)
                    .map_err(|error| error.to_string())?;
            if ![bounds.x, bounds.y, bounds.width, bounds.height]
                .iter()
                .all(|value| value.is_finite())
                || bounds.width < 0.0
                || bounds.height < 0.0
            {
                return Err("Invalid browser bounds".to_owned());
            }
            {
                let mut panel = state.inner.lock().map_err(|error| error.to_string())?;
                panel.bounds = bounds;
                panel.visible = true;
            }
            state.attach(app)?;
            Ok(Value::Null)
        }
        "browserHide" => {
            state
                .inner
                .lock()
                .map_err(|error| error.to_string())?
                .visible = false;
            state.attach(app)?;
            Ok(Value::Null)
        }
        "browserTab" => {
            match argument(args, 0)? {
                "new" => {
                    state.open(
                        app,
                        args.get(2).and_then(Value::as_str).unwrap_or("about:blank"),
                        true,
                    )?;
                }
                "select" => {
                    let (id, _) = state.target(app, Some(argument(args, 1)?))?;
                    state
                        .inner
                        .lock()
                        .map_err(|error| error.to_string())?
                        .active = Some(id);
                    state.attach(app)?;
                }
                "close" => {
                    let (id, view) = state.target(app, Some(argument(args, 1)?))?;
                    view.close().map_err(|error| error.to_string())?;
                    state.remove(&id)?;
                    state.attach(app)?;
                }
                _ => return Err("Unknown browser tab action".to_owned()),
            }
            state.publish(app);
            state.state()
        }
        "browserNavigate" => state.navigate(app, argument(args, 0)?, None, false, None),
        "browserCommand" => state.command(app, argument(args, 0)?, None),
        "browserOpenExternal" => {
            let (_, view) = state.target(app, None)?;
            let url = view.url().map_err(|error| error.to_string())?;
            if !matches!(url.scheme(), "http" | "https") {
                return Err("Only HTTP and HTTPS browser pages can be opened externally".to_owned());
            }
            open::that(url.as_str()).map_err(|error| error.to_string())?;
            Ok(Value::Null)
        }
        "browserLogs" => {
            refresh(app, state)?;
            let mut panel = state.inner.lock().map_err(|error| error.to_string())?;
            if args.first().and_then(Value::as_bool) == Some(true) {
                panel.logs.clear();
            }
            Ok(json!(panel.logs))
        }
        "browserPick" => {
            let (_, view) = state.target(app, None)?;
            view.set_focus().map_err(|error| error.to_string())?;
            let elements =
                evaluate::evaluate(&view, include_str!("picker.js"), Duration::from_secs(300))?;
            let Some(elements) = elements.as_array() else {
                return Ok(Value::Null);
            };
            let mut results = Vec::new();
            for element in elements {
                let mut result = json!({"element":element});
                let rect = &element["rect"];
                let clip = json!({"x":rect["x"].as_f64().unwrap_or(0.0).max(0.0), "y":rect["y"].as_f64().unwrap_or(0.0).max(0.0), "width":rect["width"].as_f64().unwrap_or(1.0).max(1.0), "height":rect["height"].as_f64().unwrap_or(1.0).max(1.0), "scale":1});
                if let Ok(image) = platform::screenshot(&view, Some(clip)) {
                    let encoded = image["data"].as_str().ok_or("Invalid browser image")?;
                    let bytes = base64::engine::general_purpose::STANDARD
                        .decode(encoded)
                        .map_err(|error| error.to_string())?;
                    let app_state = app.state::<crate::state::AppState>();
                    let stored = app_state
                        .attachments
                        .lock()
                        .map_err(|error| error.to_string())?
                        .image("element.png", &bytes)
                        .map_err(|error| error.message)?;
                    result["attachmentId"] = stored[0]["id"].clone();
                    result["image"] = json!(format!("data:image/png;base64,{}", encoded));
                }
                results.push(result);
            }
            Ok(json!(results))
        }
        "browserPickCancel" => {
            let (_, view) = state.target(app, None)?;
            view.eval("window.__primePicker?.cancel()")
                .map_err(|error| error.to_string())?;
            Ok(Value::Null)
        }
        _ => Err(format!("Unknown browser method: {method}")),
    }
}

pub fn refresh(app: &AppHandle, state: &BrowserState) -> Result<(), String> {
    let tabs = state
        .inner
        .lock()
        .map_err(|error| error.to_string())?
        .tabs
        .clone();
    let mut changed = false;
    for tab in tabs {
        let Some(view) = app.get_webview(&panel::label(&tab.id)) else {
            continue;
        };
        if let Ok(history) = platform::history(&view) {
            if let (Some(index), Some(entries)) = (
                history["currentIndex"].as_u64(),
                history["entries"].as_array(),
            ) {
                if let Ok(mut panel) = state.inner.lock() {
                    if let Some(current) =
                        panel.tabs.iter_mut().find(|current| current.id == tab.id)
                    {
                        let history: Vec<String> = entries
                            .iter()
                            .filter_map(|entry| entry["url"].as_str().map(str::to_owned))
                            .collect();
                        changed |= current.index != index as usize || current.history != history;
                        current.index = index as usize;
                        current.history = history;
                    }
                }
            }
        }
        if let Ok(value) = evaluate::immediate(&view, "window.__primeBrowserRead?.() ?? ({url:location.href,title:document.title,loading:document.readyState!=='complete',logs:[]})") {
            if let Ok(mut panel) = state.inner.lock() {
                if let Some(current) = panel.tabs.iter_mut().find(|current| current.id == tab.id) {
                    let url = value["url"].as_str().unwrap_or(&current.url).to_owned();
                    let title = value["title"].as_str().unwrap_or(&current.title).to_owned();
                    let loading = value["loading"].as_bool().unwrap_or(current.loading);
                    changed |= current.url != url || current.title != title || current.loading != loading;
                    current.url = url; current.title = title; current.loading = loading;
                }
            }
            if let Some(logs) = value["logs"].as_array() { for entry in logs { state.log(app, entry.clone()); } }
        }
    }
    if changed {
        state.publish(app);
    }
    Ok(())
}
