use std::sync::Arc;

use http_body_util::{BodyExt, Full};
use hyper::body::{Bytes, Incoming};
use hyper::{header, Method, Request, Response, StatusCode};
use serde::Deserialize;

use super::fs;
use super::message;
use super::providers;
use super::session;
use crate::server::state::AppState;

pub type BoxBody = http_body_util::combinators::BoxBody<Bytes, std::convert::Infallible>;

const VERSION: &str = env!("CARGO_PKG_VERSION");

pub async fn route(
    request: Request<Incoming>,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    if request.method() == Method::OPTIONS {
        return Ok(Response::builder()
            .status(StatusCode::NO_CONTENT)
            .body(empty())
            .unwrap());
    }

    let path = request.uri().path().to_string();
    let query = request.uri().query().unwrap_or("").to_string();
    let method = request.method().clone();

    if method == Method::GET && path == "/" {
        let model = state.resolve_model();
        return Ok(json_response(&serde_json::json!({
            "name": "agent",
            "version": VERSION,
            "model": public_model(&model),
        })));
    }

    if method == Method::GET && path == "/config" {
        let config = state.config.read().unwrap().clone().unwrap_or_default();
        let mut value = serde_json::to_value(config).unwrap_or_default();
        redact_config(&mut value);
        return Ok(json_response(&value));
    }

    if method == Method::POST && path == "/config" {
        return handle_config(request, state).await;
    }

    if method == Method::GET && path == "/session" {
        return session::list(state);
    }

    if method == Method::POST && path == "/session" {
        return session::create(state).await;
    }

    if path.starts_with("/session/") {
        return handle_session_path(method, request, &path, state).await;
    }

    if method == Method::GET && path == "/providers" {
        return providers::list(state).await;
    }

    if method == Method::POST && path == "/provider" {
        return providers::connect(request, state).await;
    }

    if method == Method::GET && path == "/models" {
        return providers::models(state).await;
    }

    if method == Method::POST && path == "/model" {
        return providers::select_model(request, state).await;
    }

    if method == Method::POST && path == "/favorite" {
        return providers::favorite(request, state).await;
    }

    if method == Method::POST && path == "/message" {
        return message::post(request, state).await;
    }

    if method == Method::POST && path == "/cancel" {
        return handle_cancel(request, state).await;
    }

    if method == Method::POST && path == "/permission" {
        return handle_permission(request, state).await;
    }

    if method == Method::GET && path == "/tools" {
        let tools = agent_tools::builtin::register_builtin(agent_tools::ToolRegistry::new());
        let schemas: Vec<serde_json::Value> = tools
            .schemas()
            .iter()
            .map(|schema| serde_json::to_value(schema).unwrap_or_default())
            .collect();
        return Ok(json_response(&serde_json::json!({ "data": schemas })));
    }

    if method == Method::GET && path == "/fs/find" {
        return fs::find(&query);
    }

    Ok(Response::builder()
        .status(StatusCode::NOT_FOUND)
        .body(err_body("not found"))
        .unwrap())
}

async fn handle_session_path(
    method: Method,
    request: Request<Incoming>,
    path: &str,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let mut segments = path["/session/".len()..].splitn(2, '/');
    let id_segment = segments.next().unwrap_or_default();
    let action = segments.next().unwrap_or_default();
    let id = match uuid::Uuid::parse_str(id_segment) {
        Ok(id) => id,
        Err(_) => {
            return Ok(Response::builder()
                .status(StatusCode::BAD_REQUEST)
                .body(err_body("invalid session id"))
                .unwrap());
        }
    };

    match (method, action) {
        (Method::GET, "") => session::get(id, state),
        (Method::POST, "rename") => {
            let body = match read_json::<RenameBody>(request).await {
                Ok(body) => body,
                Err(response) => return Ok(response),
            };
            session::rename(id, state, body.title)
        }
        (Method::POST, "fork") => session::fork(id, state),
        (Method::POST, "compact") => session::compact(id, state),
        (Method::POST, "delete") | (Method::DELETE, "") => session::delete(id, state),
        _ => Ok(Response::builder()
            .status(StatusCode::METHOD_NOT_ALLOWED)
            .body(err_body("method not allowed"))
            .unwrap()),
    }
}

#[derive(Deserialize)]
struct RenameBody {
    title: String,
}

#[derive(Deserialize)]
struct CancelBody {
    session_id: uuid::Uuid,
}

