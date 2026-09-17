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
            .header("Access-Control-Allow-Origin", "*")
            .header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            .header("Access-Control-Allow-Headers", "content-type")
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
        return handle_session_path(method, &path, state).await;
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
    path: &str,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let rest = &path["/session/".len()..];
    let id = match uuid::Uuid::parse_str(rest) {
        Ok(id) => id,
        Err(_) => {
            return Ok(Response::builder()
                .status(StatusCode::BAD_REQUEST)
                .body(err_body("invalid session id"))
                .unwrap());
        }
    };

    if method == Method::GET {
        return session::get(id, state);
    }

    Ok(Response::builder()
        .status(StatusCode::METHOD_NOT_ALLOWED)
        .body(err_body("method not allowed"))
        .unwrap())
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

pub async fn read_body(
    request: Request<Incoming>,
) -> Result<Bytes, Response<BoxBody>> {
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
        .body(Full::new(Bytes::from(bytes))
            .map_err(|never| match never {})
            .boxed())
        .unwrap()
}

pub fn json_response_ok<T: serde::Serialize>(value: &T) -> Response<BoxBody> {
    json_response(&serde_json::to_value(value).unwrap_or_default())
}

pub fn ndjson_response() -> hyper::http::response::Builder {
    Response::builder()
        .header(header::CONTENT_TYPE, "application/x-ndjson")
        .header("Cache-Control", "no-cache")
}

pub fn empty() -> BoxBody {
    Full::new(Bytes::new()).map_err(|never| match never {}).boxed()
}

pub fn not_found() -> Response<BoxBody> {
    Response::builder()
        .status(StatusCode::NOT_FOUND)
        .body(err_body("not found"))
        .unwrap()
}

pub fn err_body(message: &str) -> BoxBody {
    let bytes = serde_json::to_vec(&serde_json::json!({ "error": message })).unwrap_or_default();
    Full::new(Bytes::from(bytes))
        .map_err(|never| match never {})
        .boxed()
}

pub fn into_infallible(
    response: Response<BoxBody>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    Ok(response)
}