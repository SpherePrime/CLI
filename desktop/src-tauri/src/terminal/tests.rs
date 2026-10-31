use super::*;
use std::time::{Duration, Instant};

#[test]
fn startup_cursor_handles_split_query_without_consuming_later_reports() {
    let mut cursor = output::StartupCursor::new(true);
    assert_eq!(cursor.consume("\u{1b}["), (String::new(), false));
    assert_eq!(cursor.consume("6nready"), ("ready".into(), true));
    assert_eq!(cursor.consume("\u{1b}[6n"), ("\u{1b}[6n".into(), false));
    let mut normal = output::StartupCursor::new(true);
    assert_eq!(normal.consume("prompt"), ("prompt".into(), false));
}

#[test]
fn agent_creation_release_and_close_respect_user_ownership() {
    let directory = tempfile::tempdir().unwrap();
    let manager = TerminalManager::new();
    let emitter: Emitter = Arc::new(|_, _| {});
    let user = create(&manager, directory.path());
    assert!(manager
        .dispatch_agent(
            "workspace",
            directory.path(),
            &json!({"action":"close","id":user.id}),
            emitter.clone()
        )
        .is_err());
    let agent = manager
        .dispatch_agent(
            "workspace",
            directory.path(),
            &json!({"action":"create"}),
            emitter.clone(),
        )
        .unwrap();
    assert_eq!(agent["owner"], "agent");
    let released = manager
        .dispatch_agent(
            "workspace",
            directory.path(),
            &json!({"action":"release","id":agent["id"]}),
            emitter.clone(),
        )
        .unwrap();
    assert_eq!(released["owner"], "user");
    assert!(manager
        .dispatch_agent(
            "workspace",
            directory.path(),
            &json!({"action":"close","id":agent["id"]}),
            emitter
        )
        .is_err());
    manager.shutdown().unwrap();
}

fn create(manager: &TerminalManager, directory: &Path) -> TerminalSnapshot {
    let emitter: Emitter = Arc::new(|_, _| {});
    manager
        .create("workspace", directory, TerminalOptions::default(), emitter)
        .unwrap()
}

fn wait_output(manager: &TerminalManager, id: &str, needle: &str) -> Value {
    let deadline = Instant::now() + Duration::from_secs(20);
    loop {
        let output = manager.find("workspace", id).unwrap().read();
        if output["output"].as_str().unwrap().contains(needle) {
            return output;
        }
        assert!(
            Instant::now() < deadline,
            "Terminal output did not contain {needle}: {output}"
        );
        std::thread::sleep(Duration::from_millis(30));
    }
}

#[test]
fn actual_shell_unicode_resize_ownership_and_cleanup() {
    let directory = tempfile::tempdir().unwrap();
    let manager = TerminalManager::new();
    let terminal = create(&manager, directory.path());
    let session = manager.find("workspace", &terminal.id).unwrap();
    assert!(terminal.pid > 0);
    assert_eq!(terminal.owner, "user");
    assert!(manager.find("different-workspace", &terminal.id).is_err());
    assert!(session.write("echo forbidden\r", "agent").is_err());
    assert!(session.report("echo not-a-report\r").is_err());
    let resized = session.resize(101, 37).unwrap();
    assert_eq!((resized.cols, resized.rows), (101, 37));
    session.owner("agent").unwrap();
    assert!(session.write("echo forbidden\r", "user").is_err());
    let command = if cfg!(windows) {
        "Write-Output ([string][char]0x41f + [char]0x440 + [char]0x438 + [char]0x432 + [char]0x435 + [char]0x442)\r"
    } else {
        "printf '\\320\\237\\321\\200\\320\\270\\320\\262\\320\\265\\321\\202\\n'\n"
    };
    session.write(command, "agent").unwrap();
    let output = wait_output(&manager, &terminal.id, "Привет");
    assert!(output["sequence"].as_u64().unwrap() > terminal.sequence);
    manager.close_workspace("workspace").unwrap();
    assert_eq!(manager.count(), 0);
    assert!(manager
        .create(
            "workspace",
            directory.path(),
            TerminalOptions::default(),
            Arc::new(|_, _| {})
        )
        .is_err());
    manager.shutdown().unwrap();
}

