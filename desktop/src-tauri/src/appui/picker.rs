use std::sync::atomic::{AtomicBool, AtomicIsize, Ordering};
use std::sync::mpsc::{self, Receiver, RecvTimeoutError, Sender};
use std::sync::{Arc, Mutex, OnceLock};
use std::thread;
use std::time::Duration;

use ::windows::core::w;
use ::windows::Win32::Foundation::{
    COLORREF, HINSTANCE, HWND, LPARAM, LRESULT, POINT, RECT, WPARAM,
};
use ::windows::Win32::Graphics::Gdi::{
    CombineRgn, CreateRectRgn, CreateSolidBrush, DeleteObject, SetWindowRgn, RGN_DIFF,
};
use ::windows::Win32::System::Threading::GetCurrentThreadId;
use ::windows::Win32::UI::Input::KeyboardAndMouse::VK_ESCAPE;
use ::windows::Win32::UI::WindowsAndMessaging::{
    CallNextHookEx, CreateWindowExW, DefWindowProcW, DestroyWindow, GetCursorPos, GetMessageW,
    GetWindowRect, PostQuitMessage, PostThreadMessageW, RegisterClassW, SetLayeredWindowAttributes,
    SetWindowPos, SetWindowsHookExW, ShowWindow, UnhookWindowsHookEx, HWND_TOPMOST,
    KBDLLHOOKSTRUCT, LWA_ALPHA, MSG, MSLLHOOKSTRUCT, SWP_NOACTIVATE, SW_HIDE, SW_SHOWNOACTIVATE,
    WH_KEYBOARD_LL, WH_MOUSE_LL, WM_APP, WM_KEYDOWN, WM_LBUTTONDOWN, WM_LBUTTONUP, WM_QUIT,
    WM_RBUTTONDOWN, WM_SYSKEYDOWN, WNDCLASSW, WS_EX_LAYERED, WS_EX_NOACTIVATE, WS_EX_TOOLWINDOW,
    WS_EX_TOPMOST, WS_EX_TRANSPARENT, WS_POPUP,
};

use super::types::UiBounds;

const WM_HIGHLIGHT: u32 = WM_APP + 0x51;
const FRAME: i32 = 3;
const HIGHLIGHT_COLOR: u32 = 0x0035_8CFF;

enum PickEvent {
    Picked(i32, i32),
    Cancelled,
}

static EVENTS: OnceLock<Mutex<Option<Sender<PickEvent>>>> = OnceLock::new();
static HIGHLIGHT: OnceLock<Mutex<Option<UiBounds>>> = OnceLock::new();
static TARGET: AtomicIsize = AtomicIsize::new(0);
static SWALLOW_UP: AtomicBool = AtomicBool::new(false);
static ACTIVE: AtomicBool = AtomicBool::new(false);

fn events() -> &'static Mutex<Option<Sender<PickEvent>>> {
    EVENTS.get_or_init(|| Mutex::new(None))
}

fn highlight_slot() -> &'static Mutex<Option<UiBounds>> {
    HIGHLIGHT.get_or_init(|| Mutex::new(None))
}

pub struct PickSession {
    events: Receiver<PickEvent>,
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

    pub fn highlight(&self, bounds: Option<UiBounds>) {
        if let Ok(mut slot) = highlight_slot().lock() {
            *slot = bounds;
        }
        unsafe {
            let _ = PostThreadMessageW(self.thread_id, WM_HIGHLIGHT, WPARAM(0), LPARAM(0));
        }
    }
}

pub enum Poll {
    Picked(i32, i32),
    Waiting,
}

