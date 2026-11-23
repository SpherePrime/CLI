use crate::{
    error::{HostError, HostResult},
    files,
    state::AppState,
};
use rand::RngCore;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    collections::HashMap,
    fs::File,
    io::{Read, Seek, SeekFrom, Write},
    net::{TcpListener, TcpStream},
    path::{Component, Path, PathBuf},
    sync::{Arc, Mutex, RwLock},
    time::Duration,
};

const STREAM_CHUNK_SIZE: usize = 64 * 1024;
const MAX_HTTP_HEADER_BYTES: usize = 16 * 1024;

struct MediaEntry {
    path: PathBuf,
    mime: String,
}

#[derive(Default)]
struct MediaRegistry {
    entries: HashMap<String, MediaEntry>,
    index: HashMap<(PathBuf, String), String>,
}

pub struct MediaServer {
    token: String,
    port: Mutex<Option<u16>>,
    entries: Arc<RwLock<MediaRegistry>>,
}

impl Default for MediaServer {
    fn default() -> Self {
        Self::new()
    }
}

impl MediaServer {
    pub fn new() -> Self {
        Self {
            token: generate_token(),
            port: Mutex::new(None),
            entries: Arc::new(RwLock::new(MediaRegistry::default())),
        }
    }

    pub fn resolve(
        &self,
        state: &AppState,
        workspace_id: &str,
        raw_path: &str,
    ) -> HostResult<Value> {
        let trimmed = raw_path.trim();
        if trimmed.is_empty() {
            return Err(HostError::new(
                "UNSUPPORTED_MEDIA",
                "Unsupported media format",
            ));
        }
        if is_passthrough_url(trimmed) {
            let (mime, name) = passthrough_metadata(trimmed);
            return Ok(json!({
                "url": trimmed,
                "mime": mime,
                "name": name,
            }));
        }
        let normalized = normalize_local_path(trimmed)?;
        let mime = media_mime_for_path(Path::new(&normalized))?;
        let canonical_path = if let Some(profile) = crate::remote::workspace_profile(state, workspace_id) {
            resolve_remote_cached(state, &profile, workspace_id, &normalized)?
        } else {
            resolve_local_file(state, workspace_id, &normalized)?
        };
        let port = self.ensure_started()?;
        let id = self.register(canonical_path.clone(), mime);
        let name = canonical_path
            .file_name()
            .map(|value| value.to_string_lossy().into_owned())
            .unwrap_or_default();
        let url = format!("http://127.0.0.1:{port}/media/{id}?token={}", self.token);
        Ok(json!({
            "url": url,
            "mime": mime,
            "name": name,
        }))
    }

    fn ensure_started(&self) -> HostResult<u16> {
        let mut guard = self.port.lock().unwrap();
        if let Some(port) = *guard {
            return Ok(port);
        }
        let listener = TcpListener::bind(("127.0.0.1", 0))?;
        let port = listener.local_addr()?.port();
        let entries = Arc::clone(&self.entries);
        let token = self.token.clone();
        std::thread::spawn(move || serve_loop(listener, entries, token));
        *guard = Some(port);
        Ok(port)
    }

    fn register(&self, canonical_path: PathBuf, mime: &str) -> String {
        let key = (canonical_path.clone(), mime.to_owned());
        {
            let registry = self.entries.read().unwrap();
            if let Some(existing) = registry.index.get(&key) {
                return existing.clone();
            }
        }
        let mut registry = self.entries.write().unwrap();
        if let Some(existing) = registry.index.get(&key) {
            return existing.clone();
        }
        let id = uuid::Uuid::new_v4().to_string();
        registry.entries.insert(
            id.clone(),
            MediaEntry {
                path: canonical_path,
                mime: mime.to_owned(),
            },
        );
        registry.index.insert(key, id.clone());
        id
    }
}

pub fn video_mime_for_path(path: &Path) -> Option<&'static str> {
    let extension = path
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or("")
        .to_lowercase();
    match extension.as_str() {
        "mp4" => Some("video/mp4"),
        "m4v" => Some("video/x-m4v"),
        "webm" => Some("video/webm"),
        "mov" => Some("video/quicktime"),
        "ogv" => Some("video/ogg"),
        _ => None,
    }
}

