use serde_json::{json, Value};
use std::{
    collections::HashMap,
    sync::{
        atomic::{AtomicBool, Ordering},
        mpsc, Mutex, OnceLock,
    },
    thread,
    time::{Duration, Instant},
};
use tauri::Webview;

const EVALUATION_TIMEOUT: &str = "Browser JavaScript evaluation timed out";
const PROBE_WAIT: Duration = Duration::from_secs(5);

#[derive(Clone, Copy, Default)]
struct TabHealth {
    unresponsive_since: Option<Instant>,
    evaluating_since: Option<Instant>,
}

fn health() -> &'static Mutex<HashMap<String, TabHealth>> {
    static HEALTH: OnceLock<Mutex<HashMap<String, TabHealth>>> = OnceLock::new();
    HEALTH.get_or_init(Default::default)
}

fn update_health(tab: &str, change: impl FnOnce(&mut TabHealth)) {
    if let Ok(mut tabs) = health().lock() {
        change(tabs.entry(tab.to_owned()).or_default());
    }
}

fn mark_unresponsive(state: &mut TabHealth) {
    if state.unresponsive_since.is_none() {
        state.unresponsive_since = Instant::now()
            .checked_sub(PROBE_WAIT)
            .or(Some(Instant::now()));
    }
}

pub fn immediate(view: &Webview, expression: &str) -> Result<Value, String> {
    let (sender, receiver) = mpsc::channel();
    let script = format!("(() => {{ try {{ return {{ ok: true, value: ({expression}) ?? null }} }} catch(error) {{ return {{ ok: false, error: String(error) }} }} }})()");
    view.eval_with_callback(script, move |result| {
        let _ = sender.send(result);
    })
    .map_err(|error| error.to_string())?;
    let encoded = receiver
        .recv_timeout(PROBE_WAIT)
        .map_err(|_| EVALUATION_TIMEOUT.to_owned())?;
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

pub fn evaluate_until_done(
    view: &Webview,
    tab: &str,
    expression: &str,
    cancel: &AtomicBool,
) -> Result<Value, String> {
    let key = serde_json::to_string(&format!("__prime_eval_{}", uuid::Uuid::new_v4()))
        .map_err(|error| error.to_string())?;
    let script = format!("(() => {{ const slot = {{done:false}}; window[{key}] = slot; Promise.resolve().then(() => ({expression})).then(value => {{slot.done=true; slot.ok=true; slot.value=value ?? null}}, error => {{slot.done=true; slot.ok=false; slot.error=String(error)}}); return true }})()");
    immediate(view, &script)?;
    update_health(tab, |state| {
        state.evaluating_since = Some(Instant::now());
        state.unresponsive_since = None;
    });
    let outcome = loop {
        if cancel.load(Ordering::Acquire) {
            break Err("Browser evaluation canceled".to_owned());
        }
        match immediate(view, &format!("window[{key}] ?? {{missing:true}}")) {
            Ok(result) => {
                update_health(tab, |state| state.unresponsive_since = None);
                if result["missing"] == true {
                    break Err("Page navigated while browser evaluation was running".to_owned());
                }
                if result["done"] == true {
                    break if result["ok"] == true {
                        Ok(result["value"].clone())
                    } else {
                        Err(result["error"]
                            .as_str()
                            .unwrap_or("Browser evaluation failed")
                            .to_owned())
                    };
                }
                thread::sleep(Duration::from_millis(40));
            }
            Err(error) if error == EVALUATION_TIMEOUT => update_health(tab, mark_unresponsive),
            Err(error) => break Err(error),
        }
    };
    update_health(tab, |state| state.evaluating_since = None);
    let _ = view.eval(format!("delete window[{key}]"));
    outcome
}

pub fn page_health(view: &Webview, tab: &str) -> Value {
    let responsive = immediate(view, "true").is_ok();
    if responsive {
        update_health(tab, |state| state.unresponsive_since = None);
    } else {
        update_health(tab, mark_unresponsive);
    }
    let state = health()
        .lock()
        .ok()
        .and_then(|tabs| tabs.get(tab).copied())
        .unwrap_or_default();
    health_report(tab, responsive, state)
}

fn health_report(tab: &str, responsive: bool, state: TabHealth) -> Value {
    let seconds =
        |since: Option<Instant>| since.map(|at| at.elapsed().as_secs_f64()).unwrap_or(0.0);
    json!({
        "tabId": tab,
        "responsive": responsive,
        "unresponsiveSeconds": if responsive { 0.0 } else { seconds(state.unresponsive_since) },
        "evaluating": state.evaluating_since.is_some(),
        "evaluatingSeconds": seconds(state.evaluating_since),
    })
}

pub fn text(value: impl Into<String>) -> Value {
    json!([{ "type": "text", "text": value.into() }])
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn health_report_distinguishes_busy_page_from_running_script() {
        let busy = TabHealth {
            unresponsive_since: Instant::now().checked_sub(Duration::from_secs(12)),
            evaluating_since: Instant::now().checked_sub(Duration::from_secs(40)),
        };
        let report = health_report("tab", false, busy);
        assert_eq!(report["responsive"], false);
        assert!(report["unresponsiveSeconds"].as_f64().unwrap() >= 12.0);
        assert_eq!(report["evaluating"], true);
        let waiting = health_report(
            "tab",
            true,
            TabHealth {
                unresponsive_since: None,
                evaluating_since: busy.evaluating_since,
            },
        );
        assert_eq!(waiting["responsive"], true);
        assert_eq!(waiting["unresponsiveSeconds"], 0.0);
        assert!(waiting["evaluatingSeconds"].as_f64().unwrap() >= 40.0);
    }
}
