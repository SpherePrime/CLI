use serde_json::{json, Value};
use tauri::{AppHandle, Manager};

use super::find::Query;
use super::types::Locator;
use super::ui_action::UiAction;
use crate::browser::workbench::WorkbenchContext;

fn tool(name: &str, description: &str, properties: Value, required: &[&str]) -> Value {
    json!({"name":name,"description":description,"inputSchema":{"type":"object","properties":properties,"required":required,"additionalProperties":false}})
}

fn locator_properties() -> Value {
    json!({
        "reference": {"type":"string","minLength":1},
        "automationId": {"type":"string","minLength":1},
        "role": {"type":"string","minLength":1},
        "name": {"type":"string","minLength":1},
        "path": {"type":"string","minLength":1}
    })
}

fn locator_tool(name: &str, description: &str, extra: Value, required: &[&str]) -> Value {
    let mut properties = locator_properties();
    if let (Some(properties), Some(extra)) = (properties.as_object_mut(), extra.as_object()) {
        for (key, value) in extra {
            properties.insert(key.clone(), value.clone());
        }
    }
    tool(name, description, properties, required)
}

pub fn definitions() -> Vec<Value> {
    vec![
        tool(
            "app_launch",
            "Start the application being developed from the project and take control of its window. Use this to test native desktop, Electron or Tauri apps only; websites and web apps are tested in the built-in browser with browser_* tools. It runs the command in the project, waits for the new window, attaches to it and returns its interface tree. No vision is needed: every later action returns the interface changes as text. Process output is available through app_output. For Electron or Chromium apps add --remote-debugging-port=N to the command and pass debugPort=N to read the DOM.",
            json!({
                "command": {"type":"string","minLength":1,"maxLength":1000,"description":"Shell command, for example npm run dev or target\\debug\\app.exe"},
                "cwd": {"type":"string","maxLength":300,"description":"Directory inside the project"},
                "windowTitle": {"type":"string","maxLength":200,"description":"Part of the expected window title"},
                "waitSeconds": {"type":"integer","minimum":1,"maximum":300},
                "debugPort": {"type":"integer","minimum":1,"maximum":65535}
            }),
            &["command"],
        ),
        tool(
            "app_targets",
            "List visible application windows and the window attached to this chat.",
            json!({}),
            &[],
        ),
        tool(
            "app_attach",
            "Take control of an already running application window, matched by part of its title, process id or hwnd from app_targets. Returns its interface tree.",
            json!({
                "title": {"type":"string","minLength":1,"maxLength":200},
                "pid": {"type":"integer","minimum":1},
                "hwnd": {"type":"string","minLength":1},
                "debugPort": {"type":"integer","minimum":1,"maximum":65535}
            }),
            &[],
        ),
        tool(
            "app_close",
            "Stop the application started with app_launch and release its window.",
            json!({}),
            &[],
        ),
        tool(
            "app_output",
            "Read the latest console output of the application started with app_launch. Use it after crashes, hangs or unexpected behaviour.",
            json!({}),
            &[],
        ),
        tool(
            "app_snapshot",
            "Read the full interface tree of the attached application as text: element references, roles, names, values, states, bounds in screen pixels and actions. Prefer app_find for large interfaces.",
            json!({}),
            &[],
        ),
        tool(
            "app_find",
            "Find elements of the attached application by text (name, value or automationId, case-insensitive) and/or role. Returns matching elements with references, bounds and paths.",
            json!({"text":{"type":"string","minLength":1,"maxLength":200},"role":{"type":"string","minLength":1,"maxLength":60}}),
            &[],
        ),
        tool(
            "app_wait",
            "Wait until an element matching text and/or role appears, or disappears with gone=true. Use after actions that load data, open windows or run long operations.",
            json!({"text":{"type":"string","minLength":1,"maxLength":200},"role":{"type":"string","minLength":1,"maxLength":60},"gone":{"type":"boolean"},"timeoutMs":{"type":"integer","minimum":100,"maximum":120000}}),
            &[],
        ),
        tool(
            "app_audit",
            "Check the layout of the current screen without vision using element geometry: overlapping elements, controls closer than 4 px, click targets smaller than 24 px, elements clipped by the window edge, unnamed controls, possibly truncated labels and duplicate names. The result is added to the test report.",
            json!({}),
            &[],
        ),
        tool(
            "app_check",
            "Record the result of one verification in the test report shown to the user, for example that saving works or that an error message appears. Use a short stable name; recording the same name again replaces the result.",
            json!({"name":{"type":"string","minLength":1,"maxLength":120},"passed":{"type":"boolean"},"details":{"type":"string","maxLength":1000}}),
            &["name","passed"],
        ),
        locator_tool(
            "app_click",
            "Activate one element of the attached application. Identify it by reference from the latest tree, by automationId, by role and name, or by path. Returns the interface changes caused by the click.",
            json!({}),
            &[],
        ),
        locator_tool(
            "app_type",
            "Type text into one element of the attached application. Returns the interface changes.",
            json!({"text":{"type":"string","minLength":1,"maxLength":1000}}),
            &["text"],
        ),
        locator_tool(
            "app_set_value",
            "Set the value of one element through its value pattern. Returns the interface changes.",
            json!({"value":{"type":"string","maxLength":4000}}),
            &["value"],
        ),
        locator_tool(
            "app_toggle",
            "Toggle a checkbox, switch or radio element. Returns the interface changes.",
            json!({}),
            &[],
        ),
        locator_tool(
            "app_select",
            "Select a list item, tab or option element. Returns the interface changes.",
            json!({}),
            &[],
        ),
        locator_tool(
            "app_focus",
            "Move keyboard focus to one element.",
            json!({}),
            &[],
        ),
        locator_tool(
            "app_scroll",
            "Scroll a container or element. Returns the interface changes.",
            json!({"direction":{"type":"string","enum":["up","down","left","right"]},"amount":{"type":"string","enum":["small","large"]}}),
            &["direction","amount"],
        ),
        locator_tool(
            "app_expand",
            "Expand or collapse a menu, tree item or section. Returns the interface changes.",
            json!({"state":{"type":"string","enum":["expanded","collapsed"]}}),
            &["state"],
        ),
        tool(
            "app_press_key",
            "Send a key or key combination to the attached application, for example ENTER or CTRL+S. Desktop-wide shortcuts are refused. Returns the interface changes.",
            json!({"key":{"type":"string","minLength":1,"maxLength":60}}),
            &["key"],
        ),
        locator_tool(
            "app_screenshot",
            "Capture one element as an image, only when the model can see images. Use window_view for the whole window.",
            json!({}),
            &[],
        ),
        tool(
            "app_test_explore",
            "Walk the attached application and activate its actionable elements one by one, reporting what was visited, what was skipped as risky and where the walk stopped. Finds crashes and dead ends.",
            json!({"maxSteps":{"type":"integer","minimum":1,"maximum":100}}),
            &[],
        ),
        tool(
            "app_test_list",
            "List repeatable test scenarios stored in the project root as prime-app-test*.yaml or .json files.",
            json!({}),
            &[],
        ),
        tool(
            "app_test_run",
            "Run one repeatable test scenario from the project root and return the per-step result. Steps: attach, wait_for, click, type, set_value, toggle, select, focus, press_key, scroll, assert, screenshot, stop.",
            json!({"path":{"type":"string","minLength":1,"maxLength":200}}),
            &["path"],
        ),
    ]
}

