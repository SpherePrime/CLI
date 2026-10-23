use super::workbench::WorkbenchContext;
use serde_json::{json, Value};
use tauri::{AppHandle, Manager};

fn tool(name: &str, description: &str, properties: Value, required: &[&str]) -> Value {
    json!({"name":name,"description":description,"inputSchema":{"type":"object","properties":properties,"required":required,"additionalProperties":false}})
}

pub fn definitions() -> Vec<Value> {
    let mut tools = vec![
        tool("launch_list", "List this project's app launch profiles, process status and URLs.", json!({}), &[]),
        tool("launch_save", "Save an app launch profile in this project. Commands execute only when launch_start or launch_restart is called.", json!({"id":{"type":"string","format":"uuid"},"name":{"type":"string","minLength":1,"maxLength":80},"command":{"type":"string","minLength":1,"maxLength":4000},"url":{"type":"string","maxLength":2000}}), &["name","command"]),
    ];
    for action in ["start", "stop", "restart", "remove", "logs"] {
        tools.push(tool(
            &format!("launch_{action}"),
            &format!("{action} a saved app launch profile in this project."),
            json!({"id":{"type":"string","format":"uuid"}}),
            &["id"],
        ));
    }
    tools
}

pub fn run(
    app: &AppHandle,
    context: &WorkbenchContext,
    root: &std::path::Path,
    name: &str,
    args: &Value,
) -> Result<Value, String> {
    let state = app.state::<crate::state::AppState>();
    let snapshot = state
        .launches
        .handle(
            app.clone(),
            &state.terminals,
            &context.workspace_id,
            root,
            &json!({"action":"list"}),
        )
        .map_err(|error| error.message)?;
    if let Some(id) = args.get("id") {
        let id = id.as_str().ok_or("Launch profile id must be a string")?;
        if !snapshot["configs"]
            .as_array()
            .is_some_and(|configs| configs.iter().any(|config| config["id"] == id))
        {
            return Err("Launch profile not found in this project".to_owned());
        }
    }
    if name == "launch_logs" {
        let id = args["id"].as_str().ok_or("Launch profile id is required")?;
        let process = snapshot["processes"]
            .as_array()
            .and_then(|processes| processes.iter().find(|process| process["configId"] == id));
        return if let Some(process) = process {
            let terminal = state
                .terminals
                .agent_call(
                    app.clone(),
                    &context.workspace_id,
                    &context.workspace_id,
                    root,
                    &json!({"action":"read","id":process["terminalId"]}),
                )
                .map_err(|error| error.message)?;
            Ok(json!({"process":process,"terminal":terminal}))
        } else {
            Ok(json!({"process":{"status":"stopped"}}))
        };
    }
    let action = name.strip_prefix("launch_").ok_or("Invalid launch tool")?;
    let request = if action == "save" {
        json!({"action":"save","input":args})
    } else {
        json!({"action":action,"id":args.get("id")})
    };
    state
        .launches
        .handle(
            app.clone(),
            &state.terminals,
            &context.workspace_id,
            root,
            &request,
        )
        .map_err(|error| error.message)
}
