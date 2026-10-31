use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::mpsc;
use std::sync::{Mutex, OnceLock};
use std::thread;
use std::time::Duration;

use ::windows::Win32::Foundation::{LPARAM, LRESULT, WPARAM};
use ::windows::Win32::System::Threading::GetCurrentThreadId;
use ::windows::Win32::UI::WindowsAndMessaging::{
    CallNextHookEx, GetMessageW, PostThreadMessageW, SetWindowsHookExW, UnhookWindowsHookEx,
    KBDLLHOOKSTRUCT, MSLLHOOKSTRUCT, MSG, WH_KEYBOARD_LL, WH_MOUSE_LL, WM_KEYDOWN, WM_LBUTTONUP,
    WM_QUIT,
};

#[derive(Clone, Debug, PartialEq)]
pub enum Event {
    Click { x: i32, y: i32 },
    Key { name: String },
}

pub struct Recording {
    thread_id: u32,
}

static EVENTS: OnceLock<Mutex<Vec<Event>>> = OnceLock::new();
static ACTIVE: AtomicBool = AtomicBool::new(false);

fn events() -> &'static Mutex<Vec<Event>> {
    EVENTS.get_or_init(|| Mutex::new(Vec::new()))
}

pub fn start() -> Result<Recording, String> {
    if ACTIVE.swap(true, Ordering::AcqRel) {
        return Err("Recording is already running".to_owned());
    }
    events().lock().map_err(|error| error.to_string())?.clear();
    let (ready, started) = mpsc::channel::<Result<u32, String>>();
    thread::spawn(move || unsafe {
        let thread_id = GetCurrentThreadId();
        let mouse = SetWindowsHookExW(WH_MOUSE_LL, Some(mouse_proc), None, 0);
        let keyboard = SetWindowsHookExW(WH_KEYBOARD_LL, Some(keyboard_proc), None, 0);
        match (mouse, keyboard) {
            (Ok(mouse), Ok(keyboard)) => {
                let _ = ready.send(Ok(thread_id));
                let mut message = MSG::default();
                while GetMessageW(&mut message, None, 0, 0).as_bool() {}
                let _ = UnhookWindowsHookEx(mouse);
                let _ = UnhookWindowsHookEx(keyboard);
            }
            _ => {
                ACTIVE.store(false, Ordering::Release);
                let _ = ready.send(Err("The input hooks could not be installed".to_owned()));
            }
        }
    });
    let thread_id = started
        .recv()
        .map_err(|_| "The recorder stopped".to_owned())??;
    Ok(Recording { thread_id })
}

pub fn stop(recording: &Recording) -> Result<Vec<Event>, String> {
    ACTIVE.store(false, Ordering::Release);
    unsafe {
        let _ = PostThreadMessageW(recording.thread_id, WM_QUIT, WPARAM(0), LPARAM(0));
    }
    thread::sleep(Duration::from_millis(150));
    let collected = events()
        .lock()
        .map_err(|error| error.to_string())?
        .clone();
    Ok(collected)
}

unsafe extern "system" fn mouse_proc(code: i32, wparam: WPARAM, lparam: LPARAM) -> LRESULT {
    if code >= 0 && ACTIVE.load(Ordering::Acquire) && wparam.0 as u32 == WM_LBUTTONUP {
        let info = &*(lparam.0 as *const MSLLHOOKSTRUCT);
        if let Ok(mut events) = events().lock() {
            events.push(Event::Click {
                x: info.pt.x,
                y: info.pt.y,
            });
        }
    }
    CallNextHookEx(None, code, wparam, lparam)
}

unsafe extern "system" fn keyboard_proc(code: i32, wparam: WPARAM, lparam: LPARAM) -> LRESULT {
    if code >= 0 && ACTIVE.load(Ordering::Acquire) && wparam.0 as u32 == WM_KEYDOWN {
        let info = &*(lparam.0 as *const KBDLLHOOKSTRUCT);
        if let Some(name) = key_name(info.vkCode) {
            if let Ok(mut events) = events().lock() {
                events.push(Event::Key { name });
            }
        }
    }
    CallNextHookEx(None, code, wparam, lparam)
}

pub fn key_name(code: u32) -> Option<String> {
    let name = match code {
        0x0D => "ENTER",
        0x09 => "TAB",
        0x1B => "ESC",
        0x08 => "BACKSPACE",
        0x2E => "DELETE",
        0x20 => "SPACE",
        0x24 => "HOME",
        0x23 => "END",
        0x21 => "PAGEUP",
        0x22 => "PAGEDOWN",
        0x25 => "LEFT",
        0x26 => "UP",
        0x27 => "RIGHT",
        0x28 => "DOWN",
        0x70..=0x7B => return Some(format!("F{}", code - 0x6F)),
        _ => return None,
    };
    Some(name.to_owned())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn maps_recorded_keys_to_supported_names() {
        assert_eq!(key_name(0x0D).as_deref(), Some("ENTER"));
        assert_eq!(key_name(0x09).as_deref(), Some("TAB"));
        assert_eq!(key_name(0x1B).as_deref(), Some("ESC"));
        assert_eq!(key_name(0x70).as_deref(), Some("F1"));
        assert_eq!(key_name(0x7B).as_deref(), Some("F12"));
        assert!(key_name(0x41).is_none());
        for code in [0x0D, 0x09, 0x1B, 0x08, 0x2E, 0x20, 0x24, 0x23, 0x21, 0x22, 0x25, 0x26, 0x27, 0x28] {
            let name = key_name(code).unwrap();
            assert!(
                crate::computer::types::key_codes(&name).is_ok(),
                "{name} is not accepted by the input layer"
            );
        }
    }
}
