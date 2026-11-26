use crate::asr::assets::AsrAssets;
use serde::Deserialize;
use std::io::{BufRead, BufReader};
use std::process::{Child, ChildStdin, Command, Stdio};
use std::sync::mpsc;
use std::thread;
use std::time::Duration;

#[derive(Clone, Debug, Deserialize)]
pub struct Handshake {
    pub addr: String,
    pub token: String,
    pub langs: Vec<String>,
    pub chunk_ms: u32,
}

pub struct AsrProcess {
    child: Child,
    stdin: Option<ChildStdin>,
    pub handshake: Handshake,
    #[cfg(windows)]
    job: Option<WindowsJob>,
}

impl AsrProcess {
    pub fn spawn(assets: &AsrAssets) -> Result<Self, String> {
        let mut command = Command::new(&assets.binary_path);
        command
            .arg("--asr")
            .arg(&assets.asr_model_path)
            .arg("--vad")
            .arg(&assets.vad_model_path)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());

        #[cfg(windows)]
        {
            use std::os::windows::process::CommandExt;
            const CREATE_NO_WINDOW: u32 = 0x0800_0000;
            command.creation_flags(CREATE_NO_WINDOW);
        }

        let mut child = command.spawn().map_err(|err| {
            format!(
                "failed to launch prime-asr ({}): {err}",
                assets.binary_path.display()
            )
        })?;

        #[cfg(windows)]
        let job = assign_kill_on_close_job(&child).ok();

        let stdin = child.stdin.take();
        let stdout = child
            .stdout
            .take()
            .ok_or_else(|| "prime-asr stdout unavailable".to_string())?;
        if let Some(stderr) = child.stderr.take() {
            thread::spawn(move || {
                let reader = BufReader::new(stderr);
                for line in reader.lines().map_while(Result::ok) {
                    if !line.trim().is_empty() {
                        crate::logging::error(&format!("prime-asr: {line}"));
                    }
                }
            });
        }

        let (tx, rx) = mpsc::channel();
        thread::spawn(move || {
            let mut reader = BufReader::new(stdout);
            let mut line = String::new();
            let result = match reader.read_line(&mut line) {
                Ok(0) => Err("prime-asr exited before sending handshake".to_string()),
                Ok(_) => parse_handshake(&line),
                Err(err) => Err(format!("failed reading prime-asr handshake: {err}")),
            };
            let _ = tx.send(result);
        });

        let handshake = match rx.recv_timeout(Duration::from_secs(30)) {
            Ok(Ok(handshake)) => handshake,
            Ok(Err(err)) => {
                let _ = child.kill();
                let _ = child.wait();
                return Err(err);
            }
            Err(_) => {
                let _ = child.kill();
                let _ = child.wait();
                return Err("timed out waiting for prime-asr handshake".to_string());
            }
        };

        Ok(Self {
            child,
            stdin,
            handshake,
            #[cfg(windows)]
            job,
        })
    }

    pub fn is_running(&mut self) -> bool {
        matches!(self.child.try_wait(), Ok(None))
    }

    pub fn terminate(&mut self) {
        self.stdin.take();
        for _ in 0..10 {
            if matches!(self.child.try_wait(), Ok(Some(_))) {
                break;
            }
            thread::sleep(Duration::from_millis(25));
        }
        if matches!(self.child.try_wait(), Ok(None)) {
            let _ = self.child.kill();
            let _ = self.child.wait();
        }
        #[cfg(windows)]
        {
            self.job.take();
        }
    }
}

impl Drop for AsrProcess {
    fn drop(&mut self) {
        self.terminate();
    }
}

pub fn parse_handshake(line: &str) -> Result<Handshake, String> {
    let trimmed = line.trim();
    let parsed: Handshake = serde_json::from_str(trimmed)
        .map_err(|err| format!("invalid prime-asr handshake ({trimmed}): {err}"))?;
    if parsed.addr.trim().is_empty() || parsed.token.trim().is_empty() {
        return Err("incomplete prime-asr handshake".to_string());
    }
    Ok(parsed)
}

#[cfg(windows)]
struct WindowsJob(windows::Win32::Foundation::HANDLE);

#[cfg(windows)]
unsafe impl Send for WindowsJob {}

#[cfg(windows)]
impl Drop for WindowsJob {
    fn drop(&mut self) {
        unsafe {
            let _ = windows::Win32::Foundation::CloseHandle(self.0);
        }
    }
}

#[cfg(windows)]
fn assign_kill_on_close_job(child: &Child) -> Result<WindowsJob, String> {
    use std::mem::size_of;
    use std::os::windows::io::AsRawHandle;
    use windows::Win32::Foundation::HANDLE;
    use windows::Win32::System::JobObjects::{
        AssignProcessToJobObject, CreateJobObjectW, JOBOBJECT_EXTENDED_LIMIT_INFORMATION,
        JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, JobObjectExtendedLimitInformation,
        SetInformationJobObject,
    };

    unsafe {
        let job = CreateJobObjectW(None, None).map_err(|err| err.to_string())?;
        let mut info = JOBOBJECT_EXTENDED_LIMIT_INFORMATION::default();
        info.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
        SetInformationJobObject(
            job,
            JobObjectExtendedLimitInformation,
            &info as *const _ as *const _,
            size_of::<JOBOBJECT_EXTENDED_LIMIT_INFORMATION>() as u32,
        )
        .map_err(|err| err.to_string())?;
        let process_handle = HANDLE(child.as_raw_handle() as *mut _);
        AssignProcessToJobObject(job, process_handle).map_err(|err| err.to_string())?;
        Ok(WindowsJob(job))
    }
}
