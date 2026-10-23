use super::{
    tools,
    workbench::{self, WorkbenchContext},
    BrowserState,
};
use base64::Engine;
use serde_json::{json, Value};
use std::{
    io::{BufReader, Read, Write},
    net::{TcpListener, TcpStream},
    sync::{
        atomic::{AtomicBool, AtomicUsize, Ordering},
        Arc,
    },
    thread,
    time::Duration,
};
use tauri::{AppHandle, Manager};

const MAX_BODY: usize = 1024 * 1024;
const MAX_HEADERS: usize = 16 * 1024;
const MAX_CONNECTIONS: usize = 16;

pub struct Server {
    pub url: String,
    pub token: String,
    stopping: Arc<AtomicBool>,
}

impl Server {
    pub fn shutdown(&self) {
        self.stopping.store(true, Ordering::Release);
    }
}

impl Drop for Server {
    fn drop(&mut self) {
        self.shutdown();
    }
}

struct Request {
    method: String,
    path: String,
    authorization: String,
    origin: Option<String>,
    workspace: Option<String>,
    length: usize,
}

fn read_headers(reader: &mut impl Read) -> Result<Request, String> {
    let mut bytes = Vec::new();
    while !bytes.ends_with(b"\r\n\r\n") {
        if bytes.len() >= MAX_HEADERS {
            return Err("HTTP headers exceed 16 KiB".to_owned());
        }
        let mut byte = [0u8; 1];
        reader
            .read_exact(&mut byte)
            .map_err(|error| error.to_string())?;
        bytes.push(byte[0]);
    }
    let header = std::str::from_utf8(&bytes).map_err(|error| error.to_string())?;
    let mut lines = header.split("\r\n");
    let first: Vec<&str> = lines.next().unwrap_or("").split_whitespace().collect();
    if first.len() != 3 || !matches!(first[2], "HTTP/1.1" | "HTTP/1.0") {
        return Err("Invalid HTTP request line".to_owned());
    }
    let mut request = Request {
        method: first[0].to_owned(),
        path: first[1].to_owned(),
        authorization: String::new(),
        origin: None,
        workspace: None,
        length: 0,
    };
    let mut content_length = None;
    for line in lines.filter(|line| !line.is_empty()) {
        let (name, value) = line.split_once(':').ok_or("Invalid HTTP header")?;
        let value = value.trim();
        match name.to_ascii_lowercase().as_str() {
            "authorization" => {
                if !request.authorization.is_empty() {
                    return Err("Duplicate authorization header".to_owned());
                }
                request.authorization = value.to_owned();
            }
            "content-length" => {
                if content_length.is_some() {
                    return Err("Duplicate content length".to_owned());
                }
                content_length = Some(
                    value
                        .parse::<usize>()
                        .map_err(|_| "Invalid content length")?,
                );
            }
            "transfer-encoding" => return Err("Chunked requests are not supported".to_owned()),
            "origin" => request.origin = Some(value.to_owned()),
            "x-prime-workspace" => {
                if request.workspace.is_some() {
                    return Err("Duplicate workspace header".to_owned());
                }
                request.workspace = Some(value.to_owned());
            }
            _ => {}
        }
    }
    request.length = content_length.unwrap_or(0);
    if request.length > MAX_BODY {
        return Err("HTTP body exceeds 1 MiB".to_owned());
    }
    Ok(request)
}

fn authorized(header: &str, token: &str) -> bool {
    let expected = format!("Bearer {token}");
    if header.len() != expected.len() {
        return false;
    }
    header
        .as_bytes()
        .iter()
        .zip(expected.as_bytes())
        .fold(0u8, |different, (left, right)| different | (left ^ right))
        == 0
}

fn send(stream: &mut TcpStream, status: u16, value: Option<&Value>) -> Result<(), String> {
    let body = value.map(Value::to_string).unwrap_or_default();
    let reason = match status {
        200 => "OK",
        202 => "Accepted",
        400 => "Bad Request",
        401 => "Unauthorized",
        403 => "Forbidden",
        404 => "Not Found",
        405 => "Method Not Allowed",
        503 => "Service Unavailable",
        _ => "Error",
    };
    write!(stream, "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\nCache-Control: no-store\r\n\r\n{body}", body.len()).map_err(|error| error.to_string())
}

fn reply(id: &Value, result: Value) -> Value {
    json!({"jsonrpc":"2.0","id":id,"result":result})
}
fn fail(id: &Value, code: i32, message: &str) -> Value {
    json!({"jsonrpc":"2.0","id":id,"error":{"code":code,"message":message}})
}

