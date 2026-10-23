use super::types::{Bounds, WindowInfo};
use windows::{
    core::{w, BOOL},
    Win32::{
        Foundation::{CloseHandle, FILETIME, HANDLE, HWND, LPARAM, RECT},
        Graphics::Dwm::{DwmGetWindowAttribute, DWMWA_CLOAKED, DWMWA_EXTENDED_FRAME_BOUNDS},
        System::Threading::{GetProcessTimes, OpenProcess, PROCESS_QUERY_LIMITED_INFORMATION},
        UI::WindowsAndMessaging::{
            EnumWindows, GetPropW, GetWindowRect, GetWindowTextLengthW, GetWindowTextW,
            GetWindowThreadProcessId, IsWindow, IsWindowVisible, SetPropW,
        },
    },
};

pub fn handle(value: &str) -> Result<HWND, String> {
    if value.is_empty() || value.len() > 20 || !value.bytes().all(|byte| byte.is_ascii_digit()) {
        return Err("Invalid window handle".to_owned());
    }
    let number = value
        .parse::<usize>()
        .map_err(|_| "Invalid window handle")?;
    let window = HWND(number as *mut _);
    if number == 0 || !unsafe { IsWindow(Some(window)).as_bool() } {
        return Err("The selected window is closed".to_owned());
    }
    Ok(window)
}

fn process_created(pid: u32) -> Result<u64, String> {
    unsafe {
        let process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
            .map_err(|error| error.to_string())?;
        let mut created = FILETIME::default();
        let mut exited = FILETIME::default();
        let mut kernel = FILETIME::default();
        let mut user = FILETIME::default();
        let result = GetProcessTimes(process, &mut created, &mut exited, &mut kernel, &mut user);
        let _ = CloseHandle(process);
        result.map_err(|error| error.to_string())?;
        Ok(((created.dwHighDateTime as u64) << 32) | created.dwLowDateTime as u64)
    }
}

pub fn info(value: &str) -> Result<WindowInfo, String> {
    let window = handle(value)?;
    unsafe {
        let length = GetWindowTextLengthW(window).clamp(0, 32768) as usize;
        let mut title = vec![0; length + 1];
        let copied = GetWindowTextW(window, &mut title).max(0) as usize;
        let mut pid = 0;
        if GetWindowThreadProcessId(window, Some(&mut pid)) == 0 || pid == 0 {
            return Err("The selected window is closed".to_owned());
        }
        let mut rect = RECT::default();
        if DwmGetWindowAttribute(
            window,
            DWMWA_EXTENDED_FRAME_BOUNDS,
            (&mut rect as *mut RECT).cast(),
            std::mem::size_of::<RECT>() as u32,
        )
        .is_err()
        {
            GetWindowRect(window, &mut rect).map_err(|error| error.to_string())?;
        }
        let identity = GetPropW(window, w!("PrimeDesktop.AccessIdentity"));
        Ok(WindowInfo {
            hwnd: value.to_owned(),
            title: String::from_utf16_lossy(&title[..copied]),
            pid,
            identity: (!identity.is_invalid()).then(|| (identity.0 as usize).to_string()),
            bounds: Bounds {
                x: rect.left,
                y: rect.top,
                width: rect.right.saturating_sub(rect.left),
                height: rect.bottom.saturating_sub(rect.top),
            },
            process_created: process_created(pid).unwrap_or(0),
        })
    }
}

pub fn claim(value: &str) -> Result<WindowInfo, String> {
    let mut current = info(value)?;
    if current.process_created == 0 {
        return Err("Cannot verify the selected process identity".to_owned());
    }
    if current.identity.is_none() {
        let bytes = *uuid::Uuid::new_v4().as_bytes();
        let identity = (u32::from_le_bytes(
            bytes[..4]
                .try_into()
                .map_err(|_| "Invalid window identity")?,
        ) & 0x7fff_ffff)
            .max(1);
        unsafe {
            SetPropW(
                handle(value)?,
                w!("PrimeDesktop.AccessIdentity"),
                Some(HANDLE(identity as usize as *mut _)),
            )
            .map_err(|_| "Cannot grant access to an elevated or protected window".to_owned())?;
        }
        current = info(value)?;
    }
    Ok(current)
}

