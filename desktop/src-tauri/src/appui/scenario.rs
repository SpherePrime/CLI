use std::fs;
use std::path::{Path, PathBuf};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use serde::Deserialize;
use serde_json::{json, Value};
use tauri::{AppHandle, Manager};

use super::locator;
use super::manager::AppUiManager;
use super::types::{Locator, UiSnapshot};
use super::uia::action::UiAction;
use crate::browser::workbench::WorkbenchContext;

const STEP_TIMEOUT: Duration = Duration::from_secs(15);
const WAIT_TIMEOUT: Duration = Duration::from_secs(10);
const POLL: Duration = Duration::from_millis(200);

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct Scenario {
    pub version: u32,
    #[serde(default)]
    pub name: Option<String>,
    #[serde(default)]
    pub launch: Option<String>,
    pub steps: Vec<Step>,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(
    tag = "action",
    rename_all = "snake_case",
    rename_all_fields = "camelCase",
    deny_unknown_fields
)]
pub enum Step {
    Attach {
        #[serde(default)]
        hwnd: Option<String>,
        #[serde(default)]
        title: Option<String>,
        #[serde(default)]
        mode: Option<String>,
    },
    WaitFor {
        locator: Locator,
        #[serde(default)]
        state: Option<String>,
        #[serde(default)]
        value: Option<String>,
        #[serde(default)]
        timeout_ms: Option<u64>,
    },
    Click {
        locator: Locator,
    },
    Type {
        locator: Locator,
        text: String,
    },
    SetValue {
        locator: Locator,
        value: String,
    },
    Toggle {
        locator: Locator,
    },
    Select {
        locator: Locator,
    },
    Focus {
        locator: Locator,
    },
    PressKey {
        key: String,
    },
    Scroll {
        locator: Locator,
        direction: String,
        amount: String,
    },
    Assert {
        locator: Locator,
        #[serde(default)]
        state: Option<String>,
        #[serde(default)]
        value: Option<String>,
    },
    Screenshot {
        name: String,
    },
    Stop,
}

impl Step {
    fn name(&self) -> &'static str {
        match self {
            Self::Attach { .. } => "attach",
            Self::WaitFor { .. } => "wait_for",
            Self::Click { .. } => "click",
            Self::Type { .. } => "type",
            Self::SetValue { .. } => "set_value",
            Self::Toggle { .. } => "toggle",
            Self::Select { .. } => "select",
            Self::Focus { .. } => "focus",
            Self::PressKey { .. } => "press_key",
            Self::Scroll { .. } => "scroll",
            Self::Assert { .. } => "assert",
            Self::Screenshot { .. } => "screenshot",
            Self::Stop => "stop",
        }
    }
}

pub fn parse(source: &str) -> Result<Scenario, String> {
    let scenario: Scenario = serde_yaml::from_str(source).map_err(|error| error.to_string())?;
    if scenario.version != 1 {
        return Err(format!(
            "Unsupported scenario version {}; expected 1",
            scenario.version
        ));
    }
    if scenario.steps.is_empty() {
        return Err("The scenario has no steps".to_owned());
    }
    Ok(scenario)
}

pub fn list(root: &Path) -> Result<Value, String> {
    let mut scenarios = Vec::new();
    let entries = fs::read_dir(root).map_err(|error| error.to_string())?;
    for entry in entries.flatten() {
        let path = entry.path();
        let name = path.file_name().and_then(|value| value.to_str()).unwrap_or("");
        if !name.starts_with("prime-app-test") {
            continue;
        }
        if !["yaml", "yml", "json"]
            .iter()
            .any(|extension| name.ends_with(&format!(".{extension}")))
        {
            continue;
        }
        let source = fs::read_to_string(&path).map_err(|error| error.to_string())?;
        match parse(&source) {
            Ok(scenario) => scenarios.push(json!({
                "path": name,
                "name": scenario.name.unwrap_or_else(|| name.to_owned()),
                "steps": scenario.steps.len(),
                "launch": scenario.launch,
            })),
            Err(error) => scenarios.push(json!({"path": name, "error": error})),
        }
    }
    Ok(json!(scenarios))
}