pub fn run(
    app: &AppHandle,
    context: &WorkbenchContext,
    name: &str,
    args: &Value,
) -> Result<Value, String> {
    dispatch(app, context, name, args).map(super::content::mcp_content)
}

fn dispatch(
    app: &AppHandle,
    context: &WorkbenchContext,
    name: &str,
    args: &Value,
) -> Result<Value, String> {
    let manager = app.state::<crate::state::AppState>().appui.clone();
    let observed = |result: Result<Value, String>| -> Result<Value, String> {
        let mut value = result?;
        let changes = manager.observe(app, context);
        if let Some(object) = value.as_object_mut() {
            object.insert("changes".to_owned(), Value::String(changes));
        }
        Ok(value)
    };
    match name {
        "app_launch" => manager.launch(app, context, args),
        "app_targets" => manager.targets(&context.workspace_id),
        "app_attach" => manager.attach_matching(app, context, args),
        "app_close" => manager.close(app, context),
        "app_output" => manager.output(&context.workspace_id),
        "app_snapshot" => manager.snapshot(app, context),
        "app_find" => manager.find(app, context, query(args)),
        "app_wait" => manager.wait(
            app,
            context,
            query(args),
            args.get("gone").and_then(Value::as_bool).unwrap_or(false),
            args.get("timeoutMs").and_then(Value::as_u64),
        ),
        "app_audit" => manager.audit(app, context),
        "app_check" => manager.check(app, context, args),
        "app_click" => observed(manager.act(app, context, locator(args), UiAction::Invoke)),
        "app_type" => observed(manager.type_text(
            app,
            context,
            locator(args),
            argument(args, "text")?,
        )),
        "app_set_value" => observed(manager.act(
            app,
            context,
            locator(args),
            UiAction::SetValue(argument(args, "value")?),
        )),
        "app_toggle" => observed(manager.act(app, context, locator(args), UiAction::Toggle)),
        "app_select" => observed(manager.act(app, context, locator(args), UiAction::Select)),
        "app_focus" => manager.act(app, context, locator(args), UiAction::Focus),
        "app_scroll" => observed(manager.act(
            app,
            context,
            locator(args),
            UiAction::Scroll(argument(args, "direction")?, argument(args, "amount")?),
        )),
        "app_press_key" => observed(manager.press_key(app, context, argument(args, "key")?)),
        "app_expand" => {
            let action = if argument(args, "state")? == "collapsed" {
                UiAction::Collapse
            } else {
                UiAction::Expand
            };
            observed(manager.act(app, context, locator(args), action))
        }
        "app_screenshot" => manager.element_png(app, context, locator(args)),
        "app_test_list" => manager.test_list(app, &context.workspace_id),
        "app_test_run" => manager.test_run(app, context, args),
        "app_test_explore" => manager.explore(app, context, args),
        _ => Err(format!("Unknown project tool: {name}")),
    }
}