pub fn arm(hwnd: &str) -> Result<(PickSession, CancelHandle), String> {
    let raw = hwnd
        .parse::<isize>()
        .map_err(|_| "Invalid window handle".to_owned())?;
    if ACTIVE.swap(true, Ordering::AcqRel) {
        return Err("Element picking is already running".to_owned());
    }
    TARGET.store(raw, Ordering::Release);
    SWALLOW_UP.store(false, Ordering::Release);
    let (sender, receiver) = mpsc::channel();
    if let Ok(mut slot) = events().lock() {
        *slot = Some(sender);
    }
    let (ready, started) = mpsc::channel::<Result<u32, String>>();
    thread::spawn(move || unsafe {
        let thread_id = GetCurrentThreadId();
        let mouse = SetWindowsHookExW(WH_MOUSE_LL, Some(mouse_proc), None, 0);
        let keyboard = SetWindowsHookExW(WH_KEYBOARD_LL, Some(keyboard_proc), None, 0);
        let (mouse, keyboard) = match (mouse, keyboard) {
            (Ok(mouse), Ok(keyboard)) => (mouse, keyboard),
            (mouse, keyboard) => {
                if let Ok(hook) = mouse {
                    let _ = UnhookWindowsHookEx(hook);
                }
                if let Ok(hook) = keyboard {
                    let _ = UnhookWindowsHookEx(hook);
                }
                let _ = ready.send(Err(
                    "The element picker could not capture the mouse".to_owned()
                ));
                return;
            }
        };
        let overlay = create_overlay();
        let _ = ready.send(Ok(thread_id));
        let mut message = MSG::default();
        while GetMessageW(&mut message, None, 0, 0).as_bool() {
            if message.message == WM_HIGHLIGHT {
                if let Some(overlay) = overlay {
                    move_overlay(overlay);
                }
            }
        }
        if let Some(overlay) = overlay {
            let _ = DestroyWindow(overlay);
        }
        let _ = UnhookWindowsHookEx(mouse);
        let _ = UnhookWindowsHookEx(keyboard);
    });
    let thread_id = match started.recv() {
        Ok(Ok(thread_id)) => thread_id,
        Ok(Err(error)) => {
            release();
            return Err(error);
        }
        Err(_) => {
            release();
            return Err("The element picker stopped".to_owned());
        }
    };
    Ok((
        PickSession { events: receiver },
        CancelHandle {
            thread_id,
            cancelled: Arc::new(AtomicBool::new(false)),
        },
    ))
}

pub fn poll(session: &PickSession, handle: &CancelHandle, wait: Duration) -> Result<Poll, String> {
    if handle.cancelled.load(Ordering::Acquire) {
        return Err("The element pick was cancelled".to_owned());
    }
    match session.events.recv_timeout(wait) {
        Ok(PickEvent::Picked(x, y)) => Ok(Poll::Picked(x, y)),
        Ok(PickEvent::Cancelled) => Err("The element pick was cancelled".to_owned()),
        Err(RecvTimeoutError::Timeout) => Ok(Poll::Waiting),
        Err(RecvTimeoutError::Disconnected) => Err("The element picker stopped".to_owned()),
    }
}

pub fn finish(handle: &CancelHandle) {
    handle.cancel();
    release();
}

pub fn cursor() -> Option<(i32, i32)> {
    let mut point = POINT::default();
    unsafe { GetCursorPos(&mut point) }.ok()?;
    Some((point.x, point.y))
}

pub fn inside_target(x: i32, y: i32) -> bool {
    let raw = TARGET.load(Ordering::Acquire);
    if raw == 0 {
        return false;
    }
    let mut rect = RECT::default();
    if unsafe { GetWindowRect(HWND(raw as *mut core::ffi::c_void), &mut rect) }.is_err() {
        return false;
    }
    x >= rect.left && x < rect.right && y >= rect.top && y < rect.bottom
}

fn release() {
    if let Ok(mut slot) = events().lock() {
        *slot = None;
    }
    if let Ok(mut slot) = highlight_slot().lock() {
        *slot = None;
    }
    TARGET.store(0, Ordering::Release);
    ACTIVE.store(false, Ordering::Release);
}

fn send(event: PickEvent) {
    if let Ok(slot) = events().lock() {
        if let Some(sender) = slot.as_ref() {
            let _ = sender.send(event);
        }
    }
}

unsafe extern "system" fn mouse_proc(code: i32, wparam: WPARAM, lparam: LPARAM) -> LRESULT {
    if code >= 0 && ACTIVE.load(Ordering::Acquire) {
        let info = &*(lparam.0 as *const MSLLHOOKSTRUCT);
        let message = wparam.0 as u32;
        if message == WM_LBUTTONDOWN && inside_target(info.pt.x, info.pt.y) {
            SWALLOW_UP.store(true, Ordering::Release);
            send(PickEvent::Picked(info.pt.x, info.pt.y));
            return LRESULT(1);
        }
        if message == WM_LBUTTONUP && SWALLOW_UP.swap(false, Ordering::AcqRel) {
            PostQuitMessage(0);
            return LRESULT(1);
        }
        if message == WM_RBUTTONDOWN && inside_target(info.pt.x, info.pt.y) {
            send(PickEvent::Cancelled);
            PostQuitMessage(0);
            return LRESULT(1);
        }
    }
    CallNextHookEx(None, code, wparam, lparam)
}

