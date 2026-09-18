use std::collections::hash_map::DefaultHasher;
use std::hash::{Hash, Hasher};
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};

const ROOT_NAME: &str = "undo";

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Snapshot {
    pub path: String,
    pub content: String,
    pub taken_at: u64,
}

fn root() -> PathBuf {
    if let Ok(override_root) = std::env::var("AGENT_SNAPSHOT_ROOT") {
        return PathBuf::from(override_root);
    }
    dirs::home_dir()
        .map(|home| home.join(".agent").join(ROOT_NAME))
        .unwrap_or_else(|| PathBuf::from(ROOT_NAME))
}

fn workdir_key(working_dir: &Path) -> String {
    let mut hasher = DefaultHasher::new();
    working_dir.to_string_lossy().hash(&mut hasher);
    format!("{:016x}", hasher.finish())
}

fn snapshot_dir_at(root: &Path, working_dir: &Path) -> PathBuf {
    root.join(workdir_key(working_dir))
}

fn snapshot_dir(working_dir: &Path) -> PathBuf {
    snapshot_dir_at(&root(), working_dir)
}

fn safe_file_name(path: &Path) -> String {
    let base = path
        .file_name()
        .map(|name| name.to_string_lossy().to_string())
        .unwrap_or_else(|| "file".to_string());
    let sanitized: String = base
        .chars()
        .map(|character| {
            if character.is_ascii_alphanumeric() || matches!(character, '.' | '_' | '-') {
                character
            } else {
                '_'
            }
        })
        .collect();
    sanitized
}

pub fn take_snapshot(working_dir: &Path, path: &Path) -> Result<Option<String>> {
    take_snapshot_at(&root(), working_dir, path)
}

pub fn take_snapshot_at(root: &Path, working_dir: &Path, path: &Path) -> Result<Option<String>> {
    if !path.exists() || !path.is_file() {
        return Ok(None);
    }
    let absolute = if path.is_absolute() {
        path.to_path_buf()
    } else {
        working_dir.join(path)
    };
    let dir = snapshot_dir_at(root, working_dir);
    std::fs::create_dir_all(&dir).with_context(|| format!("creating {}", dir.display()))?;
    let taken_at = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|duration| duration.as_nanos() as u64)
        .unwrap_or(0);
    let name = format!("{:020}-{}.json", taken_at, safe_file_name(&absolute));
    let snapshot = Snapshot {
        path: absolute.to_string_lossy().to_string(),
        content: std::fs::read_to_string(&absolute)
            .with_context(|| format!("reading {}", absolute.display()))?,
        taken_at,
    };
    let target = dir.join(name);
    std::fs::write(&target, serde_json::to_string_pretty(&snapshot)?)
        .with_context(|| format!("writing {}", target.display()))?;
    Ok(Some(target.to_string_lossy().to_string()))
}

fn list(dir: &Path) -> Result<Vec<(PathBuf, Snapshot)>> {
    let mut entries: Vec<(PathBuf, Snapshot)> = Vec::new();
    if !dir.exists() {
        return Ok(entries);
    }
    let mut paths: Vec<PathBuf> = std::fs::read_dir(dir)?
        .flatten()
        .map(|entry| entry.path())
        .filter(|path| path.extension().is_some_and(|ext| ext == "json"))
        .collect();
    paths.sort();
    for path in paths {
        let raw = std::fs::read_to_string(&path)
            .with_context(|| format!("reading {}", path.display()))?;
        if let Ok(snapshot) = serde_json::from_str::<Snapshot>(&raw) {
            entries.push((path, snapshot));
        }
    }
    Ok(entries)
}

pub fn latest_snapshot(working_dir: &Path) -> Result<Option<(PathBuf, Snapshot)>> {
    latest_snapshot_at(&root(), working_dir)
}

pub fn latest_snapshot_at(root: &Path, working_dir: &Path) -> Result<Option<(PathBuf, Snapshot)>> {
    Ok(list(&snapshot_dir_at(root, working_dir))?.pop())
}

pub fn restore_latest(working_dir: &Path) -> Result<Option<String>> {
    restore_latest_at(&root(), working_dir)
}

pub fn restore_latest_at(root: &Path, working_dir: &Path) -> Result<Option<String>> {
    let Some((_, snapshot)) = latest_snapshot_at(root, working_dir)? else {
        return Ok(None);
    };
    let target = PathBuf::from(&snapshot.path);
    let parent = target.parent().context("snapshot path has no parent")?;
    std::fs::create_dir_all(parent).with_context(|| format!("creating {}", parent.display()))?;
    std::fs::write(&target, &snapshot.content)
        .with_context(|| format!("writing {}", target.display()))?;
    remove_snapshot_at(root, working_dir, &snapshot.path)?;
    Ok(Some(target.to_string_lossy().to_string()))
}

fn remove_snapshot_at(root: &Path, working_dir: &Path, path: &str) -> Result<()> {
    let dir = snapshot_dir_at(root, working_dir);
    let mut paths: Vec<PathBuf> = std::fs::read_dir(&dir)?
        .flatten()
        .map(|entry| entry.path())
        .filter(|candidate| {
            std::fs::read_to_string(candidate)
                .ok()
                .and_then(|raw| serde_json::from_str::<Snapshot>(&raw).ok())
                .is_some_and(|snapshot| snapshot.path == path)
        })
        .collect();
    if let Some(match_path) = paths.pop() {
        std::fs::remove_file(match_path)?;
    }
    Ok(())
}

pub fn list_snapshots(working_dir: &Path) -> Result<Vec<String>> {
    let mut out = Vec::new();
    for (_, snapshot) in list(&snapshot_dir(working_dir))?.into_iter().rev() {
        out.push(format!(
            "{} (taken at {})",
            snapshot.path, snapshot.taken_at
        ));
    }
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn snapshot_and_restore_round_trip() {
        let tmp = tempfile::tempdir().unwrap();
        let root = tmp.path().join("undo");
        let dir = tmp.path().join("project");
        std::fs::create_dir_all(&dir).unwrap();
        let file = dir.join("notes.txt");
        std::fs::write(&file, "v1 content").unwrap();

        take_snapshot_at(&root, &dir, &file).unwrap();
        std::fs::write(&file, "v2 content").unwrap();

        let restored = restore_latest_at(&root, &dir).unwrap().unwrap();
        assert_eq!(restored, file.to_string_lossy().to_string());
        assert_eq!(std::fs::read_to_string(&file).unwrap(), "v1 content");
        assert!(latest_snapshot_at(&root, &dir).unwrap().is_none());
    }

    #[test]
    fn snapshot_skips_missing_files_and_empty_restores() {
        let tmp = tempfile::tempdir().unwrap();
        let root = tmp.path().join("undo");
        let dir = tmp.path().join("project");
        std::fs::create_dir_all(&dir).unwrap();
        let missing = dir.join("nope.txt");
        assert!(take_snapshot_at(&root, &dir, &missing).unwrap().is_none());
        assert!(restore_latest_at(&root, &dir).unwrap().is_none());
    }
}
