use serde_json::{json, Value};
use tauri::{AppHandle, Manager};

#[derive(Clone)]
pub struct WorkbenchContext {
    pub workspace_id: String,
    pub session_id: Option<String>,
}

fn tool(name: &str, description: &str, properties: Value, required: &[&str]) -> Value {
    json!({"name":name,"description":description,"inputSchema":{"type":"object","properties":properties,"required":required,"additionalProperties":false}})
}

pub fn definitions() -> Vec<Value> {
    let id = json!({"type":"string","minLength":1});
    let mut tools = vec![
        tool("desktop_capabilities", "Discover currently available project-bound desktop tools. Workspace/root arguments are not accepted.", json!({}), &[]),
        tool("terminal_list", "List this project's terminals, ownership and exit status. Existing user-owned terminals require explicit user handover before writes.", json!({}), &[]),
        tool("terminal_create", "Open a new agent-owned terminal in this project. Existing user-owned terminals require explicit user handover.", json!({"title":{"type":"string","minLength":1,"maxLength":120},"cols":{"type":"integer","minimum":20,"maximum":500},"rows":{"type":"integer","minimum":5,"maximum":300}}), &[]),
        tool("terminal_close", "Close an agent-owned project terminal and stop its process tree. User-owned terminals require user handover.", json!({"id":id}), &["id"]),
        tool("terminal_release", "Return an agent-owned project terminal to the user. Only the user can hand existing terminals back to the agent.", json!({"id":id}), &["id"]),
        tool("terminal_read", "Read bounded terminal output, sequence watermark, ownership and exit status in this project.", json!({"id":id}), &["id"]),
        tool("terminal_write", "Write raw terminal input only after the user handed this terminal to the agent. Include carriage return to execute a command.", json!({"id":id,"data":{"type":"string","maxLength":1048576}}), &["id","data"]),
        tool("terminal_resize", "Resize a project terminal within 20–500 columns and 5–300 rows.", json!({"id":id,"cols":{"type":"integer","minimum":20,"maximum":500},"rows":{"type":"integer","minimum":5,"maximum":300}}), &["id","cols","rows"]),
        tool("context_get", "Read persistent project goal, constraints, acceptance criteria and file sources.", json!({}), &[]),
        tool("context_update", "Update persistent project working context. Sources are limited to eight validated project text files or selection snapshots.", json!({"enabled":{"type":"boolean"},"goal":{"type":"string","maxLength":4000},"constraints":{"type":"string","maxLength":4000},"criteria":{"type":"string","maxLength":4000},"sources":{"type":"array","maxItems":8,"items":{"type":"object","properties":{"id":{"type":"string","maxLength":8192},"kind":{"type":"string","enum":["file","selection"]},"path":{"type":"string","minLength":1,"maxLength":4096},"content":{"type":"string","maxLength":12000},"startLine":{"type":"integer","minimum":1},"endLine":{"type":"integer","minimum":1}},"required":["kind","path"],"additionalProperties":false}}}), &[]),
    ];
    tools.extend(super::workbench_git::definitions());
    tools.extend(super::workbench_automation::definitions());
    tools.extend(super::workbench_launch::definitions());
    tools.extend(crate::remote::tools::definitions());
    tools.extend(crate::computer::tools::definitions());
    tools
}

fn validate(name: &str, args: &Value) -> Result<(), String> {
    let definition = definitions()
        .into_iter()
        .find(|tool| tool["name"] == name)
        .ok_or_else(|| format!("Unknown project tool: {name}"))?;
    let object = args
        .as_object()
        .ok_or("Project tool arguments must be an object")?;
    let properties = definition["inputSchema"]["properties"]
        .as_object()
        .ok_or("Invalid project tool schema")?;
    if object.keys().any(|key| !properties.contains_key(key)) {
        return Err(
            "Unknown project tool argument; workspace/root parameters are not accepted".to_owned(),
        );
    }
    for key in definition["inputSchema"]["required"]
        .as_array()
        .into_iter()
        .flatten()
        .filter_map(Value::as_str)
    {
        if !object.contains_key(key) {
            return Err(format!("Missing argument: {key}"));
        }
    }
    Ok(())
}

