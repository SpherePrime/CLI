use super::files;
use crate::{
    error::{HostError, HostResult},
    state::AppState,
};
use serde_json::{json, Value};
use std::io::Write;
use tauri::Manager;

fn schema(name: &str, description: &str, mut properties: Value, required: &[&str]) -> Value {
    properties["profileId"] = json!({"type":"string","description":"SSH profile ID or name. Omit in a remote workspace."});
    json!({"name":name,"description":description,"inputSchema":{"type":"object","properties":properties,"required":required,"additionalProperties":false}})
}

pub fn definitions() -> Vec<Value> {
    vec![
        schema("ssh_profiles", "List saved SSH profiles and connection status. Passwords and private key material are never returned.", json!({}), &[]),
        schema("ssh_connect", "Connect a saved SSH profile by profile ID or profile name using stored credentials or SSH agent and ~/.ssh/known_hosts. Returns status \"verifying\" when manual host verification is required in the SSH panel. Never request passwords in chat.", json!({}), &["profileId"]),
        schema("ssh_disconnect", "Disconnect a saved SSH profile only when requested.", json!({}), &["profileId"]),
        schema("ssh_exec", "Run a command in the selected SSH root. timeout is milliseconds, maximum 120000. Use SSH tools for remote workspace operations; no local execution fallback.", json!({"command":{"type":"string"},"description":{"type":"string"},"workingDir":{"type":"string"},"timeout":{"type":"integer","minimum":1,"maximum":120000}}), &["command"]),
        schema("ssh_list_files", "List files in a connected SSH root, respecting ignore rules and root boundaries.", json!({"path":{"type":"string"}}), &[]),
        schema("ssh_read_file", "Read a UTF-8 remote file (up to 2 MiB) or image and return its content revision. offset is a zero-based line index; limit is at most 2000 lines.", json!({"path":{"type":"string"},"offset":{"type":"integer","minimum":0},"limit":{"type":"integer","minimum":1,"maximum":2000}}), &["path"]),
        schema("ssh_write_file", "Create or save a remote text file with revision conflict protection. Optional revision must match the current content.", json!({"path":{"type":"string"},"content":{"type":"string"},"revision":{"type":"string"}}), &["path","content"]),
        schema("ssh_edit_file", "Replace exact text in a remote file with revision conflict protection.", json!({"path":{"type":"string"},"oldText":{"type":"string","minLength":1},"newText":{"type":"string"},"replaceAll":{"type":"boolean"}}), &["path","oldText","newText"]),
        schema("ssh_search", "Search remote file contents with a regular expression using ripgrep or grep.", json!({"pattern":{"type":"string"},"path":{"type":"string"}}), &["pattern"]),
        schema("ssh_glob", "Find remote file paths by a glob using ripgrep or find.", json!({"pattern":{"type":"string"},"path":{"type":"string"}}), &["pattern"]),
        schema("ssh_create_entry", "Create a new remote file or folder without replacing an existing entry.", json!({"path":{"type":"string"},"kind":{"type":"string","enum":["file","folder"]},"content":{"type":"string"}}), &["path","kind"]),
        schema("ssh_upload_file", "Upload a local workspace file inside the SSH root. Maximum 50 MiB. Existing remote files require overwrite=true.", json!({"localPath":{"type":"string"},"path":{"type":"string"},"overwrite":{"type":"boolean"}}), &["localPath","path"]),
        schema("ssh_download", "Download an HTTP(S) URL directly on the SSH server into a new project file. Maximum 50 MiB, existing files are never replaced.", json!({"url":{"type":"string"},"path":{"type":"string"}}), &["url","path"]),
        schema("ssh_download_file", "Download a remote file into a new local workspace file. Maximum 20 MiB.", json!({"remotePath":{"type":"string"},"localPath":{"type":"string"}}), &["remotePath","localPath"]),
        schema("ssh_forward_port", "Forward a remote loopback TCP port to a new local loopback port through SSH. Use the returned URL for remote development servers in the built-in browser. localPort defaults to an available port.", json!({"remotePort":{"type":"integer","minimum":1,"maximum":65535},"localPort":{"type":"integer","minimum":0,"maximum":65535},"remoteHost":{"type":"string","enum":["127.0.0.1","localhost","::1"]}}), &["remotePort"]),
        schema("ssh_forwards", "List active native SSH loopback forwards and their local URLs.", json!({}), &[]),
        schema("ssh_close_forward", "Close an existing SSH forward and its active TCP connections.", json!({"id":{"type":"string"}}), &["id"]),
    ]
}

fn string<'a>(args: &'a Value, key: &str) -> HostResult<&'a str> {
    args[key]
        .as_str()
        .ok_or_else(|| HostError::new("INVALID_ARGUMENT", format!("Expected {key}")))
}

