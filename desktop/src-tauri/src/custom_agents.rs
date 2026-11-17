use crate::{
    error::{HostError, HostResult},
    state::AppState,
};
use serde_json::{json, Value};

fn urlencoding(value: &str) -> String {
    url::form_urlencoded::byte_serialize(value.as_bytes()).collect()
}

pub fn validate_segment<'a>(value: Option<&'a str>, field: &str) -> HostResult<&'a str> {
    let segment = value.ok_or_else(|| {
        HostError::new("INVALID_ARGUMENT", format!("Missing {field}"))
    })?;
    if segment.is_empty()
        || segment != segment.trim()
        || segment.contains('/')
        || segment.contains('\\')
        || segment.contains("..")
        || segment.contains('?')
        || segment.contains('#')
        || segment.contains('%')
        || segment.chars().any(char::is_control)
    {
        return Err(HostError::new(
            "INVALID_ARGUMENT",
            format!("Invalid {field}"),
        ));
    }
    Ok(segment)
}

pub fn build_route_and_body(
    request: &Value,
) -> HostResult<(&'static str, String, Option<Value>)> {
    let action = request
        .get("action")
        .and_then(Value::as_str)
        .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing customAgents action"))?;
    match action {
        "list" => Ok(("GET", "/v1/custom-agents".to_string(), None)),
        "get" => {
            let agent_id = validate_segment(
                request.get("agentId").and_then(Value::as_str),
                "agentId",
            )?;
            Ok(("GET", format!("/v1/custom-agents/{agent_id}"), None))
        }
        "create" => {
            let input = request
                .get("input")
                .cloned()
                .unwrap_or_else(|| json!({}));
            Ok(("POST", "/v1/custom-agents".to_string(), Some(input)))
        }
        "update" => {
            let agent_id = validate_segment(
                request.get("agentId").and_then(Value::as_str),
                "agentId",
            )?;
            let patch = request
                .get("patch")
                .cloned()
                .unwrap_or_else(|| json!({}));
            Ok(("PATCH", format!("/v1/custom-agents/{agent_id}"), Some(patch)))
        }
        "delete" => {
            let agent_id = validate_segment(
                request.get("agentId").and_then(Value::as_str),
                "agentId",
            )?;
            Ok(("DELETE", format!("/v1/custom-agents/{agent_id}"), None))
        }
        "run" => {
            let agent_id = validate_segment(
                request.get("agentId").and_then(Value::as_str),
                "agentId",
            )?;
            let mut body = serde_json::Map::new();
            body.insert(
                "prompt".to_string(),
                json!(request.get("prompt").and_then(Value::as_str).unwrap_or("")),
            );
            if let Some(reason) = request.get("triggerReason").and_then(Value::as_str) {
                body.insert("trigger_reason".to_string(), json!(reason));
            }
            if let Some(chain_id) = request.get("chainId").and_then(Value::as_str) {
                body.insert("chain_id".to_string(), json!(chain_id));
            }
            if let Some(force) = request.get("force").and_then(Value::as_bool) {
                body.insert("force".to_string(), json!(force));
            }
            if let Some(sync) = request.get("sync").and_then(Value::as_bool) {
                body.insert("sync".to_string(), json!(sync));
            }
            Ok((
                "POST",
                format!("/v1/custom-agents/{agent_id}/run"),
                Some(Value::Object(body)),
            ))
        }
        "cancelRun" => {
            let agent_id = validate_segment(
                request.get("agentId").and_then(Value::as_str),
                "agentId",
            )?;
            let run_id = validate_segment(
                request.get("runId").and_then(Value::as_str),
                "runId",
            )?;
            Ok((
                "POST",
                format!("/v1/custom-agents/{agent_id}/runs/{run_id}/cancel"),
                None,
            ))
        }
        "probe" => {
            let agent_id = validate_segment(
                request.get("agentId").and_then(Value::as_str),
                "agentId",
            )?;
            Ok(("POST", format!("/v1/custom-agents/{agent_id}/probe"), None))
        }
        "runs" => {
            let base = match request
                .get("agentId")
                .and_then(Value::as_str)
                .filter(|value| !value.is_empty())
            {
                Some(raw) => {
                    let agent_id = validate_segment(Some(raw), "agentId")?;
                    format!("/v1/custom-agents/{agent_id}/runs")
                }
                None => "/v1/custom-agents/runs".to_string(),
            };
            let route = match request.get("limit").and_then(Value::as_u64) {
                Some(limit) => format!("{base}?limit={limit}"),
                None => base,
            };
            Ok(("GET", route, None))
        }
        "blackboard" => {
            let mut params = Vec::new();
            if let Some(chain_id) = request
                .get("chainId")
                .and_then(Value::as_str)
                .filter(|value| !value.is_empty())
            {
                params.push(format!("chain_id={}", urlencoding(chain_id)));
            }
            if let Some(limit) = request.get("limit").and_then(Value::as_u64) {
                params.push(format!("limit={limit}"));
            }
            let route = if params.is_empty() {
                "/v1/custom-agents/blackboard".to_string()
            } else {
                format!("/v1/custom-agents/blackboard?{}", params.join("&"))
            };
            Ok(("GET", route, None))
        }
        "postBlackboard" => {
            let input = request
                .get("input")
                .cloned()
                .unwrap_or_else(|| json!({}));
            Ok(("POST", "/v1/custom-agents/blackboard".to_string(), Some(input)))
        }
        "models" => Ok(("GET", "/v1/custom-agents/models".to_string(), None)),
        "getGlobalMemory" => Ok(("GET", "/v1/custom-agents/global-memory".to_string(), None)),
        "saveGlobalMemory" => {
            let input = request
                .get("input")
                .or_else(|| request.get("doc"))
                .cloned()
                .unwrap_or_else(|| json!({}));
            Ok((
                "PUT",
                "/v1/custom-agents/global-memory".to_string(),
                Some(input),
            ))
        }
        "dispatchSwarm" => {
            let input = request
                .get("input")
                .or_else(|| request.get("request"))
                .cloned()
                .unwrap_or_else(|| json!({}));
            Ok(("POST", "/v1/custom-agents/swarm".to_string(), Some(input)))
        }
        _ => Err(HostError::new(
            "INVALID_ARGUMENT",
            format!("Unknown customAgents action: {action}"),
        )),
    }
}

