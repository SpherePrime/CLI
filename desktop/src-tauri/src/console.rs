#[cfg(windows)]
pub fn attach_hidden() {
    use std::os::windows::process::CommandExt;
    use windows::Win32::System::Console::{AttachConsole, GetConsoleWindow};
    const CREATE_NO_WINDOW: u32 = 0x08000000;
    if unsafe { !GetConsoleWindow().is_invalid() } {
        return;
    }
    let Ok(mut host) = std::process::Command::new("cmd.exe")
        .args(["/d", "/c", "pause"])
        .stdin(std::process::Stdio::piped())
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .creation_flags(CREATE_NO_WINDOW)
        .spawn()
    else {
        return;
    };
    let mut attached = false;
    for _ in 0..200 {
        if unsafe { AttachConsole(host.id()) }.is_ok() {
            attached = true;
            break;
        }
        if matches!(host.try_wait(), Ok(Some(_))) {
            break;
        }
        std::thread::sleep(std::time::Duration::from_millis(5));
    }
    if !attached {
        eprintln!("Hidden console unavailable");
    }
    let _ = host.kill();
    let _ = host.wait();
}

#[cfg(not(windows))]
pub fn attach_hidden() {}
