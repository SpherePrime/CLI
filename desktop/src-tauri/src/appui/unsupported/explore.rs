use serde_json::Value;
use tauri::AppHandle;

use crate::appui::manager::AppUiManager;
use crate::browser::workbench::WorkbenchContext;

pub fn run(
    _app: &AppHandle,
    _context: &WorkbenchContext,
    _manager: &AppUiManager,
    _request: &Value,
) -> Result<Value, String> {
    Err("Exploring application interfaces is currently supported only on Windows".to_owned())
}
