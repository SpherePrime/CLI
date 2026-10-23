use crate::{
    error::{HostError, HostResult},
    state::AppState,
    teams::types::TeamTaskStatus,
};
use serde_json::{json, Value};
use std::time::{SystemTime, UNIX_EPOCH};

const DAY_MS: i64 = 24 * 60 * 60 * 1000;
const AGENT_CACHE_MS: i64 = 1500;

struct AgentCache {
    key: String,
    at: i64,
    lists: Vec<Value>,
}

#[derive(Default)]
pub struct JobCenter {
    agent_cache: std::sync::Mutex<Option<AgentCache>>,
}

pub fn handle(app: &tauri::AppHandle, state: &AppState, request: &Value) -> HostResult<Value> {
    let action = request["action"]
        .as_str()
        .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Invalid job request"))?;
    if action == "stop" {
        let id = request["id"]
            .as_str()
            .filter(|id| (3..=300).contains(&id.len()))
            .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Invalid job identifier"))?;
        stop(app, state, id)?;
    } else if action != "list" {
        return Err(HostError::new("INVALID_ARGUMENT", "Unknown job action"));
    }
    let center = state.jobs.lock().unwrap();
    list(app, state, &center)
}

fn now_ms() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|time| time.as_millis() as i64)
        .unwrap_or(0)
}

fn time(value: &Value) -> Option<i64> {
    if let Some(number) = value.as_i64() {
        return Some(number);
    }
    if let Some(number) = value.as_f64() {
        return Some(number as i64);
    }
    value
        .as_str()
        .and_then(|text| chrono::DateTime::parse_from_rfc3339(text).ok())
        .map(|time| time.timestamp_millis())
}

fn status_of(status: &str) -> &'static str {
    match status {
        "accepted" | "queued" => "queued",
        "completed" => "completed",
        "failed" => "failed",
        "cancelled" => "cancelled",
        "interrupted" => "interrupted",
        _ => "running",
    }
}

fn label(project: &Value) -> String {
    if project["kind"] == "chat" {
        return "Личные чаты".into();
    }
    if let Some(remote) = project["remote"].as_object() {
        let name = remote.get("name").and_then(Value::as_str).unwrap_or("");
        let root = remote
            .get("root")
            .and_then(Value::as_str)
            .unwrap_or("")
            .rsplit('/')
            .find(|part| !part.is_empty())
            .unwrap_or("/");
        return format!("{name} · {root}");
    }
    let path = project["path"].as_str().unwrap_or("");
    path.rsplit(['\\', '/'])
        .find(|part| !part.is_empty())
        .unwrap_or(path)
        .to_owned()
}

fn projects(state: &AppState) -> Vec<Value> {
    state
        .projects
        .lock()
        .unwrap()
        .values()
        .cloned()
        .collect()
}

fn project_by_key(projects: &[Value], key: &str) -> Option<Value> {
    let normalize = |value: &str| -> String {
        value
            .trim_start_matches("prime-ssh://")
            .replace('\\', "/")
            .to_lowercase()
    };
    let key = normalize(key);
    projects
        .iter()
        .find(|project| {
            project["path"]
                .as_str()
                .is_some_and(|path| normalize(path) == key)
        })
        .cloned()
}

fn project_by_workspace<'a>(projects: &'a [Value], id: &str) -> Option<&'a Value> {
    projects.iter().find(|project| project["id"] == id)
}

fn agent_jobs(state: &AppState, center: &JobCenter, projects: &[Value], owned: &[String]) -> HostResult<Vec<Value>> {
    let key = projects
        .iter()
        .filter_map(|project| project["id"].as_str())
        .collect::<Vec<_>>()
        .join("|");
    let now = now_ms();
    {
        let cache = center.agent_cache.lock().unwrap();
        if let Some(cache) = cache.as_ref() {
            if cache.key == key && now - cache.at < AGENT_CACHE_MS {
                return Ok(filter_owned(cache.lists.clone(), owned));
            }
        }
    }
    let engine = state.require_engine()?;
    let mut lists = Vec::new();
    for project in projects {
        let Some(id) = project["id"].as_str() else {
            continue;
        };
        let runs = engine
            .request("GET", &format!("/v1/workspaces/{id}/runs"), None)
            .unwrap_or(Value::Array(Vec::new()));
        let sessions = engine
            .request("GET", &format!("/v1/workspaces/{id}/sessions"), None)
            .unwrap_or(Value::Array(Vec::new()));
        let pending = engine
            .request("GET", &format!("/v1/workspaces/{id}/pending"), None)
            .unwrap_or(json!({}));
        lists.push(agent_list(project, &runs, &sessions, &pending));
    }
    *center.agent_cache.lock().unwrap() = Some(AgentCache {
        key,
        at: now,
        lists: lists.clone(),
    });
    Ok(filter_owned(lists, owned))
}

