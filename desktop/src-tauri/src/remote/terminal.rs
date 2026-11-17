use super::connection::{shell_quote, Connection};
use super::runtime;
use crate::error::{HostError, HostResult};
use russh::ChannelMsg;
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    sync::{Arc, Mutex},
    time::{Duration, SystemTime, UNIX_EPOCH},
};
use tokio::sync::{mpsc, oneshot};

pub type Emitter = Arc<dyn Fn(&str, Value) + Send + Sync>;
const OUTPUT_LIMIT: usize = 2 * 1024 * 1024;

enum Input {
    Write(String, oneshot::Sender<HostResult<()>>),
    Resize(u32, u32, oneshot::Sender<HostResult<()>>),
    Close(oneshot::Sender<HostResult<()>>),
}

struct Journal {
    snapshot: Value,
    output: String,
    pending: Vec<u8>,
    truncated: bool,
}

pub struct Terminal {
    profile: String,
    journal: Mutex<Journal>,
    input: mpsc::UnboundedSender<Input>,
    emit: Emitter,
}

impl Terminal {
    fn start(
        connection: Connection,
        workspace: &str,
        options: &Value,
        emit: Emitter,
        owner: &str,
    ) -> HostResult<Arc<Self>> {
        let dimension = |key: &str, fallback: u32, minimum: u32, maximum: u32| -> HostResult<u32> {
            let value = options
                .get(key)
                .map(|value| {
                    value.as_u64().ok_or_else(|| {
                        HostError::new("INVALID_ARGUMENT", "Invalid terminal dimensions")
                    })
                })
                .transpose()?
                .unwrap_or(fallback as u64);
            if value < minimum as u64 || value > maximum as u64 {
                return Err(HostError::new(
                    "INVALID_ARGUMENT",
                    "Terminal dimensions are out of range",
                ));
            }
            Ok(value as u32)
        };
        let cols = dimension("cols", 100, 20, 500)?;
        let rows = dimension("rows", 30, 5, 300)?;
        let shell = options["shell"].as_str().unwrap_or("/bin/sh");
        if shell.is_empty() || shell.contains('\0') || shell.len() > 4096 {
            return Err(HostError::new("INVALID_ARGUMENT", "Invalid remote shell"));
        }
        let mut command = format!(
            "cd {} && exec {}",
            shell_quote(&connection.root),
            shell_quote(shell)
        );
        if let Some(arguments) = options["args"].as_array() {
            if arguments.len() > 64 {
                return Err(HostError::new(
                    "INVALID_ARGUMENT",
                    "Too many shell arguments",
                ));
            }
            for argument in arguments {
                let argument = argument
                    .as_str()
                    .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Invalid shell argument"))?;
                if argument.contains('\0') || argument.len() > 8192 {
                    return Err(HostError::new("INVALID_ARGUMENT", "Invalid shell argument"));
                }
                command.push(' ');
                command.push_str(&shell_quote(argument));
            }
        } else {
            command.push_str(" -l");
        }
        let channel = connection.open_pty_channel(cols, rows, &command)?;
        let (mut reader, writer) = channel.split();
        let id = uuid::Uuid::new_v4().to_string();
        let (input, mut receiver) = mpsc::unbounded_channel();
        let terminal = Arc::new(Self {
            profile: connection.profile.id.clone(),
            journal: Mutex::new(Journal {
                snapshot: json!({"id":id,"workspaceId":workspace,"title":options["title"].as_str().unwrap_or("SSH Terminal"),"shell":shell,"cwd":connection.root,"pid":0,"cols":cols,"rows":rows,"owner":owner,"status":"running","createdAt":SystemTime::now().duration_since(UNIX_EPOCH).unwrap_or_default().as_millis() as u64,"sequence":0}),
                output: String::new(),
                pending: Vec::new(),
                truncated: false,
            }),
            input,
            emit,
        });
        let worker = terminal.clone();
        runtime::runtime().spawn(async move {
            let mut closed = false;
            let mut exit_code: Option<u32> = None;
            loop {
                tokio::select! {
                    input = receiver.recv() => {
                        match input {
                            Some(Input::Write(data, response)) => {
                                let result = writer
                                    .data_bytes(data.into_bytes())
                                    .await
                                    .map_err(ssh_error);
                                let _ = response.send(result);
                            }
                            Some(Input::Resize(cols, rows, response)) => {
                                let result = writer
                                    .window_change(cols, rows, 0, 0)
                                    .await
                                    .map_err(ssh_error);
                                let _ = response.send(result);
                            }
                            Some(Input::Close(response)) => {
                                let _ = writer.eof().await;
                                let _ = writer.close().await;
                                connection.close_async().await;
                                closed = true;
                                let _ = response.send(Ok(()));
                                break;
                            }
                            None => break,
                        }
                    }
                    message = reader.wait() => {
                        match message {
                            Some(ChannelMsg::Data { data }) => {
                                worker.append(&data, false);
                            }
                            Some(ChannelMsg::ExtendedData { data, .. }) => {
                                worker.append(&data, false);
                            }
                            Some(ChannelMsg::ExitStatus { exit_status }) => {
                                exit_code = Some(exit_status);
                            }
                            Some(ChannelMsg::ExitSignal { .. }) => {
                                if exit_code.is_none() {
                                    exit_code = Some(255);
                                }
                            }
                            None => break,
                            Some(_) => {}
                        }
                    }
                }
            }
            worker.append(&[], true);
            let exit = exit_code.unwrap_or(255);
            let snapshot = {
                let mut journal = worker.journal.lock().unwrap();
                journal.snapshot["sequence"] =
                    json!(journal.snapshot["sequence"].as_u64().unwrap() + 1);
                journal.snapshot["status"] = json!(if closed { "closed" } else { "exited" });
                journal.snapshot["exitCode"] = json!(exit);
                journal.snapshot.clone()
            };
            (worker.emit)(
                workspace_id(&snapshot),
                json!({"action":"exit","id":snapshot["id"],"exitCode":exit,"sequence":snapshot["sequence"]}),
            );
            (worker.emit)(
                workspace_id(&snapshot),
                json!({"action":"state","terminal":snapshot}),
            );
            connection.close_async().await;
        });
        terminal.emit_state();
        Ok(terminal)
    }

