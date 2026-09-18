use std::collections::HashMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::process::{Child, ChildStdin, Command, Stdio};
use std::sync::{Arc, Mutex, OnceLock};
use std::time::{Duration, Instant};

use anyhow::{Context, Result};
use uuid::Uuid;

pub struct ManagedProcess {
    pub id: Uuid,
    pub session: Uuid,
    pub command: String,
    pub pid: u32,
    pub started_at: Instant,
    child: Mutex<Child>,
    stdin: Mutex<Option<ChildStdin>>,
    lines: Arc<Mutex<Vec<String>>>,
    exit: Mutex<Option<i32>>,
}

impl ManagedProcess {
    pub fn write_stdin(&self, data: &str) -> Result<()> {
        let mut guard = self.stdin.lock().expect("stdin lock");
        let Some(mut stdin) = guard.take() else {
            anyhow::bail!("process stdin was closed or already used");
        };
        stdin
            .write_all(data.as_bytes())
            .context("writing to the process stdin")?;
        stdin.flush().context("flushing the process stdin")?;
        *guard = Some(stdin);
        Ok(())
    }

    pub fn output(&self, limit: usize) -> Vec<String> {
        let lines = self.lines.lock().expect("lines lock");
        if limit == 0 || lines.len() <= limit {
            lines.clone()
        } else {
            lines[lines.len() - limit..].to_vec()
        }
    }

    pub fn exit_code(&self) -> Option<i32> {
        *self.exit.lock().expect("exit lock")
    }

    pub fn try_wait(&self) -> Option<i32> {
        let mut exit = self.exit.lock().expect("exit lock");
        if let Some(code) = *exit {
            return Some(code);
        }
        match self.child.lock().expect("child lock").try_wait() {
            Ok(Some(status)) => {
                let code = status.code();
                *exit = code;
                code
            }
            _ => None,
        }
    }
}

pub struct ProcessManager {
    processes: Mutex<HashMap<Uuid, Arc<ManagedProcess>>>,
}

impl ProcessManager {
    pub fn global() -> &'static ProcessManager {
        static MANAGER: OnceLock<ProcessManager> = OnceLock::new();
        MANAGER.get_or_init(|| ProcessManager {
            processes: Mutex::new(HashMap::new()),
        })
    }

    pub fn start(
        &self,
        session: Uuid,
        command: &str,
        cwd: &std::path::Path,
    ) -> Result<Arc<ManagedProcess>> {
        let (shell, flag) = if cfg!(windows) {
            ("cmd", "/C")
        } else {
            ("sh", "-c")
        };

        let mut child = Command::new(shell)
            .arg(flag)
            .arg(command)
            .current_dir(cwd)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .with_context(|| format!("spawning process `{command}`"))?;

        let pid = child.id();
        let id = Uuid::new_v4();
        let cmd_text = command.to_string();

        let stdout = child.stdout.take();
        let stderr = child.stderr.take();
        let stdin = child.stdin.take();

        let lines: Arc<Mutex<Vec<String>>> = Arc::new(Mutex::new(Vec::new()));

        let streams: Vec<Option<Box<dyn Read + Send>>> = vec![
            stdout.map(|stream| Box::new(stream) as Box<dyn Read + Send>),
            stderr.map(|stream| Box::new(stream) as Box<dyn Read + Send>),
        ];
        for stream in streams.into_iter().flatten() {
            let lines = Arc::clone(&lines);
            std::thread::spawn(move || {
                for line in BufReader::new(stream).lines().map_while(Result::ok) {
                    lines.lock().expect("lines lock").push(line);
                }
            });
        }

        let managed = Arc::new(ManagedProcess {
            id,
            session,
            command: cmd_text,
            pid,
            started_at: Instant::now(),
            child: Mutex::new(child),
            stdin: Mutex::new(stdin),
            lines: Arc::clone(&lines),
            exit: Mutex::new(None),
        });

        self.processes
            .lock()
            .expect("processes lock")
            .insert(id, Arc::clone(&managed));
        Ok(managed)
    }

    pub fn get(&self, id: &Uuid) -> Option<Arc<ManagedProcess>> {
        self.processes
            .lock()
            .expect("processes lock")
            .get(id)
            .cloned()
    }

    pub fn stop(&self, id: &Uuid) -> Result<bool> {
        let Some(process) = self.get(id) else {
            anyhow::bail!("unknown process id");
        };
        kill_tree(&process);
        let deadline = Instant::now()
            .checked_add(Duration::from_secs(5))
            .unwrap_or_else(|| Instant::now() + Duration::from_secs(5));
        while process.try_wait().is_none() {
            if Instant::now() >= deadline {
                break;
            }
            std::thread::sleep(Duration::from_millis(40));
        }
        let stopped = process.exit_code().is_some();
        self.remove(id);
        Ok(stopped)
    }

    pub fn kill_session(&self, session: &Uuid) -> usize {
        let ids: Vec<Uuid> = self
            .processes
            .lock()
            .expect("processes lock")
            .iter()
            .filter(|(_, process)| &process.session == session)
            .map(|(id, _)| *id)
            .collect();
        let mut killed = 0;
        for id in ids {
            if self.stop(&id).is_ok() {
                killed += 1;
            }
        }
        killed
    }

    pub fn stop_all(&self) -> usize {
        let ids: Vec<Uuid> = self
            .processes
            .lock()
            .expect("processes lock")
            .keys()
            .copied()
            .collect();
        let mut stopped = 0;
        for id in ids {
            if self.stop(&id).is_ok() {
                stopped += 1;
            }
        }
        stopped
    }

    fn remove(&self, id: &Uuid) {
        self.processes.lock().expect("processes lock").remove(id);
    }
}

