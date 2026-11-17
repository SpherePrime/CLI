use super::connection::{sftp_failure, Connection};
use super::runtime;
use crate::{
    error::{HostError, HostResult},
    file_index::{filter_and_rank, should_fast_skip, IndexedEntry},
};
use base64::Engine;
use russh_sftp::protocol::{FileAttributes, OpenFlags};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    io::Write,
    path::Path,
    sync::Arc,
    time::{Duration, Instant},
};
use tokio::io::{AsyncReadExt, AsyncWriteExt};

const LIMIT: u64 = 2 * 1024 * 1024;
const CACHE_TTL: Duration = Duration::from_secs(15);

pub struct CachedIgnoreRules {
    pub rules: Arc<ignore::gitignore::Gitignore>,
    pub loaded_at: Instant,
}

pub struct CachedFileIndex {
    pub entries: Arc<Vec<IndexedEntry>>,
    pub loaded_at: Instant,
}

pub fn is_remote_symlink(perm: Option<u32>, is_file: bool, is_dir: bool) -> bool {
    match perm {
        Some(perm) => (perm & 0o170000) == 0o120000,
        None => !is_file && !is_dir,
    }
}

pub fn is_remote_symlink_stat(metadata: &FileAttributes) -> bool {
    is_remote_symlink(metadata.permissions, metadata.is_regular(), metadata.is_dir())
}

fn remote_path(path: &Path) -> String {
    path.to_string_lossy().replace('\\', "/")
}

fn invalidate_caches(connection: &Connection, relative: &str) {
    *connection.file_index_cache.lock().unwrap() = None;
    let name = relative.rsplit('/').next().unwrap_or(relative);
    if matches!(name, ".gitignore" | ".primeignore" | "") {
        *connection.ignore_cache.lock().unwrap() = None;
    }
}

fn failure(error: impl std::fmt::Display) -> HostError {
    HostError::new("SFTP_ERROR", error.to_string())
}

fn revision(bytes: &[u8]) -> String {
    format!("{:x}", Sha256::digest(bytes))
}

fn replace(connection: &Connection, source: &Path, target: &Path) -> HostResult<()> {
    let source = super::connection::shell_quote(&remote_path(source));
    let target = super::connection::shell_quote(&remote_path(target));
    let result = connection.execute(&format!("mv -f -- {source} {target}"), "", 30)?;
    if result["exitCode"] != 0 {
        return Err(HostError::new(
            "SFTP_RENAME",
            format!(
                "Remote atomic rename failed: {}",
                result["stderr"].as_str().unwrap_or("")
            ),
        ));
    }
    Ok(())
}

fn image_mime_for_path(relative: &str) -> Option<&'static str> {
    let name = relative.rsplit('/').next().unwrap_or(relative);
    let extension = Path::new(name)
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or("")
        .to_lowercase();
    match extension.as_str() {
        "png" => Some("image/png"),
        "jpg" | "jpeg" => Some("image/jpeg"),
        "webp" => Some("image/webp"),
        "gif" => Some("image/gif"),
        _ => None,
    }
}

pub fn video_mime_for_path(relative: &str) -> Option<&'static str> {
    let name = relative.rsplit('/').next().unwrap_or(relative);
    crate::media::video_mime_for_path(Path::new(name))
}

fn video_read_metadata(relative: &str) -> Option<Value> {
    let mime = video_mime_for_path(relative)?;
    let name = relative.rsplit('/').next().unwrap_or(relative);
    Some(json!({
        "path": relative,
        "name": name,
        "video": {
            "path": relative,
            "mime": mime
        }
    }))
}

fn read_limit_for_path(relative: &str) -> Option<u64> {
    if image_mime_for_path(relative).is_some() || video_mime_for_path(relative).is_some() {
        None
    } else {
        Some(LIMIT)
    }
}