fn connection_failure_result(
    name: &str,
    profile_id: &str,
    status: Option<super::types::ConnectionStatus>,
) -> HostResult<Value> {
    match status {
        Some(status) if status.status == "verifying" => {
            if let Some(challenge) = status.challenge {
                if name == "ssh_connect" {
                    return Ok(json!({
                        "id": profile_id,
                        "status": "verifying",
                        "challenge": challenge,
                        "message": "Host key verification required in Prime Desktop SSH panel"
                    }));
                }
            }
            Err(HostError::new(
                "SSH_HOST_KEY",
                "Host key verification required in Prime Desktop SSH panel",
            ))
        }
        Some(status) if status.status == "error" => Err(HostError::new(
            "SSH_CONNECT",
            status
                .error
                .unwrap_or_else(|| "remote.connectionFailed".to_owned()),
        )),
        _ => Err(HostError::new("SSH_DISCONNECTED", "remote.notConnected")),
    }
}

pub fn run(
    app: &tauri::AppHandle,
    workspace_id: &str,
    name: &str,
    args: &Value,
) -> HostResult<Value> {
    let state = app.state::<AppState>();
    let root = state.root(workspace_id)?;
    let scoped_profile = state
        .projects
        .lock()
        .unwrap()
        .get(workspace_id)
        .and_then(|project| project.get("remote"))
        .and_then(|remote| remote["profileId"].as_str())
        .map(str::to_owned);
    if name == "ssh_profiles" {
        state.remotes.prune_closed();
        let profiles = state.remotes.snapshot()["profiles"]
            .as_array()
            .cloned()
            .unwrap_or_default()
            .into_iter()
            .map(|mut profile| {
                profile.as_object_mut().unwrap().remove("keyPath");
                profile
            })
            .collect::<Vec<_>>();
        return Ok(json!({"profiles":profiles}));
    }
    if name == "ssh_forwards" {
        let forwards = state.remotes.forwards.list();
        let profile = match args["profileId"].as_str() {
            Some(requested) => Some(state.remotes.resolve_profile_id(requested)?),
            None => scoped_profile,
        };
        return Ok(if let Some(profile) = profile {
            json!(forwards
                .as_array()
                .unwrap()
                .iter()
                .filter(|forward| forward["profileId"] == profile)
                .cloned()
                .collect::<Vec<_>>())
        } else {
            forwards
        });
    }
    let raw_profile = match (scoped_profile.as_deref(), args["profileId"].as_str()) {
        (_, Some(requested)) => requested,
        (Some(scoped), _) => scoped,
        _ => {
            return Err(HostError::new(
                "SSH_PROFILE",
                "Select a connected SSH profile using profileId",
            ))
        }
    };
    let profile_id = state.remotes.resolve_profile_id(raw_profile)?;
    if name == "ssh_disconnect" {
        super::detach(app, &profile_id)?;
        return state
            .remotes
            .dispatch(app, &json!({"action":"disconnect","id":profile_id}));
    }
    if state.remotes.connection(&profile_id).is_err() {
        state.remotes.connect_or_wait(
            app,
            &profile_id,
            super::types::Credentials::default(),
        )?;
    }
    let connection = match state.remotes.connection(&profile_id) {
        Ok(connection) => connection,
        Err(_) => {
            return connection_failure_result(
                name,
                &profile_id,
                state.remotes.status(&profile_id),
            );
        }
    };
    if name == "ssh_connect" {
        return Ok(json!({"id":profile_id,"status":"connected"}));
    }
    if name == "ssh_forward_port" {
        let port = |key: &str, default: Option<u16>| -> HostResult<u16> {
            match args.get(key) {
                Some(value) => value
                    .as_u64()
                    .and_then(|value| u16::try_from(value).ok())
                    .ok_or_else(|| HostError::new("INVALID_ARGUMENT", format!("Invalid {key}"))),
                None => default
                    .ok_or_else(|| HostError::new("INVALID_ARGUMENT", format!("Missing {key}"))),
            }
        };
        return state.remotes.forward(
            &profile_id,
            port("remotePort", None)?,
            port("localPort", Some(0))?,
            args["remoteHost"].as_str().unwrap_or("127.0.0.1"),
        );
    }
    if name == "ssh_close_forward" {
        return state
            .remotes
            .forwards
            .close_owned(&profile_id, string(args, "id")?);
    }
    let connection = connection.lock().unwrap();
    match name {
        "ssh_exec" => connection.execute(
            string(args, "command")?,
            args["workingDir"].as_str().unwrap_or(""),
            args["timeout"]
                .as_u64()
                .unwrap_or(30000)
                .div_ceil(1000)
                .clamp(1, 120),
        ),
        "ssh_list_files" => {
            let entries = files::list(&connection, args["path"].as_str().unwrap_or(""))?;
            Ok(
                json!({"path":args["path"].as_str().unwrap_or(""),"entries":entries.as_array().unwrap().iter().take(2000).cloned().collect::<Vec<_>>(),"truncated":entries.as_array().unwrap().len()>2000}),
            )
        }
        "ssh_read_file" => {
            let mut file = files::read(&connection, string(args, "path")?)?;
            if file.get("image").is_some() || file.get("video").is_some() {
                return Ok(file);
            }
            let offset = args["offset"].as_u64().unwrap_or(0).min(usize::MAX as u64) as usize;
            let limit = args["limit"].as_u64().unwrap_or(400).clamp(1, 2000) as usize;
            let lines = file["content"]
                .as_str()
                .unwrap()
                .split('\n')
                .collect::<Vec<_>>();
            let content = lines
                .iter()
                .skip(offset)
                .take(limit)
                .copied()
                .collect::<Vec<_>>()
                .join("\n");
            let mut boundary = content.len().min(128 * 1024);
            while !content.is_char_boundary(boundary) {
                boundary -= 1;
            }
            let total = lines.len();
            let truncated = offset.saturating_add(limit) < total || boundary < content.len();
            file["content"] = json!(&content[..boundary]);
            file["offset"] = json!(offset);
            file["totalLines"] = json!(total);
            file["truncated"] = json!(truncated);
            Ok(file)
        }
        "ssh_write_file" => {
            let path = string(args, "path")?;
            let content = string(args, "content")?;
            match files::read(&connection, path) {
                Ok(current) => files::save(
                    &connection,
                    path,
                    content,
                    args["revision"]
                        .as_str()
                        .unwrap_or_else(|| current["revision"].as_str().unwrap_or("")),
                ),
                Err(error)
                    if args.get("revision").is_none()
                        && (error.message.contains("no such file")
                            || error.message.contains("No such file")) =>
                {
                    files::create(&connection, path, "file", content.as_bytes())
                }
                Err(error) => Err(error),
            }
        }
        "ssh_edit_file" => {
            let path = string(args, "path")?;
            let old = string(args, "oldText")?;
            let new = string(args, "newText")?;
            if old.is_empty() {
                return Err(HostError::new(
                    "INVALID_ARGUMENT",
                    "oldText must not be empty",
                ));
            }
            let current = files::read(&connection, path)?;
            let content = current["content"]
                .as_str()
                .ok_or_else(|| HostError::new("REMOTE_FILE", "remote.binaryFile"))?;
            let count = content.matches(old).count();
            let all = args["replaceAll"].as_bool().unwrap_or(false);
            if count == 0 || count > 1 && !all {
                return Err(HostError::new(
                    "REMOTE_EDIT",
                    "oldText must match exactly once unless replaceAll is enabled",
                ));
            }
            let updated = if all {
                content.replace(old, new)
            } else {
                content.replacen(old, new, 1)
            };
            files::save(
                &connection,
                path,
                &updated,
                current["revision"].as_str().unwrap(),
            )
        }
        "ssh_search" | "ssh_glob" => {
            let pattern = string(args, "pattern")?;
            if pattern.is_empty() || pattern.len() > 4096 {
                return Err(HostError::new("INVALID_ARGUMENT", "Invalid search pattern"));
            }
            let command = if name == "ssh_search" {
                build_ssh_search_command(pattern)
            } else {
                build_ssh_glob_command(pattern)
            };
            connection.execute(&command, args["path"].as_str().unwrap_or(""), 30)
        }
        "ssh_create_entry" => files::create(
            &connection,
            string(args, "path")?,
            string(args, "kind")?,
            args["content"].as_str().unwrap_or("").as_bytes(),
        ),
        "ssh_upload_file" => {
            let source = string(args, "localPath")?;
            let path = if std::path::Path::new(source).is_absolute() {
                dunce::canonicalize(source)?
            } else {
                crate::files::scoped_path(&root, source, false)?
            };
            if path.metadata()?.len() > 50 * 1024 * 1024 {
                return Err(HostError::new("FILE_TOO_LARGE", "Upload exceeds 50 MiB"));
            }
            files::upload(
                &connection,
                string(args, "path")?,
                &std::fs::read(path)?,
                args["overwrite"].as_bool().unwrap_or(false),
            )
        }
        "ssh_download" => {
            files::download_url(&connection, string(args, "path")?, string(args, "url")?)
        }
        "ssh_download_file" => {
            let path = crate::files::scoped_path(&root, string(args, "localPath")?, true)?;
            let bytes = files::bytes(&connection, string(args, "remotePath")?, 20 * 1024 * 1024)?;
            let mut file = std::fs::OpenOptions::new()
                .write(true)
                .create_new(true)
                .open(path)?;
            file.write_all(&bytes)?;
            file.sync_all()?;
            Ok(json!({"path":args["localPath"],"bytes":bytes.len()}))
        }
        _ => Err(HostError::unsupported(name)),
    }
}

