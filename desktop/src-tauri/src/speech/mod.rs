mod assets;
mod audio;
mod runtime;
mod supertonic;
mod text;
mod voices;

use crate::error::{HostError, HostResult};
use base64::Engine;
use serde_json::{json, Value};
use std::{
    path::PathBuf,
    sync::{
        atomic::{AtomicBool, Ordering},
        mpsc::{self, SyncSender, TrySendError},
        Arc, Mutex,
    },
    thread::JoinHandle,
    time::{Duration, Instant},
};

pub const IDLE_UNLOAD: Duration = Duration::from_secs(10 * 60);
const QUEUE_LIMIT: usize = 8;
type Emitter = Arc<dyn Fn(Value) + Send + Sync>;
type ActiveRun = Arc<Mutex<Option<Arc<ort::session::RunOptions>>>>;

struct Job {
    request: Value,
    result: SyncSender<HostResult<Value>>,
    emit: Emitter,
}

pub struct SpeechService {
    sender: Mutex<Option<SyncSender<Job>>>,
    state: Arc<Mutex<Value>>,
    directory: PathBuf,
    loaded: Arc<AtomicBool>,
    stop: Arc<AtomicBool>,
    active: ActiveRun,
    worker: Mutex<Option<JoinHandle<()>>>,
}

impl SpeechService {
    pub fn new() -> Arc<Self> {
        Self::with_directory(assets::model_directory(), IDLE_UNLOAD)
    }

    fn with_directory(directory: PathBuf, idle_unload: Duration) -> Arc<Self> {
        let (sender, receiver) = mpsc::sync_channel::<Job>(QUEUE_LIMIT);
        let state = Arc::new(Mutex::new(json!({"state":"unknown"})));
        let loaded = Arc::new(AtomicBool::new(false));
        let stop = Arc::new(AtomicBool::new(false));
        let active: ActiveRun = Arc::new(Mutex::new(None));
        let worker_active = active.clone();
        let worker_state = state.clone();
        let worker_loaded = loaded.clone();
        let worker_stop = stop.clone();
        let root = directory.clone();
        let worker = std::thread::spawn(move || {
            let mut engine = None;
            let mut last_used = Instant::now();
            loop {
                if worker_stop.load(Ordering::Acquire) {
                    while let Ok(job) = receiver.try_recv() {
                        let _ = job.result.send(Err(assets::cancelled()));
                    }
                    break;
                }
                let timeout = if engine.is_some() {
                    idle_unload.saturating_sub(last_used.elapsed())
                } else {
                    Duration::from_secs(3600)
                };
                match receiver.recv_timeout(timeout) {
                    Ok(job) => {
                        let result = process(
                            &root,
                            &mut engine,
                            &worker_stop,
                            &worker_state,
                            &job.request,
                            &job.emit,
                            &worker_active,
                        );
                        worker_loaded.store(engine.is_some(), Ordering::Release);
                        last_used = Instant::now();
                        let _ = job.result.send(result);
                    }
                    Err(mpsc::RecvTimeoutError::Timeout) => {
                        engine = None;
                        worker_loaded.store(false, Ordering::Release);
                    }
                    Err(mpsc::RecvTimeoutError::Disconnected) => break,
                }
            }
            drop(engine);
            worker_loaded.store(false, Ordering::Release);
        });
        Arc::new(Self {
            sender: Mutex::new(Some(sender)),
            state,
            directory,
            loaded,
            stop,
            active,
            worker: Mutex::new(Some(worker)),
        })
    }

    pub fn loaded(&self) -> bool {
        self.loaded.load(Ordering::Acquire)
    }

    pub fn handle(&self, app: tauri::AppHandle, request: &Value) -> HostResult<Value> {
        let request = if request["action"] == "import" {
            let Some(path) = rfd::FileDialog::new()
                .set_title("Импорт голоса")
                .add_filter("Voice style", &["json"])
                .pick_file()
            else {
                return Ok(Value::Null);
            };
            json!({"action":"import","path":path})
        } else {
            request.clone()
        };
        self.request(
            request,
            Arc::new(move |status| crate::events::emit(&app, "", "speech", status)),
        )
    }

    fn request(&self, request: Value, emit: Emitter) -> HostResult<Value> {
        if self.stop.load(Ordering::Acquire) {
            return Err(assets::cancelled());
        }
        match request["action"].as_str() {
            Some("status") => {
                let mut state = self.state.lock().unwrap();
                if state["state"] == "unknown" {
                    *state = json!({"state":if assets::ready(&self.directory) { "ready" } else { "missing" }});
                }
                return Ok(state.clone());
            }
            Some("voices") => return Ok(serde_json::to_value(voices::list(&self.directory)?)?),
            Some("prepare" | "speak" | "import" | "remove") => {}
            _ => return Err(HostError::new("INVALID_ARGUMENT", "Unknown speech action")),
        }
        let (result, receiver) = mpsc::sync_channel(1);
        {
            let sender = self.sender.lock().unwrap();
            let sender = sender.as_ref().ok_or_else(assets::cancelled)?;
            sender
                .try_send(Job {
                    request,
                    result,
                    emit,
                })
                .map_err(|error| match error {
                    TrySendError::Full(_) => {
                        HostError::new("SPEECH_BUSY", "Speech queue limit reached (8)")
                    }
                    TrySendError::Disconnected(_) => assets::cancelled(),
                })?;
        }
        receiver
            .recv_timeout(Duration::from_secs(20 * 60))
            .map_err(|_| {
                HostError::new(
                    "SPEECH_TIMEOUT",
                    "Speech request did not finish within twenty minutes",
                )
            })?
    }