pub fn handle(state: &AppState, request: &Value) -> HostResult<Value> {
    let (method, route, body) = build_route_and_body(request)?;
    state.require_engine()?.request(method, &route, body)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn validates_path_segments() {
        assert_eq!(validate_segment(Some("agent-1"), "agentId").unwrap(), "agent-1");
        assert!(validate_segment(None, "agentId").is_err());
        assert!(validate_segment(Some(""), "agentId").is_err());
        assert!(validate_segment(Some("../etc"), "agentId").is_err());
        assert!(validate_segment(Some("a/b"), "agentId").is_err());
        assert!(validate_segment(Some("a\\b"), "agentId").is_err());
        assert!(validate_segment(Some("a?b"), "agentId").is_err());
    }

    #[test]
    fn maps_custom_agents_actions_to_engine_routes() {
        let (method, route, body) = build_route_and_body(&json!({"action": "list"})).unwrap();
        assert_eq!((method, route.as_str(), body), ("GET", "/v1/custom-agents", None));

        let (method, route, body) = build_route_and_body(&json!({
            "action": "run",
            "agentId": "a1",
            "prompt": "check",
            "force": true,
            "sync": true
        }))
        .unwrap();
        assert_eq!(method, "POST");
        assert_eq!(route, "/v1/custom-agents/a1/run");
        assert_eq!(
            body.unwrap(),
            json!({"prompt": "check", "force": true, "sync": true})
        );

        let (method, route, _) = build_route_and_body(&json!({
            "action": "cancelRun",
            "agentId": "a1",
            "runId": "r2"
        }))
        .unwrap();
        assert_eq!((method, route.as_str()), ("POST", "/v1/custom-agents/a1/runs/r2/cancel"));

        let (method, route, _) = build_route_and_body(&json!({
            "action": "blackboard",
            "chainId": "c 1",
            "limit": 25
        }))
        .unwrap();
        assert_eq!(
            (method, route.as_str()),
            ("GET", "/v1/custom-agents/blackboard?chain_id=c+1&limit=25")
        );

        let (method, route, _) = build_route_and_body(&json!({"action": "models"})).unwrap();
        assert_eq!((method, route.as_str()), ("GET", "/v1/custom-agents/models"));

        let (method, route, body) =
            build_route_and_body(&json!({"action": "getGlobalMemory"})).unwrap();
        assert_eq!(
            (method, route.as_str(), body),
            ("GET", "/v1/custom-agents/global-memory", None)
        );

        let (method, route, body) = build_route_and_body(&json!({
            "action": "saveGlobalMemory",
            "input": {"memory": "m", "user_profile": "u", "soul": "s"}
        }))
        .unwrap();
        assert_eq!(method, "PUT");
        assert_eq!(route, "/v1/custom-agents/global-memory");
        assert_eq!(
            body.unwrap(),
            json!({"memory": "m", "user_profile": "u", "soul": "s"})
        );

        let (method, route, body) = build_route_and_body(&json!({
            "action": "dispatchSwarm",
            "input": {"goal": "audit", "workers": [{"agent_id": "a1", "task": "check"}]}
        }))
        .unwrap();
        assert_eq!(method, "POST");
        assert_eq!(route, "/v1/custom-agents/swarm");
        assert_eq!(
            body.unwrap(),
            json!({"goal": "audit", "workers": [{"agent_id": "a1", "task": "check"}]})
        );
    }
}
