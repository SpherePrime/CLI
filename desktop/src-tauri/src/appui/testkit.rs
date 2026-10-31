use std::sync::mpsc;
use std::thread;

use ::windows::core::w;
use ::windows::Win32::Foundation::{HWND, HINSTANCE, LPARAM, LRESULT, WPARAM};
use ::windows::Win32::UI::WindowsAndMessaging::{
    CreateWindowExW, DefWindowProcW, DestroyWindow, DispatchMessageW, FindWindowExW, GetMessageW,
    GetWindowTextW, IsWindow, PostMessageW, PostQuitMessage, RegisterClassW, SendMessageW,
    SetWindowTextW, ShowWindow, TranslateMessage, BM_CLICK, BM_GETCHECK, BN_CLICKED,
    BS_AUTOCHECKBOX, CW_USEDEFAULT, HMENU, MSG, SW_SHOW, WM_CLOSE, WM_COMMAND, WM_DESTROY,
    WNDCLASSW, WINDOW_EX_STYLE, WINDOW_STYLE, WS_BORDER, WS_CHILD, WS_OVERLAPPEDWINDOW,
    WS_TABSTOP, WS_VISIBLE,
};

const SAVE_ID: u16 = 1;
const EDIT_ID: u16 = 2;

#[derive(Clone, Copy)]
struct Handles {
    window: usize,
    button: usize,
    label: usize,
    checkbox: usize,
}

pub struct FixtureWindow {
    handles: Handles,
}

impl FixtureWindow {
    fn window(&self) -> HWND {
        HWND(self.handles.window as *mut core::ffi::c_void)
    }

    fn button(&self) -> HWND {
        HWND(self.handles.button as *mut core::ffi::c_void)
    }

    fn label_handle(&self) -> HWND {
        HWND(self.handles.label as *mut core::ffi::c_void)
    }

    fn checkbox(&self) -> HWND {
        HWND(self.handles.checkbox as *mut core::ffi::c_void)
    }

    pub fn hwnd(&self) -> String {
        self.handles.window.to_string()
    }

    pub fn is_alive(&self) -> bool {
        unsafe { IsWindow(Some(self.window())).as_bool() }
    }

    pub fn label(&self) -> String {
        let mut buffer = [0u16; 256];
        let length = unsafe { GetWindowTextW(self.label_handle(), &mut buffer) };
        String::from_utf16_lossy(&buffer[..length.clamp(0, buffer.len() as i32) as usize])
    }

    pub fn checkbox_state(&self) -> bool {
        unsafe { SendMessageW(self.checkbox(), BM_GETCHECK, None, None).0 == 1 }
    }

    pub fn click_save(&self) {
        unsafe {
            let _ = SendMessageW(self.button(), BM_CLICK, None, None);
        }
    }
}

impl Drop for FixtureWindow {
    fn drop(&mut self) {
        unsafe {
            let _ = PostMessageW(Some(self.window()), WM_CLOSE, WPARAM(0), LPARAM(0));
        }
    }
}

pub fn window() -> FixtureWindow {
    spawn(true)
}

pub fn bare_window() -> FixtureWindow {
    spawn(false)
}

fn spawn(children: bool) -> FixtureWindow {
    let (sender, receiver) = mpsc::channel();
    thread::spawn(move || match unsafe { create(children) } {
        Ok(handles) => {
            let _ = sender.send(Ok(handles));
            pump();
        }
        Err(error) => {
            let _ = sender.send(Err(error));
        }
    });
    let handles = receiver
        .recv()
        .expect("The fixture window thread stopped before creating a window")
        .expect("The fixture window could not be created");
    FixtureWindow { handles }
}