    pub fn shutdown(&self) {
        self.stop.store(true, Ordering::Release);
        if let Some(run) = self.active.lock().unwrap().as_ref() {
            let _ = run.terminate();
        }
        self.sender.lock().unwrap().take();
        if let Some(worker) = self.worker.lock().unwrap().take() {
            let _ = worker.join();
        }
    }
}

impl Drop for SpeechService {
    fn drop(&mut self) {
        self.shutdown();
    }
}

fn process(
    directory: &std::path::Path,
    engine: &mut Option<supertonic::Supertonic>,
    stop: &AtomicBool,
    state: &Mutex<Value>,
    request: &Value,
    emit: &Emitter,
    active: &ActiveRun,
) -> HostResult<Value> {
    if stop.load(Ordering::Acquire) {
        return Err(assets::cancelled());
    }
    match request["action"].as_str().unwrap_or("") {
        "import" => {
            let path = request["path"]
                .as_str()
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Voice file is required"))?;
            let voice = voices::import(directory, std::path::Path::new(path))?;
            if let Some(engine) = engine {
                engine.forget(&voice.id);
            }
            Ok(serde_json::to_value(voice)?)
        }
        "remove" => {
            let voice = request["voice"]
                .as_str()
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Voice id is required"))?;
            voices::remove(directory, voice)?;
            if let Some(engine) = engine {
                engine.forget(voice);
            }
            Ok(serde_json::to_value(voices::list(directory)?)?)
        }
        "prepare" => {
            load(directory, engine, stop, state, emit)?;
            Ok(state.lock().unwrap().clone())
        }
        "speak" => {
            let input = request["text"]
                .as_str()
                .unwrap_or("")
                .chars()
                .take(4000)
                .collect::<String>();
            let input = input.trim();
            if input.is_empty() {
                return Err(HostError::new("SPEECH_TEXT", "Пустой текст"));
            }
            let selected = request["voice"].as_str().unwrap_or("F1");
            let voice = if voices::list(directory)?
                .iter()
                .any(|voice| voice.id == selected)
            {
                selected
            } else {
                "F1"
            };
            let speed = request["speed"]
                .as_f64()
                .filter(|speed| speed.is_finite() && *speed != 0.0)
                .unwrap_or(1.05)
                .clamp(0.7, 1.6) as f32;
            load(directory, engine, stop, state, emit)?;
            let engine = engine.as_mut().unwrap();
            let options = Arc::new(ort::session::RunOptions::new().map_err(supertonic::ort_error)?);
            *active.lock().unwrap() = Some(options.clone());
            let result = engine.synthesize(input, voice, speed, stop, &options);
            active.lock().unwrap().take();
            let samples = result?;
            let wav = audio::encode_wav(&samples, engine.sample_rate())?;
            Ok(
                json!({"audio":base64::engine::general_purpose::STANDARD.encode(wav),"mime":"audio/wav"}),
            )
        }
        _ => Err(HostError::new("INVALID_ARGUMENT", "Unknown speech action")),
    }
}

fn load(
    directory: &std::path::Path,
    engine: &mut Option<supertonic::Supertonic>,
    stop: &AtomicBool,
    state: &Mutex<Value>,
    emit: &Emitter,
) -> HostResult<()> {
    if engine.is_some() {
        return Ok(());
    }
    let result = (|| {
        if !assets::ready(directory) {
            set(state, emit, json!({"state":"downloading","percent":0}));
        }
        assets::ensure(directory, stop, |percent| {
            if state.lock().unwrap()["state"] == "downloading" {
                set(
                    state,
                    emit,
                    json!({"state":"downloading","percent":percent}),
                );
            }
        })?;
        set(state, emit, json!({"state":"loading"}));
        *engine = Some(supertonic::Supertonic::load(directory, stop)?);
        set(state, emit, json!({"state":"ready"}));
        Ok::<_, HostError>(())
    })();
    if let Err(error) = &result {
        set(state, emit, json!({"state":"error","error":error.message}));
    }
    result
}

fn set(state: &Mutex<Value>, emit: &Emitter, status: Value) {
    *state.lock().unwrap() = status.clone();
    emit(status);
}

#[cfg(test)]
mod tests;
