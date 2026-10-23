use super::types::{key_codes, Action};
use ::windows::Win32::{
    Foundation::POINT,
    System::Threading::{AttachThreadInput, GetCurrentThreadId},
    UI::{Input::KeyboardAndMouse::*, WindowsAndMessaging::*},
};

fn keyboard(code: u16, scan: u16, flags: KEYBD_EVENT_FLAGS) -> INPUT {
    INPUT {
        r#type: INPUT_KEYBOARD,
        Anonymous: INPUT_0 {
            ki: KEYBDINPUT {
                wVk: VIRTUAL_KEY(code),
                wScan: scan,
                dwFlags: flags,
                time: 0,
                dwExtraInfo: 0,
            },
        },
    }
}

fn mouse(flags: MOUSE_EVENT_FLAGS, delta: i32) -> INPUT {
    INPUT {
        r#type: INPUT_MOUSE,
        Anonymous: INPUT_0 {
            mi: MOUSEINPUT {
                dx: 0,
                dy: 0,
                mouseData: delta as u32,
                dwFlags: flags,
                time: 0,
                dwExtraInfo: 0,
            },
        },
    }
}

fn send(events: &[INPUT], guard: &impl Fn() -> Result<(), String>) -> Result<(), String> {
    guard()?;
    if unsafe { SendInput(events, std::mem::size_of::<INPUT>() as i32) } != events.len() as u32 {
        return Err(
            "Windows blocked input. The target may run with elevated privileges.".to_owned(),
        );
    }
    Ok(())
}

pub fn perform(
    hwnd: &str,
    action: &Action,
    identity_guard: impl Fn() -> Result<(), String>,
) -> Result<(), String> {
    action.validate()?;
    identity_guard()?;
    let target = super::windows::handle(hwnd)?;
    if matches!(action, Action::Focus {}) {
        unsafe {
            if IsIconic(target).as_bool() {
                let _ = ShowWindow(target, SW_RESTORE);
            }
            let current = GetCurrentThreadId();
            let foreground = GetWindowThreadProcessId(GetForegroundWindow(), None);
            let attached = foreground != 0
                && foreground != current
                && AttachThreadInput(current, foreground, true).as_bool();
            let _ = BringWindowToTop(target);
            let _ = SetForegroundWindow(target);
            if attached {
                let _ = AttachThreadInput(current, foreground, false);
            }
            identity_guard()?;
            if GetForegroundWindow() != target {
                return Err(
                    "Windows did not allow focusing the selected window. Activate it and retry."
                        .to_owned(),
                );
            }
        }
        return Ok(());
    }
    let guard = || {
        identity_guard()?;
        if unsafe { GetForegroundWindow() } != target {
            return Err(
                "The selected window is not in the foreground. Use focus explicitly.".to_owned(),
            );
        }
        Ok(())
    };
    guard()?;
    match action {
        Action::Type { text } => {
            let events: Vec<INPUT> = text
                .encode_utf16()
                .flat_map(|code| {
                    [
                        keyboard(0, code, KEYEVENTF_UNICODE),
                        keyboard(0, code, KEYEVENTF_UNICODE | KEYEVENTF_KEYUP),
                    ]
                })
                .collect();
            send(&events, &guard)
        }
        Action::PressKey { key } => {
            let codes = key_codes(key)?;
            let events: Vec<INPUT> = codes
                .iter()
                .map(|code| keyboard(*code, 0, KEYBD_EVENT_FLAGS(0)))
                .chain(
                    codes
                        .iter()
                        .rev()
                        .map(|code| keyboard(*code, 0, KEYEVENTF_KEYUP)),
                )
                .collect();
            send(&events, &guard)
        }
        Action::Click { x, y, .. } | Action::Scroll { x, y, .. } => {
            let info = super::windows::info(hwnd)?;
            if *x < 0 || *y < 0 || *x >= info.bounds.width || *y >= info.bounds.height {
                return Err("Coordinates are outside the selected window".to_owned());
            }
            let point = POINT {
                x: info
                    .bounds
                    .x
                    .checked_add(*x)
                    .ok_or("Invalid pointer position")?,
                y: info
                    .bounds
                    .y
                    .checked_add(*y)
                    .ok_or("Invalid pointer position")?,
            };
            let point_guard = || {
                guard()?;
                if unsafe { GetAncestor(WindowFromPoint(point), GA_ROOT) } != target {
                    return Err("The selected point is covered by another window".to_owned());
                }
                Ok(())
            };
            point_guard()?;
            unsafe {
                SetCursorPos(point.x, point.y).map_err(|error| error.to_string())?;
            }
            let events = if let Action::Scroll { delta, .. } = action {
                vec![mouse(MOUSEEVENTF_WHEEL, *delta)]
            } else {
                let Action::Click { button, count, .. } = action else {
                    return Err("Invalid pointer action".to_owned());
                };
                let right = button.as_deref() == Some("right");
                let (down, up) = if right {
                    (MOUSEEVENTF_RIGHTDOWN, MOUSEEVENTF_RIGHTUP)
                } else {
                    (MOUSEEVENTF_LEFTDOWN, MOUSEEVENTF_LEFTUP)
                };
                (0..count.unwrap_or(1))
                    .flat_map(|_| [mouse(down, 0), mouse(up, 0)])
                    .collect()
            };
            send(&events, &point_guard)
        }
        Action::Focus {} => Ok(()),
    }
}