fn resolve_workspace(
    projects: &std::collections::HashMap<String, Value>,
    encoded: Option<&str>,
) -> Result<Option<String>, String> {
    let Some(encoded) = encoded else {
        return Ok(None);
    };
    if encoded.len() > 12000 {
        return Err("Workspace header is too long".to_owned());
    }
    let bytes = base64::engine::general_purpose::URL_SAFE_NO_PAD
        .decode(encoded)
        .map_err(|_| "Invalid workspace header")?;
    let root = std::str::from_utf8(&bytes).map_err(|_| "Invalid workspace path")?;
    Ok(projects
        .iter()
        .find(|(_, project)| {
            project["path"].as_str() == Some(root) || project["localPath"].as_str() == Some(root)
        })
        .map(|(id, _)| id.clone()))
}

fn handle(
    app: &AppHandle,
    state: &BrowserState,
    workspace: Option<&str>,
    message: &Value,
) -> Option<Value> {
    let id = message.get("id")?;
    let Some(method) = message["method"].as_str() else {
        return Some(fail(id, -32600, "Invalid JSON-RPC request"));
    };
    if message["jsonrpc"] != "2.0" {
        return Some(fail(id, -32600, "Invalid JSON-RPC version"));
    }
    Some(match method {
        "initialize" => reply(
            id,
            json!({"protocolVersion":message["params"]["protocolVersion"].as_str().unwrap_or("2025-03-26"),"capabilities":{"tools":{"listChanged":false}},"serverInfo":{"name":"prime-desktop-browser","version":"1.0.0"},"instructions":"Use desktop_capabilities to discover project-bound terminal, app launch, working context, GitHub and SSH tools. Do not supply workspace IDs or roots. Use terminal_create for new agent-owned terminals; existing user-owned terminals require explicit user handover. Use the Prime built-in browser for page automation. browser_tabs returns stable tab IDs. browser_new_tab opens in the background. Pass tabId to page tools to preserve the user's selected tab. browser_select_tab explicitly changes the visible tab. Use browser_snapshot references for clicking and typing."}),
        ),
        "ping" => reply(id, json!({})),
        "tools/list" => {
            let mut definitions = tools::definitions();
            if workspace.is_some() {
                definitions.extend(workbench::definitions());
            }
            reply(id, json!({"tools":definitions}))
        }
        "tools/call" => {
            let params = &message["params"];
            let name = params["name"].as_str().unwrap_or("");
            let arguments = params.get("arguments").cloned().unwrap_or(json!({}));
            let result = if tools::definitions().iter().any(|tool| tool["name"] == name) {
                tools::run(app, state, name, &arguments)
            } else if let Some(workspace_id) = workspace {
                let session_id = params["_meta"]["prime/sessionId"]
                    .as_str()
                    .filter(|id| {
                        !id.is_empty() && id.len() <= 128 && !id.chars().any(char::is_control)
                    })
                    .map(str::to_owned);
                let context = WorkbenchContext {
                    workspace_id: workspace_id.to_owned(),
                    session_id,
                };
                workbench::run(app, &context, name, &arguments)
            } else {
                return Some(fail(id, -32602, "Unknown tool or missing project binding"));
            };
            match result {
                Ok(content) => reply(id, json!({"content":content})),
                Err(error) => reply(
                    id,
                    json!({"content":[{"type":"text","text":error}],"isError":true}),
                ),
            }
        }
        _ => fail(id, -32601, "Method not found"),
    })
}

fn connection(
    mut stream: TcpStream,
    token: &str,
    app: &AppHandle,
    state: &BrowserState,
) -> Result<(), String> {
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .map_err(|error| error.to_string())?;
    stream
        .set_write_timeout(Some(Duration::from_secs(10)))
        .map_err(|error| error.to_string())?;
    let mut reader = BufReader::new(stream.try_clone().map_err(|error| error.to_string())?);
    let request = match read_headers(&mut reader) {
        Ok(request) => request,
        Err(_) => return send(&mut stream, 400, None),
    };
    if request.path != "/mcp" {
        return send(&mut stream, 404, None);
    }
    if !authorized(&request.authorization, token) {
        return send(&mut stream, 401, None);
    }
    if request.origin.is_some() {
        return send(&mut stream, 403, None);
    }
    let workspace = {
        let Some(app_state) = app.try_state::<crate::state::AppState>() else {
            return send(&mut stream, 503, None);
        };
        let projects = app_state
            .projects
            .lock()
            .map_err(|error| error.to_string())?;
        match resolve_workspace(&projects, request.workspace.as_deref()) {
            Ok(workspace) => workspace,
            Err(_) => return send(&mut stream, 400, None),
        }
    };
    if request.method == "DELETE" {
        return send(&mut stream, 200, None);
    }
    if request.method != "POST" {
        return send(&mut stream, 405, None);
    }
    let mut body = vec![0; request.length];
    if reader.read_exact(&mut body).is_err() {
        return send(&mut stream, 400, None);
    }
    let message: Value = match serde_json::from_slice(&body) {
        Ok(message) => message,
        Err(_) => {
            return send(
                &mut stream,
                400,
                Some(&fail(&Value::Null, -32700, "Invalid JSON")),
            )
        }
    };
    let response = if let Some(messages) = message.as_array() {
        if messages.is_empty() || messages.len() > 32 {
            return send(
                &mut stream,
                400,
                Some(&fail(&Value::Null, -32600, "Invalid JSON-RPC batch")),
            );
        }
        let replies: Vec<Value> = messages
            .iter()
            .filter_map(|message| handle(app, state, workspace.as_deref(), message))
            .collect();
        if replies.is_empty() {
            None
        } else {
            Some(json!(replies))
        }
    } else {
        handle(app, state, workspace.as_deref(), &message)
    };
    send(
        &mut stream,
        if response.is_some() { 200 } else { 202 },
        response.as_ref(),
    )
}