    fn append(&self, bytes: &[u8], finish: bool) {
        let mut journal = self.journal.lock().unwrap();
        journal.pending.extend_from_slice(bytes);
        let text = decode(&mut journal.pending, finish);
        if text.is_empty() {
            return;
        }
        journal.output.push_str(&text);
        if journal.output.len() > OUTPUT_LIMIT {
            let mut boundary = journal.output.len() - OUTPUT_LIMIT;
            while !journal.output.is_char_boundary(boundary) {
                boundary += 1;
            }
            journal.output.drain(..boundary);
            journal.truncated = true;
        }
        journal.snapshot["sequence"] = json!(journal.snapshot["sequence"].as_u64().unwrap() + 1);
        let snapshot = journal.snapshot.clone();
        drop(journal);
        (self.emit)(
            workspace_id(&snapshot),
            json!({"action":"data","id":snapshot["id"],"data":text,"sequence":snapshot["sequence"]}),
        );
    }

    pub fn snapshot(&self) -> Value {
        self.journal.lock().unwrap().snapshot.clone()
    }

    fn emit_state(&self) {
        let snapshot = self.snapshot();
        (self.emit)(
            workspace_id(&snapshot),
            json!({"action":"state","terminal":snapshot}),
        );
    }

    pub fn read(&self) -> Value {
        let journal = self.journal.lock().unwrap();
        json!({"terminal":journal.snapshot,"output":journal.output,"sequence":journal.snapshot["sequence"],"truncated":journal.truncated})
    }

    fn send(&self, build: impl FnOnce(oneshot::Sender<HostResult<()>>) -> Input) -> HostResult<()> {
        let (response, receiver) = oneshot::channel();
        self.input
            .send(build(response))
            .map_err(|_| HostError::new("TERMINAL_EXITED", "SSH terminal has exited"))?;
        runtime::block_on(async move {
            match tokio::time::timeout(Duration::from_secs(20), receiver).await {
                Ok(Ok(result)) => result,
                Ok(Err(_)) => Err(HostError::new("TERMINAL_EXITED", "SSH terminal has exited")),
                Err(_) => Err(HostError::new("TERMINAL_TIMEOUT", "SSH terminal did not respond")),
            }
        })
    }

    fn write(&self, data: &str, source: &str, report: bool) -> HostResult<()> {
        let snapshot = self.snapshot();
        if snapshot["status"] != "running" {
            return Err(HostError::new("TERMINAL_EXITED", "SSH terminal has exited"));
        }
        if !report && snapshot["owner"] != source {
            return Err(HostError::new(
                "TERMINAL_OWNER",
                "Terminal ownership must be transferred before writing",
            ));
        }
        if data.len() > 1024 * 1024 {
            return Err(HostError::new(
                "TERMINAL_LIMIT",
                "Terminal input exceeds 1 MiB",
            ));
        }
        self.send(|response| Input::Write(data.into(), response))
    }

    fn resize(&self, cols: u32, rows: u32) -> HostResult<Value> {
        if !(20..=500).contains(&cols) || !(5..=300).contains(&rows) {
            return Err(HostError::new(
                "INVALID_ARGUMENT",
                "Terminal dimensions are out of range",
            ));
        }
        self.send(|response| Input::Resize(cols, rows, response))?;
        {
            let mut journal = self.journal.lock().unwrap();
            journal.snapshot["cols"] = json!(cols);
            journal.snapshot["rows"] = json!(rows);
        }
        self.emit_state();
        Ok(self.snapshot())
    }

