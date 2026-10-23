use std::{
    path::Path,
    process::{Command, Stdio},
};

const APP_PROCESSES: &[&str] = &["PrimeDesktop.exe", "prime-native.exe"];

#[cfg(windows)]
fn hidden(command: &mut Command) -> &mut Command {
    use std::os::windows::process::CommandExt;
    command.creation_flags(0x08000000)
}

#[cfg(not(windows))]
fn hidden(command: &mut Command) -> &mut Command {
    command
}

pub fn app_running() -> bool {
    APP_PROCESSES.iter().any(|name| {
        hidden(
            Command::new("tasklist")
                .args(["/FI", &format!("IMAGENAME eq {name}"), "/FO", "CSV", "/NH"]),
        )
        .stdin(Stdio::null())
        .output()
        .map(|output| {
            String::from_utf8_lossy(&output.stdout)
                .to_lowercase()
                .contains(&format!("\"{}\"", name.to_lowercase()))
        })
        .unwrap_or(false)
    })
}

pub fn run_hidden(binary: &Path, arguments: &str) -> std::io::Result<i32> {
    let mut command = Command::new(binary);
    raw_arguments(&mut command, arguments);
    let status = hidden(&mut command)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()?;
    Ok(status.code().unwrap_or(-1))
}

pub fn launch(executable: &Path) -> std::io::Result<()> {
    Command::new(executable)
        .current_dir(executable.parent().unwrap_or(Path::new(".")))
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .map(|_| ())
}

#[cfg(windows)]
fn raw_arguments(command: &mut Command, arguments: &str) {
    use std::os::windows::process::CommandExt;
    command.raw_arg(arguments);
}

#[cfg(not(windows))]
fn raw_arguments(command: &mut Command, arguments: &str) {
    command.args(arguments.split_whitespace());
}