fn filter_owned(lists: Vec<Value>, owned: &[String]) -> Vec<Value> {
    lists
        .into_iter()
        .filter_map(|list| list.as_array().cloned())
        .flatten()
        .filter(|job| {
            !owned.contains(
                &job["id"]
                    .as_str()
                    .and_then(|id| id.rsplit(':').next())
                    .unwrap_or("")
                    .to_owned(),
            )
        })
        .collect()
}

fn agent_list(project: &Value, runs: &Value, sessions: &Value, pending: &Value) -> Value {
    let titles: std::collections::HashMap<String, String> = sessions
        .as_array()
        .map(|items| {
            items
                .iter()
                .filter_map(|item| {
                    Some((
                        item["id"].as_str()?.to_owned(),
                        item["title"].as_str().unwrap_or("").to_owned(),
                    ))
                })
                .collect()
        })
        .unwrap_or_default();
    let mut attention: std::collections::HashMap<String, &str> = std::collections::HashMap::new();
    if let Some(items) = pending["permissions"].as_array() {
        for item in items {
            if let Some(session) = item["session_id"].as_str() {
                attention.insert(session.to_owned(), "permission");
            }
        }
    }
    let questions = pending["questions"]
        .as_array()
        .cloned()
        .or_else(|| pending["question"].as_array().cloned())
        .unwrap_or_default();
    for item in &questions {
        if let Some(session) = item["session_id"].as_str() {
            attention.insert(session.to_owned(), "question");
        }
    }
    let list = runs
        .as_array()
        .cloned()
        .or_else(|| runs["runs"].as_array().cloned())
        .unwrap_or_default();
    let now = now_ms();
    Value::Array(
        list.into_iter()
            .filter(|run| {
                time(&run["finished_at"])
                    .is_none_or(|finished| now - finished < DAY_MS)
            })
            .map(|run| {
                let raw = run["status"].as_str().unwrap_or("");
                let status = status_of(raw);
                let active = status == "queued" || status == "running";
                let session = run["session_id"].as_str().unwrap_or("");
                json!({
                    "id": format!("agent:{}:{session}:{}", project["id"].as_str().unwrap_or(""), run["run_id"].as_str().unwrap_or("")),
                    "kind": "agent",
                    "title": titles.get(session).cloned().unwrap_or_else(|| "Чат".into()),
                    "projectKey": project["path"],
                    "projectName": label(project),
                    "workspaceId": project["id"],
                    "sessionId": run["session_id"],
                    "status": if active { attention.get(session).copied().map(|kind| json!(kind)).unwrap_or(json!(status)) } else { json!(status) },
                    "attention": active.then(|| attention.get(session).copied()).flatten(),
                    "startedAt": time(&run["accepted_at"]),
                    "finishedAt": time(&run["finished_at"]),
                    "error": run["error"].as_str().filter(|error| !error.is_empty()),
                    "stoppable": active,
                })
            })
            .collect(),
    )
}

fn automation_jobs(state: &AppState, projects: &[Value]) -> Vec<Value> {
    let activity = state.automations.activity();
    let Some(runs) = activity.as_array() else {
        return Vec::new();
    };
    let now = now_ms();
    runs.iter()
        .filter(|run| {
            time(&run["finished_at"]).is_none_or(|finished| now - finished < DAY_MS)
        })
        .map(|run| {
            let root = run["root"].as_str().unwrap_or("");
            let project = project_by_key(projects, root);
            json!({
                "id": format!("automation:{root}:{}", run["id"].as_str().unwrap_or("")),
                "kind": "automation",
                "title": run["title"],
                "projectKey": root,
                "projectName": project.as_ref().map(label).unwrap_or_else(|| root.rsplit(['\\','/']).find(|part| !part.is_empty()).unwrap_or(root).to_owned()),
                "workspaceId": project.as_ref().and_then(|project| project["id"].as_str().map(str::to_owned)),
                "sessionId": run["session_id"],
                "status": if run["status"] == "claimed" { json!("queued") } else { run["status"].clone() },
                "startedAt": run["created_at"],
                "finishedAt": run["finished_at"],
                "error": run["error"],
                "stoppable": matches!(run["status"].as_str(), Some("claimed" | "running")),
            })
        })
        .collect()
}