    fn close(&self) -> HostResult<()> {
        if self.snapshot()["status"] != "running" {
            return Ok(());
        }
        self.send(Input::Close)
    }
}

fn workspace_id(snapshot: &Value) -> &str {
    snapshot["workspaceId"].as_str().unwrap()
}

fn ssh_error(error: russh::Error) -> HostError {
    HostError::new("SSH_TERMINAL", error.to_string())
}

fn decode(pending: &mut Vec<u8>, finish: bool) -> String {
    let mut output = String::new();
    loop {
        match std::str::from_utf8(pending) {
            Ok(text) => {
                output.push_str(text);
                pending.clear();
                break;
            }
            Err(error) => {
                let valid = error.valid_up_to();
                if valid > 0 {
                    output.push_str(std::str::from_utf8(&pending[..valid]).unwrap());
                    pending.drain(..valid);
                }
                match error.error_len() {
                    Some(length) => {
                        output.push('\u{fffd}');
                        pending.drain(..length);
                    }
                    None => {
                        if finish {
                            output.push('\u{fffd}');
                            pending.clear();
                        }
                        break;
                    }
                }
            }
        }
    }
    output
}

#[derive(Default)]
pub struct Terminals {
    items: Mutex<HashMap<String, Arc<Terminal>>>,
}

impl Terminals {
    pub fn count(&self) -> usize {
        self.items
            .lock()
            .unwrap()
            .values()
            .filter(|terminal| terminal.snapshot()["status"] == "running")
            .count()
    }

    pub fn call(
        &self,
        manager: &super::RemoteManager,
        app: tauri::AppHandle,
        profile: &str,
        workspace: &str,
        request: &Value,
        source: &str,
    ) -> HostResult<Value> {
        let emit: Emitter = Arc::new(move |workspace, payload| {
            crate::events::emit(&app, workspace, "terminal", payload)
        });
        let action = request["action"]
            .as_str()
            .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing terminal action"))?;
        if source == "agent"
            && !matches!(
                action,
                "list" | "read" | "write" | "resize" | "create" | "close" | "release"
            )
        {
            return Err(HostError::new(
                "TERMINAL_OWNER",
                "Only the user can transfer existing terminal ownership",
            ));
        }
        if action == "list" {
            return Ok(json!(self
                .items
                .lock()
                .unwrap()
                .values()
                .map(|terminal| terminal.snapshot())
                .filter(|snapshot| snapshot["workspaceId"] == workspace)
                .collect::<Vec<_>>()));
        }
        if action == "create" {
            if self.items.lock().unwrap().len() >= 12 {
                return Err(HostError::new("TERMINAL_LIMIT", "Maximum 12 SSH terminals"));
            }
            let connection = manager.dedicated_connection(profile)?;
            let terminal = Terminal::start(
                connection,
                workspace,
                request.get("options").unwrap_or(&Value::Null),
                emit,
                source,
            )?;
            let snapshot = terminal.snapshot();
            self.items
                .lock()
                .unwrap()
                .insert(snapshot["id"].as_str().unwrap().into(), terminal);
            return Ok(snapshot);
        }
        let id = request["id"]
            .as_str()
            .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing terminal id"))?;
        let terminal = self
            .items
            .lock()
            .unwrap()
            .get(id)
            .cloned()
            .ok_or_else(|| HostError::new("TERMINAL_NOT_FOUND", "SSH terminal not found"))?;
        if terminal.snapshot()["workspaceId"] != workspace {
            return Err(HostError::new(
                "TERMINAL_SCOPE",
                "SSH terminal is outside this workspace",
            ));
        }
        if source == "agent"
            && matches!(action, "close" | "release")
            && terminal.snapshot()["owner"] != "agent"
        {
            return Err(HostError::new(
                "TERMINAL_OWNER",
                "Only agent-owned terminals can be closed or released by the agent",
            ));
        }
        match action {
            "read" => Ok(terminal.read()),
            "write" | "report" => {
                terminal.write(
                    request["data"].as_str().ok_or_else(|| {
                        HostError::new("INVALID_ARGUMENT", "Missing terminal input")
                    })?,
                    source,
                    action == "report",
                )?;
                Ok(Value::Null)
            }
            "resize" => terminal.resize(
                request["cols"].as_u64().unwrap_or(0) as u32,
                request["rows"].as_u64().unwrap_or(0) as u32,
            ),
            "close" => {
                terminal.close()?;
                self.items.lock().unwrap().remove(id);
                Ok(Value::Null)
            }
            "owner" => {
                let owner = request["owner"]
                    .as_str()
                    .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing terminal owner"))?;
                if !["user", "agent"].contains(&owner) {
                    return Err(HostError::new("INVALID_ARGUMENT", "Invalid terminal owner"));
                }
                terminal.journal.lock().unwrap().snapshot["owner"] = json!(owner);
                terminal.emit_state();
                Ok(terminal.snapshot())
            }
            "release" if source == "agent" => {
                terminal.journal.lock().unwrap().snapshot["owner"] = json!("user");
                terminal.emit_state();
                Ok(terminal.snapshot())
            }
            _ => Err(HostError::unsupported("remote terminal action")),
        }
    }

