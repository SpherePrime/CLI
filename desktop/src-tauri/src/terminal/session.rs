use super::{
    output::{StartupCursor, TerminalOutput, Utf8Decoder},
    process::ProcessTree,
    types::TerminalSnapshot,
};
use crate::error::{HostError, HostResult};
use portable_pty::{ChildKiller, MasterPty, PtySize};
use serde_json::{json, Value};
use std::{
    io::{Read, Write},
    sync::{
        mpsc::{self, SyncSender},
        Arc, Condvar, Mutex,
    },
    time::{Duration, Instant},
};

pub type Emitter = Arc<dyn Fn(&str, Value) + Send + Sync>;

struct Journal {
    snapshot: TerminalSnapshot,
    output: TerminalOutput,
}
struct WriteRequest {
    data: String,
    result: SyncSender<Result<(), String>>,
}

pub struct Session {
    journal: Mutex<Journal>,
    master: Mutex<Option<Box<dyn MasterPty + Send>>>,
    writer: Mutex<Option<SyncSender<WriteRequest>>>,
    killer: Mutex<Box<dyn ChildKiller + Send + Sync>>,
    tree: ProcessTree,
    exited: (Mutex<bool>, Condvar),
    reader_done: (Mutex<bool>, Condvar),
    initialized: (Mutex<bool>, Condvar),
    writer_done: Arc<(Mutex<bool>, Condvar)>,
    emit: Emitter,
}

impl Session {
    pub fn start(
        snapshot: TerminalSnapshot,
        master: Box<dyn MasterPty + Send>,
        mut child: Box<dyn portable_pty::Child + Send + Sync>,
        emit: Emitter,
    ) -> HostResult<Arc<Self>> {
        let tree = match ProcessTree::attach(snapshot.pid) {
            Ok(tree) => tree,
            Err(error) => {
                let _ = child.kill();
                let _ = child.wait();
                return Err(error);
            }
        };
        let resources = (|| {
            let reader = master.try_clone_reader().map_err(pty_error)?;
            let writer = master.take_writer().map_err(pty_error)?;
            Ok::<_, HostError>((reader, writer))
        })();
        let (reader, writer) = match resources {
            Ok(resources) => resources,
            Err(error) => {
                let _ = tree.terminate();
                let _ = child.kill();
                let _ = child.wait();
                return Err(error);
            }
        };
        let killer = child.clone_killer();
        let (write_sender, write_receiver) = mpsc::sync_channel::<WriteRequest>(8);
        let writer_done = Arc::new((Mutex::new(false), Condvar::new()));
        let writing_done = writer_done.clone();
        std::thread::spawn(move || {
            let mut writer = writer;
            while let Ok(request) = write_receiver.recv() {
                let result = writer
                    .write_all(request.data.as_bytes())
                    .and_then(|_| writer.flush())
                    .map_err(|error| error.to_string());
                let _ = request.result.send(result);
            }
            drop(writer);
            *writing_done.0.lock().unwrap() = true;
            writing_done.1.notify_all();
        });
        let session = Arc::new(Self {
            journal: Mutex::new(Journal {
                snapshot,
                output: TerminalOutput::default(),
            }),
            master: Mutex::new(Some(master)),
            writer: Mutex::new(Some(write_sender)),
            killer: Mutex::new(killer),
            tree,
            exited: (Mutex::new(false), Condvar::new()),
            reader_done: (Mutex::new(false), Condvar::new()),
            initialized: (Mutex::new(!cfg!(windows)), Condvar::new()),
            writer_done,
            emit,
        });
        session.state();
        let reading = session.clone();
        std::thread::spawn(move || reading.read_loop(reader));
        let waiting = session.clone();
        std::thread::spawn(move || {
            let result = child.wait();
            *waiting.exited.0.lock().unwrap() = true;
            waiting.exited.1.notify_all();
            let _ = waiting.tree.terminate();
            waiting.writer.lock().unwrap().take();
            waiting.master.lock().unwrap().take();
            let _ = wait_for(&waiting.reader_done, Duration::from_secs(2));
            let mut journal = waiting.journal.lock().unwrap();
            if journal.snapshot.status == "running" {
                let exit_code = result.map(|status| status.exit_code()).unwrap_or(1);
                journal.snapshot.status = "exited".into();
                journal.snapshot.exit_code = Some(exit_code);
                journal.snapshot.sequence += 1;
                (waiting.emit)(
                    &journal.snapshot.workspace_id,
                    json!({"action":"exit","id":journal.snapshot.id,"exitCode":exit_code,"sequence":journal.snapshot.sequence}),
                );
                journal.snapshot.sequence += 1;
                (waiting.emit)(
                    &journal.snapshot.workspace_id,
                    json!({"action":"state","terminal":journal.snapshot}),
                );
            }
        });
        if !wait_for(&session.initialized, Duration::from_secs(5)) {
            let _ = session.close();
            return Err(HostError::new(
                "TERMINAL_STARTUP",
                "Terminal cursor initialization timed out",
            ));
        }
        Ok(session)
    }

