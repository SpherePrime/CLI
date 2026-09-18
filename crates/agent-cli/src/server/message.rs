use std::sync::Arc;

use agent_core::{AgentEngine, EngineEvent};
use futures::StreamExt;
use http_body_util::{BodyExt, StreamBody};
use hyper::body::{Bytes, Frame, Incoming};
use hyper::{Request, Response, StatusCode};
use tokio::sync::mpsc;
use tokio_stream::wrappers::ReceiverStream;
use uuid::Uuid;

use super::api::{read_json, BoxBody, MessageBody};
use super::state::AppState;

pub async fn post(
    request: Request<Incoming>,
    state: Arc<AppState>,
) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let body: MessageBody = match read_json(request).await {
        Ok(body) => body,
        Err(response) => return Ok(response),
    };

    let session_id = body.session_id.unwrap_or_else(Uuid::new_v4);
    let config = state.config.read().unwrap().clone().unwrap_or_default();
    let workspace = state.workspace.clone();

    let working_dir = match resolve_project_dir(&state, &body, &workspace) {
        ResolveDir::Dir(dir) => dir,
        ResolveDir::Stream(response) => return Ok(response),
    };

    let (engine_tx, mut engine_rx) = mpsc::channel::<EngineEvent>(256);
    let mut engine = match AgentEngine::new(config, working_dir, engine_tx.clone()) {
        Ok(engine) => engine.with_session(session_id),
        Err(error) => {
            return Ok(stream_error(&format!("engine init error: {error}")));
        }
    };
    state.register_engine(session_id, engine.cancel_handle(), engine.approver.clone());
    let run_tx = engine_tx.clone();
    drop(engine_tx);

    let run_text = body.text.clone();
    let engine_state = state.clone();
    tokio::spawn(async move {
        if let Err(error) = engine.run(&run_text, &run_tx).await {
            let _ = run_tx.send(EngineEvent::error(error.to_string())).await;
        }
        engine_state.unregister_engine(&session_id);
    });
    let text = body.text.clone();

    let (tx, rx) = mpsc::channel::<Bytes>(256);
    let forward = tx.clone();
    tokio::spawn(async move {
        emit(
            &forward,
            serde_json::json!({ "type": "session.created", "session": { "id": session_id } }),
        )
        .await;
        emit(
            &forward,
            serde_json::json!({
                "type": "message.created",
                "message": { "id": Uuid::new_v4(), "role": "user", "content": text }
            }),
        )
        .await;
        while let Some(event) = engine_rx.recv().await {
            let value = serde_json::to_value(&event).unwrap_or(serde_json::Value::Null);
            emit(&forward, value).await;
        }
        emit(&forward, serde_json::json!({ "type": "done" })).await;
    });

    let stream_body = StreamBody::new(
        ReceiverStream::new(rx)
            .map(|bytes| Ok::<Frame<Bytes>, std::convert::Infallible>(Frame::data(bytes))),
    );
    let boxed: BoxBody = BoxBody::new(stream_body);

    Ok(Response::builder()
        .status(StatusCode::OK)
        .header(hyper::header::CONTENT_TYPE, "application/x-ndjson")
        .header("Cache-Control", "no-cache")
        .body(boxed)
        .unwrap())
}

enum ResolveDir {
    Dir(std::path::PathBuf),
    Stream(Response<BoxBody>),
}

fn resolve_project_dir(
    state: &AppState,
    body: &MessageBody,
    workspace: &std::path::Path,
) -> ResolveDir {
    let Some(storage) = state.storage() else {
        return ResolveDir::Stream(stream_error("storage unavailable"));
    };
    let Some(session_id) = body.session_id else {
        return ResolveDir::Dir(workspace.to_path_buf());
    };
    let record = match storage.read_session(&session_id) {
        Ok(record) => record,
        Err(_) => {
            return ResolveDir::Stream(stream_error("session not found"));
        }
    };
    match record.project_path.as_deref().filter(|p| p.exists()) {
        Some(path) => ResolveDir::Dir(path.to_path_buf()),
        None if body.readonly => ResolveDir::Dir(workspace.to_path_buf()),
        None => {
            let name = record.project_name.as_deref().unwrap_or("unknown project");
            let missing = serde_json::json!({
                "type": "session.project_missing",
                "session": { "id": session_id, "project_name": name },
                "project_path": record.project_path,
            });
            let done = serde_json::json!({ "type": "done" });
            let payload = format!(
                "{}\n{}\n",
                serde_json::to_string(&missing).unwrap_or_default(),
                serde_json::to_string(&done).unwrap_or_default()
            );
            ResolveDir::Stream(
                Response::builder()
                    .status(StatusCode::OK)
                    .header(hyper::header::CONTENT_TYPE, "application/x-ndjson")
                    .body(
                        http_body_util::Full::new(Bytes::from(payload))
                            .map_err(|never| match never {})
                            .boxed(),
                    )
                    .unwrap(),
            )
        }
    }
}

async fn emit(tx: &mpsc::Sender<Bytes>, value: serde_json::Value) {
    let mut bytes = serde_json::to_vec(&value).unwrap_or_default();
    bytes.push(b'\n');
    let _ = tx.send(Bytes::from(bytes)).await;
}

fn stream_error(message: &str) -> Response<BoxBody> {
    let payload = format!(
        "{}\n{}\n",
        serde_json::json!({ "type": "error", "message": message }),
        serde_json::json!({ "type": "done" })
    );
    Response::builder()
        .status(StatusCode::OK)
        .header(hyper::header::CONTENT_TYPE, "application/x-ndjson")
        .body(
            http_body_util::Full::new(Bytes::from(payload))
                .map_err(|never| match never {})
                .boxed(),
        )
        .unwrap()
}