fn kill_tree(process: &ManagedProcess) {
    let pid = process.pid;
    let mut guard = process.child.lock().expect("child lock");
    let child_is_alive = guard.try_wait().map_or(true, |status| status.is_none());
    if !child_is_alive {
        return;
    }
    kill_child(pid, &mut guard);
}

#[cfg(windows)]
fn kill_child(pid: u32, child: &mut Child) {
    let _ = Command::new("taskkill")
        .args(["/F", "/T", "/PID", &pid.to_string()])
        .status();
    let _ = child.kill();
    let _ = child.wait();
}

#[cfg(unix)]
fn kill_child(pid: u32, child: &mut Child) {
    let _ = Command::new("pkill")
        .arg("-TERM")
        .arg("-P")
        .arg(pid.to_string())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status();
    let _ = child.kill();
    let _ = child.wait();
}

#[cfg(not(any(windows, unix)))]
fn kill_child(_pid: u32, child: &mut Child) {
    let _ = child.kill();
    let _ = child.wait();
}

pub async fn wait_for_output(process: &ManagedProcess, needle: &str, timeout: Duration) -> bool {
    let started = Instant::now();
    while started.elapsed() < timeout {
        if process.output(0).iter().any(|line| line.contains(needle)) {
            return true;
        }
        tokio::time::sleep(Duration::from_millis(40)).await;
    }
    false
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn process_captures_output_until_exit() {
        let session = Uuid::new_v4();
        let manager = ProcessManager {
            processes: Mutex::new(HashMap::new()),
        };
        let cwd = std::env::current_dir().unwrap();

        let command = if cfg!(windows) {
            "ping -n 3 127.0.0.1 >nul && echo done"
        } else {
            "echo done"
        };
        let process = manager.start(session, command, &cwd).expect("spawn");

        assert!(
            wait_for_output(&process, "done", Duration::from_secs(6)).await,
            "process output should be captured"
        );
        assert!(process.try_wait().is_some(), "process should exit itself");

        assert!(manager.stop(&process.id).expect("stop"));
        assert!(manager.get(&process.id).is_none());
        manager.kill_session(&session);
        manager.stop_all();
    }

    #[tokio::test]
    async fn process_stop_kills_running_process() {
        let session = Uuid::new_v4();
        let manager = ProcessManager {
            processes: Mutex::new(HashMap::new()),
        };
        let cwd = std::env::current_dir().unwrap();

        let command = if cfg!(windows) {
            "ping -n 30 127.0.0.1 >nul"
        } else {
            "sleep 60"
        };
        let process = manager.start(session, command, &cwd).expect("spawn");

        tokio::time::sleep(Duration::from_millis(400)).await;
        assert!(process.try_wait().is_none(), "process should be running");
        process.write_stdin("hello\n").expect("write stdin");

        assert!(manager.stop(&process.id).expect("stop"));
        assert!(manager.get(&process.id).is_none());
        assert!(process.exit_code().is_some(), "process should be dead");
    }
}