fn read_bytes(connection: &Connection, relative: &str, limit: Option<u64>) -> HostResult<Vec<u8>> {
    let path = connection.resolve(relative, false)?;
    let path_str = remote_path(&path);
    runtime::block_on(async {
        let metadata = connection
            .sftp
            .metadata(&path_str)
            .await
            .map_err(sftp_failure)?;
        if !metadata.is_regular() {
            return Err(HostError::new("REMOTE_FILE", "remote.notFile"));
        }
        if let Some(limit) = limit {
            if metadata.size.is_some_and(|size| size > limit) {
                return Err(HostError::new("REMOTE_FILE", "remote.fileTooLarge"));
            }
        }
        let mut file = connection
            .sftp
            .open(&path_str)
            .await
            .map_err(sftp_failure)?;
        let mut bytes = Vec::new();
        match limit {
            Some(limit) => {
                file.take(limit.saturating_add(1))
                    .read_to_end(&mut bytes)
                    .await?;
                if bytes.len() as u64 > limit {
                    return Err(HostError::new("REMOTE_FILE", "remote.fileTooLarge"));
                }
            }
            None => {
                file.read_to_end(&mut bytes).await?;
            }
        }
        Ok(bytes)
    })
}

pub fn bytes(connection: &Connection, relative: &str, limit: u64) -> HostResult<Vec<u8>> {
    read_bytes(connection, relative, Some(limit))
}

pub fn read_to_cache(connection: &Connection, relative: &str, target: &Path) -> HostResult<u64> {
    let path = connection.resolve(relative, false)?;
    let path_str = remote_path(&path);
    runtime::block_on(async {
        let metadata = connection
            .sftp
            .metadata(&path_str)
            .await
            .map_err(sftp_failure)?;
        if !metadata.is_regular() {
            return Err(HostError::new("NOT_FOUND", "Media file not found"));
        }
        if metadata.size == Some(0) {
            return Err(HostError::new("EMPTY_MEDIA", "Media file is empty"));
        }
        if target.is_file() {
            if let Ok(local_meta) = target.metadata() {
                if local_meta.len() > 0 && metadata.size == Some(local_meta.len()) {
                    return Ok(local_meta.len());
                }
            }
        }
        let parent = target
            .parent()
            .ok_or_else(|| HostError::new("IO_ERROR", "Invalid cache target"))?;
        std::fs::create_dir_all(parent)?;
        let mut remote_file = connection
            .sftp
            .open(&path_str)
            .await
            .map_err(sftp_failure)?;
        let mut temporary = tempfile::NamedTempFile::new_in(parent)?;
        let mut buffer = [0u8; 64 * 1024];
        let mut written: u64 = 0;
        loop {
            let count = remote_file.read(&mut buffer).await?;
            if count == 0 {
                break;
            }
            temporary.write_all(&buffer[..count])?;
            written += count as u64;
        }
        if written == 0 {
            return Err(HostError::new("EMPTY_MEDIA", "Media file is empty"));
        }
        temporary.flush()?;
        temporary.as_file().sync_all()?;
        temporary.persist(target).map_err(|error| error.error)?;
        Ok(written)
    })
}

fn ignore_rules(connection: &Connection) -> HostResult<Arc<ignore::gitignore::Gitignore>> {
    {
        let cache = connection.ignore_cache.lock().unwrap();
        if let Some(cached) = cache.as_ref() {
            if cached.loaded_at.elapsed() < CACHE_TTL {
                return Ok(Arc::clone(&cached.rules));
            }
        }
    }
    let mut rules = ignore::gitignore::GitignoreBuilder::new("/");
    for name in [".gitignore", ".primeignore"] {
        match bytes(connection, name, LIMIT) {
            Ok(content) => {
                let content = String::from_utf8(content)
                    .map_err(|_| HostError::new("REMOTE_IGNORE", "Ignore file is not UTF-8"))?;
                for line in content.lines() {
                    rules.add_line(None, line).map_err(failure)?;
                }
            }
            Err(error)
                if error.message.contains("no such file")
                    || error.message.contains("No such file") => {}
            Err(error) => return Err(error),
        }
    }
    let built = Arc::new(rules.build().map_err(failure)?);
    *connection.ignore_cache.lock().unwrap() = Some(CachedIgnoreRules {
        rules: Arc::clone(&built),
        loaded_at: Instant::now(),
    });
    Ok(built)
}

