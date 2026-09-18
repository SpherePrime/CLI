use std::path::{Component, Path, PathBuf};

use anyhow::{bail, Result};

pub fn resolve_path(working_dir: &Path, raw: &str) -> Result<PathBuf> {
    if raw.trim().is_empty() {
        bail!("path is required");
    }
    let candidate = if Path::new(raw).is_absolute() {
        PathBuf::from(raw)
    } else {
        working_dir.join(raw)
    };
    let normalized = normalize(&candidate);
    if !normalized.starts_with(working_dir) {
        bail!("path escapes working directory: {raw}");
    }
    Ok(normalized)
}

pub fn normalize(path: &Path) -> PathBuf {
    let mut out = PathBuf::new();
    for component in path.components() {
        match component {
            Component::CurDir => {}
            Component::ParentDir => {
                out.pop();
            }
            other => out.push(other.as_os_str()),
        }
    }
    out
}

pub fn display(path: &Path) -> String {
    path.to_string_lossy().replace('\\', "/")
}

pub fn truncate(text: &str, max_bytes: usize) -> String {
    if text.len() <= max_bytes {
        return text.to_string();
    }
    let mut end = max_bytes;
    while end > 0 && !text.is_char_boundary(end) {
        end -= 1;
    }
    format!(
        "{}\n...[truncated {} bytes]",
        &text[..end],
        text.len() - end
    )
}
