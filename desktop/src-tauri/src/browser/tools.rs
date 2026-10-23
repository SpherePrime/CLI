use super::{
    evaluate::{self, text},
    panel::BrowserState,
    platform,
};
use serde_json::{json, Value};
use std::{
    thread,
    time::{Duration, Instant},
};
use tauri::{AppHandle, Webview};

fn schema(name: &str, description: &str, mut properties: Value, required: &[&str]) -> Value {
    properties["tabId"] = json!({"type":"string", "description":"Stable tab ID from browser_tabs. Defaults to the selected tab."});
    json!({"name":name,"description":description,"inputSchema":{"type":"object","properties":properties,"required":required}})
}

pub fn definitions() -> Vec<Value> {
    vec![
        schema("browser_tabs", "List built-in browser tabs with stable IDs and selected tab.", json!({}), &[]),
        schema("browser_new_tab", "Open a separate built-in browser tab in the background by default.", json!({"url":{"type":"string"},"activate":{"type":"boolean"}}), &[]),
        schema("browser_select_tab", "Select a built-in browser tab for the user.", json!({}), &["tabId"]),
        schema("browser_close_tab", "Close a built-in browser tab.", json!({}), &["tabId"]),
        schema("browser_navigate", "Open a URL, local file or search in the built-in browser. Prefer this browser for page automation.", json!({"url":{"type":"string"},"width":{"type":"number","minimum":320,"maximum":2560}}), &["url"]),
        schema("browser_resize", "Change browser panel width in pixels for desktop, tablet or phone layouts.", json!({"width":{"type":"number","minimum":320,"maximum":2560}}), &["width"]),
        schema("browser_snapshot", "Read page URL, title, visible text and numbered interactive elements.", json!({}), &[]),
        schema("browser_click", "Click an element by ref from browser_snapshot.", json!({"ref":{"type":"integer","minimum":1}}), &["ref"]),
        schema("browser_type", "Replace text in an input or editable element by ref.", json!({"ref":{"type":"integer","minimum":1},"text":{"type":"string"},"submit":{"type":"boolean"}}), &["ref","text"]),
        schema("browser_press_key", "Press a keyboard key in the target browser tab.", json!({"key":{"type":"string"}}), &["key"]),
        schema("browser_scroll", "Scroll the target browser page up or down.", json!({"direction":{"type":"string","enum":["up","down"]}}), &["direction"]),
        schema("browser_back", "Go back in the target browser tab history.", json!({}), &[]),
        schema("browser_screenshot", "Capture the visible browser page as PNG.", json!({}), &[]),
        schema("browser_evaluate", "Evaluate an asynchronous JavaScript expression and return its JSON result.", json!({"expression":{"type":"string"}}), &["expression"]),
    ]
}

fn string<'a>(args: &'a Value, name: &str) -> Result<&'a str, String> {
    args[name]
        .as_str()
        .ok_or_else(|| format!("{name} must be a string"))
}
fn selector(args: &Value) -> Result<String, String> {
    let reference = args["ref"]
        .as_u64()
        .filter(|value| *value > 0)
        .ok_or("ref must be a positive integer from browser_snapshot")?;
    Ok(format!("[data-prime-ref=\"{reference}\"]"))
}
fn width(args: &Value) -> Result<f64, String> {
    args["width"]
        .as_f64()
        .filter(|value| value.is_finite() && *value >= 320.0 && *value <= 2560.0)
        .ok_or_else(|| "width must be between 320 and 2560 pixels".to_owned())
}

fn settle(view: &Webview, state: &BrowserState, id: &str) -> Result<(), String> {
    thread::sleep(Duration::from_millis(350));
    let deadline = Instant::now() + Duration::from_secs(30);
    loop {
        let loading = {
            let panel = state.inner.lock().map_err(|error| error.to_string())?;
            let tab = panel
                .tabs
                .iter()
                .find(|tab| tab.id == id)
                .ok_or("Browser tab was closed")?;
            if let Some(error) = &tab.load_error {
                return Err(error.clone());
            }
            tab.loading
        };
        match evaluate::immediate(view, "document.readyState !== 'loading'") {
            Ok(Value::Bool(true)) if !loading => return Ok(()),
            _ if Instant::now() < deadline => thread::sleep(Duration::from_millis(100)),
            _ => return Err("Browser page did not finish navigation within 30 seconds".to_owned()),
        }
    }
}

fn snapshot(id: &str, view: &Webview) -> Result<Value, String> {
    let page = evaluate::immediate(view, include_str!("snapshot.js"))?;
    let elements = page["elements"]
        .as_array()
        .map(|items| {
            items
                .iter()
                .filter_map(Value::as_str)
                .collect::<Vec<_>>()
                .join("\n")
        })
        .unwrap_or_default();
    Ok(text(format!("Tab ID: {id}\nURL: {}\nTitle: {}\n\nInteractive elements (use ref with browser_click / browser_type):\n{}\n\nPage text:\n{}", page["url"].as_str().unwrap_or(""), page["title"].as_str().unwrap_or(""), if elements.is_empty() { "(none)" } else { &elements }, page["text"].as_str().unwrap_or("(empty)"))))
}

