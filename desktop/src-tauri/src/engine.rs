use crate::{
    error::{HostError, HostResult},
    events,
};
use libloading::Library;
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    ffi::{c_char, CStr, CString},
    mem::ManuallyDrop,
    path::Path,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex,
    },
};

type Version = unsafe extern "C" fn() -> u32;
type Create = unsafe extern "C" fn(*const c_char) -> u64;
type Request = unsafe extern "C" fn(u64, *const c_char) -> u64;
type Read = unsafe extern "C" fn(u64, u64, u32) -> *mut c_char;
type RequestAction = unsafe extern "C" fn(u64, u64) -> i32;
type Free = unsafe extern "C" fn(*mut c_char);
type Shutdown = unsafe extern "C" fn(u64) -> i32;
type LastError = unsafe extern "C" fn() -> *mut c_char;

pub struct Engine {
    _library: ManuallyDrop<Library>,
    handle: u64,
    request: Request,
    read: Read,
    cancel: RequestAction,
    release: RequestAction,
    free: Free,
    shutdown: Shutdown,
    last_error: LastError,
    subscriptions: Mutex<HashMap<String, u64>>,
    closed: AtomicBool,
}

impl Engine {
    #[cfg(test)]
    pub fn load(directory: &Path) -> HostResult<Arc<Self>> {
        Self::load_bundled(directory, None, true)
    }

    pub fn load_bundled(
        directory: &Path,
        resources: Option<&Path>,
        isolated: bool,
    ) -> HostResult<Arc<Self>> {
        std::fs::create_dir_all(directory.join("engine-data"))?;
        std::env::set_var("PRIME_DESKTOP", "1");
        std::env::set_var("PRIME_DISABLE_METRICS", "1");
        if isolated {
            std::fs::create_dir_all(directory.join("config"))?;
            std::env::set_var("PRIME_GLOBAL_CONFIG", directory.join("config"));
            std::env::set_var("PRIME_GLOBAL_DATA", directory.join("engine-data"));
            std::env::set_var("PRIME_CACHE_DIR", directory.join("cache"));
        }
        let path = crate::engine_resource::resolve(resources)?;
        unsafe {
            let library = ManuallyDrop::new(Library::new(&path).map_err(|error| {
                HostError::new("ENGINE_UNAVAILABLE", format!("{}: {error}", path.display()))
            })?);
            macro_rules! symbol {
                ($name:literal, $type:ty) => {
                    *library
                        .get::<$type>(concat!($name, "\0").as_bytes())
                        .map_err(|error| HostError::new("ENGINE_ABI", error.to_string()))?
                };
            }
            let version = symbol!("prime_engine_abi_version", Version);
            if version() != 1 {
                return Err(HostError::new("ENGINE_ABI", "Unsupported Go engine ABI"));
            }
            let create = symbol!("prime_engine_create", Create);
            let request = symbol!("prime_engine_request", Request);
            let read = symbol!("prime_engine_read", Read);
            let cancel = symbol!("prime_engine_cancel", RequestAction);
            let release = symbol!("prime_engine_release_request", RequestAction);
            let free = symbol!("prime_engine_free", Free);
            let shutdown = symbol!("prime_engine_shutdown", Shutdown);
            let last_error = symbol!("prime_engine_last_error", LastError);
            let options = CString::new(json!({"working_dir":directory,"data_dir":directory.join("engine-data"),"attachment_root":directory.join("attachments"),"debug":cfg!(debug_assertions)}).to_string()).unwrap();
            let handle = create(options.as_ptr());
            if handle == 0 {
                let pointer = last_error();
                let message = if pointer.is_null() {
                    "Engine creation failed".into()
                } else {
                    let value = CStr::from_ptr(pointer).to_string_lossy().into_owned();
                    free(pointer);
                    value
                };
                return Err(HostError::new("ENGINE_CREATE", message));
            }
            Ok(Arc::new(Self {
                _library: library,
                handle,
                request,
                read,
                cancel,
                release,
                free,
                shutdown,
                last_error,
                subscriptions: Mutex::new(HashMap::new()),
                closed: AtomicBool::new(false),
            }))
        }
    }

    fn start(&self, method: &str, path: &str, body: Option<Value>) -> HostResult<u64> {
        if self.closed.load(Ordering::Acquire) {
            return Err(HostError::new("ENGINE_CLOSED", "Engine is shutting down"));
        }
        let request = CString::new(json!({"method":method,"path":path,"body":body.map(|value|value.to_string()).unwrap_or_default()}).to_string()).map_err(|_|HostError::new("INVALID_ARGUMENT","NUL byte in request"))?;
        let id = unsafe { (self.request)(self.handle, request.as_ptr()) };
        if id == 0 {
            unsafe {
                let pointer = (self.last_error)();
                let message = if pointer.is_null() {
                    "Engine request failed".into()
                } else {
                    let message = CStr::from_ptr(pointer).to_string_lossy().into_owned();
                    (self.free)(pointer);
                    message
                };
                return Err(HostError::new("ENGINE_REQUEST", message));
            }
        }
        Ok(id)
    }

