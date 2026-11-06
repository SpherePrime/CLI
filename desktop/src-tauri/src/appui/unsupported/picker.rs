use std::time::Duration;

const UNSUPPORTED: &str = "Picking application elements is currently supported only on Windows";

pub struct PickSession;

#[derive(Clone)]
pub struct CancelHandle;

impl CancelHandle {
    pub fn cancel(&self) {}
}

pub fn arm() -> Result<(PickSession, CancelHandle), String> {
    Err(UNSUPPORTED.to_owned())
}

pub fn wait(_session: &PickSession, _handle: &CancelHandle, _timeout: Duration) -> Result<(), String> {
    Err(UNSUPPORTED.to_owned())
}