pub fn run(
    app: &AppHandle,
    state: &BrowserState,
    name: &str,
    args: &Value,
) -> Result<Value, String> {
    if name == "browser_tabs" {
        super::refresh(app, state)?;
        let current = state.state()?;
        return Ok(text(
            json!({"tabs":current["tabs"],"activeTab":current["activeTab"]}).to_string(),
        ));
    }
    if name == "browser_new_tab" {
        let activate = args["activate"].as_bool().unwrap_or(false);
        let result = state.open(app, args["url"].as_str().unwrap_or("about:blank"), activate)?;
        let (id, view) = state.target(app, result["id"].as_str())?;
        settle(&view, state, &id)?;
        super::refresh(app, state)?;
        if activate {
            state.reveal(app, true, None);
        }
        let current = state.state()?;
        let mut tab = current["tabs"]
            .as_array()
            .and_then(|tabs| tabs.iter().find(|tab| tab["id"] == result["id"]))
            .cloned()
            .ok_or("New tab was closed")?;
        tab["tabId"] = result["id"].clone();
        return Ok(text(tab.to_string()));
    }
    let requested = args.get("tabId").and_then(Value::as_str);
    if matches!(name, "browser_select_tab" | "browser_close_tab") && requested.is_none() {
        return Err("tabId is required".to_owned());
    }
    let (id, view) = state.target(app, requested)?;
    match name {
        "browser_select_tab" | "browser_close_tab" => {
            let action = if name == "browser_select_tab" {
                "select"
            } else {
                "close"
            };
            let result = super::dispatch(app, state, "browserTab", &[json!(action), json!(id)])?;
            if action == "select" {
                state.reveal(app, true, None);
            }
            return Ok(text(result.to_string()));
        }
        "browser_navigate" => {
            state.navigate(
                app,
                string(args, "url")?,
                Some(&id),
                true,
                args.get("width").map(|_| width(args)).transpose()?,
            )?;
            settle(&view, state, &id)?;
        }
        "browser_snapshot" => {}
        "browser_resize" => {
            let requested = width(args)?;
            state.reveal(app, true, Some(requested));
            thread::sleep(Duration::from_millis(350));
            let actual = evaluate::immediate(&view, "window.innerWidth")?;
            return Ok(text(format!(
                "Requested {requested}px. Page viewport is now {actual}px wide."
            )));
        }
        "browser_click" => {
            let selector =
                serde_json::to_string(&selector(args)?).map_err(|error| error.to_string())?;
            let found = evaluate::immediate(&view, &format!("(() => {{ const element = document.querySelector({selector}); if (!element) return false; element.scrollIntoView({{block:'center'}}); element.click(); return true }})()"))?;
            if found != true {
                return Err("Element not found. Call browser_snapshot again.".to_owned());
            }
            settle(&view, state, &id)?;
        }
        "browser_type" => {
            let selector =
                serde_json::to_string(&selector(args)?).map_err(|error| error.to_string())?;
            let found = evaluate::immediate(&view, &format!("(() => {{ const element = document.querySelector({selector}); if (!element) return false; element.scrollIntoView({{block:'center'}}); element.focus(); if (element.select) element.select(); else if (element.isContentEditable) {{ const selection = getSelection(); const range = document.createRange(); range.selectNodeContents(element); selection.removeAllRanges(); selection.addRange(range); }} return true }})()"))?;
            if found != true {
                return Err("Element not found. Call browser_snapshot again.".to_owned());
            }
            platform::insert_text(&view, string(args, "text")?)?;
            if args["submit"] == true {
                platform::press_key(&view, "Enter")?;
            }
            settle(&view, state, &id)?;
        }
        "browser_press_key" => {
            platform::press_key(&view, string(args, "key")?)?;
            settle(&view, state, &id)?;
        }
        "browser_scroll" => {
            let direction = match string(args, "direction")? {
                "up" => -1,
                "down" => 1,
                _ => return Err("direction must be up or down".to_owned()),
            };
            evaluate::immediate(
                &view,
                &format!("window.scrollBy({{top:{direction} * innerHeight * 0.8}})"),
            )?;
            thread::sleep(Duration::from_millis(350));
        }
        "browser_back" => {
            state.command(app, "back", Some(&id))?;
            settle(&view, state, &id)?;
        }
        "browser_screenshot" => return Ok(json!([platform::screenshot(&view, None)?])),
        "browser_evaluate" => {
            let value =
                evaluate::evaluate(&view, string(args, "expression")?, Duration::from_secs(30))?;
            return Ok(text(
                value.to_string().chars().take(20000).collect::<String>(),
            ));
        }
        _ => return Err(format!("Unknown browser tool: {name}")),
    }
    super::refresh(app, state)?;
    snapshot(&id, &view)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn browser_mcp_preserves_existing_tool_names() {
        let definitions = definitions();
        let names: std::collections::BTreeSet<&str> = definitions
            .iter()
            .map(|tool| tool["name"].as_str().unwrap())
            .collect();
        let expected = [
            "browser_tabs",
            "browser_new_tab",
            "browser_select_tab",
            "browser_close_tab",
            "browser_navigate",
            "browser_resize",
            "browser_snapshot",
            "browser_click",
            "browser_type",
            "browser_press_key",
            "browser_scroll",
            "browser_back",
            "browser_screenshot",
            "browser_evaluate",
        ]
        .into_iter()
        .collect();
        assert_eq!(names, expected);
        assert_eq!(definitions.len(), names.len());
        for tool in definitions {
            assert_eq!(tool["inputSchema"]["type"], "object");
            assert_eq!(tool["inputSchema"]["properties"]["tabId"]["type"], "string");
        }
    }
}