pub fn start(app: &AppHandle, state: BrowserState) -> Result<Server, String> {
    let listener =
        TcpListener::bind((std::net::Ipv4Addr::LOCALHOST, 0)).map_err(|error| error.to_string())?;
    listener
        .set_nonblocking(true)
        .map_err(|error| error.to_string())?;
    let url = format!(
        "http://127.0.0.1:{}/mcp",
        listener
            .local_addr()
            .map_err(|error| error.to_string())?
            .port()
    );
    let token = format!(
        "{}{}",
        uuid::Uuid::new_v4().simple(),
        uuid::Uuid::new_v4().simple()
    );
    let stopping = Arc::new(AtomicBool::new(false));
    let worker_stopping = stopping.clone();
    let worker_token = token.clone();
    let app = app.clone();
    thread::spawn(move || {
        let connections = Arc::new(AtomicUsize::new(0));
        while !worker_stopping.load(Ordering::Acquire) {
            match listener.accept() {
                Ok((mut stream, address)) => {
                    if !address.ip().is_loopback() {
                        let _ = send(&mut stream, 403, None);
                        continue;
                    }
                    if connections.load(Ordering::Acquire) >= MAX_CONNECTIONS {
                        let _ = stream.set_write_timeout(Some(Duration::from_secs(1)));
                        let _ = send(&mut stream, 503, None);
                        continue;
                    }
                    connections.fetch_add(1, Ordering::AcqRel);
                    let connections = connections.clone();
                    let token = worker_token.clone();
                    let app = app.clone();
                    let state = state.clone();
                    thread::spawn(move || {
                        let _ = connection(stream, &token, &app, &state);
                        connections.fetch_sub(1, Ordering::AcqRel);
                    });
                }
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    thread::sleep(Duration::from_millis(50))
                }
                Err(_) => break,
            }
        }
    });
    Ok(Server {
        url,
        token,
        stopping,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bearer_token_is_exact() {
        assert!(authorized("Bearer secret", "secret"));
        assert!(!authorized("Bearer secreu", "secret"));
        assert!(!authorized("bearer secret", "secret"));
        assert!(!authorized("", "secret"));
    }

    #[test]
    fn http_parser_preserves_body_boundary() {
        let mut bytes =
            &b"POST /mcp HTTP/1.1\r\nAuthorization: Bearer secret\r\nContent-Length: 2\r\n\r\n{}"[..];
        let request = read_headers(&mut bytes).unwrap();
        assert_eq!(request.path, "/mcp");
        assert_eq!(request.length, 2);
        assert_eq!(bytes, b"{}");
    }

    #[test]
    fn workspace_headers_resolve_only_already_open_projects() {
        let projects =
            std::collections::HashMap::from([("bound".to_owned(), json!({"path":"/project"}))]);
        let known = base64::engine::general_purpose::URL_SAFE_NO_PAD.encode("/project");
        let unknown = base64::engine::general_purpose::URL_SAFE_NO_PAD.encode("/other");
        assert_eq!(
            resolve_workspace(&projects, Some(&known))
                .unwrap()
                .as_deref(),
            Some("bound")
        );
        assert!(resolve_workspace(&projects, Some(&unknown))
            .unwrap()
            .is_none());
        assert!(resolve_workspace(&projects, Some("/not-base64")).is_err());
        assert!(resolve_workspace(&projects, None).unwrap().is_none());
    }

    #[test]
    fn http_parser_rejects_oversize_ambiguous_and_chunked_requests() {
        for headers in [
            "POST /mcp HTTP/1.1\r\nContent-Length: 1048577\r\n\r\n",
            "POST /mcp HTTP/1.1\r\nContent-Length: 0\r\nContent-Length: 0\r\n\r\n",
            "POST /mcp HTTP/1.1\r\nTransfer-Encoding: chunked\r\n\r\n",
            "POST /mcp HTTP/1.1\r\nAuthorization: a\r\nAuthorization: b\r\n\r\n",
        ] {
            assert!(read_headers(&mut headers.as_bytes()).is_err());
        }
    }
}