    fn read_loop(&self, mut reader: Box<dyn Read + Send>) {
        let mut buffer = [0u8; 16384];
        let mut decoder = Utf8Decoder::default();
        let mut startup = StartupCursor::new(cfg!(windows));
        loop {
            match reader.read(&mut buffer) {
                Ok(0) => break,
                Ok(length) => {
                    let (text, reply) = startup.consume(&decoder.decode(&buffer[..length], false));
                    if reply && self.raw_write("\u{1b}[1;1R").is_err() {
                        break;
                    }
                    if !startup.active {
                        *self.initialized.0.lock().unwrap() = true;
                        self.initialized.1.notify_all();
                    }
                    self.output(&text);
                }
                Err(error) if error.kind() == std::io::ErrorKind::Interrupted => continue,
                Err(_) => break,
            }
        }
        self.output(&decoder.decode(&[], true));
        *self.reader_done.0.lock().unwrap() = true;
        self.reader_done.1.notify_all();
    }

    fn output(&self, data: &str) {
        if data.is_empty() {
            return;
        }
        let queries = data.matches("\u{1b}[c").count() + data.matches("\u{1b}[0c").count();
        if queries > 0 {
            let _ = self.raw_write(&"\u{1b}[?61;4c".repeat(queries));
        }
        let mut journal = self.journal.lock().unwrap();
        if journal.snapshot.status != "running" {
            return;
        }
        journal.output.append(data);
        journal.snapshot.sequence += 1;
        (self.emit)(
            &journal.snapshot.workspace_id,
            json!({"action":"data","id":journal.snapshot.id,"data":data,"sequence":journal.snapshot.sequence}),
        );
    }

    pub fn snapshot(&self) -> TerminalSnapshot {
        self.journal.lock().unwrap().snapshot.clone()
    }

    pub fn read(&self) -> Value {
        let journal = self.journal.lock().unwrap();
        json!({"terminal":journal.snapshot,"output":journal.output.read(),"sequence":journal.snapshot.sequence,"truncated":journal.output.truncated})
    }

    pub fn state(&self) -> TerminalSnapshot {
        let mut journal = self.journal.lock().unwrap();
        journal.snapshot.sequence += 1;
        (self.emit)(
            &journal.snapshot.workspace_id,
            json!({"action":"state","terminal":journal.snapshot}),
        );
        journal.snapshot.clone()
    }

    pub fn write(&self, data: &str, source: &str) -> HostResult<TerminalSnapshot> {
        {
            let journal = self.journal.lock().unwrap();
            require_running(&journal.snapshot)?;
            if journal.snapshot.owner != source {
                return Err(HostError::new(
                    "TERMINAL_OWNER",
                    "Terminal ownership must be transferred by the user before writing",
                ));
            }
        }
        self.raw_write(data)?;
        Ok(self.snapshot())
    }

    pub fn report(&self, data: &str) -> HostResult<()> {
        require_running(&self.snapshot())?;
        static REPORT: std::sync::OnceLock<regex::Regex> = std::sync::OnceLock::new();
        let pattern = REPORT.get_or_init(|| {
            regex::Regex::new(
                r"^(?:\x1b\[[?>]?[\d;]*[cRn]|\x1b\[[IO]|\x1b\][\d;]*[^\x07\x1b]*(?:\x07|\x1b\\))+$",
            )
            .unwrap()
        });
        if data.len() > 256 || !pattern.is_match(data) {
            return Err(HostError::new("TERMINAL_REPORT", "Invalid terminal report"));
        }
        self.raw_write(data)
    }