pub fn image_mime_for_path(path: &Path) -> Option<&'static str> {
    let extension = path
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or("")
        .to_lowercase();
    match extension.as_str() {
        "png" => Some("image/png"),
        "jpg" | "jpeg" => Some("image/jpeg"),
        "webp" => Some("image/webp"),
        "gif" => Some("image/gif"),
        "bmp" => Some("image/bmp"),
        "avif" => Some("image/avif"),
        _ => None,
    }
}

pub fn media_mime_for_path(path: &Path) -> HostResult<&'static str> {
    video_mime_for_path(path)
        .or_else(|| image_mime_for_path(path))
        .ok_or_else(|| HostError::new("UNSUPPORTED_MEDIA", "Unsupported media format"))
}

fn generate_token() -> String {
    let mut bytes = [0u8; 32];
    rand::rng().fill_bytes(&mut bytes);
    let mut token = String::with_capacity(64);
    for byte in bytes {
        token.push_str(&format!("{byte:02x}"));
    }
    token
}

fn is_passthrough_url(raw_path: &str) -> bool {
    let lower = raw_path.to_ascii_lowercase();
    lower.starts_with("http://")
        || lower.starts_with("https://")
        || lower.starts_with("blob:")
        || lower.starts_with("data:")
}

fn passthrough_metadata(raw_path: &str) -> (String, String) {
    if raw_path.to_ascii_lowercase().starts_with("data:") {
        let header = raw_path[5..].split(',').next().unwrap_or("");
        let mime = header.split(';').next().unwrap_or("").trim();
        let mime = if mime.contains('/') && !mime.chars().any(char::is_whitespace) {
            mime.to_owned()
        } else {
            String::new()
        };
        return (mime, String::new());
    }
    if let Ok(url) = url::Url::parse(raw_path) {
        if url.scheme() == "blob" {
            return (String::new(), String::new());
        }
        let name = url
            .path_segments()
            .and_then(|segments| segments.filter(|segment| !segment.is_empty()).last())
            .unwrap_or("")
            .to_owned();
        let mime = if name.is_empty() {
            String::new()
        } else {
            video_mime_for_path(Path::new(&name))
                .or_else(|| image_mime_for_path(Path::new(&name)))
                .unwrap_or("")
                .to_owned()
        };
        return (mime, name);
    }
    (String::new(), String::new())
}

fn strip_query_and_fragment(raw_path: &str) -> &str {
    let without_fragment = raw_path.split('#').next().unwrap_or(raw_path);
    without_fragment.split('?').next().unwrap_or(without_fragment)
}

fn percent_decode(input: &str) -> HostResult<String> {
    let bytes = input.as_bytes();
    let mut output = Vec::with_capacity(bytes.len());
    let mut index = 0;
    while index < bytes.len() {
        if bytes[index] == b'%' && index + 2 < bytes.len() {
            let hex = std::str::from_utf8(&bytes[index + 1..index + 3])
                .map_err(|_| HostError::new("INVALID_PATH", "Invalid media file URL"))?;
            if let Ok(value) = u8::from_str_radix(hex, 16) {
                output.push(value);
                index += 3;
                continue;
            }
        }
        output.push(bytes[index]);
        index += 1;
    }
    String::from_utf8(output).map_err(|_| HostError::new("INVALID_PATH", "Invalid media file URL"))
}

fn normalize_local_path(raw_path: &str) -> HostResult<String> {
    if raw_path.to_ascii_lowercase().starts_with("file://") {
        let parsed = url::Url::parse(raw_path)
            .map_err(|_| HostError::new("INVALID_PATH", "Invalid media file URL"))?;
        if let Ok(file_path) = parsed.to_file_path() {
            return Ok(file_path.to_string_lossy().into_owned());
        }
        let decoded = percent_decode(parsed.path())?;
        if cfg!(windows) {
            let bytes = decoded.as_bytes();
            if bytes.len() >= 3
                && bytes[0] == b'/'
                && bytes[1].is_ascii_alphabetic()
                && bytes[2] == b':'
            {
                return Ok(decoded[1..].to_owned());
            }
        }
        return Ok(decoded);
    }
    Ok(strip_query_and_fragment(raw_path).to_owned())
}