fn team_jobs(state: &AppState, projects: &[Value]) -> Vec<Value> {
    let now = now_ms();
    state
        .teams
        .all()
        .iter()
        .flat_map(|team| {
            let project = project_by_key(projects, &team.root);
            team.tasks.iter().filter_map(move |task| {
                let finished_at = task.finished_at?;
                if now - finished_at > DAY_MS {
                    return None;
                }
                Some(json!({
                    "id": format!("team:{}:{}:{}", team.root, team.id, task.id),
                    "kind": "team",
                    "title": format!("{}: {}", team.title, task.title),
                    "projectKey": team.root,
                    "projectName": project.as_ref().map(label).unwrap_or_else(|| team.root.clone()),
                    "workspaceId": project.as_ref().and_then(|project| project["id"].as_str().map(str::to_owned)),
                    "sessionId": task.session_id,
                    "status": if task.status == TeamTaskStatus::Starting { json!("running") } else { json!(task.status.as_str()) },
                    "startedAt": task.started_at,
                    "finishedAt": task.finished_at,
                    "error": task.error,
                    "stoppable": !task.status.finished(),
                }))
            })
        })
        .collect()
}

fn launch_jobs(state: &AppState, projects: &[Value]) -> Vec<Value> {
    state
        .launches
        .all()
        .as_array()
        .map(|launches| {
            launches
                .iter()
                .map(|app| {
                    let project = project_by_workspace(projects, app["workspaceId"].as_str().unwrap_or(""));
                    json!({
                        "id": format!("app:{}", app["configId"].as_str().unwrap_or("")),
                        "kind": "app",
                        "title": app["name"],
                        "projectKey": app["key"],
                        "projectName": project.map(label).unwrap_or_else(|| app["key"].as_str().unwrap_or("").to_owned()),
                        "workspaceId": app["workspaceId"],
                        "status": if app["status"] == "running" { json!("running") } else if app["exitCode"].is_null() { json!("completed") } else { json!("failed") },
                        "startedAt": app["startedAt"],
                        "detail": app["url"],
                        "error": (app["status"] == "exited" && !app["exitCode"].is_null())
                            .then(|| json!(format!("exit {}", app["exitCode"]))),
                        "stoppable": app["status"] == "running",
                    })
                })
                .collect()
        })
        .unwrap_or_default()
}

fn terminal_jobs(state: &AppState, projects: &[Value]) -> Vec<Value> {
    let launch_terminals: Vec<String> = state
        .launches
        .all()
        .as_array()
        .map(|launches| {
            launches
                .iter()
                .filter_map(|app| app["terminalId"].as_str().map(str::to_owned))
                .collect()
        })
        .unwrap_or_default();
    state
        .terminals
        .snapshots()
        .into_iter()
        .filter(|terminal| {
            !launch_terminals.contains(&terminal.id) && terminal.status != "closed"
        })
        .map(|terminal| {
            let project = project_by_workspace(projects, &terminal.workspace_id);
            json!({
                "id": format!("terminal:{}:{}", terminal.workspace_id, terminal.id),
                "kind": "terminal",
                "title": terminal.title,
                "projectKey": project.and_then(|project| project["path"].as_str().map(str::to_owned)).unwrap_or_default(),
                "projectName": project.map(label).unwrap_or_default(),
                "workspaceId": terminal.workspace_id,
                "status": if terminal.status == "running" { json!("running") } else if terminal.exit_code.is_none() { json!("completed") } else { json!("failed") },
                "startedAt": terminal.created_at,
                "detail": (terminal.owner == "agent").then(|| json!("agent")),
                "stoppable": terminal.status == "running",
            })
        })
        .collect()
}

