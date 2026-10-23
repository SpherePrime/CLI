use crate::{error::{HostError, HostResult}, permissions};
use tauri::Manager;

#[tauri::command]
pub fn desktop_window(app:tauri::AppHandle,webview:tauri::Webview,action:String)->HostResult<()> {
    permissions::require_trusted(&webview)?;
    let window=app.get_webview_window("main").ok_or_else(||HostError::new("WINDOW_NOT_FOUND","Main window unavailable"))?;
    let result=match action.as_str() {
        "minimize"=>window.minimize(),
        "maximize"=>window.is_maximized().and_then(|maximized|if maximized{window.unmaximize()}else{window.maximize()}),
        "close"=>window.close(),
        "drag"=>window.start_dragging(),
        _=>return Err(HostError::new("INVALID_ARGUMENT","Unknown window action"))
    };
    result.map_err(|error|HostError::new("WINDOW_ERROR",error.to_string()))
}