pub fn list(connection: &Connection, relative: &str) -> HostResult<Value> {
    let path = connection.resolve(relative, false)?;
    let path_str = remote_path(&path);
    let rules = ignore_rules(connection)?;
    let entries = runtime::block_on(connection.sftp.read_dir(&path_str)).map_err(sftp_failure)?;
    let mut output = Vec::new();
    for entry in entries {
        let name = entry.file_name();
        if [
            ".",
            "..",
            ".git",
            "node_modules",
            ".superpowers",
            ".env",
            ".env.local",
            ".prime-native-trash",
        ]
        .contains(&name.as_str())
        {
            continue;
        }
        let relative = if relative.is_empty() {
            name.clone()
        } else {
            format!("{}/{name}", relative.trim_end_matches('/'))
        };
        let metadata = entry.metadata();
        let is_dir = if is_remote_symlink_stat(&metadata) {
            let Ok(resolved) = connection.resolve(&relative, false) else {
                continue;
            };
            runtime::block_on(connection.sftp.metadata(remote_path(&resolved)))
                .map(|stat| stat.is_dir())
                .unwrap_or(false)
        } else {
            metadata.is_dir()
        };
        if rules.matched(format!("/{relative}"), is_dir).is_ignore() {
            continue;
        }
        output.push(json!({"name": name, "path": relative, "directory": is_dir}));
    }
    output.sort_by(|left, right| {
        right["directory"]
            .as_bool()
            .cmp(&left["directory"].as_bool())
            .then_with(|| left["name"].as_str().cmp(&right["name"].as_str()))
    });
    Ok(json!(output))
}

fn parse_remote_search_output(stdout: &str) -> Vec<IndexedEntry> {
    stdout
        .lines()
        .filter_map(|line| {
            let trimmed = line.trim().trim_start_matches("./");
            if trimmed.is_empty() || trimmed == "." {
                return None;
            }
            let name = trimmed.rsplit('/').next().unwrap_or(trimmed).to_owned();
            if should_fast_skip(&name, false) || name == ".prime-native-trash" {
                return None;
            }
            Some(IndexedEntry::from_relative(name, trimmed.to_owned(), false))
        })
        .take(5000)
        .collect()
}

pub fn search(connection: &Connection, query: &str, limit: usize) -> HostResult<Value> {
    let cached = {
        let guard = connection.file_index_cache.lock().unwrap();
        guard.as_ref().and_then(|entry| {
            if entry.loaded_at.elapsed() < CACHE_TTL {
                Some(Arc::clone(&entry.entries))
            } else {
                None
            }
        })
    };
    if let Some(entries) = cached {
        return Ok(filter_and_rank(&entries, query, limit));
    }

    let command = "if command -v rg >/dev/null 2>&1; then rg --files --hidden -g '!.git' -g '!node_modules' -g '!dist' -g '!build' -g '!target' -g '!.next' -g '!.turbo' -g '!.venv' -g '!__pycache__' -g '!vendor' -g '!.prime' -g '!.superpowers' -g '!.env' -g '!.env.local' -- . | head -n 5000; else find . -type d \\( -name .git -o -name node_modules -o -name dist -o -name build -o -name target -o -name .next -o -name .turbo -o -name .venv -o -name __pycache__ -o -name vendor -o -name .prime -o -name .superpowers \\) -prune -o -type f -print | head -n 5000; fi";
    if let Ok(result) = connection.execute(command, "", 15) {
        if result["exitCode"] == 0 {
            let entries = Arc::new(parse_remote_search_output(
                result["stdout"].as_str().unwrap_or(""),
            ));
            *connection.file_index_cache.lock().unwrap() = Some(CachedFileIndex {
                entries: Arc::clone(&entries),
                loaded_at: Instant::now(),
            });
            return Ok(filter_and_rank(&entries, query, limit));
        }
    }

    let fallback = list(connection, "")?;
    let entries: Vec<IndexedEntry> = fallback
        .as_array()
        .map(|items| {
            items
                .iter()
                .filter_map(|item| {
                    let name = item["name"].as_str()?.to_owned();
                    let path = item["path"].as_str()?.to_owned();
                    let directory = item["directory"].as_bool().unwrap_or(false);
                    Some(IndexedEntry::from_relative(name, path, directory))
                })
                .collect()
        })
        .unwrap_or_default();
    Ok(filter_and_rank(&entries, query, limit))
}