    fn next(&self, id: u64, timeout: u32) -> HostResult<Option<Value>> {
        let pointer = unsafe { (self.read)(self.handle, id, timeout) };
        if pointer.is_null() {
            return Ok(None);
        }
        let bytes = unsafe { CStr::from_ptr(pointer).to_bytes().to_vec() };
        unsafe {
            (self.free)(pointer);
        }
        Ok(Some(serde_json::from_slice(&bytes)?))
    }

    pub fn request(&self, method: &str, path: &str, body: Option<Value>) -> HostResult<Value> {
        let id = self.start(method, path, body)?;
        let result = (|| {
            let mut status = 200;
            let mut data = String::new();
            let deadline = std::time::Instant::now() + std::time::Duration::from_secs(120);
            loop {
                if std::time::Instant::now() > deadline {
                    unsafe {
                        (self.cancel)(self.handle, id);
                    }
                    return Err(HostError::new(
                        "ENGINE_TIMEOUT",
                        "Engine request exceeded 120 seconds",
                    ));
                }
                let Some(event) = self.next(id, 250)? else {
                    continue;
                };
                match event["type"].as_str().unwrap_or("") {
                    "headers" => {
                        status = event["status"].as_u64().unwrap_or(200);
                    }
                    "data" => {
                        data.push_str(event["data"].as_str().unwrap_or(""));
                        if data.len() > 64 * 1024 * 1024 {
                            return Err(HostError::new(
                                "ENGINE_LIMIT",
                                "Engine response exceeds 64 MiB",
                            ));
                        }
                    }
                    "error" => {
                        return Err(HostError::new(
                            "ENGINE_ERROR",
                            event["error"].as_str().unwrap_or("Engine error"),
                        ))
                    }
                    "complete" => break,
                    _ => {}
                }
            }
            let value = if data.trim().is_empty() {
                Value::Null
            } else {
                serde_json::from_str(&data)
                    .map_err(|error| HostError::new("ENGINE_RESPONSE", error.to_string()))?
            };
            if status >= 400 {
                return Err(HostError::new(
                    "ENGINE_HTTP",
                    value["error"]
                        .as_str()
                        .map(str::to_owned)
                        .unwrap_or_else(|| format!("Engine status {status}: {value}")),
                ));
            }
            Ok(value)
        })();
        unsafe {
            (self.release)(self.handle, id);
        }
        result
    }

    pub fn subscribe(
        self: &Arc<Self>,
        app: tauri::AppHandle,
        workspace: String,
        client: &str,
    ) -> HostResult<()> {
        let id = self.start(
            "GET",
            &format!("/v1/workspaces/{workspace}/events?client_id={client}"),
            None,
        )?;
        let ready = (|| {
            let deadline = std::time::Instant::now() + std::time::Duration::from_secs(10);
            loop {
                if self.closed.load(Ordering::Acquire) || std::time::Instant::now() >= deadline {
                    return Err(HostError::new("ENGINE_EVENTS", "Event subscription did not become ready"));
                }
                let Some(event) = self.next(id, 100)? else { continue; };
                match event["type"].as_str().unwrap_or("") {
                    "headers" if event["status"].as_u64() == Some(200) => return Ok(()),
                    "headers" | "error" | "complete" => return Err(HostError::new("ENGINE_EVENTS", format!("Event subscription failed: {event}"))),
                    _ => {}
                }
            }
        })();
        if let Err(error) = ready {
            unsafe {
                (self.cancel)(self.handle, id);
                (self.release)(self.handle, id);
            }
            return Err(error);
        }
        self.subscriptions
            .lock()
            .unwrap()
            .insert(workspace.clone(), id);
        let engine = self.clone();
        std::thread::spawn(move || {
            let mut pending = String::new();
            let mut batch = crate::event_batch::EventBatch::default();
            loop {
                if engine.subscriptions.lock().unwrap().get(&workspace) != Some(&id) {
                    break;
                }
                for (kind,payload) in batch.due(std::time::Instant::now()) {
                    events::emit(&app,&workspace,&kind,payload);
                }
                let event = match engine.next(id, 25) {
                    Ok(Some(value)) => value,
                    Ok(None) => continue,
                    Err(error) => {
                        events::emit(&app, &workspace, "connection", json!("error"));
                        eprintln!("Engine events: {}", error.message);
                        break;
                    }
                };
                match event["type"].as_str().unwrap_or("") {
                    "data" => {
                        pending.push_str(event["data"].as_str().unwrap_or(""));
                        while let Some(boundary) = pending.find("\n\n") {
                            let frame = pending[..boundary].to_string();
                            pending.drain(..boundary + 2);
                            let mut kind = "message";
                            let mut data = Vec::new();
                            for line in frame.lines() {
                                if let Some(value) = line.strip_prefix("event:") {
                                    kind = value.trim();
                                }
                                if let Some(value) = line.strip_prefix("data:") {
                                    data.push(value.trim_start());
                                }
                            }
                            if let Ok(value) = serde_json::from_str::<Value>(&data.join("\n")) {
                                let kind = value["type"].as_str().unwrap_or(kind);
                                let envelope = value.get("payload").unwrap_or(&value);
                                let payload = envelope.get("payload").unwrap_or(envelope);
                                let payload = if kind == "message" {
                                    payload.clone()
                                } else {
                                    crate::redact::value(payload.clone())
                                };
                                if kind == "run_complete" { automation_failure(&app,&workspace,&payload); }
                                for (kind,payload) in batch.push(kind,json!({"action":envelope.get("type"),"data":payload}),frame.len(),std::time::Instant::now()) {
                                    events::emit(&app,&workspace,&kind,payload);
                                }
                            }
                        }
                        if pending.len() > 8 * 1024 * 1024 {
                            events::emit(&app, &workspace, "connection", json!("error"));
                            break;
                        }
                    }
                    "error" | "complete" => {
                        events::emit(&app, &workspace, "connection", json!("disconnected"));
                        break;
                    }
                    "headers" if event["status"].as_u64().unwrap_or(200) >= 400 => {
                        events::emit(&app, &workspace, "connection", json!("error"));
                        break;
                    }
                    _ => {}
                }
            }
            if !engine.closed.load(Ordering::Acquire) {
                for (kind,payload) in batch.flush() {events::emit(&app,&workspace,&kind,payload);}
            }
            unsafe {
                (engine.cancel)(engine.handle, id);
                (engine.release)(engine.handle, id);
            }
            let mut subscriptions = engine.subscriptions.lock().unwrap();
            if subscriptions.get(&workspace) == Some(&id) {
                subscriptions.remove(&workspace);
            }
        });
        Ok(())
    }