pub fn verify(value: &str, pid: u32, identity: &str, created: u64) -> Result<WindowInfo, String> {
    let current = info(value)?;
    if current.pid != pid
        || current.process_created == 0
        || current.process_created != created
        || current.identity.as_deref() != Some(identity)
    {
        return Err("The selected window was closed or replaced. Request access again.".to_owned());
    }
    Ok(current)
}

pub fn list() -> Result<Vec<WindowInfo>, String> {
    unsafe extern "system" fn enumerate(window: HWND, data: LPARAM) -> BOOL {
        let output = &mut *(data.0 as *mut Vec<WindowInfo>);
        if output.len() >= 1024 || !IsWindowVisible(window).as_bool() {
            return BOOL(1);
        }
        let mut cloaked = 0u32;
        if DwmGetWindowAttribute(window, DWMWA_CLOAKED, (&mut cloaked as *mut u32).cast(), 4)
            .is_ok()
            && cloaked != 0
        {
            return BOOL(1);
        }
        let value = (window.0 as usize).to_string();
        if let Ok(info) = claim(&value).or_else(|_| info(&value)) {
            if !info.title.trim().is_empty()
                && info.title != "Program Manager"
                && info.bounds.width >= 80
                && info.bounds.height >= 50
            {
                output.push(info);
            }
        }
        BOOL(1)
    }
    let mut output = Vec::new();
    unsafe {
        EnumWindows(
            Some(enumerate),
            LPARAM((&mut output as *mut Vec<WindowInfo>) as isize),
        )
        .map_err(|error| error.to_string())?;
    }
    Ok(output)
}

#[cfg(test)]
mod tests {
    use super::*;
    use ::windows::Win32::UI::WindowsAndMessaging::{
        CreateWindowExW, DestroyWindow, WINDOW_EX_STYLE, WS_OVERLAPPEDWINDOW,
    };

    #[test]
    fn rejects_replaced_properties_and_process_creation_times() {
        struct Fixture(HWND);
        impl Drop for Fixture {
            fn drop(&mut self) {
                unsafe {
                    let _ = DestroyWindow(self.0);
                }
            }
        }
        let fixture = Fixture(unsafe {
            CreateWindowExW(
                WINDOW_EX_STYLE(0),
                w!("STATIC"),
                w!("Prime identity fixture"),
                WS_OVERLAPPEDWINDOW,
                0,
                0,
                200,
                100,
                None,
                None,
                None,
                None,
            )
            .unwrap()
        });
        let hwnd = (fixture.0 .0 as usize).to_string();
        let info = claim(&hwnd).unwrap();
        let identity = info.identity.as_deref().unwrap();
        assert!(verify(&hwnd, info.pid, identity, info.process_created).is_ok());
        assert!(verify(&hwnd, info.pid, identity, info.process_created + 1).is_err());
        assert!(verify(
            &hwnd,
            info.pid.wrapping_add(1),
            identity,
            info.process_created
        )
        .is_err());
        unsafe {
            SetPropW(
                fixture.0,
                w!("PrimeDesktop.AccessIdentity"),
                Some(HANDLE((identity.parse::<usize>().unwrap() + 1) as *mut _)),
            )
            .unwrap();
        }
        assert!(verify(&hwnd, info.pid, identity, info.process_created).is_err());
    }

    #[test]
    fn invalid_handles_are_never_accepted() {
        for value in [
            "",
            "0",
            "-1",
            "1e10",
            "0x123",
            "123/4",
            "999999999999999999999",
        ] {
            assert!(handle(value).is_err(), "{value}");
        }
    }
}