pub fn read(connection: &Connection, relative: &str) -> HostResult<Value> {
    if let Some(video) = video_read_metadata(relative) {
        let path = connection.resolve(relative, false)?;
        let metadata =
            runtime::block_on(connection.sftp.metadata(remote_path(&path))).map_err(sftp_failure)?;
        if !metadata.is_regular() {
            return Err(HostError::new("REMOTE_FILE", "remote.notFile"));
        }
        return Ok(video);
    }
    let mime = image_mime_for_path(relative);
    let bytes = read_bytes(connection, relative, read_limit_for_path(relative))?;
    let name = relative.rsplit('/').next().unwrap_or(relative);
    if let Some(mime) = mime {
        return Ok(
            json!({"path":relative,"name":name,"image":format!("data:{mime};base64,{}",base64::engine::general_purpose::STANDARD.encode(&bytes))}),
        );
    }
    if bytes.contains(&0) {
        return Err(HostError::new("REMOTE_FILE", "remote.binaryFile"));
    }
    let content = std::str::from_utf8(&bytes)
        .map_err(|_| HostError::new("REMOTE_FILE", "Remote file is not UTF-8"))?;
    Ok(json!({"path":relative,"name":name,"content":content,"revision":revision(&bytes)}))
}

pub fn save(
    connection: &Connection,
    relative: &str,
    content: &str,
    expected: &str,
) -> HostResult<Value> {
    let path = connection.resolve(relative, false)?;
    let original = bytes(connection, relative, LIMIT)?;
    if revision(&original) != expected {
        return Err(HostError::new("REVISION_CONFLICT", "remote.fileChanged"));
    }
    if original.contains(&0) {
        return Err(HostError::new("REMOTE_FILE", "remote.binaryFile"));
    }
    let mut content = content.as_bytes().to_vec();
    if original.starts_with(&[239, 187, 191]) && !content.starts_with(&[239, 187, 191]) {
        let mut prefixed = vec![239, 187, 191];
        prefixed.extend(content);
        content = prefixed;
    }
    if content.len() as u64 > LIMIT {
        return Err(HostError::new("REMOTE_FILE", "remote.fileTooLarge"));
    }
    let temporary = path.with_file_name(format!(".prime-native-{}.tmp", uuid::Uuid::new_v4()));
    let temp_str = remote_path(&temporary);
    let metadata =
        runtime::block_on(connection.sftp.metadata(remote_path(&path))).map_err(sftp_failure)?;
    let permissions = metadata.permissions.unwrap_or(0o600) & 0o777;
    let result = (|| {
        runtime::block_on(async {
            let mut file = connection
                .sftp
                .open_with_flags(
                    &temp_str,
                    OpenFlags::WRITE | OpenFlags::CREATE | OpenFlags::EXCLUDE,
                )
                .await
                .map_err(sftp_failure)?;
            let mut attrs = FileAttributes::empty();
            attrs.permissions = Some(permissions);
            file.set_metadata(attrs).await.map_err(sftp_failure)?;
            file.write_all(&content).await?;
            file.flush().await?;
            file.shutdown().await?;
            Ok::<(), HostError>(())
        })?;
        if revision(&bytes(connection, relative, LIMIT)?) != expected {
            return Err(HostError::new("REVISION_CONFLICT", "remote.fileChanged"));
        }
        replace(connection, &temporary, &path)?;
        invalidate_caches(connection, relative);
        Ok(json!({"revision":revision(&content)}))
    })();
    let _ = runtime::block_on(connection.sftp.remove_file(&temp_str));
    result
}

