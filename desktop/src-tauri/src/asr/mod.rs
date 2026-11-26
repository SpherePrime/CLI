pub mod assets;
pub mod process;
pub mod protocol;
#[cfg(test)]
mod tests;

use crate::asr::assets::{AsrAssets, ensure_assets};
use crate::asr::process::AsrProcess;
use crate::asr::protocol::pb::asr_service_client::AsrServiceClient;
use crate::asr::protocol::pb::{
    AudioChunk, CancelSession, FinishSession, RecognizeRequest, ShutdownRequest, StartSession,
    StatusRequest, TranscribeRequest, recognize_event, recognize_request,
};
use crate::asr::protocol::with_token;
use crate::error::{HostError, HostResult};
use crate::state::AppState;
use base64::Engine as _;
use serde::Serialize;
use serde_json::{Value, json};
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::time::Duration;
use tauri::{AppHandle, Manager};
use tokio::sync::{mpsc, oneshot};
use tokio_stream::StreamExt;
use tokio_stream::wrappers::UnboundedReceiverStream;
use tonic::transport::Channel;
use uuid::Uuid;

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct AsrStatusSnapshot {
    pub state: String,
    pub ready: bool,
    pub languages: Vec<String>,
    pub chunk_ms: u32,
    pub progress: Option<f32>,
    pub message: Option<String>,
}

impl Default for AsrStatusSnapshot {
    fn default() -> Self {
        Self {
            state: "idle".to_string(),
            ready: false,
            languages: vec!["en".to_string(), "ru".to_string(), "uk".to_string()],
            chunk_ms: 320,
            progress: None,
            message: None,
        }
    }
}

struct ActiveSession {
    session_id: String,
    request_tx: mpsc::UnboundedSender<RecognizeRequest>,
    final_rx: Option<oneshot::Receiver<Result<String, String>>>,
}

struct ServiceInner {
    status: AsrStatusSnapshot,
    process: Option<AsrProcess>,
    channel: Option<Channel>,
    token: Option<String>,
    active_session: Option<ActiveSession>,
    warming: bool,
    shutting_down: bool,
}

pub struct AsrService {
    inner: Arc<Mutex<ServiceInner>>,
}

impl AsrService {
    pub fn new() -> Arc<Self> {
        Arc::new(Self {
            inner: Arc::new(Mutex::new(ServiceInner {
                status: AsrStatusSnapshot::default(),
                process: None,
                channel: None,
                token: None,
                active_session: None,
                warming: false,
                shutting_down: false,
            })),
        })
    }

    pub fn warmup(self: &Arc<Self>, app: AppHandle) {
        {
            let mut inner = self.inner.lock().unwrap();
            if inner.shutting_down || inner.warming {
                return;
            }
            if let Some(proc) = inner.process.as_mut() {
                if proc.is_running() && inner.channel.is_some() {
                    return;
                }
            }
            inner.warming = true;
            inner.status.state = "loading".to_string();
            inner.status.ready = false;
            inner.status.message = Some("Загрузка модели распознавания речи…".to_string());
        }
        self.emit_status(&app);

        let service = Arc::clone(self);
        std::thread::spawn(move || {
            let result = service.ensure_connected_blocking(&app);
            if let Err(err) = result {
                crate::logging::error(&format!("ASR warmup failed: {err}"));
                let mut inner = service.inner.lock().unwrap();
                inner.warming = false;
                if !inner.shutting_down {
                    inner.status.state = "error".to_string();
                    inner.status.ready = false;
                    inner.status.progress = None;
                    inner.status.message = Some(err);
                }
                drop(inner);
                service.emit_status(&app);
            } else {
                let mut inner = service.inner.lock().unwrap();
                inner.warming = false;
                drop(inner);
                service.emit_status(&app);
            }
        });
    }

