use serde_json::{json, Value};
use std::{
    sync::mpsc,
    thread,
    time::{Duration, Instant},
};
use tauri::Webview;

pub fn immediate(view: &Webview, expression: &str) -> Result<Value, String> {
    let (sender, receiver) = mpsc::channel();
    let script = format!("(() => {{ try {{ return {{ ok: true, value: ({expression}) ?? null }} }} catch(error) {{ return {{ ok: false, error: String(error) }} }} }})()");
    view.eval_with_callback(script, move |result| {
        let _ = sender.send(result);
    })
    .map_err(|error| error.to_string())?;
    let encoded = receiver
        .recv_timeout(Duration::from_secs(5))
        .map_err(|_| "Browser JavaScript evaluation timed out".to_owned())?;
    let result: Value = serde_json::from_str(&encoded)
        .map_err(|error| format!("Invalid browser result: {error}"))?;
    if result["ok"] == true {
        Ok(result["value"].clone())
    } else {
        Err(result["error"]
            .as_str()
            .unwrap_or("Browser JavaScript failed")
            .to_owned())
    }
}

pub fn evaluate(view: &Webview, expression: &str, timeout: Duration) -> Result<Value, String> {
    let key = serde_json::to_string(&format!("__prime_eval_{}", uuid::Uuid::new_v4()))
        .map_err(|error| error.to_string())?;
    let script = format!("(() => {{ const slot = {{done:false}}; window[{key}] = slot; Promise.resolve().then(() => ({expression})).then(value => {{slot.done=true; slot.ok=true; slot.value=value ?? null}}, error => {{slot.done=true; slot.ok=false; slot.error=String(error)}}); return true }})()");
    immediate(view, &script)?;
    let deadline = Instant::now() + timeout;
    loop {
        let result = immediate(view, &format!("window[{key}] ?? {{missing:true}}"))?;
        if result["missing"] == true {
            return Err("Page navigated while browser evaluation was running".to_owned());
        }
        if result["done"] == true {
            let _ = view.eval(format!("delete window[{key}]"));
            return if result["ok"] == true {
                Ok(result["value"].clone())
            } else {
                Err(result["error"]
                    .as_str()
                    .unwrap_or("Browser evaluation failed")
                    .to_owned())
            };
        }
        if Instant::now() >= deadline {
            let _ = view.eval(format!("delete window[{key}]"));
            return Err("Browser evaluation timed out".to_owned());
        }
        thread::sleep(Duration::from_millis(40));
    }
}

pub fn text(value: impl Into<String>) -> Value {
    json!([{ "type": "text", "text": value.into() }])
}
