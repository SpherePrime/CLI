use std::time::Duration;

use crate::appui::types::UiBounds;

const UNSUPPORTED: &str = "Picking application elements is currently supported only on Windows";

pub struct PickSession;

#[derive(Clone)]
pub struct CancelHandle;

impl CancelHandle {
    pub fn cancel(&self) {}

    pub fn highlight(&self, _bounds: Option<UiBounds>) {}
}

pub enum Poll {
    Picked(i32, i32),
    Waiting,
}

pub fn arm(_hwnd: &str) -> Result<(PickSession, CancelHandle), String> {
    Err(UNSUPPORTED.to_owned())
}

pub fn poll(_session: &PickSession, _handle: &CancelHandle, _wait: Duration) -> Result<Poll, String> {
    Err(UNSUPPORTED.to_owned())
}

pub fn finish(_handle: &CancelHandle) {}

pub fn cursor() -> Option<(i32, i32)> {
    None
}

pub fn inside_target(_x: i32, _y: i32) -> bool {
    false
}
