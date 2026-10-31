use serde_json::{json, Value};
use tauri::{AppHandle, Manager};

use super::types::Locator;
use super::uia::action::UiAction;
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
            "app_targets",
            "List visible application windows that can be attached for interface testing, and the window currently attached to this chat. Attaching requires the window fields returned here.",
            json!({}),
            &[],
        ),
        tool(
            "app_attach",
            "Attach this chat to one application window and grant view or control through the normal chat permission policy. Copy the window fields exactly from app_targets. Control allows clicks, typing and scenarios; view allows reading the interface only.",
            json!({"hwnd":{"type":"string","minLength":1},"pid":{"type":"integer","minimum":1},"identity":{"type":"string","minLength":1},"title":{"type":"string","minLength":1},"mode":{"type":"string","enum":["view","control"]}}),
            &["hwnd","pid","identity","title","mode"],
        ),
        tool(
            "app_detach",
            "Detach the application window from this chat. The window grant stays until it is revoked.",
            json!({}),
            &[],
        ),
        tool(
            "app_snapshot",
            "Read the interface tree of the attached application as text with element references, roles, names, values, states, bounds and available actions. Works without screenshots and without a vision model. Take a new snapshot after the interface changes.",
            json!({}),
            &[],
        ),
        locator_tool(
            "app_click",
            "Activate one element of the attached application. Identify it by reference from the latest app_snapshot, or by automationId, role and name, or by path.",
            json!({}),
            &[],
        ),
        locator_tool(
            "app_type",
            "Type text into one element of the attached application. Uses the element value pattern when available and falls back to focus plus keyboard input, which requires the window to be in the foreground.",
            json!({"text":{"type":"string","minLength":1,"maxLength":1000}}),
            &["text"],
        ),
        locator_tool(
            "app_set_value",
            "Set the value of one element of the attached application through its value pattern.",
            json!({"value":{"type":"string","maxLength":4000}}),
            &["value"],
        ),
        locator_tool(
            "app_toggle",
            "Change the state of a checkbox, switch or radio element of the attached application.",
            json!({}),
            &[],
        ),
        locator_tool(
            "app_select",
            "Select a list item, tab or option element of the attached application.",
            json!({}),
            &[],
        ),
        locator_tool(
            "app_focus",
            "Move keyboard focus to one element of the attached application.",
            json!({}),
            &[],
        ),
        locator_tool(
            "app_scroll",
            "Scroll the container or element of the attached application.",
            json!({"direction":{"type":"string","enum":["up","down","left","right"]},"amount":{"type":"string","enum":["small","large"]}}),
            &["direction","amount"],
        ),
        locator_tool(
            "app_expand",
            "Expand or collapse a menu, tree item or section of the attached application.",
            json!({"state":{"type":"string","enum":["expanded","collapsed"]}}),
            &["state"],
        ),
        tool(
            "app_press_key",
            "Send a key or key combination to the attached application. The window must be in the foreground. Desktop-wide shortcuts are refused.",
            json!({"key":{"type":"string","minLength":1,"maxLength":60}}),
            &["key"],
        ),
        locator_tool(
            "app_screenshot",
            "Capture one element of the attached application as an image attachment. Use window_view for the whole window.",
            json!({}),
            &[],
        ),
        tool(
            "app_test_list",
            "List the application test scenarios stored in the project root as prime-app-test*.json files.",
            json!({}),
            &[],
        ),
        tool(
            "app_test_run",
            "Run one application test scenario from the project root and return the per-step result with the artifacts directory. Steps: attach, wait_for, click, type, set_value, toggle, select, focus, press_key, scroll, assert, screenshot, stop.",
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
    let manager = app.state::<crate::state::AppState>().appui.clone();
    match name {
        "app_targets" => manager.targets(&context.workspace_id),
        "app_attach" => manager.attach(app, context, args),
        "app_detach" => manager.detach(app, context),
        "app_snapshot" => manager.snapshot(app, context),
        "app_click" => manager.act(app, context, locator(args), UiAction::Invoke),
        "app_type" => manager.type_text(app, context, locator(args), argument(args, "text")?),
        "app_set_value" => {
            manager.act(app, context, locator(args), UiAction::SetValue(argument(args, "value")?))
        }
        "app_toggle" => manager.act(app, context, locator(args), UiAction::Toggle),
        "app_select" => manager.act(app, context, locator(args), UiAction::Select),
        "app_focus" => manager.act(app, context, locator(args), UiAction::Focus),
        "app_scroll" => manager.act(
            app,
            context,
            locator(args),
            UiAction::Scroll(argument(args, "direction")?, argument(args, "amount")?),
        ),
        "app_press_key" => manager.press_key(app, context, argument(args, "key")?),
        "app_expand" => {
            let action = if argument(args, "state")? == "collapsed" {
                UiAction::Collapse
            } else {
                UiAction::Expand
            };
            manager.act(app, context, locator(args), action)
        }        "app_screenshot" => manager.element_png(app, context, locator(args)),
        "app_test_list" => manager.test_list(app, &context.workspace_id),
        "app_test_run" => manager.test_run(app, context, args),
        _ => Err(format!("Unknown project tool: {name}")),
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