fn resolve_remote_cached(
    state: &AppState,
    profile: &str,
    workspace_id: &str,
    normalized_path: &str,
) -> HostResult<PathBuf> {
    let cache_dir = state.directory.join("media-cache");
    std::fs::create_dir_all(&cache_dir)?;
    let mut hasher = Sha256::new();
    hasher.update(workspace_id.as_bytes());
    hasher.update(b"\0");
    hasher.update(normalized_path.as_bytes());
    let digest = format!("{:x}", hasher.finalize());
    let extension = Path::new(normalized_path)
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or("bin")
        .to_lowercase();
    let cache_path = cache_dir.join(format!("{digest}.{extension}"));
    let connection = state.remotes.connection(profile)?;
    let connection = connection.lock().unwrap();
    let remote_path = normalized_path.replace('\\', "/");
    let size = crate::remote::files::read_to_cache(&connection, &remote_path, &cache_path)?;
    if size == 0 {
        return Err(HostError::new("EMPTY_MEDIA", "Media file is empty"));
    }
    Ok(dunce::canonicalize(&cache_path)?)
}

fn is_allowed_prime_temp_path(candidate: &Path) -> bool {
    let temp_dir = std::env::temp_dir();
    let canon_temp = dunce::canonicalize(&temp_dir).unwrap_or_else(|_| temp_dir.clone());
    for base in [&canon_temp, &temp_dir] {
        if let Ok(rel) = candidate.strip_prefix(base) {
            if let Some(Component::Normal(first)) = rel.components().next() {
                let first_str = first.to_string_lossy();
                if first_str.starts_with("prime-gamedev-")
                    || first_str.starts_with("prime-designer-")
                    || first_str.starts_with("prime-media-")
                {
                    return true;
                }
            }
        }
    }
    false
}

fn is_path_inside_allowed_areas(
    state: &AppState,
    root: Option<&Path>,
    canon_root: Option<&Path>,
    candidate: &Path,
) -> bool {
    let inside_root = root.is_some_and(|dir| candidate.starts_with(dir))
        || canon_root.is_some_and(|dir| candidate.starts_with(dir));
    let attachments_dir = state.directory.join("attachments");
    let canon_attachments = dunce::canonicalize(&attachments_dir).ok();
    let inside_attachments = candidate.starts_with(&attachments_dir)
        || canon_attachments
            .as_ref()
            .is_some_and(|dir| candidate.starts_with(dir));
    inside_root || inside_attachments || is_allowed_prime_temp_path(candidate)
}

pub fn inspect_local_path(state: &AppState, workspace_id: &str, raw_path: &str) -> Value {
    let trimmed = raw_path.trim();
    if trimmed.is_empty()
        || is_passthrough_url(trimmed)
        || trimmed.to_ascii_lowercase().starts_with("prime:")
    {
        return json!({ "exists": false, "kind": "missing" });
    }
    if !workspace_id.is_empty() && crate::remote::workspace_profile(state, workspace_id).is_some() {
        return json!({ "exists": false, "kind": "missing" });
    }
    let Ok(normalized) = normalize_local_path(trimmed) else {
        return json!({ "exists": false, "kind": "missing" });
    };
    let candidate = Path::new(&normalized);
    if candidate
        .components()
        .any(|component| matches!(component, Component::ParentDir))
    {
        return json!({ "exists": false, "kind": "missing" });
    }
    let root = if workspace_id.is_empty() {
        None
    } else {
        state.root(workspace_id).ok()
    };
    let canon_root = root
        .as_ref()
        .and_then(|dir| dunce::canonicalize(dir).ok());
    let resolved = if candidate.is_absolute() {
        dunce::canonicalize(candidate).ok()
    } else if let Some(ref root_dir) = root {
        files::scoped_path(root_dir, &normalized, false).ok()
    } else {
        None
    };
    let Some(canon_target) = resolved else {
        return json!({ "exists": false, "kind": "missing" });
    };
    let name = canon_target
        .file_name()
        .map(|value| value.to_string_lossy().into_owned())
        .filter(|value| !value.is_empty())
        .unwrap_or_else(|| canon_target.to_string_lossy().into_owned());
    let path_str = canon_target.to_string_lossy().into_owned();
    if canon_target.is_dir() {
        return json!({
            "exists": true,
            "kind": "dir",
            "name": name,
            "path": path_str,
            "previewable": false,
            "revealable": true,
        });
    }
    if !canon_target.is_file() {
        return json!({ "exists": false, "kind": "missing" });
    }
    let allowed = is_path_inside_allowed_areas(
        state,
        root.as_deref(),
        canon_root.as_deref(),
        &canon_target,
    );
    if let Some(mime) = video_mime_for_path(&canon_target) {
        return json!({
            "exists": true,
            "kind": "video",
            "mime": mime,
            "name": name,
            "path": path_str,
            "previewable": allowed,
            "revealable": true,
        });
    }
    if let Some(mime) = image_mime_for_path(&canon_target) {
        return json!({
            "exists": true,
            "kind": "image",
            "mime": mime,
            "name": name,
            "path": path_str,
            "previewable": allowed,
            "revealable": true,
        });
    }
    json!({
        "exists": true,
        "kind": "file",
        "name": name,
        "path": path_str,
        "previewable": false,
        "revealable": true,
    })
}

