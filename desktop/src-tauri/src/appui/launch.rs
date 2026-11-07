use std::collections::HashSet;
use std::fs::File;
use std::io::{Read, Seek, SeekFrom};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::thread;
use std::time::{Duration, Instant};

use crate::computer::types::WindowInfo;

const POLL: Duration = Duration::from_millis(300);
const OUTPUT_TAIL: u64 = 12 * 1024;
pub const DEFAULT_WAIT: Duration = Duration::from_secs(60);
pub const MAX_WAIT: Duration = Duration::from_secs(300);

pub struct Launched {
    pub command: String,
    pub pid: u32,
    pub output: PathBuf,
    child: Child,
}

pub struct Request<'a> {
    pub command: &'a str,
    pub cwd: Option<&'a str>,
}

pub fn spawn(root: &Path, request: &Request) -> Result<Launched, String> {
    let directory = working_directory(root, request.cwd)?;
    let output =
        std::env::temp_dir().join(format!("prime-app-{}-{}.log", std::process::id(), unique()));
    let log = File::create(&output).map_err(|error| error.to_string())?;
    let errors = log.try_clone().map_err(|error| error.to_string())?;
    let mut command = shell(request.command);
    command
        .current_dir(&directory)
        .stdin(Stdio::null())
        .stdout(Stdio::from(log))
        .stderr(Stdio::from(errors));
    let child = command
        .spawn()
        .map_err(|error| format!("The application could not be started: {error}"))?;
    Ok(Launched {
        command: request.command.to_owned(),
        pid: child.id(),
        output,
        child,
    })
}

pub fn wait_for_window(
    launched: &mut Launched,
    before: &HashSet<String>,
    title: Option<&str>,
    wait: Duration,
) -> Result<WindowInfo, String> {
    let deadline = Instant::now() + wait;
    let own = std::process::id();
    let needle = title.map(str::to_lowercase);
    loop {
        let windows = crate::computer::windows::list()?;
        let titled = |window: &WindowInfo| {
            needle
                .as_deref()
                .map_or(true, |needle| window.title.to_lowercase().contains(needle))
        };
        let fresh = windows.iter().find(|window| {
            window.pid != own
                && (window.pid == launched.pid || !before.contains(&window.hwnd))
                && titled(window)
        });
        if let Some(window) = fresh {
            return Ok(window.clone());
        }
        if let Ok(Some(status)) = launched.child.try_wait() {
            if !status.success() {
                return Err(format!(
                    "The application exited with {status} before opening a window.\n{}",
                    output_tail(&launched.output)
                ));
            }
        }
        if Instant::now() >= deadline {
            if needle.is_some() {
                if let Some(window) = windows
                    .iter()
                    .find(|window| window.pid != own && titled(window))
                {
                    return Ok(window.clone());
                }
            }
            return Err(format!(
                "No new application window appeared within {} seconds. Pass windowTitle or a longer waitSeconds.\n{}",
                wait.as_secs(),
                output_tail(&launched.output)
            ));
        }
        thread::sleep(POLL);
    }
}

pub fn stop(launched: &mut Launched) {
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        let _ = Command::new("taskkill")
            .args(["/PID", &launched.pid.to_string(), "/T", "/F"])
            .creation_flags(CREATE_NO_WINDOW)
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status();
    }
    let _ = launched.child.kill();
    let _ = launched.child.wait();
}

pub fn running(launched: &mut Launched) -> bool {
    matches!(launched.child.try_wait(), Ok(None))
}

pub fn output_tail(path: &Path) -> String {
    let Ok(mut file) = File::open(path) else {
        return String::new();
    };
    let length = file.metadata().map(|meta| meta.len()).unwrap_or(0);
    if length > OUTPUT_TAIL {
        let _ = file.seek(SeekFrom::Start(length - OUTPUT_TAIL));
    }
    let mut bytes = Vec::new();
    let _ = file.read_to_end(&mut bytes);
    let text = String::from_utf8_lossy(&bytes).trim().to_owned();
    if text.is_empty() {
        "The application has not written any output.".to_owned()
    } else {
        format!("Application output:\n{text}")
    }
}

fn working_directory(root: &Path, cwd: Option<&str>) -> Result<PathBuf, String> {
    let root = root.canonicalize().map_err(|error| error.to_string())?;
    let Some(cwd) = cwd.filter(|cwd| !cwd.trim().is_empty()) else {
        return Ok(root);
    };
    let directory = root
        .join(cwd)
        .canonicalize()
        .map_err(|_| format!("The directory {cwd} does not exist in the project"))?;
    if !directory.starts_with(&root) {
        return Err("The working directory must be inside the project".to_owned());
    }
    Ok(directory)
}

#[cfg(windows)]
const CREATE_NO_WINDOW: u32 = 0x0800_0000;

#[cfg(windows)]
fn shell(line: &str) -> Command {
    use std::os::windows::process::CommandExt;
    let mut command = Command::new("cmd");
    command
        .arg("/D")
        .arg("/S")
        .arg("/C")
        .raw_arg(format!("\"{line}\""));
    command.creation_flags(CREATE_NO_WINDOW);
    command
}

#[cfg(not(windows))]
fn shell(line: &str) -> Command {
    let mut command = Command::new("sh");
    command.arg("-c").arg(line);
    command
}

fn unique() -> u128 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|value| value.as_nanos())
        .unwrap_or(0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_working_directory_stays_inside_the_project() {
        let root = std::env::temp_dir().join(format!("prime-launch-{}", std::process::id()));
        std::fs::create_dir_all(root.join("app")).unwrap();
        assert!(working_directory(&root, Some("app"))
            .unwrap()
            .ends_with("app"));
        assert!(working_directory(&root, None).is_ok());
        assert!(working_directory(&root, Some("..")).is_err());
        assert!(working_directory(&root, Some("missing")).is_err());
        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    fn output_of_a_finished_command_is_captured() {
        let root = std::env::temp_dir();
        let mut launched = spawn(
            &root,
            &Request {
                command: "echo prime-launch-check",
                cwd: None,
            },
        )
        .unwrap();
        let _ = launched.child.wait();
        assert!(output_tail(&launched.output).contains("prime-launch-check"));
        let _ = std::fs::remove_file(&launched.output);
    }
}