fn context_update(
    app: &AppHandle,
    context: &WorkbenchContext,
    root: &std::path::Path,
    key: &str,
    remote: bool,
    patch: &Value,
) -> Result<Value, String> {
    if remote && patch.get("sources").is_some() {
        return Err(
            "Remote context sources require the SSH file adapter; local fallback is not allowed"
                .to_owned(),
        );
    }
    let mut patch = patch.clone();
    for name in ["goal", "constraints", "criteria"] {
        if let Some(value) = patch.get(name) {
            if value
                .as_str()
                .filter(|value| value.chars().count() <= 4000)
                .is_none()
            {
                return Err(format!("{name} must contain at most 4000 characters"));
            }
        }
    }
    if patch
        .get("enabled")
        .is_some_and(|value| !value.is_boolean())
    {
        return Err("enabled must be boolean".to_owned());
    }
    if let Some(sources) = patch.get_mut("sources") {
        let sources = sources
            .as_array_mut()
            .filter(|sources| sources.len() <= 8)
            .ok_or("Context sources must contain at most eight files")?;
        for source in sources {
            let object = source
                .as_object()
                .ok_or("Context source must be an object")?;
            if object.keys().any(|key| {
                !["id", "kind", "path", "content", "startLine", "endLine"].contains(&key.as_str())
            }) {
                return Err("Unknown context source field".to_owned());
            }
            let path = source["path"]
                .as_str()
                .filter(|path| {
                    !path.is_empty() && path.chars().count() <= 4096 && !path.contains('\0')
                })
                .ok_or("Invalid context source path")?;
            let file = crate::files::read(root, path).map_err(|error| error.message)?;
            if file["content"].as_str().is_none() || file.get("image").is_some() {
                return Err("Context source must be a project text file".to_owned());
            }
            if source.get("content").is_some_and(|value| {
                value
                    .as_str()
                    .filter(|value| value.chars().count() <= 12000)
                    .is_none()
            }) {
                return Err("Invalid context source content".to_owned());
            }
            for line in ["startLine", "endLine"] {
                if source
                    .get(line)
                    .is_some_and(|value| value.as_u64().filter(|line| *line > 0).is_none())
                {
                    return Err(format!("Invalid context source {line}"));
                }
            }
            match source["kind"].as_str() {
                Some("file") => {}
                Some("selection") => {
                    let start = source["startLine"]
                        .as_u64()
                        .filter(|line| *line > 0)
                        .ok_or("Selection requires a positive startLine")?;
                    let end = source["endLine"]
                        .as_u64()
                        .filter(|line| *line >= start)
                        .ok_or("Selection requires a valid endLine")?;
                    let _ = end;
                    if source["content"]
                        .as_str()
                        .filter(|text| text.chars().count() <= 12000)
                        .is_none()
                    {
                        return Err(
                            "Selection content exceeds 12000 characters or is missing".to_owned()
                        );
                    }
                }
                _ => return Err("Invalid context source kind".to_owned()),
            }
            if source.get("id").is_none() {
                source["id"] = json!(uuid::Uuid::new_v4().to_string());
            } else if source["id"]
                .as_str()
                .filter(|id| !id.is_empty() && id.len() <= 8192)
                .is_none()
            {
                return Err("Invalid context source ID".to_owned());
            }
        }
    }
    let state = app.state::<crate::state::AppState>();
    let key = key.to_owned();
    let mut settings = state.settings.lock().map_err(|error| error.to_string())?;
    let mut value = settings.get()["agentContexts"][&key]
        .as_object()
        .cloned()
        .map(Value::Object)
        .unwrap_or_else(empty_context);
    value["enabled"] = json!(true);
    value
        .as_object_mut()
        .ok_or("Invalid saved context")?
        .extend(patch.as_object().ok_or("Invalid context patch")?.clone());
    settings
        .update(&json!({"agentContexts":{key.clone():value}}))
        .map_err(|error| error.message)?;
    drop(settings);
    crate::events::emit(
        app,
        &context.workspace_id,
        "workbench",
        json!({"action":"contextChanged","key":key,"context":value}),
    );
    Ok(value)
}

fn empty_context() -> Value {
    json!({"enabled":false,"goal":"","constraints":"","criteria":"","sources":[]})
}