pub fn resolve_reveal_target(
    state: &AppState,
    workspace_id: &str,
    raw_path: &str,
) -> HostResult<PathBuf> {
    let trimmed = raw_path.trim();
    if trimmed.is_empty() {
        return state.root(workspace_id);
    }
    let normalized = normalize_local_path(trimmed)?;
    let candidate = Path::new(&normalized);
    if candidate
        .components()
        .any(|component| matches!(component, Component::ParentDir))
    {
        return Err(HostError::new(
            "PATH_DENIED",
            "Parent directory traversal is not allowed",
        ));
    }
    if candidate.is_absolute() {
        let canonical = dunce::canonicalize(candidate).map_err(|error| {
            if error.kind() == std::io::ErrorKind::NotFound {
                HostError::new("NOT_FOUND", "Target path does not exist")
            } else {
                error.into()
            }
        })?;
        return Ok(canonical);
    }
    let root = state.root(workspace_id)?;
    files::scoped_path(&root, &normalized, false)
}

fn resolve_local_file(
    state: &AppState,
    workspace_id: &str,
    normalized_path: &str,
) -> HostResult<PathBuf> {
    let candidate = Path::new(normalized_path);
    if workspace_id.is_empty() {
        if !candidate.is_absolute()
            || candidate
                .components()
                .any(|component| matches!(component, Component::ParentDir))
        {
            return Err(HostError::new("PATH_DENIED", "Path must be absolute"));
        }
        let canon_target = dunce::canonicalize(candidate).map_err(|error| {
            if error.kind() == std::io::ErrorKind::NotFound {
                HostError::new("NOT_FOUND", "Media file not found")
            } else {
                error.into()
            }
        })?;
        if !is_path_inside_allowed_areas(state, None, None, &canon_target) {
            return Err(HostError::new("PATH_DENIED", "Path leaves allowed media directories"));
        }
        if !canon_target.is_file() {
            return Err(HostError::new("NOT_FOUND", "Media file not found"));
        }
        return Ok(canon_target);
    }
    let root = state.root(workspace_id)?;
    match files::scoped_path(&root, normalized_path, false) {
        Ok(path) => {
            if !path.is_file() {
                return Err(HostError::new("NOT_FOUND", "Media file not found"));
            }
            Ok(path)
        }
        Err(scoped_error) => {
            if !candidate.is_absolute()
                && candidate
                    .components()
                    .all(|component| matches!(component, Component::Normal(_) | Component::CurDir))
                && std::fs::symlink_metadata(root.join(candidate))
                    .is_err_and(|error| error.kind() == std::io::ErrorKind::NotFound)
            {
                return Err(HostError::new("NOT_FOUND", "Media file not found"));
            }
            if !candidate.is_absolute()
                || candidate
                    .components()
                    .any(|component| matches!(component, Component::ParentDir))
            {
                return Err(scoped_error);
            }
            let canon_root = dunce::canonicalize(&root)?;
            match dunce::canonicalize(candidate) {
                Ok(canon_target) => {
                    if !is_path_inside_allowed_areas(
                        state,
                        Some(&root),
                        Some(&canon_root),
                        &canon_target,
                    ) {
                        return Err(scoped_error);
                    }
                    if !canon_target.is_file() {
                        return Err(HostError::new("NOT_FOUND", "Media file not found"));
                    }
                    Ok(canon_target)
                }
                Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                    if is_path_inside_allowed_areas(
                        state,
                        Some(&root),
                        Some(&canon_root),
                        candidate,
                    ) {
                        Err(HostError::new("NOT_FOUND", "Media file not found"))
                    } else {
                        Err(scoped_error)
                    }
                }
                Err(error) => Err(error.into()),
            }
        }
    }
}

fn serve_loop(listener: TcpListener, entries: Arc<RwLock<MediaRegistry>>, token: String) {
    for stream in listener.incoming() {
        let Ok(stream) = stream else {
            continue;
        };
        let entries = Arc::clone(&entries);
        let token = token.clone();
        std::thread::spawn(move || {
            let _ = handle_connection(stream, &entries, &token);
        });
    }
}