#[derive(Deserialize)]
struct PermissionBody {
    session_id: uuid::Uuid,
    id: uuid::Uuid,
    decision: String,
    #[serde(default)]
    remember: bool,
}

async fn handle_cancel(
    request: Request<Incoming>,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let body = match read_json::<CancelBody>(request).await {
        Ok(body) => body,
        Err(response) => return Ok(response),
    };
    let cancelled = state.cancel_session(&body.session_id);
    Ok(json_response(
        &serde_json::json!({ "cancelled": cancelled }),
    ))
}

async fn handle_permission(
    request: Request<Incoming>,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let body = match read_json::<PermissionBody>(request).await {
        Ok(body) => body,
        Err(response) => return Ok(response),
    };
    let decision = match body.decision.as_str() {
        "allow" | "once" => agent_permissions::PermissionDecision::Allow,
        "deny" | "reject" => agent_permissions::PermissionDecision::Deny,
        _ => {
            return Ok(Response::builder()
                .status(StatusCode::BAD_REQUEST)
                .body(err_body("decision must be allow or deny"))
                .unwrap());
        }
    };
    let resolved = state
        .resolve_permission(&body.session_id, body.id, decision, body.remember)
        .await;
    Ok(json_response(&serde_json::json!({ "resolved": resolved })))
}

async fn handle_config(
    request: Request<Incoming>,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let body = match read_json::<serde_json::Value>(request).await {
        Ok(body) => body,
        Err(_) => {
            return Ok(Response::builder()
                .status(StatusCode::BAD_REQUEST)
                .body(err_body("invalid config"))
                .unwrap());
        }
    };

    let mut config = state.config.read().unwrap().clone().unwrap_or_default();
    if let Some(model) = body.get("model") {
        if let Ok(parsed) = serde_json::from_value::<agent_config::ModelConfig>(model.clone()) {
            config.model = Some(parsed.clone());
            *state.model.write().unwrap() = Some(parsed);
        }
    }
    let _ = agent_config::ConfigLoader::save_global(&config);
    *state.config.write().unwrap() = Some(config.clone());

    let mut body = serde_json::to_value(config.clone()).unwrap_or_default();
    redact_config(&mut body);

    Ok(json_response(&body))
}

fn public_model(model: &agent_config::ModelConfig) -> serde_json::Value {
    serde_json::json!({
        "provider": model.provider,
        "model": model.model,
        "base_url": model.base_url,
        "temperature": model.temperature,
        "max_tokens": model.max_tokens,
    })
}

fn redact_config(value: &mut serde_json::Value) {
    if let Some(map) = value.as_object_mut() {
        if let Some(model) = map.get_mut("model") {
            if let Some(model_map) = model.as_object_mut() {
                model_map.remove("api_key_env");
            }
        }
    }
}

#[derive(Deserialize)]
pub struct MessageBody {
    pub session_id: Option<uuid::Uuid>,
    pub text: String,
}

#[allow(clippy::result_large_err)]
pub async fn read_body(request: Request<Incoming>) -> Result<Bytes, Response<BoxBody>> {
    let collected = match request.into_body().collect().await {
        Ok(collected) => collected,
        Err(_) => {
            return Err(Response::builder()
                .status(StatusCode::BAD_REQUEST)
                .body(err_body("failed to read body"))
                .unwrap());
        }
    };
    Ok(collected.to_bytes())
}

#[allow(clippy::result_large_err)]
pub async fn read_json<T: for<'de> Deserialize<'de>>(
    request: Request<Incoming>,
) -> Result<T, Response<BoxBody>> {
    let bytes = read_body(request).await?;
    match serde_json::from_slice(&bytes) {
        Ok(value) => Ok(value),
        Err(_) => Err(Response::builder()
            .status(StatusCode::BAD_REQUEST)
            .body(err_body("invalid json body"))
            .unwrap()),
    }
}

pub fn json_response(value: &serde_json::Value) -> Response<BoxBody> {
    let bytes = serde_json::to_vec(value).unwrap_or_default();
    Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, "application/json")
        .body(
            Full::new(Bytes::from(bytes))
                .map_err(|never| match never {})
                .boxed(),
        )
        .unwrap()
}

pub fn empty() -> BoxBody {
    Full::new(Bytes::new())
        .map_err(|never| match never {})
        .boxed()
}

pub fn err_body(message: &str) -> BoxBody {
    let bytes = serde_json::to_vec(&serde_json::json!({ "error": message })).unwrap_or_default();
    Full::new(Bytes::from(bytes))
        .map_err(|never| match never {})
        .boxed()
}