pub fn run(
    app: &AppHandle,
    context: &WorkbenchContext,
    name: &str,
    args: &Value,
) -> Result<Value, String> {
    validate(name, args)?;
    let state = app.state::<crate::state::AppState>();
    let root = state
        .root(&context.workspace_id)
        .map_err(|error| error.message)?;
    let project = state
        .projects
        .lock()
        .map_err(|error| error.to_string())?
        .get(&context.workspace_id)
        .cloned()
        .ok_or("Project is not open")?;
    let remote = project.get("remote").is_some_and(Value::is_object);
    let key = project["path"]
        .as_str()
        .ok_or("Project path is unavailable")?;
    let result = match name {
        "desktop_capabilities" => {
            json!({"tools":definitions().into_iter().map(|tool| json!({"name":tool["name"],"description":tool["description"]})).collect::<Vec<_>>(),"context":"Project-bound; workspace/root arguments are not accepted.","sessionBound":context.session_id.is_some(),"browser":"Use browser tools in this MCP session for navigation, DOM snapshots/evaluation, input and screenshots."})
        }
        "terminal_list" | "terminal_read" | "terminal_write" | "terminal_resize"
        | "terminal_create" | "terminal_close" | "terminal_release" => {
            let mut request = if name == "terminal_create" {
                json!({"options":args})
            } else {
                args.clone()
            };
            request["action"] = json!(name.trim_start_matches("terminal_"));
            if remote {
                let profile = project["remote"]["profileId"]
                    .as_str()
                    .ok_or("SSH profile binding is unavailable")?;
                state
                    .remotes
                    .terminals
                    .call(
                        &state.remotes,
                        app.clone(),
                        profile,
                        &context.workspace_id,
                        &request,
                        "agent",
                    )
                    .map_err(|error| error.message)?
            } else {
                state
                    .terminals
                    .agent_call(
                        app.clone(),
                        &context.workspace_id,
                        &context.workspace_id,
                        &root,
                        &request,
                    )
                    .map_err(|error| error.message)?
            }
        }
        name if name.starts_with("git_") || name.starts_with("github_") => {
            super::workbench_git::run(&root, &context.workspace_id, remote, name, args)?
        }
        name if name.starts_with("automation_") => {
            super::workbench_automation::run(app, key, name, args)?
        }
        "context_get" => state
            .settings
            .lock()
            .map_err(|error| error.to_string())?
            .get()["agentContexts"][key]
            .as_object()
            .cloned()
            .map(Value::Object)
            .unwrap_or_else(empty_context),
        "context_update" => context_update(app, context, &root, key, remote, args)?,
        name if name.starts_with("ssh_") => {
            let mut result = crate::remote::tools::run(app, &context.workspace_id, name, args)
                .map_err(|error| error.message)?;
            if let Some(image) = result["image"].as_str().map(str::to_owned) {
                let (mime, data) = image
                    .strip_prefix("data:")
                    .and_then(|image| image.split_once(";base64,"))
                    .filter(|(mime, _)| mime.starts_with("image/"))
                    .ok_or("Invalid SSH image result")?;
                if let Some(object) = result.as_object_mut() {
                    object.remove("image");
                }
                return Ok(
                    json!([{"type":"text","text":result.to_string()},{"type":"image","mimeType":mime,"data":data}]),
                );
            }
            result
        }
        name if name.starts_with("launch_") => {
            if remote {
                return Err("Remote launch tools require the SSH launch adapter; local execution is not allowed".to_owned());
            }
            super::workbench_launch::run(app, context, &root, name, args)?
        }
        name if name.starts_with("window_") => {
            return crate::computer::tools::run(app, context, name, args)
        }
        _ => return Err(format!("Unknown project tool: {name}")),
    };
    Ok(super::evaluate::text(result.to_string()))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn schemas_never_accept_workspace_or_root_overrides() {
        let mut names = std::collections::HashSet::new();
        for tool in definitions() {
            let name = tool["name"].as_str().unwrap();
            assert!(names.insert(name.to_owned()), "Duplicate tool: {name}");
            assert!(tool["inputSchema"]["properties"]
                .get("workspaceId")
                .is_none());
            assert!(validate(name, &json!({"workspaceId":"other"})).is_err());
            assert!(validate(name, &json!({"root":"/tmp"})).is_err());
        }
        for name in ["git_revert_commit", "git_restore_commit"] {
            assert!(validate(name, &json!({"sha":"a".repeat(40)})).is_err());
        }
    }
}
