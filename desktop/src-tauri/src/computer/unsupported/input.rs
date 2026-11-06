use crate::computer::types::Action;

pub fn perform(
    _hwnd: &str,
    _action: &Action,
    _identity_guard: impl Fn() -> Result<(), String>,
) -> Result<(), String> {
    Err("Desktop window control requires Windows".to_owned())
}