pub fn create(
    connection: &Connection,
    relative: &str,
    kind: &str,
    content: &[u8],
) -> HostResult<Value> {
    let path = connection.resolve(relative, true)?;
    let path_str = remote_path(&path);
    if content.len() as u64 > 20 * 1024 * 1024 {
        return Err(HostError::new("REMOTE_FILE", "remote.fileTooLarge"));
    }
    match kind {
        "file" => {
            runtime::block_on(async {
                let mut file = connection
                    .sftp
                    .open_with_flags(
                        &path_str,
                        OpenFlags::WRITE | OpenFlags::CREATE | OpenFlags::EXCLUDE,
                    )
                    .await
                    .map_err(sftp_failure)?;
                let mut attrs = FileAttributes::empty();
                attrs.permissions = Some(0o600);
                file.set_metadata(attrs).await.map_err(sftp_failure)?;
                file.write_all(content).await?;
                file.flush().await?;
                file.shutdown().await?;
                Ok::<(), HostError>(())
            })?;
        }
        "folder" => {
            runtime::block_on(connection.sftp.create_dir(&path_str)).map_err(sftp_failure)?;
        }
        _ => {
            return Err(HostError::new(
                "INVALID_ARGUMENT",
                "Entry kind must be file or folder",
            ))
        }
    }
    invalidate_caches(connection, relative);
    Ok(json!({"path":relative}))
}

pub fn trash(connection: &Connection, relative: &str) -> HostResult<Value> {
    if relative.is_empty() || relative == "." || relative.starts_with(".prime-native-trash") {
        return Err(HostError::new("REMOTE_PATH", "remote.pathOutside"));
    }
    let source = connection.resolve(relative, false)?;
    let trash_relative = ".prime-native-trash";
    let trash = match connection.resolve(trash_relative, false) {
        Ok(path) => path,
        Err(_) => {
            create(connection, trash_relative, "folder", &[])?;
            connection.resolve(trash_relative, false)?
        }
    };
    let target = format!(
        "{}/{}-{}",
        remote_path(&trash).trim_end_matches('/'),
        uuid::Uuid::new_v4(),
        source.file_name().unwrap_or_default().to_string_lossy()
    );
    runtime::block_on(connection.sftp.rename(remote_path(&source), target))
        .map_err(sftp_failure)?;
    invalidate_caches(connection, relative);
    Ok(Value::Null)
}

pub fn upload(
    connection: &Connection,
    relative: &str,
    bytes: &[u8],
    overwrite: bool,
) -> HostResult<Value> {
    if bytes.len() > 50 * 1024 * 1024 {
        return Err(HostError::new("FILE_TOO_LARGE", "Upload exceeds 50 MiB"));
    }
    let target = connection.resolve(relative, true)?;
    let temporary = target.with_file_name(format!(".prime-upload-{}", uuid::Uuid::new_v4()));
    let temp_str = remote_path(&temporary);
    let result = (|| {
        runtime::block_on(async {
            let mut file = connection
                .sftp
                .open_with_flags(
                    &temp_str,
                    OpenFlags::WRITE | OpenFlags::CREATE | OpenFlags::EXCLUDE,
                )
                .await
                .map_err(sftp_failure)?;
            let mut attrs = FileAttributes::empty();
            attrs.permissions = Some(0o600);
            file.set_metadata(attrs).await.map_err(sftp_failure)?;
            file.write_all(bytes).await?;
            file.flush().await?;
            file.shutdown().await?;
            Ok::<(), HostError>(())
        })?;
        if overwrite {
            replace(connection, &temporary, &target)?;
        } else {
            let command = format!(
                "ln -- {} {}",
                super::connection::shell_quote(&temp_str),
                super::connection::shell_quote(&remote_path(&target))
            );
            let result = connection.execute(&command, "", 30)?;
            if result["exitCode"] != 0 {
                return Err(HostError::new(
                    "REMOTE_UPLOAD",
                    "Remote destination already exists or could not be linked",
                ));
            }
        }
        Ok(json!({"path":relative,"bytes":bytes.len(),"uploaded":true}))
    })();
    let _ = runtime::block_on(connection.sftp.remove_file(&temp_str));
    if result.is_ok() {
        invalidate_caches(connection, relative);
    }
    result
}