unsafe extern "system" fn keyboard_proc(code: i32, wparam: WPARAM, lparam: LPARAM) -> LRESULT {
    let message = wparam.0 as u32;
    if code >= 0
        && ACTIVE.load(Ordering::Acquire)
        && (message == WM_KEYDOWN || message == WM_SYSKEYDOWN)
    {
        let info = &*(lparam.0 as *const KBDLLHOOKSTRUCT);
        if info.vkCode == VK_ESCAPE.0 as u32 {
            send(PickEvent::Cancelled);
            PostQuitMessage(0);
            return LRESULT(1);
        }
    }
    CallNextHookEx(None, code, wparam, lparam)
}

unsafe extern "system" fn overlay_proc(
    window: HWND,
    message: u32,
    wparam: WPARAM,
    lparam: LPARAM,
) -> LRESULT {
    DefWindowProcW(window, message, wparam, lparam)
}

unsafe fn create_overlay() -> Option<HWND> {
    let class = w!("PrimeAppPickHighlight");
    let definition = WNDCLASSW {
        lpfnWndProc: Some(overlay_proc),
        hInstance: HINSTANCE::default(),
        lpszClassName: class,
        hbrBackground: CreateSolidBrush(COLORREF(HIGHLIGHT_COLOR)),
        ..Default::default()
    };
    RegisterClassW(&definition);
    let window = CreateWindowExW(
        WS_EX_LAYERED | WS_EX_TRANSPARENT | WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE,
        class,
        w!(""),
        WS_POPUP,
        0,
        0,
        0,
        0,
        None,
        None,
        None,
        None,
    )
    .ok()?;
    let _ = SetLayeredWindowAttributes(window, COLORREF(0), 235, LWA_ALPHA);
    Some(window)
}

unsafe fn move_overlay(window: HWND) {
    let bounds = highlight_slot().lock().ok().and_then(|slot| *slot);
    let Some(bounds) = bounds.filter(|bounds| bounds.width > 0 && bounds.height > 0) else {
        let _ = ShowWindow(window, SW_HIDE);
        return;
    };
    let width = bounds.width + FRAME * 2;
    let height = bounds.height + FRAME * 2;
    let outer = CreateRectRgn(0, 0, width, height);
    let inner = CreateRectRgn(FRAME, FRAME, width - FRAME, height - FRAME);
    let _ = CombineRgn(Some(outer), Some(outer), Some(inner), RGN_DIFF);
    let _ = DeleteObject(inner.into());
    let _ = SetWindowPos(
        window,
        Some(HWND_TOPMOST),
        bounds.x - FRAME,
        bounds.y - FRAME,
        width,
        height,
        SWP_NOACTIVATE,
    );
    SetWindowRgn(window, Some(outer), true);
    let _ = ShowWindow(window, SW_SHOWNOACTIVATE);
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::appui::testkit;

    #[test]
    fn the_picker_highlights_inside_the_target_and_can_be_cancelled() {
        let fixture = testkit::window();
        let (session, handle) = arm(&fixture.hwnd()).unwrap();
        assert!(arm(&fixture.hwnd()).is_err());
        let mut rect = RECT::default();
        unsafe {
            GetWindowRect(
                HWND(fixture.hwnd().parse::<isize>().unwrap() as *mut _),
                &mut rect,
            )
        }
        .unwrap();
        assert!(inside_target(rect.left + 5, rect.top + 5));
        assert!(!inside_target(rect.right + 50, rect.bottom + 50));
        handle.highlight(Some(UiBounds {
            x: rect.left,
            y: rect.top,
            width: 60,
            height: 30,
        }));
        assert!(matches!(
            poll(&session, &handle, Duration::from_millis(50)),
            Ok(Poll::Waiting)
        ));
        finish(&handle);
        assert!(poll(&session, &handle, Duration::from_millis(50)).is_err());
        let (_, again) = arm(&fixture.hwnd()).unwrap();
        finish(&again);
    }
}