    pub fn unsubscribe(&self, workspace: &str) {
        if let Some(id) = self.subscriptions.lock().unwrap().remove(workspace) {
            unsafe {
                (self.cancel)(self.handle, id);
            }
        }
    }

    pub fn close(&self) {
        if self.closed.swap(true, Ordering::AcqRel) {
            return;
        }
        let subscriptions: Vec<u64> = self
            .subscriptions
            .lock()
            .unwrap()
            .drain()
            .map(|(_, id)| id)
            .collect();
        for id in subscriptions {
            unsafe {
                (self.cancel)(self.handle, id);
            }
        }
        unsafe {
            (self.shutdown)(self.handle);
        }
    }
}

fn automation_failure(app:&tauri::AppHandle,workspace:&str,payload:&Value) {
    use tauri::Manager;
    let error=payload["error"].as_str().unwrap_or("").trim();
    if error.is_empty() || payload["cancelled"].as_bool().unwrap_or(false) || error.to_lowercase().contains("context canceled") {return;}
    let Some(state)=app.try_state::<crate::state::AppState>() else{return;};
    let root=state.projects.lock().unwrap().get(workspace).and_then(|project|project["path"].as_str()).map(str::to_owned);
    let Some(root)=root else{return;};
    let event=json!({"root":root,"type":"runFailed","sourceRunId":payload["run_id"]});
    let manager=state.automations.clone();let app=app.clone();
    tauri::async_runtime::spawn_blocking(move || {if let Err(error)=manager.event(app,&event){eprintln!("Automation trigger: {}",error.message);}});
}

impl Drop for Engine {
    fn drop(&mut self) {
        self.close();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn native_engine_round_trip() {
        let library = crate::engine_resource::resolve(None);
        if library.is_err() {
            eprintln!("skipping: native engine library is not built for this platform");
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let engine = Engine::load(directory.path()).unwrap();
        let client = uuid::Uuid::new_v4().to_string();
        let workspace = engine
            .request(
                "POST",
                "/v1/workspaces",
                Some(json!({"path":directory.path(),"client_id":client,"channels":[]})),
            )
            .unwrap();
        let id = workspace["id"].as_str().unwrap();
        let sessions = engine
            .request("GET", &format!("/v1/workspaces/{id}/sessions"), None)
            .unwrap();
        assert!(sessions.is_array() || sessions.is_null(), "{sessions}");
        let created = engine
            .request(
                "POST",
                &format!("/v1/workspaces/{id}/sessions"),
                Some(json!({"title":"Native integration"})),
            )
            .unwrap();
        assert_eq!(created["title"], "Native integration");
        assert!(created["id"].is_string());
        engine
            .request("DELETE", &format!("/v1/clients/{client}"), None)
            .unwrap();
        engine.close();
    }
}