fn handle_connection(
    mut stream: TcpStream,
    entries: &RwLock<MediaRegistry>,
    expected_token: &str,
) -> std::io::Result<()> {
    stream.set_read_timeout(Some(Duration::from_secs(10)))?;
    stream.set_write_timeout(Some(Duration::from_secs(30)))?;
    let Some(head) = read_http_head(&mut stream)? else {
        return write_simple_response(&mut stream, 400, "Bad Request", &[]);
    };
    let mut lines = head.split("\r\n");
    let Some(request_line) = lines.next() else {
        return write_simple_response(&mut stream, 400, "Bad Request", &[]);
    };
    let mut parts = request_line.split_whitespace();
    let (Some(method), Some(target), Some(version)) = (parts.next(), parts.next(), parts.next())
    else {
        return write_simple_response(&mut stream, 400, "Bad Request", &[]);
    };
    if !version.starts_with("HTTP/1.") {
        return write_simple_response(&mut stream, 400, "Bad Request", &[]);
    }
    let is_head = match method {
        "GET" => false,
        "HEAD" => true,
        _ => return write_simple_response(&mut stream, 405, "Method Not Allowed", &[]),
    };
    let (raw_path, raw_query) = target.split_once('?').unwrap_or((target, ""));
    let request_token = url::form_urlencoded::parse(raw_query.as_bytes())
        .find(|(key, _)| key == "token")
        .map(|(_, value)| value.into_owned());
    if request_token.as_deref() != Some(expected_token) {
        return write_simple_response(&mut stream, 401, "Unauthorized", &[]);
    }
    let Some(id) = raw_path.strip_prefix("/media/") else {
        return write_simple_response(&mut stream, 404, "Not Found", &[]);
    };
    if id.is_empty() || id.contains('/') {
        return write_simple_response(&mut stream, 404, "Not Found", &[]);
    }
    let mut range_header = None;
    for line in lines {
        if let Some((name, value)) = line.split_once(':') {
            if name.trim().eq_ignore_ascii_case("range") {
                range_header = Some(value.trim().to_owned());
            }
        }
    }
    let entry = {
        let registry = entries.read().unwrap();
        registry
            .entries
            .get(id)
            .map(|item| (item.path.clone(), item.mime.clone()))
    };
    let Some((file_path, mime)) = entry else {
        return write_simple_response(&mut stream, 404, "Not Found", &[]);
    };
    let Ok(mut file) = File::open(&file_path) else {
        return write_simple_response(&mut stream, 404, "Not Found", &[]);
    };
    let Ok(metadata) = file.metadata() else {
        return write_simple_response(&mut stream, 404, "Not Found", &[]);
    };
    let file_len = metadata.len();
    if let Some(range_value) = range_header {
        match parse_byte_range(&range_value, file_len) {
            Ok((start, end)) => {
                let content_len = end - start + 1;
                let headers = format!(
                    "HTTP/1.1 206 Partial Content\r\nContent-Type: {mime}\r\nContent-Length: {content_len}\r\nContent-Range: bytes {start}-{end}/{file_len}\r\nAccept-Ranges: bytes\r\nAccess-Control-Allow-Origin: *\r\nCache-Control: no-store\r\nConnection: close\r\n\r\n"
                );
                stream.write_all(headers.as_bytes())?;
                if !is_head {
                    stream_range(&mut stream, &mut file, start, content_len)?;
                }
            }
            Err(()) => {
                let content_range = format!("bytes */{file_len}");
                return write_simple_response(
                    &mut stream,
                    416,
                    "Range Not Satisfiable",
                    &[("Content-Range", content_range.as_str())],
                );
            }
        }
    } else {
        let headers = format!(
            "HTTP/1.1 200 OK\r\nContent-Type: {mime}\r\nContent-Length: {file_len}\r\nAccept-Ranges: bytes\r\nAccess-Control-Allow-Origin: *\r\nCache-Control: no-store\r\nConnection: close\r\n\r\n"
        );
        stream.write_all(headers.as_bytes())?;
        if !is_head {
            stream_range(&mut stream, &mut file, 0, file_len)?;
        }
    }
    stream.flush()
}

