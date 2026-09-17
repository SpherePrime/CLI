use std::net::{SocketAddr, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, ExitCode, Stdio};
use std::time::Duration;

const READY_TIMEOUT: Duration = Duration::from_secs(10);
const READY_POLL: Duration = Duration::from_millis(150);

pub fn run(port: u16) -> ExitCode {
    let address: SocketAddr = format!("127.0.0.1:{port}").parse().expect("valid address");

    let mut server = None;
    if !server_is_up(address) {
        match start_server(port) {
            Ok(child) => server = Some(child),
            Err(error) => {
                eprintln!("cannot start server: {error}");
                return ExitCode::FAILURE;
            }
        }
    }

    if !wait_for_server(address) {
        eprintln!("server did not become ready on {address}");
        stop_server(server);
        return ExitCode::FAILURE;
    }

    let status = launch_tui(port);
    stop_server(server);

    match status {
        Ok(true) => ExitCode::SUCCESS,
        Ok(false) => ExitCode::FAILURE,
        Err(error) => {
            eprintln!("{error}");
            ExitCode::FAILURE
        }
    }
}

fn start_server(port: u16) -> std::io::Result<Child> {
    let exe = std::env::current_exe()?;
    Command::new(exe)
        .args(["serve", "--host", "127.0.0.1", "--port", &port.to_string()])
        .stdin(Stdio::null())
        .spawn()
}

fn stop_server(server: Option<Child>) {
    if let Some(mut child) = server {
        let _ = child.kill();
        let _ = child.wait();
    }
}

fn launch_tui(port: u16) -> Result<bool, String> {
    let port_arg = port.to_string();

    if let Some(bin) = find_tui_binary() {
        let status = Command::new(bin)
            .args(["--port", &port_arg])
            .current_dir(safe_working_dir())
            .status()
            .map_err(|error| format!("cannot launch TUI binary: {error}"))?;
        return Ok(status.success());
    }

    if let Some(dir) = find_tui_dir() {
        let status = Command::new("bun")
            .args(["run", "src/index.tsx", "--port", &port_arg])
            .current_dir(&dir)
            .status()
            .map_err(|error| format!("cannot launch bun TUI: {error}"))?;
        return Ok(status.success());
    }

    Err("TUI not found: build it with `bun build --compile` or set AGENT_TUI_DIR".into())
}

fn find_tui_binary() -> Option<PathBuf> {
    if let Ok(path) = std::env::var("AGENT_TUI_BIN") {
        let path = PathBuf::from(path);
        if path.is_file() {
            return Some(path);
        }
    }
    let name = if cfg!(windows) {
        "agent-tui.exe"
    } else {
        "agent-tui"
    };
    let beside = std::env::current_exe().ok()?.parent()?.join(name);
    beside.is_file().then_some(beside)
}

fn find_tui_dir() -> Option<PathBuf> {
    if let Ok(dir) = std::env::var("AGENT_TUI_DIR") {
        let path = PathBuf::from(dir);
        if path.join("src/index.tsx").is_file() {
            return Some(path);
        }
    }

    if let Ok(cwd) = std::env::current_dir() {
        let path = cwd.join("packages").join("tui");
        if path.join("src/index.tsx").is_file() {
            return Some(path);
        }
    }

    let baked = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../packages/tui");
    if baked.join("src/index.tsx").is_file() {
        return Some(baked);
    }

    None
}

fn safe_working_dir() -> PathBuf {
    std::env::current_exe()
        .ok()
        .and_then(|exe| exe.parent().map(Path::to_path_buf))
        .unwrap_or_else(std::env::temp_dir)
}

fn server_is_up(address: SocketAddr) -> bool {
    TcpStream::connect_timeout(&address, Duration::from_millis(300)).is_ok()
}

fn wait_for_server(address: SocketAddr) -> bool {
    let attempts = READY_TIMEOUT.as_millis() / READY_POLL.as_millis();
    for _ in 0..attempts {
        if server_is_up(address) {
            return true;
        }
        std::thread::sleep(READY_POLL);
    }
    false
}