    pub fn handle(self: &Arc<Self>, app: AppHandle, payload: &Value) -> HostResult<Value> {
        let action = payload
            .get("action")
            .and_then(Value::as_str)
            .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing ASR action"))?;

        match action {
            "status" => Ok(json!(self.status_snapshot())),
            "warmup" => {
                self.warmup(app);
                Ok(json!(self.status_snapshot()))
            }
            "start" => {
                let language = payload
                    .get("language")
                    .and_then(Value::as_str)
                    .unwrap_or("auto")
                    .to_string();
                let start_language = payload
                    .get("startLanguage")
                    .and_then(Value::as_str)
                    .unwrap_or("")
                    .to_string();
                let session_id = self.start_session(&app, language, start_language)?;
                Ok(json!({ "sessionId": session_id, "ready": true }))
            }
            "push" => {
                let session_id = payload
                    .get("sessionId")
                    .and_then(Value::as_str)
                    .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing sessionId"))?;
                let pcm_b64 = payload
                    .get("pcmBase64")
                    .and_then(Value::as_str)
                    .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing pcmBase64"))?;
                let pcm = base64::engine::general_purpose::STANDARD
                    .decode(pcm_b64)
                    .map_err(|err| HostError::new("INVALID_ARGUMENT", format!("Invalid PCM base64: {err}")))?;
                self.push_audio(session_id, pcm)?;
                Ok(json!({ "ok": true }))
            }
            "finish" => {
                let session_id = payload
                    .get("sessionId")
                    .and_then(Value::as_str)
                    .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing sessionId"))?;
                let text = self.finish_session(session_id)?;
                Ok(json!({ "text": text }))
            }
            "cancel" => {
                let session_id = payload.get("sessionId").and_then(Value::as_str);
                self.cancel_session(session_id);
                Ok(json!({ "ok": true }))
            }
            "transcribe" => {
                let pcm_b64 = payload
                    .get("pcmBase64")
                    .and_then(Value::as_str)
                    .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing pcmBase64"))?;
                let language = payload
                    .get("language")
                    .and_then(Value::as_str)
                    .unwrap_or("auto")
                    .to_string();
                let start_language = payload
                    .get("startLanguage")
                    .and_then(Value::as_str)
                    .unwrap_or("")
                    .to_string();
                let pcm = base64::engine::general_purpose::STANDARD
                    .decode(pcm_b64)
                    .map_err(|err| HostError::new("INVALID_ARGUMENT", format!("Invalid PCM base64: {err}")))?;
                let (text, detected) = self.transcribe_once(&app, pcm, language, start_language)?;
                Ok(json!({ "text": text, "language": detected }))
            }
            other => Err(HostError::new(
                "INVALID_ARGUMENT",
                format!("Unknown ASR action: {other}"),
            )),
        }
    }

    pub fn shutdown(&self) {
        let (channel, token, mut process) = {
            let mut inner = self.inner.lock().unwrap();
            inner.shutting_down = true;
            if let Some(session) = inner.active_session.take() {
                let _ = session.request_tx.send(RecognizeRequest {
                    payload: Some(recognize_request::Payload::Cancel(CancelSession {})),
                });
            }
            let channel = inner.channel.take();
            let token = inner.token.take();
            let process = inner.process.take();
            inner.status.ready = false;
            inner.status.state = "idle".to_string();
            (channel, token, process)
        };

        if let (Some(channel), Some(token)) = (channel, token) {
            let _ = tauri::async_runtime::block_on(async move {
                let mut client = AsrServiceClient::new(channel);
                if let Ok(req) = with_token(ShutdownRequest {}, &token) {
                    let _ = tokio::time::timeout(
                        Duration::from_millis(500),
                        client.shutdown(req),
                    )
                    .await;
                }
            });
        }
        if let Some(proc) = process.as_mut() {
            proc.terminate();
        }
    }

    fn status_snapshot(&self) -> AsrStatusSnapshot {
        self.inner.lock().unwrap().status.clone()
    }

    fn emit_status(&self, app: &AppHandle) {
        let status = self.status_snapshot();
        crate::events::emit(app, "", "asr", json!({ "type": "status", "status": status }));
    }

    fn resolve_dirs(app: &AppHandle) -> (Option<PathBuf>, PathBuf) {
        let resource_dir = app.path().resource_dir().ok();
        let data_dir = app.state::<AppState>().directory.clone();
        (resource_dir, data_dir)
    }

