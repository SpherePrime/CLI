use super::connection::Connection;
use crate::error::{HostError, HostResult};
use base64::Engine;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use ssh2::{OpenFlags, OpenType, RenameFlags};
use std::{
    io::{Read, Write},
    path::Path,
};

const LIMIT: u64 = 2 * 1024 * 1024;

fn failure(error: impl std::fmt::Display) -> HostError {
    HostError::new("SFTP_ERROR", error.to_string())
}
fn revision(bytes: &[u8]) -> String {
    format!("{:x}", Sha256::digest(bytes))
}

fn replace(connection: &Connection, source: &Path, target: &Path) -> HostResult<()> {
    let source = super::connection::shell_quote(&source.to_string_lossy().replace('\\', "/"));
    let target = super::connection::shell_quote(&target.to_string_lossy().replace('\\', "/"));
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

pub fn bytes(connection: &Connection, relative: &str, limit: u64) -> HostResult<Vec<u8>> {
    let path = connection.resolve(relative, false)?;
    let metadata = connection.sftp.stat(&path).map_err(failure)?;
    if !metadata.is_file() {
        return Err(HostError::new("REMOTE_FILE", "remote.notFile"));
    }
    if metadata.size.is_some_and(|size| size > limit) {
        return Err(HostError::new("REMOTE_FILE", "remote.fileTooLarge"));
    }
    let mut bytes = Vec::new();
    connection
        .sftp
        .open(&path)
        .map_err(failure)?
        .take(limit + 1)
        .read_to_end(&mut bytes)?;
    if bytes.len() as u64 > limit {
        return Err(HostError::new("REMOTE_FILE", "remote.fileTooLarge"));
    }
    Ok(bytes)
}

pub fn list(connection: &Connection, relative: &str) -> HostResult<Value> {
    let path = connection.resolve(relative, false)?;
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
    let rules = rules.build().map_err(failure)?;
    let mut output = Vec::new();
    for (path, metadata) in connection.sftp.readdir(&path).map_err(failure)? {
        let name = path
            .file_name()
            .unwrap_or_default()
            .to_string_lossy()
            .into_owned();
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
        if rules
            .matched(format!("/{relative}"), metadata.is_dir())
            .is_ignore()
        {
            continue;
        }
        if connection.resolve(&relative, false).is_err() {
            continue;
        }
        output.push(json!({"name":name,"path":relative,"directory":metadata.is_dir()}));
    }
    output.sort_by(|left, right| {
        right["directory"]
            .as_bool()
            .cmp(&left["directory"].as_bool())
            .then_with(|| left["name"].as_str().cmp(&right["name"].as_str()))
    });
    Ok(json!(output))
}

pub fn read(connection: &Connection, relative: &str) -> HostResult<Value> {
    let bytes = bytes(connection, relative, LIMIT)?;
    let name = relative.rsplit('/').next().unwrap_or(relative);
    let extension = name.rsplit('.').next().unwrap_or("").to_lowercase();
    let mime = match extension.as_str() {
        "png" => Some("image/png"),
        "jpg" | "jpeg" => Some("image/jpeg"),
        "webp" => Some("image/webp"),
        "gif" => Some("image/gif"),
        _ => None,
    };
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
    let metadata = connection.sftp.stat(&path).map_err(failure)?;
    let result = (|| {
        let mut file = connection
            .sftp
            .open_mode(
                &temporary,
                OpenFlags::WRITE | OpenFlags::CREATE | OpenFlags::EXCLUSIVE,
                metadata.perm.unwrap_or(0o600) as i32 & 0o777,
                OpenType::File,
            )
            .map_err(failure)?;
        file.write_all(&content)?;
        file.flush()?;
        drop(file);
        if revision(&bytes(connection, relative, LIMIT)?) != expected {
            return Err(HostError::new("REVISION_CONFLICT", "remote.fileChanged"));
        }
        replace(connection, &temporary, &path)?;
        Ok(json!({"revision":revision(&content)}))
    })();
    let _ = connection.sftp.unlink(&temporary);
    result
}

pub fn create(
    connection: &Connection,
    relative: &str,
    kind: &str,
    content: &[u8],
) -> HostResult<Value> {
    let path = connection.resolve(relative, true)?;
    if content.len() as u64 > 20 * 1024 * 1024 {
        return Err(HostError::new("REMOTE_FILE", "remote.fileTooLarge"));
    }
    match kind {
        "file" => {
            let mut file = connection
                .sftp
                .open_mode(
                    &path,
                    OpenFlags::WRITE | OpenFlags::CREATE | OpenFlags::EXCLUSIVE,
                    0o600,
                    OpenType::File,
                )
                .map_err(failure)?;
            file.write_all(content)?;
            file.flush()?;
        }
        "folder" => connection.sftp.mkdir(&path, 0o755).map_err(failure)?,
        _ => {
            return Err(HostError::new(
                "INVALID_ARGUMENT",
                "Entry kind must be file or folder",
            ))
        }
    }
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
    let target = trash.join(format!(
        "{}-{}",
        uuid::Uuid::new_v4(),
        source.file_name().unwrap_or_default().to_string_lossy()
    ));
    connection
        .sftp
        .rename(&source, Path::new(&target), Some(RenameFlags::ATOMIC))
        .map_err(failure)?;
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
    let result = (|| {
        let mut file = connection
            .sftp
            .open_mode(
                &temporary,
                OpenFlags::WRITE | OpenFlags::CREATE | OpenFlags::EXCLUSIVE,
                0o600,
                OpenType::File,
            )
            .map_err(failure)?;
        file.write_all(bytes)?;
        file.flush()?;
        drop(file);
        if overwrite {
            replace(connection, &temporary, &target)?;
        } else {
            let command = format!(
                "ln -- {} {}",
                super::connection::shell_quote(&temporary.to_string_lossy().replace('\\', "/")),
                super::connection::shell_quote(&target.to_string_lossy().replace('\\', "/"))
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
    let _ = connection.sftp.unlink(&temporary);
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
    let temporary = super::connection::shell_quote(&temporary.to_string_lossy().replace('\\', "/"));
    let target = super::connection::shell_quote(&path.to_string_lossy().replace('\\', "/"));
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
    Ok(json!({"path":relative,"downloaded":true}))
}
