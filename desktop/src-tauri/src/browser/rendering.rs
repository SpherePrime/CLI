use super::panel::BrowserState;
use std::{thread, time::Duration};
use tauri::AppHandle;

const FIRST_FRAME_DELAY: Duration = Duration::from_millis(200);

pub struct RenderingGuard {
    app: AppHandle,
    state: BrowserState,
    id: String,
}

impl Drop for RenderingGuard {
    fn drop(&mut self) {
        if self.state.release_rendering(&self.id) {
            let _ = self.state.attach(&self.app);
        }
    }
}

pub fn keep_rendering(app: &AppHandle, state: &BrowserState, id: &str) -> Result<RenderingGuard, String> {
    let started = state.hold_rendering(id)?;
    let guard = RenderingGuard {
        app: app.clone(),
        state: state.clone(),
        id: id.to_owned(),
    };
    if started {
        state.attach(app)?;
        if !state.is_shown(id)? {
            thread::sleep(FIRST_FRAME_DELAY);
        }
    }
    Ok(guard)
}
