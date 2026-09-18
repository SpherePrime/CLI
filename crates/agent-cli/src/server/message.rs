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
    let working_dir = std::env::current_dir().unwrap_or_default();

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