pub fn download_url(connection: &Connection, relative: &str, source: &str) -> HostResult<Value> {
    let url = url::Url::parse(source).map_err(failure)?;
    if !["http", "https"].contains(&url.scheme())
        || !url.username().is_empty()
        || url.password().is_some()
    {
        return Err(HostError::new(
            "INVALID_URL",
            "Expected HTTP(S) URL without credentials",
        ));
    }
    let path = connection.resolve(relative, true)?;
    let temporary = path.with_file_name(format!(".prime-download-{}", uuid::Uuid::new_v4()));
    let temporary = super::connection::shell_quote(&remote_path(&temporary));
    let target = super::connection::shell_quote(&remote_path(&path));
    let source = super::connection::shell_quote(url.as_str());
    let cleanup = super::connection::shell_quote(&format!("rm -f -- {temporary}"));
    let command=format!("umask 077; trap {cleanup} EXIT HUP INT TERM; ( set -C; : > {temporary} ) && ( ulimit -f 102400; if command -v curl >/dev/null 2>&1; then curl --fail --location --silent --show-error --connect-timeout 15 --max-time 110 --max-filesize 52428800 --proto '=http,https' --proto-redir '=http,https' --output {temporary} -- {source}; elif command -v wget >/dev/null 2>&1; then wget --quiet --timeout=30 --tries=1 --output-document={temporary} -- {source}; else printf 'curl or wget is required' >&2; exit 127; fi ) && test \"$(wc -c < {temporary})\" -le 52428800 && ln -- {temporary} {target}");
    let result = connection.execute(&command, "", 120)?;
    if result["exitCode"] != 0 {
        return Err(HostError::new(
            "REMOTE_DOWNLOAD",
            format!(
                "Remote download failed: {}",
                result["stderr"].as_str().unwrap_or("")
            ),
        ));
    }
    invalidate_caches(connection, relative);
    Ok(json!({"path":relative,"downloaded":true}))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn image_mime_and_read_limit_depend_on_extension() {
        assert_eq!(image_mime_for_path("photo.png"), Some("image/png"));
        assert_eq!(image_mime_for_path("dir/PHOTO.JPG"), Some("image/jpeg"));
        assert_eq!(image_mime_for_path("pic.jpeg"), Some("image/jpeg"));
        assert_eq!(image_mime_for_path("anim.webp"), Some("image/webp"));
        assert_eq!(image_mime_for_path("loop.gif"), Some("image/gif"));
        assert_eq!(image_mime_for_path("notes.txt"), None);
        assert_eq!(image_mime_for_path("dir/png"), None);

        assert_eq!(read_limit_for_path("photo.png"), None);
        assert_eq!(read_limit_for_path("dir/PHOTO.JPG"), None);
        assert_eq!(read_limit_for_path("notes.txt"), Some(LIMIT));
        assert_eq!(read_limit_for_path("dir/png"), Some(LIMIT));
    }

    #[test]
    fn video_mime_and_metadata_cover_supported_extensions() {
        assert_eq!(video_mime_for_path("clip.mp4"), Some("video/mp4"));
        assert_eq!(video_mime_for_path("dir/CLIP.M4V"), Some("video/x-m4v"));
        assert_eq!(video_mime_for_path("movie.webm"), Some("video/webm"));
        assert_eq!(video_mime_for_path("take.mov"), Some("video/quicktime"));
        assert_eq!(video_mime_for_path("stream.ogv"), Some("video/ogg"));
        assert_eq!(video_mime_for_path("notes.txt"), None);

        assert_eq!(read_limit_for_path("clip.mp4"), None);
        let metadata = video_read_metadata("out/demo.mp4").unwrap();
        assert_eq!(metadata["path"], "out/demo.mp4");
        assert_eq!(metadata["name"], "demo.mp4");
        assert_eq!(metadata["video"]["path"], "out/demo.mp4");
        assert_eq!(metadata["video"]["mime"], "video/mp4");
        assert!(metadata.get("content").is_none());
    }

    #[test]
    fn remote_symlink_detection_uses_mode_bits_and_missing_perm_fallback() {
        assert!(is_remote_symlink(Some(0o120777), false, false));
        assert!(!is_remote_symlink(Some(0o100644), true, false));
        assert!(!is_remote_symlink(Some(0o040755), false, true));
        assert!(is_remote_symlink(None, false, false));
        assert!(!is_remote_symlink(None, true, false));
    }

    #[test]
    fn remote_search_output_strips_prefix_and_skips_secrets() {
        let parsed = parse_remote_search_output("./src/app/App.tsx\n.env\n./README.md\n\n");
        let paths: Vec<&str> = parsed.iter().map(|entry| entry.path.as_str()).collect();
        assert_eq!(paths, vec!["src/app/App.tsx", "README.md"]);
    }
}