#[test]
fn shell_exit_retains_tail_and_exit_code() {
    let directory = tempfile::tempdir().unwrap();
    let manager = TerminalManager::new();
    let terminal = create(&manager, directory.path());
    let session = manager.find("workspace", &terminal.id).unwrap();
    session
        .write(
            if cfg!(windows) {
                "Write-Output ('native-' + 'done'); exit 7\r"
            } else {
                "printf 'native-%s\\n' 'done'; exit 7\n"
            },
            "user",
        )
        .unwrap();
    let deadline = Instant::now() + Duration::from_secs(20);
    loop {
        let snapshot = session.snapshot();
        if snapshot.status == "exited" {
            assert_eq!(snapshot.exit_code, Some(7));
            break;
        }
        assert!(Instant::now() < deadline, "Shell did not exit");
        std::thread::sleep(Duration::from_millis(30));
    }
    assert!(session.read()["output"]
        .as_str()
        .unwrap()
        .contains("native-done"));
    assert_eq!(manager.count(), 0);
    manager.close_one("workspace", &terminal.id).unwrap();
    assert_eq!(manager.count(), 0);
}

#[test]
fn dimensions_and_arguments_are_validated() {
    assert_eq!(dimension(Some(2.0), 80, 20, 500).unwrap(), 20);
    assert_eq!(dimension(Some(1000.0), 80, 20, 500).unwrap(), 500);
    assert!(dimension(Some(f64::NAN), 80, 20, 500).is_err());
    let directory = tempfile::tempdir().unwrap();
    let manager = TerminalManager::new();
    assert!(manager
        .create(
            "workspace",
            directory.path(),
            TerminalOptions {
                shell: Some("bad\0shell".into()),
                ..Default::default()
            },
            Arc::new(|_, _| {})
        )
        .is_err());
}

#[cfg(windows)]
#[test]
fn shutdown_terminates_descendant_processes() {
    use windows::Win32::{
        Foundation::CloseHandle,
        System::Threading::{GetExitCodeProcess, OpenProcess, PROCESS_QUERY_LIMITED_INFORMATION},
    };
    let directory = tempfile::tempdir().unwrap();
    let manager = TerminalManager::new();
    let terminal = create(&manager, directory.path());
    let pid_file = directory.path().join("child.pid");
    let command = format!("$child = Start-Process powershell.exe -ArgumentList '-NoProfile -Command Start-Sleep -Seconds 60' -PassThru; Set-Content -LiteralPath '{}' -Value $child.Id\r", pid_file.display().to_string().replace('\'', "''"));
    manager
        .find("workspace", &terminal.id)
        .unwrap()
        .write(&command, "user")
        .unwrap();
    let deadline = Instant::now() + Duration::from_secs(20);
    let pid = loop {
        if let Ok(value) = std::fs::read_to_string(&pid_file) {
            if let Ok(pid) = value.trim().parse::<u32>() {
                break pid;
            }
        }
        assert!(Instant::now() < deadline, "Child PID file was not created");
        std::thread::sleep(Duration::from_millis(30));
    };
    let child = unsafe { OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, pid) }.unwrap();
    manager.shutdown().unwrap();
    let deadline = Instant::now() + Duration::from_secs(5);
    loop {
        let mut status = 0;
        unsafe {
            GetExitCodeProcess(child, &mut status).unwrap();
        }
        if status != 259 {
            break;
        }
        assert!(
            Instant::now() < deadline,
            "Terminal descendant survived shutdown"
        );
        std::thread::sleep(Duration::from_millis(20));
    }
    unsafe {
        CloseHandle(child).unwrap();
    }
}
