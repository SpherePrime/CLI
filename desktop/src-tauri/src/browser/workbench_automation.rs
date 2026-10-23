use serde_json::{json, Value};
use tauri::{AppHandle, Manager};

fn tool(name: &str, description: &str, properties: Value, required: &[&str]) -> Value {
    json!({"name":name,"description":description,"inputSchema":{"type":"object","properties":properties,"required":required,"additionalProperties":false}})
}

fn trigger_schema() -> Value {
    let time = json!({"type":"string","pattern":"^([01]\\d|2[0-3]):[0-5]\\d$"});
    let minutes = json!({"type":"number","minimum":1,"maximum":525600});
    json!({"oneOf":[
        {"type":"object","properties":{"type":{"const":"interval"},"minutes":minutes},"required":["type","minutes"],"additionalProperties":false},
        {"type":"object","properties":{"type":{"const":"daily"},"time":time},"required":["type","time"],"additionalProperties":false},
        {"type":"object","properties":{"type":{"const":"weekly"},"time":time,"days":{"type":"array","minItems":1,"maxItems":7,"items":{"type":"integer","minimum":0,"maximum":6}}},"required":["type","time","days"],"additionalProperties":false},
        {"type":"object","properties":{"type":{"const":"filesChanged"},"cooldownMinutes":minutes},"required":["type","cooldownMinutes"],"additionalProperties":false},
        {"type":"object","properties":{"type":{"const":"runFailed"},"cooldownMinutes":minutes},"required":["type","cooldownMinutes"],"additionalProperties":false}
    ]})
}

pub fn definitions() -> Vec<Value> {
    let id = json!({"type":"string","format":"uuid"});
    let fields = json!({"id":id,"title":{"type":"string","minLength":1,"maxLength":160},"prompt":{"type":"string","minLength":1,"maxLength":100000},"enabled":{"type":"boolean","default":true},"trigger":trigger_schema()});
    let mut tools = vec![
        tool("automation_list", "List this project's saved schedules, event triggers and run history.", json!({}), &[]),
        tool("automation_save", "Create a persistent automation or replace a project rule. Runs in independent chats; daily/weekly times use local timezone, days are 0 Sunday through 6 Saturday. Desktop must remain running.", fields.clone(), &["title","prompt","trigger"]),
        tool("automation_update", "Update selected fields of an existing project automation, including resuming with enabled=true.", fields, &["id"]),
        tool("automation_pause", "Pause a project automation without cancelling an active run.", json!({"id":id}), &["id"]),
        tool("automation_delete", "Delete an inactive automation rule in this project.", json!({"id":id}), &["id"]),
        tool("automation_run", "Run a saved automation now in this project.", json!({"id":id}), &["id"]),
    ];
    for action in ["cancel", "open"] {
        tools.push(tool(
            &format!("automation_{action}"),
            if action == "cancel" {
                "Cancel an active automation run in this project."
            } else {
                "Open an automation run's chat in the desktop."
            },
            json!({"runId":id}),
            &["runId"],
        ));
    }
    tools
}

fn validate(args: &Value) -> Result<(), String> {
    for field in ["id", "runId"] {
        if let Some(value) = args.get(field) {
            uuid::Uuid::parse_str(value.as_str().ok_or("Automation ID must be a UUID")?)
                .map_err(|_| "Automation ID must be a UUID")?;
        }
    }
    for (field, maximum) in [("title", 160), ("prompt", 100000)] {
        if let Some(value) = args.get(field) {
            if value
                .as_str()
                .filter(|value| !value.trim().is_empty() && value.chars().count() <= maximum)
                .is_none()
            {
                return Err(format!("Invalid automation {field}"));
            }
        }
    }
    if args.get("enabled").is_some_and(|value| !value.is_boolean()) {
        return Err("enabled must be boolean".into());
    }
    if let Some(trigger) = args.get("trigger") {
        let kind = trigger["type"]
            .as_str()
            .ok_or("Invalid automation trigger")?;
        let allowed: &[&str] = match kind {
            "interval" => &["type", "minutes"],
            "daily" => &["type", "time"],
            "weekly" => &["type", "time", "days"],
            "filesChanged" | "runFailed" => &["type", "cooldownMinutes"],
            _ => return Err("Invalid automation trigger".into()),
        };
        let fields = trigger.as_object().ok_or("Invalid automation trigger")?;
        if fields.len() != allowed.len()
            || fields.keys().any(|key| !allowed.contains(&key.as_str()))
        {
            return Err("Unknown or missing automation trigger field".into());
        }
        for field in ["minutes", "cooldownMinutes"] {
            if let Some(value) = trigger.get(field) {
                if value
                    .as_f64()
                    .filter(|value| value.is_finite() && (1.0..=525600.0).contains(value))
                    .is_none()
                {
                    return Err("Invalid automation interval".into());
                }
            }
        }
        if let Some(time) = trigger.get("time") {
            let time = time.as_str().ok_or("Invalid automation time")?.as_bytes();
            if time.len() != 5
                || time[2] != b':'
                || ![time[0], time[1], time[3], time[4]]
                    .iter()
                    .all(u8::is_ascii_digit)
                || time[0] > b'2'
                || (time[0] == b'2' && time[1] > b'3')
                || time[3] > b'5'
            {
                return Err("Invalid automation time".into());
            }
        }
        if let Some(days) = trigger.get("days") {
            if days
                .as_array()
                .filter(|days| {
                    (1..=7).contains(&days.len())
                        && days
                            .iter()
                            .all(|day| day.as_u64().is_some_and(|day| day <= 6))
                })
                .is_none()
            {
                return Err("Invalid automation weekdays".into());
            }
        }
    }
    Ok(())
}

