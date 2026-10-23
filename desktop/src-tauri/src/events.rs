use serde_json::{json, Value};
use tauri::Emitter;

pub fn emit(app: &tauri::AppHandle, workspace: &str, kind: &str, payload: Value) {
    let _ = app.emit_to(
        "main",
        "prime:event",
        json!({"workspaceId":workspace,"type":kind,"payload":payload}),
    );
}