fn list(app: &tauri::AppHandle, state: &AppState, center: &JobCenter) -> HostResult<Value> {
    let projects = projects(state);
    let owned = [
        state
            .automations
            .activity()
            .as_array()
            .map(|runs| {
                runs.iter()
                    .filter_map(|run| run["id"].as_str().map(str::to_owned))
                    .collect::<Vec<_>>()
            })
            .unwrap_or_default(),
        state.teams.owned_run_ids(),
    ]
    .concat();
    let mut jobs = Vec::new();
    jobs.extend(agent_jobs(state, center, &projects, &owned)?);
    jobs.extend(automation_jobs(state, &projects));
    jobs.extend(team_jobs(state, &projects));
    jobs.extend(launch_jobs(state, &projects));
    jobs.extend(terminal_jobs(state, &projects));
    let _ = app;
    jobs.sort_by_key(|job| std::cmp::Reverse(time(&job["startedAt"]).unwrap_or(0)));
    Ok(json!(jobs))
}

fn stop(app: &tauri::AppHandle, state: &AppState, id: &str) -> HostResult<()> {
    let mut parts = id.split(':');
    let kind = parts.next().unwrap_or("");
    let rest: Vec<&str> = parts.collect();
    match kind {
        "agent" => {
            let [workspace, session] = rest.as_slice() else {
                return Err(unknown_job());
            };
            state.require_engine()?.request(
                "POST",
                &format!("/v1/workspaces/{workspace}/agent/sessions/{session}/cancel"),
                None,
            )?;
        }
        "automation" => {
            if rest.len() < 2 {
                return Err(unknown_job());
            }
            let run_id = rest.last().unwrap().to_owned();
            let root = rest[..rest.len() - 1].join(":");
            state.automations.handle(
                app.clone(),
                &json!({"action":"cancel","root":root,"runId":run_id}),
            )?;
        }
        "team" => {
            if rest.len() < 3 {
                return Err(unknown_job());
            }
            let task_id = rest.last().unwrap().to_owned();
            let team_id = rest[rest.len() - 2].to_owned();
            let root = rest[..rest.len() - 2].join(":");
            state.teams.handle(
                &root,
                &json!({"action":"stopTask","teamId":team_id,"taskId":task_id}),
            )?;
        }
        "app" => {
            let [config] = rest.as_slice() else {
                return Err(unknown_job());
            };
            state.launches.stop_config(&state.terminals, config)?;
        }
        "terminal" => {
            let [workspace, terminal] = rest.as_slice() else {
                return Err(unknown_job());
            };
            match state.terminals.close_one(workspace, terminal) {
                Ok(()) => {}
                Err(error) if error.code == "TERMINAL_NOT_FOUND" => {}
                Err(error) => return Err(error),
            }
        }
        _ => return Err(unknown_job()),
    }
    Ok(())
}

fn unknown_job() -> HostError {
    HostError::new("JOB_NOT_FOUND", "Неизвестная задача")
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn statuses_map_like_the_original_job_center() {
        assert_eq!(status_of("accepted"), "queued");
        assert_eq!(status_of("queued"), "queued");
        assert_eq!(status_of("running"), "running");
        assert_eq!(status_of("completed"), "completed");
        assert_eq!(status_of("failed"), "failed");
        assert_eq!(status_of("interrupted"), "interrupted");
        assert_eq!(status_of("unknown"), "running");
    }

    #[test]
    fn timestamps_accept_numbers_and_iso_strings() {
        assert_eq!(time(&json!(1700000000000i64)), Some(1700000000000));
        assert_eq!(time(&json!("2026-01-01T00:00:00Z")).is_some(), true);
        assert_eq!(time(&json!(null)), None);
    }

    #[test]
    fn labels_follow_the_original_naming() {
        assert_eq!(label(&json!({"kind":"chat"})), "Личные чаты");
        assert_eq!(
            label(&json!({"kind":"project","path":"C:/work/app"})),
            "app"
        );
        assert_eq!(
            label(&json!({"kind":"project","remote":{"name":"Server","root":"/srv/app"}})),
            "Server · app"
        );
    }

    #[test]
    fn owned_agent_runs_are_filtered_by_run_id() {
        let jobs = vec![
            json!([{"id":"agent:ws:session:run1"}]),
            json!([{"id":"agent:ws:session:run2"}]),
        ];
        let filtered = filter_owned(jobs, &["run1".to_owned()]);
        assert_eq!(filtered.len(), 1);
        assert_eq!(filtered[0]["id"], "agent:ws:session:run2");
    }

    #[test]
    fn project_keys_match_case_insensitive_paths() {
        let projects = vec![json!({"id":"ws","path":"C:/Work/App"})];
        assert!(project_by_key(&projects, r"c:\work\app").is_some());
        assert!(project_by_key(&projects, "C:/other").is_none());
    }
}