const REMOTE_SKIP_DIRS: [&str; 11] = [
    ".git",
    "node_modules",
    "dist",
    "build",
    "target",
    ".next",
    ".turbo",
    ".venv",
    "__pycache__",
    "vendor",
    ".prime",
];

fn build_ssh_search_command(pattern: &str) -> String {
    let quoted = super::connection::shell_quote(pattern);
    let grep_excludes = REMOTE_SKIP_DIRS
        .iter()
        .map(|dir| format!("--exclude-dir={dir}"))
        .collect::<Vec<_>>()
        .join(" ");
    format!(
        "if command -v rg >/dev/null 2>&1; then rg -n --no-heading --color never -m 100 -- {quoted} .; else grep -RInI -m 100 {grep_excludes} -- {quoted} .; fi"
    )
}

fn build_ssh_glob_command(pattern: &str) -> String {
    let quoted = super::connection::shell_quote(pattern);
    let find = if pattern.contains('/') {
        format!(
            "-path {}",
            super::connection::shell_quote(&format!("./{}", pattern.trim_start_matches("./")))
        )
    } else {
        format!("-name {quoted}")
    };
    let find_prunes = REMOTE_SKIP_DIRS
        .iter()
        .map(|dir| format!("-name {dir}"))
        .collect::<Vec<_>>()
        .join(" -o ");
    format!(
        "if command -v rg >/dev/null 2>&1; then rg --files --hidden -g '!.git' -g {quoted} -- .; else find . -type d \\( {find_prunes} \\) -prune -o -type f {find} -print | head -n 200; fi"
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ssh_search_and_glob_skip_heavy_directories_and_cap_find_output() {
        let search = build_ssh_search_command("hello");
        let glob = build_ssh_glob_command("*.rs");
        for dir in REMOTE_SKIP_DIRS {
            assert!(
                search.contains(&format!("--exclude-dir={dir}")),
                "missing --exclude-dir={dir}"
            );
            assert!(
                glob.contains(&format!("-name {dir}")),
                "missing -name {dir} in find prune"
            );
        }
        assert!(glob.ends_with("| head -n 200; fi"));
    }

    #[test]
    fn ssh_connect_and_tool_error_surfacing_for_connection_statuses() {
        let verifying = super::super::types::ConnectionStatus {
            status: "verifying".into(),
            error: None,
            challenge: Some(super::super::types::Challenge {
                fingerprint: "SHA256:testfp".into(),
                previous: None,
            }),
        };

        let connect_result =
            connection_failure_result("ssh_connect", "prof-1", Some(verifying.clone())).unwrap();
        assert_eq!(connect_result["id"], "prof-1");
        assert_eq!(connect_result["status"], "verifying");
        assert_eq!(connect_result["challenge"]["fingerprint"], "SHA256:testfp");
        assert_eq!(
            connect_result["message"],
            "Host key verification required in Prime Desktop SSH panel"
        );

        let exec_err =
            connection_failure_result("ssh_exec", "prof-1", Some(verifying)).unwrap_err();
        assert_eq!(exec_err.code, "SSH_HOST_KEY");
        assert_eq!(
            exec_err.message,
            "Host key verification required in Prime Desktop SSH panel"
        );

        let err_status = super::super::types::ConnectionStatus {
            status: "error".into(),
            error: Some("remote.authFailed".into()),
            challenge: None,
        };
        let connect_err =
            connection_failure_result("ssh_connect", "prof-1", Some(err_status)).unwrap_err();
        assert_eq!(connect_err.code, "SSH_CONNECT");
        assert_eq!(connect_err.message, "remote.authFailed");

        let disconnected_err = connection_failure_result("ssh_exec", "prof-1", None).unwrap_err();
        assert_eq!(disconnected_err.code, "SSH_DISCONNECTED");
        assert_eq!(disconnected_err.message, "remote.notConnected");
    }

    #[test]
    fn ssh_connect_definition_documents_name_known_hosts_and_verifying() {
        let defs = definitions();
        let ssh_connect = defs
            .iter()
            .find(|def| def["name"] == "ssh_connect")
            .unwrap();
        let description = ssh_connect["description"].as_str().unwrap();
        assert!(description.contains("known_hosts"));
        assert!(description.contains("verifying"));
        assert!(description.contains("name"));
    }
}
