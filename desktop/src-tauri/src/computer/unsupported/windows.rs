use crate::computer::types::WindowInfo;

pub fn list() -> Result<Vec<WindowInfo>, String> {
    Err("Desktop window viewing and control are currently supported only on Windows".to_owned())
}
