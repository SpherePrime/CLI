use super::{
    evaluate::{self, text},
    panel::BrowserState,
    platform, rendering, scenario, screenshot,
};
use serde_json::{json, Value};
use std::{
    sync::atomic::AtomicBool,
    thread,
    time::{Duration, Instant},
};
use tauri::{AppHandle, Webview};

fn schema(name: &str, description: &str, mut properties: Value, required: &[&str]) -> Value {
    properties["tabId"] = json!({"type":"string", "description":"Stable tab ID from browser_tabs. Defaults to this chat's own tab, created in the background when missing."});
    json!({"name":name,"description":description,"inputSchema":{"type":"object","properties":properties,"required":required}})
}

pub fn definitions() -> Vec<Value> {
    vec![
        schema("browser_tabs", "List built-in browser tabs of this chat with stable IDs and the chat's default tab.", json!({}), &[]),
        schema("browser_new_tab", "Open a separate built-in browser tab in the background by default.", json!({"url":{"type":"string"},"activate":{"type":"boolean"}}), &[]),
        schema("browser_select_tab", "Make a tab the default for this chat and show it if the user is viewing this chat.", json!({}), &["tabId"]),
        schema("browser_close_tab", "Close a built-in browser tab.", json!({}), &["tabId"]),
        schema("browser_navigate", "Open a URL, local file or search in the built-in browser. Prefer this browser for page automation.", json!({"url":{"type":"string"},"width":{"type":"number","minimum":320,"maximum":2560}}), &["url"]),
        schema("browser_resize", "Change browser panel width in pixels for desktop, tablet or phone layouts. To check several widths, use resize steps in browser_run.", json!({"width":{"type":"number","minimum":320,"maximum":2560}}), &["width"]),
        schema("browser_snapshot", "Read page URL, title, visible text and numbered interactive elements.", json!({}), &[]),
        schema("browser_click", "Click an element by ref from browser_snapshot. To verify a flow, put the clicks in one browser_run scenario.", json!({"ref":{"type":"integer","minimum":1}}), &["ref"]),
        schema("browser_type", "Replace text in an input or editable element by ref.", json!({"ref":{"type":"integer","minimum":1},"text":{"type":"string"},"submit":{"type":"boolean"}}), &["ref","text"]),
        schema("browser_press_key", "Press a keyboard key in the target browser tab.", json!({"key":{"type":"string"}}), &["key"]),
        schema("browser_scroll", "Scroll the target browser page up or down.", json!({"direction":{"type":"string","enum":["up","down"]}}), &["direction"]),
        schema("browser_back", "Go back in the target browser tab history.", json!({}), &[]),
        schema("browser_screenshot", "Capture the page as PNG. Default: the visible viewport. fullPage: the whole scrollable page (lazy content is loaded first); add tiles to split a long page into sections the model can read. ref: one element from browser_snapshot in full, even beyond the viewport; padding adds CSS px around it to show its neighbours. clip: any page region {x,y,width,height} in CSS px (element boxes come from browser_snapshot). zoom 1-3 renders sharper for small text, icons, borders and focus rings. A full page reaches the model heavily downscaled: judge composition from it and details from close-ups.", json!({"fullPage":{"type":"boolean"},"tiles":{"type":"boolean"},"ref":{"type":"integer","minimum":1},"padding":{"type":"number","minimum":0,"maximum":400},"clip":{"type":"object","properties":{"x":{"type":"number"},"y":{"type":"number"},"width":{"type":"number"},"height":{"type":"number"}},"required":["x","y","width","height"]},"zoom":{"type":"number","minimum":1,"maximum":3}}), &[]),
        schema("browser_run", "Default tool for verifying or testing a page: the whole check in one call. Steps run in order and return a pass/fail log and every screenshot at once. Use it instead of separate resize/click/type/scroll/screenshot calls. Actions: navigate{url}, resize{width}, click{selector|text}, hover{selector|text}, focus{selector|text}, type{selector|text,value,submit?}, select{selector|text,option}, press_key{key}, scroll{to:top|bottom | selector|text}, wait_for{selector|text,gone?,timeoutMs?}, wait{ms}, assert{selector|text,contains?,absent?}, evaluate{expression}, screenshot{name?,fullPage?,tiles?,selector|text,padding?,clip?,zoom?}. Any target can also be ref from browser_snapshot. text matches visible text, labels and placeholders. click and hover move the real mouse, so :hover styles and pointer handlers work; a covered target is reported in the log. A failed action stops the run unless continueOnError; failed asserts are recorded and the run continues. Up to 80 steps and 16 screenshot steps.", json!({"steps":{"type":"array","items":super::scenario_parse::step_schema()},"continueOnError":{"type":"boolean"}}), &["steps"]),
        schema("browser_evaluate", "Evaluate an asynchronous JavaScript expression and return its JSON result. Runs until the expression settles; a long run continues in the background and can be checked with mcp_job.", json!({"expression":{"type":"string"}}), &["expression"]),
        schema("browser_health", "Check whether a browser tab responds and whether a script evaluation is still running in it.", json!({}), &[]),
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
    Ok(text(format!("Tab ID: {id}\nURL: {}\nTitle: {}\n\nInteractive elements (use ref with browser_click / browser_type; @x,y WxH is the page box in CSS px for screenshot clip):\n{}\n\nPage text:\n{}", page["url"].as_str().unwrap_or(""), page["title"].as_str().unwrap_or(""), if elements.is_empty() { "(none)" } else { &elements }, page["text"].as_str().unwrap_or("(empty)"))))
}

pub fn run(
    app: &AppHandle,
    state: &BrowserState,
    session: Option<&str>,
    name: &str,
    args: &Value,
    cancel: &AtomicBool,
) -> Result<Value, String> {
    if name == "browser_tabs" {
        super::refresh(app, state)?;
        return Ok(text(state.session_state(session)?.to_string()));
    }
    if name == "browser_new_tab" {
        let activate = args["activate"].as_bool().unwrap_or(false);
        let result = state.open(
            app,
            args["url"].as_str().unwrap_or("about:blank"),
            false,
            session,
        )?;
        let (id, view) = state.target(app, result["id"].as_str())?;
        settle(&view, state, &id)?;
        super::refresh(app, state)?;
        if activate {
            if let Some(session) = session {
                state.bind_agent(session, &id)?;
            }
            state.reveal(app, session, Some(&id), true, None);
        }
        let current = state.session_state(session)?;
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
    let (id, view) = state.agent_target(app, session, requested)?;
    let _rendering = if name == "browser_close_tab" {
        None
    } else {
        Some(rendering::keep_rendering(app, state, &id)?)
    };
    match name {
        "browser_select_tab" => {
            if let Some(session) = session {
                state.bind_agent(session, &id)?;
            }
            state.reveal(app, session, Some(&id), true, None);
            return Ok(text(state.session_state(session)?.to_string()));
        }
        "browser_close_tab" => {
            view.close().map_err(|error| error.to_string())?;
            state.remove(&id)?;
            state.attach(app)?;
            state.publish(app);
            return Ok(text(state.session_state(session)?.to_string()));
        }
        "browser_navigate" => {
            let requested_width = args.get("width").map(|_| width(args)).transpose()?;
            state.set_viewport(&id, requested_width)?;
            state.navigate(app, string(args, "url")?, Some(&id))?;
            state.reveal(app, session, Some(&id), true, requested_width);
            state.attach(app)?;
            state.publish(app);
            settle(&view, state, &id)?;
        }
        "browser_snapshot" => {}
        "browser_resize" => {
            let requested = width(args)?;
            state.set_viewport(&id, Some(requested))?;
            state.reveal(app, session, Some(&id), true, Some(requested));
            state.attach(app)?;
            state.publish(app);
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
        "browser_run" => {
            let plan = scenario::parse(args)?;
            let settle_tab = || settle(&view, state, &id);
            let resize_tab = |requested: f64| -> Result<(), String> {
                if !(320.0..=2560.0).contains(&requested) {
                    return Err("width must be between 320 and 2560 pixels".to_owned());
                }
                state.set_viewport(&id, Some(requested))?;
                state.reveal(app, session, Some(&id), true, Some(requested));
                state.attach(app)?;
                state.publish(app);
                thread::sleep(Duration::from_millis(350));
                Ok(())
            };
            let browser = scenario::Browser {
                app,
                state,
                session,
                id: &id,
                view: &view,
                cancel,
                settle: &settle_tab,
                resize: &resize_tab,
            };
            let result = scenario::run(&browser, plan);
            super::refresh(app, state)?;
            return result;
        }
        "browser_screenshot" => {
            let shot = screenshot::target(args, selector)?;
            return screenshot::capture(&view, &id, &shot, cancel).map(Value::Array);
        }
        "browser_health" => return Ok(text(evaluate::page_health(&view, &id).to_string())),
        "browser_evaluate" => {
            let value =
                evaluate::evaluate_until_done(&view, &id, string(args, "expression")?, cancel)?;
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
            "browser_run",
            "browser_evaluate",
            "browser_health",
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
