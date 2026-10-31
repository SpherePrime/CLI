use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::mpsc::{self, Receiver, RecvTimeoutError};
use std::sync::Arc;
use std::thread;
use std::time::{Duration, Instant};

use ::windows::Win32::Foundation::{LPARAM, WPARAM};
use ::windows::Win32::System::Threading::GetCurrentThreadId;
use ::windows::Win32::UI::Input::KeyboardAndMouse::{
    RegisterHotKey, UnregisterHotKey, MOD_ALT, MOD_CONTROL,
};
use ::windows::Win32::UI::WindowsAndMessaging::{
    GetMessageW, PostThreadMessageW, MSG, WM_HOTKEY, WM_QUIT,
};

const PICK_HOTKEY_ID: i32 = 0x5053;
const PICK_KEY: u32 = b'P' as u32;

pub struct PickSession {
    pressed: Receiver<()>,
}

#[derive(Clone)]
pub struct CancelHandle {
    thread_id: u32,
    cancelled: Arc<AtomicBool>,
}

impl CancelHandle {
    pub fn cancel(&self) {
        self.cancelled.store(true, Ordering::Release);
        unsafe {
            let _ = PostThreadMessageW(self.thread_id, WM_QUIT, WPARAM(0), LPARAM(0));
        }
    }
}

pub fn arm() -> Result<(PickSession, CancelHandle), String> {
    let (ready, started) = mpsc::channel::<Result<u32, String>>();
    let (pressed, waiting) = mpsc::channel::<()>();
    let cancelled = Arc::new(AtomicBool::new(false));
    thread::spawn(move || unsafe {
        let thread_id = GetCurrentThreadId();
        if RegisterHotKey(None, PICK_HOTKEY_ID, MOD_CONTROL | MOD_ALT, PICK_KEY).is_err() {
            let _ = ready.send(Err(
                "Ctrl+Alt+P is already used by another application".to_owned()
            ));
            return;
        }
        let _ = ready.send(Ok(thread_id));
        let mut message = MSG::default();
        while GetMessageW(&mut message, None, 0, 0).as_bool() {
            if message.message == WM_HOTKEY && message.wParam.0 as i32 == PICK_HOTKEY_ID {
                let _ = pressed.send(());
                break;
            }
        }
        let _ = UnregisterHotKey(None, PICK_HOTKEY_ID);
    });
    let thread_id = started
        .recv()
        .map_err(|_| "The element picker stopped".to_owned())??;
    Ok((
        PickSession { pressed: waiting },
        CancelHandle {
            thread_id,
            cancelled,
        },
    ))
}

pub fn wait(
    session: &PickSession,
    handle: &CancelHandle,
    timeout: Duration,
) -> Result<(), String> {
    let deadline = Instant::now() + timeout;
    loop {
        if handle.cancelled.load(Ordering::Acquire) {
            return Err("The element pick was cancelled".to_owned());
        }
        let remaining = deadline.saturating_duration_since(Instant::now());
        if remaining.is_zero() {
            handle.cancel();
            return Err("The element was not picked in time".to_owned());
        }
        match session
            .pressed
            .recv_timeout(remaining.min(Duration::from_millis(100)))
        {
            Ok(()) => return Ok(()),
            Err(RecvTimeoutError::Timeout) => continue,
            Err(RecvTimeoutError::Disconnected) => {
                return if handle.cancelled.load(Ordering::Acquire) {
                    Err("The element pick was cancelled".to_owned())
                } else {
                    Err("The element picker stopped".to_owned())
                }
            }
        }
    }
}