fn read_http_head(stream: &mut TcpStream) -> std::io::Result<Option<String>> {
    let mut buffer = Vec::with_capacity(1024);
    let mut chunk = [0u8; 1024];
    loop {
        let count = match stream.read(&mut chunk) {
            Ok(count) => count,
            Err(_) => return Ok(None),
        };
        if count == 0 {
            return Ok(None);
        }
        buffer.extend_from_slice(&chunk[..count]);
        if let Some(end) = buffer.windows(4).position(|window| window == b"\r\n\r\n") {
            return Ok(std::str::from_utf8(&buffer[..end]).ok().map(str::to_owned));
        }
        if buffer.len() > MAX_HTTP_HEADER_BYTES {
            return Ok(None);
        }
    }
}

fn write_simple_response(
    stream: &mut TcpStream,
    status: u16,
    reason: &str,
    extra_headers: &[(&str, &str)],
) -> std::io::Result<()> {
    let mut response = format!("HTTP/1.1 {status} {reason}\r\n");
    for (name, value) in extra_headers {
        response.push_str(&format!("{name}: {value}\r\n"));
    }
    response.push_str(
        "Accept-Ranges: bytes\r\nContent-Length: 0\r\nAccess-Control-Allow-Origin: *\r\nCache-Control: no-store\r\nConnection: close\r\n\r\n",
    );
    stream.write_all(response.as_bytes())?;
    stream.flush()
}

fn stream_range(
    stream: &mut TcpStream,
    file: &mut File,
    start: u64,
    length: u64,
) -> std::io::Result<()> {
    file.seek(SeekFrom::Start(start))?;
    let mut limited = file.take(length);
    let mut buffer = [0u8; STREAM_CHUNK_SIZE];
    loop {
        let count = limited.read(&mut buffer)?;
        if count == 0 {
            break;
        }
        if let Err(error) = stream.write_all(&buffer[..count]) {
            if matches!(
                error.kind(),
                std::io::ErrorKind::BrokenPipe
                    | std::io::ErrorKind::ConnectionReset
                    | std::io::ErrorKind::ConnectionAborted
            ) {
                return Ok(());
            }
            return Err(error);
        }
    }
    Ok(())
}