pub fn run(
    app: &AppHandle,
    context: &WorkbenchContext,
    manager: &AppUiManager,
    root: &Path,
    request: &Value,
) -> Result<Value, String> {
    let name = request
        .get("path")
        .and_then(Value::as_str)
        .filter(|value| !value.is_empty())
        .ok_or("Missing argument: path")?;
    if name.contains('/') || name.contains('\\') || name.contains("..") {
        return Err("The scenario path must be a file name in the project root".to_owned());
    }
    let source = fs::read_to_string(root.join(name)).map_err(|error| error.to_string())?;
    let scenario = parse(&source)?;
    let output = output_directory(app, name)?;
    let mut results = Vec::new();
    let mut passed = true;
    for (index, step) in scenario.steps.iter().enumerate() {
        let started = Instant::now();
        let outcome = execute(app, context, manager, step, &output, index);
        let record = json!({
            "index": index + 1,
            "action": step.name(),
            "ok": outcome.is_ok(),
            "ms": started.elapsed().as_millis() as u64,
            "error": outcome.as_ref().err().cloned(),
        });
        results.push(record);        if let Err(error) = outcome {
            passed = false;
            results.last_mut().map(|record| {
                record["error"] = json!(error);
                record
            });
            let _ = write_json(
                &output.join("results.json"),
                &summary(name, &scenario, passed, &results, &output),
            );
            break;
        }
        let _ = write_json(
            &output.join("results.json"),
            &summary(name, &scenario, passed, &results, &output),
        );
    }
    let summary = summary(name, &scenario, passed, &results, &output);
    write_json(&output.join("results.json"), &summary)?;
    Ok(summary)
}

fn summary(
    name: &str,
    scenario: &Scenario,
    passed: bool,
    results: &[Value],
    output: &Path,
) -> Value {
    json!({
        "scenario": name,
        "name": scenario.name,
        "passed": passed,
        "steps": results,
        "artifacts": output.to_string_lossy(),
    })
}

fn execute(
    app: &AppHandle,
    context: &WorkbenchContext,
    manager: &AppUiManager,
    step: &Step,
    output: &Path,
    index: usize,
) -> Result<(), String> {
    match step {
        Step::Attach { hwnd, title, mode } => {
            let window = resolve_window(hwnd.as_deref(), title.as_deref())?;
            manager.attach(
                app,
                context,
                &json!({
                    "hwnd": window.hwnd,
                    "pid": window.pid,
                    "identity": window.identity.unwrap_or_default(),
                    "title": window.title,
                    "mode": mode.clone().unwrap_or_else(|| "control".to_owned()),
                }),
            )?;
            Ok(())
        }
        Step::WaitFor {
            locator,
            state,
            value,
            timeout_ms,
        } => {
            let deadline = Instant::now()
                + Duration::from_millis(timeout_ms.unwrap_or(WAIT_TIMEOUT.as_millis() as u64));
            loop {
                let snapshot = snapshot_of(app, context, manager)?;
                let found = locator::resolve(&snapshot, locator)
                    .ok()
                    .and_then(|reference| find(&snapshot.nodes, &reference).cloned());
                if matches(&found, state.as_deref(), value.as_deref()) {
                    return Ok(());
                }
                if Instant::now() >= deadline {
                    return Err(format!(
                        "The element did not reach the expected state within {} ms",
                        timeout_ms.unwrap_or(WAIT_TIMEOUT.as_millis() as u64)
                    ));
                }
                std::thread::sleep(POLL);
            }
        }
        Step::Click { locator } => act(app, context, manager, locator, UiAction::Invoke),
        Step::Type { locator, text } => {
            manager.type_text(app, context, locator.clone(), text.clone())?;
            Ok(())
        }
        Step::SetValue { locator, value } => act(
            app,
            context,
            manager,
            locator,
            UiAction::SetValue(value.clone()),
        ),
        Step::Toggle { locator } => act(app, context, manager, locator, UiAction::Toggle),
        Step::Select { locator } => act(app, context, manager, locator, UiAction::Select),
        Step::Focus { locator } => act(app, context, manager, locator, UiAction::Focus),
        Step::PressKey { key } => {
            manager.press_key(app, context, key.clone())?;
            Ok(())
        }
        Step::Scroll {
            locator,
            direction,
            amount,
        } => act(
            app,
            context,
            manager,
            locator,
            UiAction::Scroll(direction.clone(), amount.clone()),
        ),
        Step::Assert {
            locator,
            state,
            value,
        } => {
            let snapshot = snapshot_of(app, context, manager)?;
            let reference = locator::resolve(&snapshot, locator)?;
            let node = find(&snapshot.nodes, &reference)
                .ok_or_else(|| "The element is no longer available".to_owned())?;
            if matches(&Some(node.clone()), state.as_deref(), value.as_deref()) {
                Ok(())
            } else {
                Err(format!(
                    "{} \"{}\" does not match the expected state",
                    node.role, node.name
                ))
            }
        }
        Step::Screenshot { name } => {
            let target = manager.window(app, context)?;
            let capture = crate::computer::capture::capture(&target)?;
            write(&output.join(format!("{:02}-{name}.png", index + 1)), &capture.png)?;
            Ok(())
        }
        Step::Stop => Ok(()),
    }
}

fn act(
    app: &AppHandle,
    context: &WorkbenchContext,
    manager: &AppUiManager,
    locator: &Locator,
    action: UiAction,
) -> Result<(), String> {
    manager.act(app, context, locator.clone(), action)?;
    Ok(())
}

fn snapshot_of(
    app: &AppHandle,
    context: &WorkbenchContext,
    manager: &AppUiManager,
) -> Result<UiSnapshot, String> {
    let value = manager.snapshot(app, context)?;
    serde_json::from_value(value["snapshot"].clone()).map_err(|error| error.to_string())
}

