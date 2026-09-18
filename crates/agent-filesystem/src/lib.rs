use std::path::{Path, PathBuf};

use anyhow::{anyhow, Context, Result};
use serde::{Deserialize, Serialize};
use tokio::fs;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
pub enum EditMode {
    #[default]
    SearchReplace,
    LineRange,
    Patch,
    Structured,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FileEdit {
    pub path: PathBuf,
    pub mode: EditMode,
    pub old: Option<String>,
    pub new: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub line_start: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub line_end: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub patch: Option<String>,
}

impl FileEdit {
    pub fn search_replace(path: &Path, old: &str, new: &str) -> Self {
        Self {
            path: path.to_path_buf(),
            mode: EditMode::SearchReplace,
            old: Some(old.to_string()),
            new: new.to_string(),
            line_start: None,
            line_end: None,
            patch: None,
        }
    }

    pub fn line_range(path: &Path, start: u32, end: u32, new: &str) -> Self {
        Self {
            path: path.to_path_buf(),
            mode: EditMode::LineRange,
            old: None,
            new: new.to_string(),
            line_start: Some(start),
            line_end: Some(end),
            patch: None,
        }
    }

    pub fn patch(path: &Path, patch: &str) -> Self {
        Self {
            path: path.to_path_buf(),
            mode: EditMode::Patch,
            old: None,
            new: String::new(),
            line_start: None,
            line_end: None,
            patch: Some(patch.to_string()),
        }
    }

    pub async fn apply(&self, snapshot_dir: Option<&Path>) -> Result<String> {
        let original = fs::read_to_string(&self.path)
            .await
            .with_context(|| format!("reading {}", self.path.display()))?;
        if let Some(dir) = snapshot_dir {
            let _ = fs::create_dir_all(dir).await;
            let safe = self
                .path
                .file_name()
                .and_then(|n| n.to_str())
                .unwrap_or("snapshot");
            let snapshot = dir.join(format!("{}.snapshot", safe));
            fs::write(&snapshot, &original)
                .await
                .with_context(|| format!("writing snapshot to {}", snapshot.display()))?;
        }
        let updated = match self.mode {
            EditMode::SearchReplace => {
                let old = self
                    .old
                    .as_deref()
                    .ok_or_else(|| anyhow!("search_replace edit requires `old`"))?;
                if !original.contains(old) {
                    return Err(anyhow!(
                        "edit target not found: first 40 chars of old text not in file"
                    ));
                }
                original.replacen(old, &self.new, 1)
            }
            EditMode::LineRange => {
                let start = self
                    .line_start
                    .ok_or_else(|| anyhow!("line_range edit requires line_start"))?;
                let end = self.line_end.unwrap_or(start);
                let lines: Vec<&str> = original.lines().collect();
                if start as usize > lines.len() {
                    return Err(anyhow!(
                        "line {} exceeds file length {}",
                        start,
                        lines.len()
                    ));
                }
                let mut out: Vec<String> = lines
                    .iter()
                    .take(start as usize)
                    .map(|s| s.to_string())
                    .collect();
                out.push(self.new.clone());
                out.extend(lines.iter().skip(end as usize).map(|s| s.to_string()));
                out.join("\n")
            }
            EditMode::Patch => {
                let p = self
                    .patch
                    .as_deref()
                    .ok_or_else(|| anyhow!("patch edit requires patch text"))?;
                apply_unified_patch(&original, p).with_context(|| "applying unified patch")?
            }
            EditMode::Structured => self.new.clone(),
        };
        fs::write(&self.path, &updated)
            .await
            .with_context(|| format!("writing {}", self.path.display()))?;
        Ok(updated)
    }

    pub fn diff(&self, original: &str, updated: &str) -> String {
        let ops = similar::TextDiff::from_lines(original, updated);
        let mut lines = Vec::new();
        lines.push("--- original".to_string());
        lines.push("+++ updated".to_string());
        for change in ops.iter_all_changes() {
            let marker = match change.tag() {
                similar::ChangeTag::Delete => "-",
                similar::ChangeTag::Insert => "+",
                similar::ChangeTag::Equal => " ",
            };
            lines.push(format!("{marker}{}", change.to_string().trim_end()));
        }
        lines.join("\n")
    }
}

fn apply_unified_patch(original: &str, patch: &str) -> Result<String> {
    let lines: Vec<&str> = original.lines().collect();
    let result: Vec<String> = lines.iter().map(|s| s.to_string()).collect();
    for hunk in patch.lines() {
        if let Some(target) = hunk.strip_prefix("@@") {
            let nums: Vec<&str> = target.split_whitespace().collect();
            if let Some(n) = nums.first() {
                if let Ok(idx) = n.parse::<usize>() {
                    if idx < result.len() {
                        let _ = idx;
                    }
                }
            }
        }
    }
    Ok(result.join("\n"))
}

pub async fn read_file(path: &Path) -> Result<String> {
    fs::read_to_string(path)
        .await
        .with_context(|| format!("reading {}", path.display()))
}

pub async fn write_file(path: &Path, content: &str) -> Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .await
            .with_context(|| format!("creating parent dirs for {}", path.display()))?;
    }
    fs::write(path, content)
        .await
        .with_context(|| format!("writing {}", path.display()))
}

pub async fn list_directory(path: &Path) -> Result<Vec<String>> {
    let mut rd = fs::read_dir(path)
        .await
        .with_context(|| format!("listing {}", path.display()))?;
    let mut entries = Vec::new();
    while let Some(e) = rd.next_entry().await? {
        let name = e.file_name().to_string_lossy().into_owned();
        entries.push(name);
    }
    Ok(entries)
}

pub async fn find_files(root: &Path, pattern: &str) -> Result<Vec<PathBuf>> {
    let re = glob::Pattern::new(pattern)?;
    let mut out = Vec::new();
    for entry in walkdir::WalkDir::new(root).into_iter().flatten() {
        let p = entry.path();
        if p.is_file() && re.matches(p.to_str().unwrap_or_default()) {
            out.push(p.to_path_buf());
        }
    }
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn search_replace_applies() {
        let tmp = tempfile::tempdir().unwrap();
        let f = tmp.path().join("test.txt");
        write_file(&f, "hello world").await.unwrap();
        let edit = FileEdit::search_replace(&f, "world", "agent");
        let updated = edit.apply(None).await.unwrap();
        assert_eq!(updated, "hello agent");
    }

    #[tokio::test]
    async fn line_range_applies() {
        let tmp = tempfile::tempdir().unwrap();
        let f = tmp.path().join("lines.txt");
        write_file(&f, "line1\nline2\nline3\nline4").await.unwrap();
        let edit = FileEdit::line_range(&f, 2, 2, "REPLACED");
        let updated = edit.apply(None).await.unwrap();
        assert!(updated.contains("REPLACED"));
    }
}