    pub fn close_workspace(&self, workspace: &str) {
        let items = self
            .items
            .lock()
            .unwrap()
            .values()
            .filter(|terminal| terminal.snapshot()["workspaceId"] == workspace)
            .cloned()
            .collect::<Vec<_>>();
        for terminal in items {
            let _ = terminal.close();
            self.items
                .lock()
                .unwrap()
                .remove(terminal.snapshot()["id"].as_str().unwrap());
        }
    }

    pub fn close_profile(&self, profile: &str) {
        let workspaces = self
            .items
            .lock()
            .unwrap()
            .values()
            .filter(|terminal| terminal.profile == profile)
            .map(|terminal| {
                terminal.snapshot()["workspaceId"]
                    .as_str()
                    .unwrap()
                    .to_owned()
            })
            .collect::<Vec<_>>();
        for workspace in workspaces {
            self.close_workspace(&workspace);
        }
    }
}

#[cfg(test)]
mod tests {
    #[test]
    fn unicode_across_ssh_chunks() {
        let mut pending = vec![0xd0];
        assert_eq!(super::decode(&mut pending, false), "");
        pending.push(0x9f);
        assert_eq!(super::decode(&mut pending, false), "П");
    }

    #[test]
    #[ignore]
    fn native_remote_pty_round_trip() {
        use super::*;
        use crate::remote::{
            connection::{handshake, Handshake},
            types::{Credentials, Profile},
        };
        let key = std::env::var("PRIME_NATIVE_SSH_TEST_KEY")
            .expect("Set PRIME_NATIVE_SSH_TEST_KEY to an isolated SSH fixture identity");
        let root = std::env::var("PRIME_NATIVE_SSH_TEST_ROOT")
            .unwrap_or_else(|_| "/tmp/prime-native-ssh-fixture/project".into());
        let port = std::env::var("PRIME_NATIVE_SSH_TEST_PORT")
            .unwrap_or_else(|_| "22349".into())
            .parse::<u16>()
            .unwrap();
        let profile=serde_json::from_value::<Profile>(json!({"name":"Fixture","host":"127.0.0.1","port":port,"user":"root","root":root,"auth":"key","keyPath":key,"runtime":"bundled"})).unwrap().validate().unwrap();
        let fingerprint = match handshake(&profile, None).unwrap() {
            Handshake::Challenge(challenge) => challenge.fingerprint,
            _ => panic!("Unknown host was trusted"),
        };
        let session = match handshake(&profile, Some(&fingerprint)).unwrap() {
            Handshake::Trusted(session) => session,
            _ => panic!("Expected trusted fixture"),
        };
        let connection =
            Connection::authenticate(profile, session, &Credentials::default()).unwrap();
        let terminal = Terminal::start(
            connection,
            "fixture-workspace",
            &json!({"cols":80,"rows":24}),
            Arc::new(|_, _| {}),
            "user",
        )
        .unwrap();
        assert!(terminal.write("echo unsafe\r", "agent", false).is_err());
        terminal
            .write("printf '\\nREMOTE_PTY_Привет\\n'\r", "user", false)
            .unwrap();
        let deadline = std::time::Instant::now() + Duration::from_secs(10);
        loop {
            if terminal.read()["output"]
                .as_str()
                .unwrap()
                .contains("\r\nREMOTE_PTY_Привет\r\n")
            {
                break;
            }
            assert!(std::time::Instant::now() < deadline, "{}", terminal.read());
            std::thread::sleep(Duration::from_millis(10));
        }
        let resized = terminal.resize(77, 17).unwrap();
        assert_eq!(resized["cols"], 77);
        assert_eq!(resized["rows"], 17);
        terminal.journal.lock().unwrap().snapshot["owner"] = json!("agent");
        assert!(terminal.write("echo blocked\r", "user", false).is_err());
        terminal
            .write("printf 'AGENT_OWNERSHIP_OK\\n'\r", "agent", false)
            .unwrap();
        terminal.close().unwrap();
        let deadline = std::time::Instant::now() + Duration::from_secs(3);
        while terminal.snapshot()["status"] == "running" {
            assert!(std::time::Instant::now() < deadline);
            std::thread::sleep(Duration::from_millis(10));
        }
        assert_eq!(terminal.snapshot()["status"], "closed");
    }
}