    fn raw_write(&self, data: &str) -> HostResult<()> {
        if data.len() > 1024 * 1024 {
            return Err(HostError::new(
                "TERMINAL_LIMIT",
                "Terminal input is too large",
            ));
        }
        let (result, receiver) = mpsc::sync_channel(1);
        {
            let sender = self.writer.lock().unwrap();
            let sender = sender
                .as_ref()
                .ok_or_else(|| HostError::new("TERMINAL_EXITED", "Terminal is not running"))?;
            sender
                .try_send(WriteRequest {
                    data: data.into(),
                    result,
                })
                .map_err(|_| {
                    HostError::new("TERMINAL_BUSY", "Terminal input queue is full or closed")
                })?;
        }
        match receiver.recv_timeout(Duration::from_secs(5)) {
            Ok(Ok(())) => Ok(()),
            Ok(Err(error)) => Err(HostError::new("TERMINAL_IO", error)),
            Err(_) => {
                let _ = self.tree.terminate();
                Err(HostError::new(
                    "TERMINAL_TIMEOUT",
                    "Terminal input timed out",
                ))
            }
        }
    }

    pub fn resize(&self, cols: u16, rows: u16) -> HostResult<TerminalSnapshot> {
        require_running(&self.snapshot())?;
        self.master
            .lock()
            .unwrap()
            .as_ref()
            .ok_or_else(|| HostError::new("TERMINAL_EXITED", "Terminal is not running"))?
            .resize(PtySize {
                rows,
                cols,
                pixel_width: 0,
                pixel_height: 0,
            })
            .map_err(pty_error)?;
        {
            let mut journal = self.journal.lock().unwrap();
            journal.snapshot.cols = cols;
            journal.snapshot.rows = rows;
        }
        Ok(self.state())
    }

    pub fn owner(&self, owner: &str) -> HostResult<TerminalSnapshot> {
        let mut journal = self.journal.lock().unwrap();
        require_running(&journal.snapshot)?;
        journal.snapshot.owner = owner.into();
        drop(journal);
        Ok(self.state())
    }

    pub fn close(&self) -> HostResult<()> {
        {
            let mut journal = self.journal.lock().unwrap();
            if journal.snapshot.status != "closed" {
                journal.snapshot.status = "closed".into();
                journal.snapshot.sequence += 1;
                (self.emit)(
                    &journal.snapshot.workspace_id,
                    json!({"action":"state","terminal":journal.snapshot}),
                );
            }
        }
        self.tree.terminate()?;
        let _ = self.killer.lock().unwrap().kill();
        if !wait_for(&self.exited, Duration::from_secs(5)) {
            return Err(HostError::new(
                "TERMINAL_TIMEOUT",
                "Terminal process did not exit within five seconds",
            ));
        }
        self.writer.lock().unwrap().take();
        self.master.lock().unwrap().take();
        if !wait_for(&self.writer_done, Duration::from_secs(3)) {
            return Err(HostError::new(
                "TERMINAL_TIMEOUT",
                "Terminal input writer did not stop",
            ));
        }
        if !wait_for(&self.reader_done, Duration::from_secs(3)) {
            return Err(HostError::new(
                "TERMINAL_TIMEOUT",
                "Terminal output reader did not stop",
            ));
        }
        Ok(())
    }
}

fn require_running(snapshot: &TerminalSnapshot) -> HostResult<()> {
    if snapshot.status != "running" {
        return Err(HostError::new(
            "TERMINAL_EXITED",
            format!("Terminal is {}", snapshot.status),
        ));
    }
    Ok(())
}

fn wait_for(latch: &(Mutex<bool>, Condvar), timeout: Duration) -> bool {
    let deadline = Instant::now() + timeout;
    let mut done = latch.0.lock().unwrap();
    while !*done {
        let Some(remaining) = deadline.checked_duration_since(Instant::now()) else {
            return false;
        };
        let result = latch.1.wait_timeout(done, remaining).unwrap();
        done = result.0;
    }
    true
}

pub fn pty_error(error: impl std::fmt::Display) -> HostError {
    HostError::new("TERMINAL_PTY", error.to_string())
}