fn parse_byte_range(header: &str, file_len: u64) -> Result<(u64, u64), ()> {
    let trimmed = header.trim();
    if !trimmed
        .get(..6)
        .is_some_and(|prefix| prefix.eq_ignore_ascii_case("bytes="))
    {
        return Err(());
    }
    let spec = trimmed[6..].trim();
    if spec.is_empty() || spec.contains(',') {
        return Err(());
    }
    let (start_str, end_str) = spec.split_once('-').ok_or(())?;
    let start_str = start_str.trim();
    let end_str = end_str.trim();
    if file_len == 0 {
        return Err(());
    }
    if start_str.is_empty() && end_str.is_empty() {
        return Err(());
    }
    if start_str.is_empty() {
        let suffix: u64 = end_str.parse().map_err(|_| ())?;
        if suffix == 0 {
            return Err(());
        }
        let len = suffix.min(file_len);
        return Ok((file_len - len, file_len - 1));
    }
    let start: u64 = start_str.parse().map_err(|_| ())?;
    if start >= file_len {
        return Err(());
    }
    if end_str.is_empty() {
        return Ok((start, file_len - 1));
    }
    let end: u64 = end_str.parse().map_err(|_| ())?;
    if end < start {
        return Err(());
    }
    Ok((start, end.min(file_len - 1)))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn send_http(port: u16, request: &str) -> (String, Vec<u8>) {
        let mut stream = TcpStream::connect(("127.0.0.1", port)).unwrap();
        stream.write_all(request.as_bytes()).unwrap();
        stream.flush().unwrap();
        let mut raw = Vec::new();
        stream.read_to_end(&mut raw).unwrap();
        let split = raw
            .windows(4)
            .position(|window| window == b"\r\n\r\n")
            .unwrap();
        let headers = String::from_utf8(raw[..split].to_vec()).unwrap();
        let body = raw[split + 4..].to_vec();
        (headers, body)
    }

    #[test]
    fn http_server_handles_full_head_ranges_errors_and_auth() {
        let dir = tempfile::tempdir().unwrap();
        let file_path = dir.path().join("sample.mp4");
        std::fs::write(&file_path, b"0123456789").unwrap();
        let canonical = dunce::canonicalize(&file_path).unwrap();

        let server = MediaServer::new();
        let port = server.ensure_started().unwrap();
        let id = server.register(canonical, "video/mp4");
        let token = server.token.clone();

        let (headers, body) = send_http(
            port,
            &format!("GET /media/{id}?token={token} HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n"),
        );
        assert!(headers.starts_with("HTTP/1.1 200 OK\r\n"));
        assert!(headers.contains("Content-Length: 10\r\n"));
        assert!(headers.contains("Accept-Ranges: bytes\r\n"));
        assert!(headers.contains("Content-Type: video/mp4\r\n"));
        assert_eq!(body, b"0123456789");

        let (headers, body) = send_http(
            port,
            &format!("HEAD /media/{id}?token={token} HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n"),
        );
        assert!(headers.starts_with("HTTP/1.1 200 OK\r\n"));
        assert!(headers.contains("Content-Length: 10\r\n"));
        assert!(body.is_empty());

        let (headers, body) = send_http(
            port,
            &format!(
                "GET /media/{id}?token={token} HTTP/1.1\r\nHost: 127.0.0.1\r\nRange: bytes=0-4\r\n\r\n"
            ),
        );
        assert!(headers.starts_with("HTTP/1.1 206 Partial Content\r\n"));
        assert!(headers.contains("Content-Range: bytes 0-4/10\r\n"));
        assert!(headers.contains("Content-Length: 5\r\n"));
        assert_eq!(body, b"01234");

        let (headers, body) = send_http(
            port,
            &format!(
                "GET /media/{id}?token={token} HTTP/1.1\r\nHost: 127.0.0.1\r\nrange: bytes=5-\r\n\r\n"
            ),
        );
        assert!(headers.starts_with("HTTP/1.1 206 Partial Content\r\n"));
        assert!(headers.contains("Content-Range: bytes 5-9/10\r\n"));
        assert!(headers.contains("Content-Length: 5\r\n"));
        assert_eq!(body, b"56789");

        let (headers, body) = send_http(
            port,
            &format!(
                "GET /media/{id}?token={token} HTTP/1.1\r\nHost: 127.0.0.1\r\nRange: bytes=-4\r\n\r\n"
            ),
        );
        assert!(headers.starts_with("HTTP/1.1 206 Partial Content\r\n"));
        assert!(headers.contains("Content-Range: bytes 6-9/10\r\n"));
        assert!(headers.contains("Content-Length: 4\r\n"));
        assert_eq!(body, b"6789");

        let (headers, _) = send_http(
            port,
            &format!(
                "GET /media/{id}?token={token} HTTP/1.1\r\nHost: 127.0.0.1\r\nRange: bytes=50-\r\n\r\n"
            ),
        );
        assert!(headers.starts_with("HTTP/1.1 416 Range Not Satisfiable\r\n"));
        assert!(headers.contains("Content-Range: bytes */10\r\n"));

        let (headers, _) = send_http(
            port,
            &format!("GET /media/{id}?token=bad-token HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n"),
        );
        assert!(headers.starts_with("HTTP/1.1 401 Unauthorized\r\n"));

        let (headers, _) = send_http(
            port,
            &format!("GET /media/missing-id?token={token} HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n"),
        );
        assert!(headers.starts_with("HTTP/1.1 404 Not Found\r\n"));
    }

    #[test]
    fn resolve_enforces_root_containment_and_supports_urls_and_attachments() {
        let app_dir = tempfile::tempdir().unwrap();
        let workspace_dir = tempfile::tempdir().unwrap();
        let outside_dir = tempfile::tempdir().unwrap();

        std::fs::create_dir_all(workspace_dir.path().join("out")).unwrap();
        let local_video = workspace_dir.path().join("out").join("my clip.mp4");
        std::fs::write(&local_video, b"video-bytes").unwrap();

        let outside_video = outside_dir.path().join("secret.mp4");
        std::fs::write(&outside_video, b"secret").unwrap();

        let text_file = workspace_dir.path().join("notes.txt");
        std::fs::write(&text_file, b"hello").unwrap();

        let state = AppState::new(app_dir.path().to_path_buf()).unwrap();
        state.projects.lock().unwrap().insert(
            "ws-1".to_owned(),
            json!({"id": "ws-1", "path": workspace_dir.path()}),
        );

        let attachment_video = app_dir.path().join("attachments").join("attached.webm");
        std::fs::write(&attachment_video, b"webm-bytes").unwrap();

        let traversal = state.media.resolve(&state, "ws-1", "../secret.mp4");
        assert_eq!(traversal.unwrap_err().code, "PATH_DENIED");

        let outside_abs = state
            .media
            .resolve(&state, "ws-1", outside_video.to_str().unwrap());
        assert_eq!(outside_abs.unwrap_err().code, "PATH_DENIED");

        let unsupported = state.media.resolve(&state, "ws-1", "notes.txt");
        assert_eq!(unsupported.unwrap_err().code, "UNSUPPORTED_MEDIA");

        let missing = state.media.resolve(&state, "ws-1", "out/missing.mp4");
        assert_eq!(missing.unwrap_err().code, "NOT_FOUND");

        let first = state
            .media
            .resolve(&state, "ws-1", "out/my clip.mp4?t=1#frag")
            .unwrap();
        let second = state
            .media
            .resolve(
                &state,
                "ws-1",
                url::Url::from_file_path(&local_video).unwrap().as_str(),
            )
            .unwrap();
        assert_eq!(first["url"], second["url"]);
        assert_eq!(first["mime"], "video/mp4");
        assert_eq!(first["name"], "my clip.mp4");

        let attached = state
            .media
            .resolve(&state, "ws-1", attachment_video.to_str().unwrap())
            .unwrap();
        assert_eq!(attached["mime"], "video/webm");
        assert_eq!(attached["name"], "attached.webm");

        let remote_http = state
            .media
            .resolve(&state, "", "https://example.com/demo.mp4?raw=1")
            .unwrap();
        assert_eq!(remote_http["url"], "https://example.com/demo.mp4?raw=1");
        assert_eq!(remote_http["mime"], "video/mp4");
        assert_eq!(remote_http["name"], "demo.mp4");

        let data_url = state
            .media
            .resolve(&state, "", "data:video/mp4;base64,AAAA")
            .unwrap();
        assert_eq!(data_url["url"], "data:video/mp4;base64,AAAA");
        assert_eq!(data_url["mime"], "video/mp4");
    }

    #[test]
    fn inspect_local_path_and_prime_temp_media_work() {
        let app_dir = tempfile::tempdir().unwrap();
        let workspace_dir = tempfile::tempdir().unwrap();
        let prime_temp_dir = std::env::temp_dir().join(format!(
            "prime-gamedev-test-{}",
            uuid::Uuid::new_v4()
        ));
        std::fs::create_dir_all(workspace_dir.path().join("frames")).unwrap();
        std::fs::create_dir_all(&prime_temp_dir).unwrap();

        let img_path = workspace_dir.path().join("frames").join("shot.png");
        std::fs::write(&img_path, b"png").unwrap();
        let temp_vid = prime_temp_dir.join("run.webm");
        std::fs::write(&temp_vid, b"webm").unwrap();

        let state = AppState::new(app_dir.path().to_path_buf()).unwrap();
        state.projects.lock().unwrap().insert(
            "ws-1".to_owned(),
            json!({"id": "ws-1", "path": workspace_dir.path()}),
        );

        let inspected_img = inspect_local_path(&state, "ws-1", "frames/shot.png");
        assert_eq!(inspected_img["exists"], true);
        assert_eq!(inspected_img["kind"], "image");
        assert_eq!(inspected_img["previewable"], true);
        assert_eq!(inspected_img["name"], "shot.png");

        let inspected_dir = inspect_local_path(&state, "ws-1", "frames");
        assert_eq!(inspected_dir["exists"], true);
        assert_eq!(inspected_dir["kind"], "dir");
        assert_eq!(inspected_dir["revealable"], true);

        let inspected_temp = inspect_local_path(&state, "", temp_vid.to_str().unwrap());
        assert_eq!(inspected_temp["exists"], true);
        assert_eq!(inspected_temp["kind"], "video");
        assert_eq!(inspected_temp["previewable"], true);

        let resolved_temp_media = state
            .media
            .resolve(&state, "", temp_vid.to_str().unwrap())
            .unwrap();
        assert_eq!(resolved_temp_media["mime"], "video/webm");

        let missing = inspect_local_path(&state, "ws-1", "frames/nope.png");
        assert_eq!(missing["exists"], false);
        assert_eq!(missing["kind"], "missing");

        let revealed = resolve_reveal_target(&state, "ws-1", temp_vid.to_str().unwrap()).unwrap();
        assert_eq!(revealed, dunce::canonicalize(&temp_vid).unwrap());

        let _ = std::fs::remove_dir_all(&prime_temp_dir);
    }
}