fn matches(node: &Option<super::types::UiNode>, state: Option<&str>, value: Option<&str>) -> bool {
    let Some(node) = node else {
        return state == Some("gone");
    };
    match state.unwrap_or("exists") {
        "gone" => false,
        "exists" => value.is_none_or(|value| node.value.as_deref() == Some(value)),
        "enabled" => node.states.iter().any(|item| item == "enabled"),
        "disabled" => !node.states.iter().any(|item| item == "enabled"),
        "value" => value.is_some_and(|value| node.value.as_deref() == Some(value)),
        other => {
            let _ = other;
            value.is_none_or(|value| node.value.as_deref() == Some(value))
        }
    }
}

fn find<'a>(nodes: &'a [super::types::UiNode], reference: &str) -> Option<&'a super::types::UiNode> {
    for node in nodes {
        if node.reference == reference {
            return Some(node);
        }
        if let Some(found) = find(&node.children, reference) {
            return Some(found);
        }
    }
    None
}

fn resolve_window(hwnd: Option<&str>, title: Option<&str>) -> Result<crate::computer::types::WindowInfo, String> {
    let windows = crate::computer::windows::list()?;
    let found = windows.into_iter().find(|window| {
        hwnd.is_some_and(|value| window.hwnd == value)
            || title.is_some_and(|value| window.title.contains(value))
    });
    found.ok_or_else(|| "The application window was not found".to_owned())
}

fn output_directory(app: &AppHandle, name: &str) -> Result<PathBuf, String> {
    let directory = app.state::<crate::state::AppState>().directory.clone();
    let stamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|value| value.as_millis())
        .unwrap_or(0);
    let output = directory
        .join("app-tests")
        .join(format!("{stamp}-{}", name.trim_end_matches(".yaml").trim_end_matches(".yml").trim_end_matches(".json")));
    fs::create_dir_all(&output).map_err(|error| error.to_string())?;
    Ok(output)
}

fn write(path: &Path, bytes: &[u8]) -> Result<(), String> {
    fs::write(path, bytes).map_err(|error| error.to_string())
}

fn write_json(path: &Path, value: &Value) -> Result<(), String> {
    let text = serde_json::to_string_pretty(value).map_err(|error| error.to_string())?;
    write(path, text.as_bytes())
}

#[cfg(test)]
mod tests {
    use super::*;

    const SAMPLE: &str = r#"{
      "version": 1,
      "name": "fixture",
      "steps": [
        {"action": "attach", "title": "Prime app UI fixture", "mode": "control"},
        {"action": "wait_for", "locator": {"role": "button", "name": "Save"}, "timeoutMs": 5000},
        {"action": "click", "locator": {"name": "Save"}},
        {"action": "assert", "locator": {"role": "text", "name": "Ready"}, "value": "Saved"},
        {"action": "screenshot", "name": "result"}
      ]
    }"#;

    #[test]
    fn parses_a_scenario_with_locations_and_states() {
        let scenario = parse(SAMPLE).unwrap();
        assert_eq!(scenario.steps.len(), 5);
        assert_eq!(scenario.name.as_deref(), Some("fixture"));
        assert_eq!(scenario.steps[0].name(), "attach");
        assert_eq!(scenario.steps[4].name(), "screenshot");
    }

    #[test]
    fn parses_a_yaml_scenario() {
        let source = "version: 1\nname: fixture\nsteps:\n  - action: attach\n    title: Prime app UI fixture\n  - action: click\n    locator:\n      role: button\n      name: Save\n";
        let scenario = parse(source).unwrap();
        assert_eq!(scenario.steps.len(), 2);
        assert_eq!(scenario.steps[1].name(), "click");
    }

    #[test]
    fn rejects_unknown_versions_steps_and_fields() {
        assert!(parse(r#"{"version": 2, "steps": []}"#).is_err());
        assert!(parse(r#"{"version": 1, "steps": []}"#).is_err());
        assert!(parse(r#"{"version": 1, "steps": [{"action": "explode"}]}"#).is_err());
        assert!(parse(r#"{"version": 1, "steps": [{"action": "click", "locator": {}, "extra": 1}]}"#).is_err());
    }

    #[test]
    fn matches_states_of_elements() {
        let node = super::super::types::UiNode {
            reference: "n3".to_owned(),
            role: "text".to_owned(),
            name: "Ready".to_owned(),
            value: Some("Saved".to_owned()),
            states: vec!["enabled".to_owned()],
            ..Default::default()
        };
        assert!(matches(&Some(node.clone()), None, Some("Saved")));
        assert!(matches(&Some(node.clone()), Some("enabled"), None));
        assert!(!matches(&Some(node.clone()), Some("disabled"), None));
        assert!(!matches(&Some(node.clone()), Some("value"), Some("Other")));
        assert!(matches(&None, Some("gone"), None));
        assert!(!matches(&None, None, None));
    }
}