unsafe fn create(children: bool) -> Result<Handles, String> {
    let class = w!("PrimeAppUiFixture");
    let definition = WNDCLASSW {
        lpfnWndProc: Some(window_proc),
        hInstance: HINSTANCE::default(),
        lpszClassName: class,
        ..Default::default()
    };
    RegisterClassW(&definition);
    let window = CreateWindowExW(
        WINDOW_EX_STYLE(0),
        class,
        w!("Prime app UI fixture"),
        WS_OVERLAPPEDWINDOW | WS_VISIBLE,
        CW_USEDEFAULT,
        CW_USEDEFAULT,
        420,
        260,
        None,
        None,
        None,
        None,
    )
    .map_err(|error| error.to_string())?;
    let _ = ShowWindow(window, SW_SHOW);
    if !children {
        return Ok(Handles {
            window: window.0 as usize,
            button: 0,
            label: 0,
            checkbox: 0,
        });
    }
    let button = CreateWindowExW(
        WINDOW_EX_STYLE(0),
        w!("BUTTON"),
        w!("Save"),
        WS_CHILD | WS_VISIBLE,
        12,
        12,
        90,
        28,
        Some(window),
        Some(HMENU(SAVE_ID as usize as *mut core::ffi::c_void)),
        None,
        None,
    )
    .map_err(|error| error.to_string())?;
    let label = CreateWindowExW(
        WINDOW_EX_STYLE(0),
        w!("STATIC"),
        w!("Ready"),
        WS_CHILD | WS_VISIBLE,
        12,
        52,
        200,
        24,
        Some(window),
        None,
        None,
        None,
    )
    .map_err(|error| error.to_string())?;
    let _edit = CreateWindowExW(
        WINDOW_EX_STYLE(0),
        w!("EDIT"),
        w!(""),
        WS_CHILD | WS_VISIBLE | WS_BORDER | WS_TABSTOP,
        12,
        120,
        200,
        24,
        Some(window),
        Some(HMENU(EDIT_ID as usize as *mut core::ffi::c_void)),
        None,
        None,
    )
    .map_err(|error| error.to_string())?;
    let checkbox = CreateWindowExW(
        WINDOW_EX_STYLE(0),
        w!("BUTTON"),
        w!("Enabled"),
        WS_CHILD | WS_VISIBLE | WINDOW_STYLE(BS_AUTOCHECKBOX as u32),
        12,
        88,
        140,
        24,
        Some(window),
        None,
        None,
        None,
    )
    .map_err(|error| error.to_string())?;
    Ok(Handles {
        window: window.0 as usize,
        button: button.0 as usize,
        label: label.0 as usize,
        checkbox: checkbox.0 as usize,
    })
}

unsafe extern "system" fn window_proc(
    window: HWND,
    message: u32,
    wparam: WPARAM,
    lparam: LPARAM,
) -> LRESULT {
    match message {
        WM_COMMAND => {
            let id = (wparam.0 & 0xffff) as u16;
            let notification = ((wparam.0 >> 16) & 0xffff) as u16;
            if id == SAVE_ID && notification == BN_CLICKED as u16 {
                if let Ok(label) = FindWindowExW(Some(window), None, w!("STATIC"), None) {
                    let _ = SetWindowTextW(label, w!("Saved"));
                }
            }
            LRESULT(0)
        }
        WM_CLOSE => {
            let _ = DestroyWindow(window);
            LRESULT(0)
        }
        WM_DESTROY => {
            PostQuitMessage(0);
            LRESULT(0)
        }
        _ => DefWindowProcW(window, message, wparam, lparam),
    }
}

fn pump() {
    let mut message = MSG::default();
    unsafe {
        while GetMessageW(&mut message, None, 0, 0).as_bool() {
            let _ = TranslateMessage(&message);
            DispatchMessageW(&message);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fixture_window_exposes_standard_controls() {
        let fixture = window();
        assert!(fixture.is_alive());
        assert_eq!(fixture.label(), "Ready");
        assert!(!fixture.checkbox_state());
    }

    #[test]
    fn save_button_updates_the_label() {
        let fixture = window();
        fixture.click_save();
        assert_eq!(fixture.label(), "Saved");
    }

    #[test]
    fn fixtures_are_independent() {
        let first = window();
        let second = window();
        assert_ne!(first.hwnd(), second.hwnd());
        first.click_save();
        assert_eq!(first.label(), "Saved");
        assert_eq!(second.label(), "Ready");
    }
}
