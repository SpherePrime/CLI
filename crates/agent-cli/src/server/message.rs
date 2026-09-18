use std::sync::Arc;
use std::time::Duration;

use agent_core::{AgentEngine, EngineEvent};
use futures::StreamExt;
use http_body_util::{BodyExt, StreamBody};
use hyper::body::{Bytes, Frame, Incoming};
use hyper::{Request, Response, StatusCode};
use tokio::sync::{mpsc, oneshot};
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
    if state.is_session_busy(&session_id) {
        return Ok(stream_error("a turn is already running for this session"));
    }
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
    state.register_engine(
        session_id,
        engine.cancel_handle(),
        engine.approver.clone(),
        engine.permission_engine(),
        engine_tx.clone(),
        engine.clock().clone(),
    );
    let run_tx = engine_tx.clone();
    drop(engine_tx);

    let run_text = body.text.clone();
    let engine_state = state.clone();
    let fallback_clock = engine.clock().clone();
    tokio::spawn(async move {
        if let Err(error) = engine.run(&run_text, &run_tx).await {
            let meta = fallback_clock.meta("server_fallback");
            let _ = run_tx
                .send(EngineEvent::Error {
                    meta,
                    message: error.to_string(),
                })
                .await;
        }
        engine_state.unregister_engine(&session_id);
    });
    let text = body.text.clone();

    let (tx, rx) = mpsc::channel::<Bytes>(256);
    let forward = tx.clone();
    let forward_state = state.clone();
    let disconnect_state = state.clone();
    let title_session_id = session_id;
    let first_user_text = text.clone();
    let title_text = text.clone();
    let (title_tx, title_rx) = oneshot::channel::<String>();
    tokio::spawn(async move {
        let Some(title) = maybe_generate_title(forward_state, title_session_id, &title_text).await
        else {
            return;
        };
        let _ = title_tx.send(title);
    });
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
                "message": { "id": Uuid::new_v4(), "role": "user", "content": first_user_text }
            }),
        )
        .await;
        let mut title_rx = title_rx;
        let mut title_emitted = false;
        let mut engine_done = false;
        let mut disconnected = false;
        while !engine_done {
            tokio::select! {
                maybe_event = engine_rx.recv(), if !engine_done => {
                    match maybe_event {
                        Some(event) => {
                            let value = serde_json::to_value(&event).unwrap_or(serde_json::Value::Null);
                            emit(&forward, value).await;
                        }
                        None => engine_done = true,
                    }
                }
                maybe_title = &mut title_rx, if !title_emitted => {
                    match maybe_title {
                        Ok(title) => {
                            emit(
                                &forward,
                                serde_json::json!({ "type": "session_title_changed", "title": title }),
                            )
                            .await;
                            title_emitted = true;
                        }
                        Err(_) => title_emitted = true,
                    }
                }
                _ = forward.closed(), if !disconnected => {
                    disconnect_state.cancel_session(&session_id);
                    disconnected = true;
                    engine_done = true;
                }
            }
        }
        if disconnected {
            return;
        }
        if !title_emitted {
            if let Ok(Ok(title)) =
                tokio::time::timeout(Duration::from_millis(500), &mut title_rx).await
            {
                emit(
                    &forward,
                    serde_json::json!({ "type": "session_title_changed", "title": title }),
                )
                .await;
            }
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

async fn maybe_generate_title(
    state: Arc<AppState>,
    session_id: Uuid,
    text: &str,
) -> Option<String> {
    let storage = state.storage()?;
    let record = storage.read_session(&session_id).ok()?;
    let has_title = record
        .metadata
        .get("title")
        .and_then(|value| value.as_str())
        .map(|value| !value.is_empty())
        .unwrap_or(false);
    if has_title {
        return None;
    }
    let title = super::title::generate_title(state.clone(), text).await?;
    let storage = state.storage()?;
    let mut record = storage.read_session(&session_id).ok()?;
    record.metadata["title"] = serde_json::Value::String(title.clone());
    record.touch();
    let _ = storage.replace_session_meta(&record);
    Some(title)
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

#[cfg(test)]
mod tests {
    use super::*;
    use agent_config::{AgentConfig, ModelConfig, ProviderKind};
    use agent_sessions::SessionManager;

    fn mock_config() -> AgentConfig {
        AgentConfig {
            model: Some(ModelConfig {
                provider: ProviderKind::Mock,
                model: "mock-1".into(),
                base_url: None,
                api_key_env: None,
                temperature: None,
                max_tokens: Some(32),
            }),
            ..Default::default()
        }
    }

    #[test]
    fn busy_guard_tracks_registered_engines() {
        let runtime = tokio::runtime::Runtime::new().unwrap();
        runtime.block_on(async {
            let tmp = tempfile::tempdir().unwrap();
            let storage = agent_storage::Storage::new(tmp.path().to_path_buf());
            let state = Arc::new(AppState::new(storage, tmp.path().to_path_buf(), None).unwrap());
            let session_id = Uuid::new_v4();
            assert!(!state.is_session_busy(&session_id));

            let (tx, _rx) = mpsc::channel::<EngineEvent>(8);
            let engine =
                AgentEngine::new(mock_config(), tmp.path().to_path_buf(), tx.clone()).unwrap();
            state.register_engine(
                session_id,
                engine.cancel_handle(),
                engine.approver.clone(),
                engine.permission_engine(),
                tx,
                engine.clock().clone(),
            );
            assert!(state.is_session_busy(&session_id));

            state.unregister_engine(&session_id);
            assert!(!state.is_session_busy(&session_id));
        });
    }

    #[test]
    fn auto_title_persists_for_untitled_session() {
        let runtime = tokio::runtime::Runtime::new().unwrap();
        runtime.block_on(async {
            let tmp = tempfile::tempdir().unwrap();
            let storage = agent_storage::Storage::new(tmp.path().to_path_buf());
            let state =
                Arc::new(AppState::new(storage.clone(), tmp.path().to_path_buf(), None).unwrap());
            *state.config.write().unwrap() = Some(mock_config());

            let session_id = Uuid::new_v4();
            let mut record = agent_storage::SessionRecord::new(
                Some(tmp.path().to_path_buf()),
                Some("mock-1".into()),
            );
            record.id = session_id;
            record.metadata = serde_json::json!({ "title": serde_json::Value::Null });
            let _ = SessionManager::new(storage.clone(), session_id).save(&record, &[]);

            maybe_generate_title(state.clone(), session_id, "Add dark mode toggle")
                .await
                .expect("expected a generated title");

            let updated = storage.read_session(&session_id).unwrap();
            let title = updated
                .metadata
                .get("title")
                .and_then(|value| value.as_str())
                .unwrap_or("");
            assert!(
                title.contains("mock"),
                "expected generated title, got: {title}"
            );
            assert!(updated.updated_at >= record.updated_at);
        });
    }

    #[test]
    fn auto_title_skips_titled_session() {
        let runtime = tokio::runtime::Runtime::new().unwrap();
        runtime.block_on(async {
            let tmp = tempfile::tempdir().unwrap();
            let storage = agent_storage::Storage::new(tmp.path().to_path_buf());
            let state =
                Arc::new(AppState::new(storage.clone(), tmp.path().to_path_buf(), None).unwrap());
            *state.config.write().unwrap() = Some(mock_config());

            let session_id = Uuid::new_v4();
            let mut record = agent_storage::SessionRecord::new(
                Some(tmp.path().to_path_buf()),
                Some("mock-1".into()),
            );
            record.id = session_id;
            record.metadata = serde_json::json!({ "title": "My title" });
            let _ = SessionManager::new(storage.clone(), session_id).save(&record, &[]);

            let title = maybe_generate_title(state.clone(), session_id, "Another message").await;
            assert_eq!(title, None);

            let updated = storage.read_session(&session_id).unwrap();
            let title = updated
                .metadata
                .get("title")
                .and_then(|value| value.as_str())
                .unwrap_or("");
            assert_eq!(title, "My title");
        });
    }
}
