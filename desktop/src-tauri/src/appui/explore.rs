use serde_json::{json, Value};
use tauri::AppHandle;

use super::manager::AppUiManager;
use super::types::{Locator, UiNode, UiSnapshot};
use super::uia::action::UiAction;
use crate::browser::workbench::WorkbenchContext;

const DEFAULT_STEPS: usize = 20;
const MAX_STEPS: usize = 100;
const SETTLE: std::time::Duration = std::time::Duration::from_millis(400);

const DANGEROUS: [&str; 22] = [
    "delete",
    "remove",
    "uninstall",
    "shutdown",
    "restart",
    "format",
    "send",
    "submit",
    "pay",
    "buy",
    "purchase",
    "confirm",
    "quit",
    "exit",
    "удалить",
    "удаление",
    "отправить",
    "оплатить",
    "купить",
    "выход",
    "закрыть",
    "подтвердить",
];

pub fn run(
    app: &AppHandle,
    context: &WorkbenchContext,
    manager: &AppUiManager,
    request: &Value,
) -> Result<Value, String> {
    let limit = request
        .get("maxSteps")
        .and_then(Value::as_u64)
        .map(|value| value as usize)
        .unwrap_or(DEFAULT_STEPS)
        .clamp(1, MAX_STEPS);
    let hwnd = manager.window(app, context)?;
    let mut visited: Vec<String> = Vec::new();
    let mut blocked: Vec<Value> = Vec::new();
    let mut steps: Vec<Value> = Vec::new();
    let mut stopped = "completed";
    for index in 0..limit {
        let snapshot = snapshot(app, context, manager)?;
        let Some(node) = candidate(&snapshot, &visited) else {
            stopped = "no more actionable elements";
            break;
        };
        visited.push(node.path.clone());
        if dangerous(&node.name) {
            blocked.push(json!({"ref": node.reference, "role": node.role, "name": node.name}));
            continue;
        }
        let action = if node.actions.iter().any(|item| item == "invoke") {
            UiAction::Invoke
        } else if node.actions.iter().any(|item| item == "toggle") {
            UiAction::Toggle
        } else {
            continue;
        };
        let locator = Locator {
            path: Some(node.path.clone()),
            ..Default::default()
        };
        let outcome = manager.act(app, context, locator, action);
        steps.push(json!({
            "index": index + 1,
            "role": node.role,
            "name": node.name,
            "path": node.path,
            "ok": outcome.is_ok(),
            "error": outcome.as_ref().err().cloned(),
        }));
        if let Err(error) = outcome {
            stopped = "an action failed";
            let _ = error;
            break;
        }
        std::thread::sleep(SETTLE);
        if crate::computer::windows::info(&hwnd).is_err() {
            stopped = "the application window disappeared";
            break;
        }
    }
    Ok(json!({
        "visited": visited.len(),
        "blocked": blocked,
        "steps": steps,
        "stopped": stopped,
    }))
}

fn snapshot(
    app: &AppHandle,
    context: &WorkbenchContext,
    manager: &AppUiManager,
) -> Result<UiSnapshot, String> {
    let value = manager.snapshot(app, context)?;
    serde_json::from_value(value["snapshot"].clone()).map_err(|error| error.to_string())
}

fn candidate<'a>(snapshot: &'a UiSnapshot, visited: &[String]) -> Option<&'a UiNode> {
    let mut found = Vec::new();
    collect(&snapshot.nodes, &mut found);
    found.into_iter().find(|node| {
        !node.path.is_empty()
            && !visited.contains(&node.path)
            && !node.states.iter().any(|state| state == "offscreen")
            && (node.actions.iter().any(|action| action == "invoke")
                || node.actions.iter().any(|action| action == "toggle"))
    })
}

fn collect<'a>(nodes: &'a [UiNode], found: &mut Vec<&'a UiNode>) {
    for node in nodes {
        found.push(node);
        collect(&node.children, found);
    }
}

fn dangerous(name: &str) -> bool {
    let name = name.to_lowercase();
    DANGEROUS.iter().any(|word| name.contains(word))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::appui::types::{Confidence, Provider};

    fn node(reference: &str, name: &str, actions: &[&str], states: &[&str]) -> UiNode {
        UiNode {
            reference: reference.to_owned(),
            role: "button".to_owned(),
            name: name.to_owned(),
            path: format!("window/button[{name}]"),
            actions: actions.iter().map(|value| (*value).to_owned()).collect(),
            states: states.iter().map(|value| (*value).to_owned()).collect(),
            source: Provider::Uia,
            confidence: Confidence::High,
            ..Default::default()
        }
    }

    fn snapshot(nodes: Vec<UiNode>) -> UiSnapshot {
        UiSnapshot {
            provider: Provider::Uia,
            hwnd: "42".to_owned(),
            title: "fixture".to_owned(),
            captured_at: 0,
            truncated: false,
            warnings: Vec::new(),
            count: nodes.len(),
            nodes,
        }
    }

    #[test]
    fn picks_the_first_unvisited_actionable_element() {
        let snapshot = snapshot(vec![
            node("n1", "Save", &["invoke"], &["enabled"]),
            node("n2", "Help", &["invoke"], &["enabled"]),
        ]);
        let first = candidate(&snapshot, &[]).unwrap();
        assert_eq!(first.path, "window/button[Save]");
        let second = candidate(&snapshot, &[first.path.clone()]).unwrap();
        assert_eq!(second.path, "window/button[Help]");
        assert!(candidate(&snapshot, &[first.path.clone(), second.path.clone()]).is_none());
    }

    #[test]
    fn skips_offscreen_and_passive_elements() {
        let snapshot = snapshot(vec![
            node("n1", "Hidden", &["invoke"], &["offscreen"]),
            node("n2", "Label", &[], &["enabled"]),
            node("n3", "Visible", &["invoke"], &["enabled"]),
        ]);
        assert_eq!(candidate(&snapshot, &[]).unwrap().path, "window/button[Visible]");
    }

    #[test]
    fn blocks_actions_with_side_effects() {
        for name in ["Delete account", "Send", "Удалить проект", "Выход"] {
            assert!(dangerous(name), "{name}");
        }
        for name in ["Save", "Settings", "Открыть"] {
            assert!(!dangerous(name), "{name}");
        }
    }
}