fn save_input(name: &str, args: &Value, snapshot: &Value) -> Result<Value, String> {
    let mut input = if name == "automation_save" {
        args.clone()
    } else {
        let rule = snapshot["rules"]
            .as_array()
            .and_then(|rules| rules.iter().find(|rule| rule["id"] == args["id"]))
            .ok_or("Automation not found in this project")?;
        let mut input = json!({});
        for key in ["id", "title", "prompt", "enabled", "trigger"] {
            input[key] = rule[key].clone();
        }
        input.as_object_mut().unwrap().extend(
            args.as_object()
                .ok_or("Invalid automation arguments")?
                .clone(),
        );
        input
    };
    if name == "automation_pause" {
        input["enabled"] = json!(false);
    }
    if input.get("enabled").is_none() {
        input["enabled"] = json!(true);
    }
    validate(&input)?;
    Ok(input)
}

pub fn run(app: &AppHandle, key: &str, name: &str, args: &Value) -> Result<Value, String> {
    validate(args)?;
    let state = app.state::<crate::state::AppState>();
    let call = |request: Value| {
        state
            .automations
            .handle(app.clone(), &request)
            .map_err(|error| error.message)
    };
    let snapshot = call(json!({"action":"list","root":key}))?;
    match name {
        "automation_list" => Ok(snapshot),
        "automation_save" | "automation_update" | "automation_pause" => {
            call(json!({"action":"save","root":key,"input":save_input(name,args,&snapshot)?}))
        }
        "automation_delete" | "automation_run" => call(
            json!({"action":if name == "automation_delete" {"remove"} else {"run"},"root":key,"id":args["id"]}),
        ),
        "automation_cancel" => call(json!({"action":"cancel","root":key,"runId":args["runId"]})),
        "automation_open" => {
            let run = snapshot["runs"]
                .as_array()
                .and_then(|runs| runs.iter().find(|run| run["id"] == args["runId"]))
                .ok_or("Run chat is not available")?;
            let workspace = run["workspaceId"]
                .as_str()
                .filter(|value| !value.is_empty())
                .ok_or("Run chat is not available")?;
            let session = run["sessionId"]
                .as_str()
                .filter(|value| !value.is_empty())
                .ok_or("Run chat is not available")?;
            crate::events::emit(
                app,
                workspace,
                "workbench",
                json!({"action":"openSession","sessionId":session}),
            );
            Ok(json!({"opened":true}))
        }
        _ => Err("Unknown automation tool".into()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn trigger_validation_rejects_unknown_fields_and_out_of_bounds_values() {
        assert!(
            validate(&json!({"trigger":{"type":"filesChanged","cooldownMinutes":525601}})).is_err()
        );
        assert!(validate(&json!({"trigger":{"type":"daily","time":"24:00"}})).is_err());
        assert!(
            validate(&json!({"trigger":{"type":"interval","minutes":1,"root":"other"}})).is_err()
        );
        assert!(
            validate(&json!({"trigger":{"type":"weekly","time":"23:59","days":[0,6]}})).is_ok()
        );
    }

    #[test]
    fn partial_updates_and_pause_preserve_project_rule_fields() {
        let rule = json!({"id":"2eb2187e-39ca-4a15-a891-789da60e5494","title":"Build","prompt":"Check build","enabled":true,"trigger":{"type":"interval","minutes":5}});
        let snapshot = json!({"rules":[rule]});
        let updated = save_input(
            "automation_update",
            &json!({"id":rule["id"],"title":"Test"}),
            &snapshot,
        )
        .unwrap();
        assert_eq!(updated["prompt"], rule["prompt"]);
        assert_eq!(updated["title"], "Test");
        assert_eq!(
            save_input("automation_pause", &json!({"id":rule["id"]}), &snapshot).unwrap()
                ["enabled"],
            false
        );
        assert!(save_input(
            "automation_pause",
            &json!({"id":"3eb2187e-39ca-4a15-a891-789da60e5494"}),
            &snapshot
        )
        .is_err());
    }
}