    fn ensure_connected_blocking(&self, app: &AppHandle) -> Result<(Channel, String), String> {
        {
            let mut inner = self.inner.lock().unwrap();
            if inner.shutting_down {
                return Err("Application is shutting down".to_string());
            }
            if let Some(proc) = inner.process.as_mut() {
                if proc.is_running() {
                    if let (Some(channel), Some(token)) =
                        (inner.channel.clone(), inner.token.clone())
                    {
                        return Ok((channel, token));
                    }
                }
            }
        }

        let (resource_dir, data_dir) = Self::resolve_dirs(app);
        let assets: AsrAssets = ensure_assets(
            resource_dir.as_deref(),
            &data_dir,
            |progress, message| {
                {
                    let mut inner = self.inner.lock().unwrap();
                    inner.status.state = "downloading".to_string();
                    inner.status.ready = false;
                    inner.status.progress = Some(progress);
                    inner.status.message = Some(message.to_string());
                }
                self.emit_status(app);
            },
        )?;

        {
            let mut inner = self.inner.lock().unwrap();
            if inner.shutting_down {
                return Err("Application is shutting down".to_string());
            }
            inner.status.state = "loading".to_string();
            inner.status.ready = false;
            inner.status.progress = None;
            inner.status.message = Some("Запуск локальной модели распознавания речи…".to_string());
        }
        self.emit_status(app);

        let process = AsrProcess::spawn(&assets)?;
        let endpoint = format!("http://{}", process.handshake.addr);
        let token = process.handshake.token.clone();

        let (channel, status_resp) = tauri::async_runtime::block_on(async {
            let channel = Channel::from_shared(endpoint.clone())
                .map_err(|err| format!("invalid prime-asr endpoint {endpoint}: {err}"))?
                .connect_timeout(Duration::from_secs(5))
                .connect()
                .await
                .map_err(|err| format!("failed to connect to prime-asr gRPC: {err}"))?;

            let mut client = AsrServiceClient::new(channel.clone());
            let req = with_token(StatusRequest {}, &token)?;
            let status = client
                .status(req)
                .await
                .map_err(|err| format!("prime-asr Status RPC failed: {err}"))?
                .into_inner();
            Ok::<_, String>((channel, status))
        })?;

        let mut inner = self.inner.lock().unwrap();
        if inner.shutting_down {
            drop(inner);
            drop(process);
            return Err("Application is shutting down".to_string());
        }
        if let Some(mut old) = inner.process.take() {
            old.terminate();
        }
        inner.status = AsrStatusSnapshot {
            state: "ready".to_string(),
            ready: status_resp.ready,
            languages: status_resp.languages,
            chunk_ms: status_resp.chunk_ms,
            progress: None,
            message: None,
        };
        inner.process = Some(process);
        inner.channel = Some(channel.clone());
        inner.token = Some(token.clone());
        Ok((channel, token))
    }

    fn start_session(
        self: &Arc<Self>,
        app: &AppHandle,
        language: String,
        start_language: String,
    ) -> HostResult<String> {
        let (channel, token) = self
            .ensure_connected_blocking(app)
            .map_err(|msg| HostError::new("ASR_UNAVAILABLE", msg))?;
        self.emit_status(app);

        let session_id = Uuid::new_v4().to_string();
        let (request_tx, request_rx) = mpsc::unbounded_channel();
        let (final_tx, final_rx) = oneshot::channel();

        request_tx
            .send(RecognizeRequest {
                payload: Some(recognize_request::Payload::Start(StartSession {
                    language,
                    start_language,
                })),
            })
            .map_err(|_| HostError::new("ASR_STREAM", "Failed to initialize ASR stream"))?;

        let app_clone = app.clone();
        let event_session_id = session_id.clone();
        let service_ref = Arc::clone(self);

        tauri::async_runtime::spawn(async move {
            let mut client = AsrServiceClient::new(channel);
            let outbound = UnboundedReceiverStream::new(request_rx);
            let req = match with_token(outbound, &token) {
                Ok(req) => req,
                Err(err) => {
                    let _ = final_tx.send(Err(err.clone()));
                    crate::events::emit(
                        &app_clone,
                        "",
                        "asr",
                        json!({ "type": "error", "sessionId": event_session_id, "message": err }),
                    );
                    return;
                }
            };

            let mut inbound = match client.stream_recognize(req).await {
                Ok(resp) => resp.into_inner(),
                Err(err) => {
                    let message = format!("ASR stream error: {err}");
                    let _ = final_tx.send(Err(message.clone()));
                    service_ref.invalidate_broken_connection();
                    crate::events::emit(
                        &app_clone,
                        "",
                        "asr",
                        json!({ "type": "error", "sessionId": event_session_id, "message": message }),
                    );
                    return;
                }
            };

            let mut final_tx = Some(final_tx);
            while let Some(item) = inbound.next().await {
                match item {
                    Ok(ev) => match ev.event {
                        Some(recognize_event::Event::Partial(partial)) => {
                            crate::events::emit(
                                &app_clone,
                                "",
                                "asr",
                                json!({
                                    "type": "partial",
                                    "sessionId": event_session_id,
                                    "committed": partial.committed,
                                    "open": partial.open,
                                    "tail": partial.tail
                                }),
                            );
                        }
                        Some(recognize_event::Event::Commit(commit)) => {
                            crate::events::emit(
                                &app_clone,
                                "",
                                "asr",
                                json!({
                                    "type": "commit",
                                    "sessionId": event_session_id,
                                    "text": commit.text
                                }),
                            );
                        }
                        Some(recognize_event::Event::Eou(eou)) => {
                            crate::events::emit(
                                &app_clone,
                                "",
                                "asr",
                                json!({
                                    "type": "eou",
                                    "sessionId": event_session_id,
                                    "atSeconds": eou.at_seconds,
                                    "probability": eou.probability
                                }),
                            );
                        }
                        Some(recognize_event::Event::Language(lang)) => {
                            crate::events::emit(
                                &app_clone,
                                "",
                                "asr",
                                json!({
                                    "type": "language",
                                    "sessionId": event_session_id,
                                    "language": lang.language
                                }),
                            );
                        }
                        Some(recognize_event::Event::FinalTranscript(final_text)) => {
                            crate::events::emit(
                                &app_clone,
                                "",
                                "asr",
                                json!({
                                    "type": "final",
                                    "sessionId": event_session_id,
                                    "text": final_text.text
                                }),
                            );
                            if let Some(tx) = final_tx.take() {
                                let _ = tx.send(Ok(final_text.text));
                            }
                            break;
                        }
                        None => {}
                    },
                    Err(err) => {
                        let message = format!("ASR stream interrupted: {err}");
                        if let Some(tx) = final_tx.take() {
                            let _ = tx.send(Err(message.clone()));
                        }
                        crate::events::emit(
                            &app_clone,
                            "",
                            "asr",
                            json!({ "type": "error", "sessionId": event_session_id, "message": message }),
                        );
                        break;
                    }
                }
            }

            if let Some(tx) = final_tx.take() {
                let _ = tx.send(Ok(String::new()));
            }
        });

        let mut inner = self.inner.lock().unwrap();
        if let Some(previous) = inner.active_session.take() {
            let _ = previous.request_tx.send(RecognizeRequest {
                payload: Some(recognize_request::Payload::Cancel(CancelSession {})),
            });
        }
        inner.active_session = Some(ActiveSession {
            session_id: session_id.clone(),
            request_tx,
            final_rx: Some(final_rx),
        });
        Ok(session_id)
    }

