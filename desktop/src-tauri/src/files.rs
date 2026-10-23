use crate::error::{HostError, HostResult};
use base64::Engine;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    fs,
    io::Write,
    path::{Component, Path, PathBuf},
};

pub fn scoped_path(root: &Path, relative: &str, create: bool) -> HostResult<PathBuf> {
    let relative = Path::new(relative);
    if relative
        .components()
        .any(|component| !matches!(component, Component::Normal(_) | Component::CurDir))
    {
        return Err(HostError::new(
            "PATH_DENIED",
            "Path must stay inside the project",
        ));
    }
    let root = root.canonicalize()?;
    let target = root.join(relative);
    let resolved = if create && !target.exists() {
        target
            .parent()
            .ok_or_else(|| HostError::new("PATH_DENIED", "Invalid path"))?
            .canonicalize()?
            .join(target.file_name().unwrap())
    } else {
        target.canonicalize()?
    };
    if !resolved.starts_with(&root) {
        return Err(HostError::new(
            "PATH_DENIED",
            "Path leaves the project through a link",
        ));
    }
    Ok(resolved)
}

pub fn list(root: &Path, relative: &str) -> HostResult<Value> {
    let directory = scoped_path(root, relative, false)?;
    let mut rules = ignore::gitignore::GitignoreBuilder::new(root);
    for name in [".gitignore", ".primeignore"] {
        rules.add(root.join(name));
    }
    let rules = rules
        .build()
        .map_err(|error| HostError::new("INVALID_IGNORE", error.to_string()))?;
    let mut items = Vec::new();
    for entry in fs::read_dir(&directory)? {
        let entry = entry?;
        let name = entry.file_name().to_string_lossy().into_owned();
        let directory = entry.file_type()?.is_dir();
        if [".git", "node_modules", ".superpowers", ".env", ".env.local"].contains(&name.as_str())
            || rules
                .matched_path_or_any_parents(entry.path(), directory)
                .is_ignore()
        {
            continue;
        }
        if scoped_path(
            root,
            &Path::new(relative).join(&name).to_string_lossy(),
            false,
        )
        .is_err()
        {
            continue;
        }
        items.push(json!({"name": name, "path":Path::new(relative).join(&name).to_string_lossy(), "directory":directory}));
    }
    items.sort_by(|left, right| {
        right["directory"]
            .as_bool()
            .cmp(&left["directory"].as_bool())
            .then_with(|| left["name"].as_str().cmp(&right["name"].as_str()))
    });
    Ok(Value::Array(items))
}

fn revision(bytes: &[u8]) -> String {
    format!("{:x}", Sha256::digest(bytes))
}

pub fn read(root: &Path, relative: &str) -> HostResult<Value> {
    let path = scoped_path(root, relative, false)?;
    if !path.is_file() {
        return Err(HostError::new("INVALID_FILE", "Select a file"));
    }
    if path.metadata()?.len() > 2 * 1024 * 1024 {
        return Err(HostError::new("FILE_TOO_LARGE", "File exceeds 2 MiB"));
    }
    let bytes = fs::read(&path)?;
    let extension = path
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or("")
        .to_lowercase();
    let mime = match extension.as_str() {
        "png" => Some("image/png"),
        "jpg" | "jpeg" => Some("image/jpeg"),
        "webp" => Some("image/webp"),
        "gif" => Some("image/gif"),
        _ => None,
    };
    if let Some(mime) = mime {
        return Ok(
            json!({"path":relative,"name":path.file_name().unwrap().to_string_lossy(),"image":format!("data:{mime};base64,{}",base64::engine::general_purpose::STANDARD.encode(&bytes))}),
        );
    }
    if bytes.contains(&0) {
        return Err(HostError::new(
            "BINARY_FILE",
            "Binary files cannot be edited",
        ));
    }
    let content = std::str::from_utf8(&bytes)
        .map_err(|_| HostError::new("INVALID_ENCODING", "File is not valid UTF-8"))?;
    Ok(
        json!({"path":relative,"name":path.file_name().unwrap().to_string_lossy(),"content":content,"revision":revision(&bytes)}),
    )
}

pub fn save(root: &Path, relative: &str, content: &str, expected: &str) -> HostResult<Value> {
    let path = scoped_path(root, relative, false)?;
    let original = fs::read(&path)?;
    if revision(&original) != expected {
        return Err(HostError::new(
            "REVISION_CONFLICT",
            "File changed on disk; reload before saving",
        ));
    }
    if original.contains(&0) {
        return Err(HostError::new(
            "BINARY_FILE",
            "Binary files cannot be edited",
        ));
    }
    let mut bytes = Vec::new();
    if original.starts_with(&[239, 187, 191]) && !content.starts_with('\u{feff}') {
        bytes.extend_from_slice(&[239, 187, 191]);
    }
    bytes.extend_from_slice(content.as_bytes());
    if bytes.len() > 2 * 1024 * 1024 {
        return Err(HostError::new("FILE_TOO_LARGE", "File exceeds 2 MiB"));
    }
    let mut temporary = tempfile::NamedTempFile::new_in(path.parent().unwrap())?;
    temporary
        .as_file()
        .set_permissions(path.metadata()?.permissions())?;
    temporary.write_all(&bytes)?;
    temporary.as_file().sync_all()?;
    if revision(&fs::read(&path)?) != expected {
        return Err(HostError::new(
            "REVISION_CONFLICT",
            "File changed on disk; reload before saving",
        ));
    }
    temporary.persist(&path).map_err(|error| error.error)?;
    Ok(json!({"revision":revision(&bytes)}))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn traversal_and_absolute_paths_are_denied() {
        let root = tempfile::tempdir().unwrap();
        assert!(scoped_path(root.path(), "../secret", true).is_err());
        assert!(scoped_path(root.path(), root.path().to_str().unwrap(), false).is_err());
    }

    #[test]
    fn external_edits_prevent_overwrite() {
        let root = tempfile::tempdir().unwrap();
        fs::write(root.path().join("file.txt"), "first").unwrap();
        let snapshot = read(root.path(), "file.txt").unwrap();
        fs::write(root.path().join("file.txt"), "external edit").unwrap();
        let error = save(
            root.path(),
            "file.txt",
            "replace",
            snapshot["revision"].as_str().unwrap(),
        )
        .unwrap_err();
        assert_eq!(error.code, "REVISION_CONFLICT");
        assert_eq!(
            fs::read_to_string(root.path().join("file.txt")).unwrap(),
            "external edit"
        );
    }
}
