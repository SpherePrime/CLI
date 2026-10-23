use super::*;
use std::time::{Duration, Instant};

fn request(
    manager: &LaunchManager,
    terminals: &TerminalManager,
    cwd: &Path,
    input: Value,
) -> Value {
    manager
        .dispatch(
            terminals,
            "workspace",
            cwd,
            &input,
            Arc::new(|_, _| {}),
            Arc::new(|_| {}),
        )
        .unwrap()
}

#[test]
fn profiles_preserve_format_and_reject_invalid_changes() {
    let directory = tempfile::tempdir().unwrap();
    let manager = LaunchManager::load(directory.path()).unwrap();
    let terminals = TerminalManager::new();
    let saved = request(
        &manager,
        &terminals,
        directory.path(),
        json!({"action":"save","input":{"name":"  Application  ","command":"echo launch","url":"http://localhost:3000"}}),
    );
    let config = saved["configs"][0].clone();
    assert_eq!(config["name"], "Application");
    assert!(uuid::Uuid::parse_str(config["id"].as_str().unwrap()).is_ok());
    let reloaded = LaunchManager::load(directory.path()).unwrap();
    let listed = request(
        &reloaded,
        &terminals,
        directory.path(),
        json!({"action":"list"}),
    );
    assert_eq!(listed["configs"], saved["configs"]);
    let bytes: Value =
        serde_json::from_slice(&std::fs::read(directory.path().join("launch.json")).unwrap())
            .unwrap();
    assert_eq!(bytes["version"], 1);
    assert_eq!(
        bytes["configs"][listed["root"].as_str().unwrap()][0],
        config
    );
    for input in [
        json!({"name":"","command":"echo test"}),
        json!({"name":"test","command":"echo first\necho second"}),
        json!({"name":"test","command":"echo test","url":"file:///private"}),
        json!({"id":"bad","name":"test","command":"echo test"}),
    ] {
        assert!(validate_config(&input).is_err());
    }
    let removed = request(
        &manager,
        &terminals,
        directory.path(),
        json!({"action":"remove","id":config["id"]}),
    );
    assert!(removed["configs"].as_array().unwrap().is_empty());
}

#[test]
fn url_detection_and_windows_start_conversion() {
    assert_eq!(
        local_url(&strip_ansi("\x1b[32mhttp://0.0.0.0:4321/path\x1b[0m")),
        Some("http://localhost:4321/path".into())
    );
    assert_eq!(
        local_url("server https://[::1]:4321/ ready"),
        Some("https://[::1]:4321/".into())
    );
    assert_eq!(local_url("https://example.com:4321"), None);
    assert_eq!(
        powershell_command("start \"\" /b app.exe --flag"),
        "Start-Process \"app.exe\" -ArgumentList '--flag'"
    );
    assert_eq!(powershell_command("npm run dev"), "npm run dev");
}

#[test]
fn real_launch_exit_url_restart_stop_and_scope() {
    let directory = tempfile::tempdir().unwrap();
    let manager = LaunchManager::load(directory.path()).unwrap();
    let terminals = TerminalManager::new();
    let command = if cfg!(windows) {
        "Write-Output ('http://127.0.0.1:' + '43123/native'); exit 7"
    } else {
        "printf 'http://127.0.0.1:%s/native\\n' 43123; exit 7"
    };
    let saved = request(
        &manager,
        &terminals,
        directory.path(),
        json!({"action":"save","input":{"name":"Native launch","command":command}}),
    );
    let id = saved["configs"][0]["id"].clone();
    let detected = Arc::new(Mutex::new(Vec::new()));
    let events = Arc::new(Mutex::new(Vec::new()));
    for action in ["start", "restart"] {
        let detected_copy = detected.clone();
        let events_copy = events.clone();
        let response = manager
            .dispatch(
                &terminals,
                "workspace",
                directory.path(),
                &json!({"action":action,"id":id}),
                Arc::new(move |_, event| events_copy.lock().unwrap().push(event)),
                Arc::new(move |url| detected_copy.lock().unwrap().push(url)),
            )
            .unwrap();
        assert_eq!(response["processes"].as_array().unwrap().len(), 1);
        let deadline = Instant::now() + Duration::from_secs(20);
        loop {
            let snapshot = manager.snapshot(saved["root"].as_str().unwrap());
            if snapshot["processes"][0]["status"] == "exited" {
                assert_eq!(snapshot["processes"][0]["exitCode"], 7);
                assert_eq!(
                    snapshot["processes"][0]["url"],
                    "http://127.0.0.1:43123/native"
                );
                break;
            }
            assert!(Instant::now() < deadline, "Launch did not exit: {snapshot}");
            std::thread::sleep(Duration::from_millis(30));
        }
    }
    assert_eq!(detected.lock().unwrap().len(), 2);
    assert!(events
        .lock()
        .unwrap()
        .iter()
        .any(|event| event["__launch"] == true));
    let other = tempfile::tempdir().unwrap();
    assert!(manager
        .dispatch(
            &terminals,
            "other",
            other.path(),
            &json!({"action":"stop","id":id}),
            Arc::new(|_, _| {}),
            Arc::new(|_| {})
        )
        .is_err());
    let stopped = request(
        &manager,
        &terminals,
        directory.path(),
        json!({"action":"stop","id":id}),
    );
    assert!(stopped["processes"].as_array().unwrap().is_empty());
    assert_eq!(terminals.count(), 0);
    assert!(manager.all().as_array().unwrap().is_empty());
}