    fn push_audio(&self, session_id: &str, pcm: Vec<u8>) -> HostResult<()> {
        let inner = self.inner.lock().unwrap();
        let Some(active) = inner.active_session.as_ref() else {
            return Ok(());
        };
        if active.session_id != session_id {
            return Ok(());
        }
        active
            .request_tx
            .send(RecognizeRequest {
                payload: Some(recognize_request::Payload::Audio(AudioChunk {
                    pcm_s16le: pcm,
                })),
            })
            .map_err(|_| HostError::new("ASR_STREAM", "ASR stream closed"))
    }

    fn finish_session(&self, session_id: &str) -> HostResult<String> {
        let final_rx = {
            let mut inner = self.inner.lock().unwrap();
            let matches = inner
                .active_session
                .as_ref()
                .is_some_and(|session| session.session_id == session_id);
            if !matches {
                return Ok(String::new());
            }
            let mut session = inner.active_session.take().unwrap();
            let _ = session.request_tx.send(RecognizeRequest {
                payload: Some(recognize_request::Payload::Finish(FinishSession {})),
            });
            session.final_rx.take()
        };

        let Some(rx) = final_rx else {
            return Ok(String::new());
        };

        tauri::async_runtime::block_on(async move {
            match tokio::time::timeout(Duration::from_secs(15), rx).await {
                Ok(Ok(Ok(text))) => Ok(text),
                Ok(Ok(Err(err))) => Err(HostError::new("ASR_STREAM", err)),
                Ok(Err(_)) => Ok(String::new()),
                Err(_) => Err(HostError::new("ASR_TIMEOUT", "Timed out finishing ASR session")),
            }
        })
    }

    fn cancel_session(&self, session_id: Option<&str>) {
        let mut inner = self.inner.lock().unwrap();
        let should_cancel = match (inner.active_session.as_ref(), session_id) {
            (Some(_), None) => true,
            (Some(active), Some(target)) => active.session_id == target,
            (None, _) => false,
        };
        if should_cancel {
            if let Some(session) = inner.active_session.take() {
                let _ = session.request_tx.send(RecognizeRequest {
                    payload: Some(recognize_request::Payload::Cancel(CancelSession {})),
                });
            }
        }
    }

    fn transcribe_once(
        &self,
        app: &AppHandle,
        pcm: Vec<u8>,
        language: String,
        start_language: String,
    ) -> HostResult<(String, String)> {
        let (channel, token) = self
            .ensure_connected_blocking(app)
            .map_err(|msg| HostError::new("ASR_UNAVAILABLE", msg))?;
        let response = tauri::async_runtime::block_on(async move {
            let mut client = AsrServiceClient::new(channel);
            let req = with_token(
                TranscribeRequest {
                    pcm_s16le: pcm,
                    language,
                    start_language,
                },
                &token,
            )
            .map_err(|msg| HostError::new("ASR_PROTOCOL", msg))?;
            client
                .transcribe(req)
                .await
                .map_err(|err| HostError::new("ASR_TRANSCRIBE", err.to_string()))
                .map(|resp| resp.into_inner())
        })?;
        Ok((response.text, response.language))
    }

    fn invalidate_broken_connection(&self) {
        let mut inner = self.inner.lock().unwrap();
        inner.channel = None;
        inner.token = None;
        if let Some(mut proc) = inner.process.take() {
            proc.terminate();
        }
        inner.status.ready = false;
        inner.status.state = "idle".to_string();
    }
}