fn query(args: &Value) -> Query {
    Query {
        text: text(args, "text"),
        role: text(args, "role"),
    }
}

fn locator(args: &Value) -> Locator {
    Locator {
        reference: text(args, "reference"),
        automation_id: text(args, "automationId"),
        role: text(args, "role"),
        name: text(args, "name"),
        path: text(args, "path"),
    }
}

fn text(args: &Value, name: &str) -> Option<String> {
    args.get(name)
        .and_then(Value::as_str)
        .filter(|value| !value.is_empty())
        .map(str::to_owned)
}

fn argument(args: &Value, name: &str) -> Result<String, String> {
    text(args, name).ok_or_else(|| format!("Missing argument: {name}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn app_tools_are_unique_and_never_accept_workspace_overrides() {
        let mut names = std::collections::HashSet::new();
        for definition in definitions() {
            let name = definition["name"].as_str().unwrap();
            assert!(name.starts_with("app_"), "{name}");
            assert!(names.insert(name.to_owned()), "Duplicate tool: {name}");
            assert!(definition["inputSchema"]["properties"]
                .get("workspaceId")
                .is_none());
            assert!(definition["inputSchema"]["properties"].get("root").is_none());
            assert_eq!(definition["inputSchema"]["additionalProperties"], false);
        }
    }

    #[test]
    fn locator_tools_require_their_extra_arguments() {
        let click = definitions()
            .into_iter()
            .find(|tool| tool["name"] == "app_click")
            .unwrap();
        let required = click["inputSchema"]["required"].as_array().unwrap();
        assert!(required.is_empty());
        let scroll = definitions()
            .into_iter()
            .find(|tool| tool["name"] == "app_scroll")
            .unwrap();
        let required = scroll["inputSchema"]["required"].as_array().unwrap();
        assert_eq!(required.len(), 2);
    }

    #[test]
    fn locators_ignore_empty_values() {
        let locator = locator(&json!({"reference": "n3", "name": "", "role": "button"}));
        assert_eq!(locator.reference.as_deref(), Some("n3"));
        assert!(locator.name.is_none());
        assert_eq!(locator.role.as_deref(), Some("button"));
    }
}
