use std::path::{Path, PathBuf};

use anyhow::{anyhow, bail, Context, Result};
use serde::{Deserialize, Serialize};
use tokio::fs;

pub mod snapshot;

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

fn parse_hunk_old_start(header: &str) -> Result<usize> {
    let rest = header.strip_prefix("@@").unwrap_or(header).trim();
    let token = rest
        .split_whitespace()
        .next()
        .ok_or_else(|| anyhow!("malformed hunk header: {header}"))?;
    let range = token
        .strip_prefix('-')
        .ok_or_else(|| anyhow!("hunk header must start with '-' range: {header}"))?;
    let start = range
        .split(',')
        .next()
        .unwrap_or("0")
        .parse::<usize>()
        .with_context(|| format!("invalid hunk start in {header}"))?;
    Ok(start)
}

fn locate_hunk(lines: &[String], expected: &[String], near: isize) -> Option<usize> {
    if expected.is_empty() {
        return Some(near.clamp(0, lines.len() as isize) as usize);
    }
    let len = lines.len();
    if expected.len() > len {
        return None;
    }
    let last = len - expected.len();
    let base = near.clamp(0, last as isize) as usize;
    let max_distance = std::cmp::max(base, last - base);
    for distance in 0..=max_distance {
        if base + distance <= last
            && lines[base + distance..base + distance + expected.len()] == *expected
        {
            return Some(base + distance);
        }
        if distance <= base && lines[base - distance..base - distance + expected.len()] == *expected
        {
            return Some(base - distance);
        }
    }
    None
}

fn apply_hunk(
    lines: &mut Vec<String>,
    old_start: usize,
    body: &[&str],
    offset: &mut isize,
) -> Result<()> {
    let mut old_lines: Vec<String> = Vec::new();
    let mut new_lines: Vec<String> = Vec::new();
    for line in body {
        match line.as_bytes().first() {
            Some(b'\\') => continue,
            Some(b'+') => new_lines.push(line[1..].to_string()),
            Some(b'-') => old_lines.push(line[1..].to_string()),
            _ => {
                let context = line.strip_prefix(' ').unwrap_or(line);
                old_lines.push(context.to_string());
                new_lines.push(context.to_string());
            }
        }
    }
    let expected = old_start.saturating_sub(1) as isize + *offset;
    let Some(position) = locate_hunk(lines, &old_lines, expected) else {
        bail!("patch context not found near line {old_start}");
    };
    lines.splice(
        position..position + old_lines.len(),
        new_lines.iter().cloned(),
    );
    *offset += new_lines.len() as isize - old_lines.len() as isize;
    Ok(())
}

pub fn apply_unified_patch(original: &str, patch: &str) -> Result<String> {
    let trailing_newline = original.ends_with('\n');
    let mut lines: Vec<String> = original.lines().map(str::to_string).collect();
    let patch_lines: Vec<&str> = patch.lines().collect();
    if !patch_lines.iter().any(|line| line.starts_with("@@")) {
        bail!("patch contains no hunks (@@)");
    }
    let mut index = 0;
    let mut offset: isize = 0;
    while index < patch_lines.len() {
        if !patch_lines[index].starts_with("@@") {
            index += 1;
            continue;
        }
        let old_start = parse_hunk_old_start(patch_lines[index])?;
        index += 1;
        let mut body: Vec<&str> = Vec::new();
        while index < patch_lines.len() {
            let line = patch_lines[index];
            if line.starts_with("@@") || line.starts_with("--- ") || line.starts_with("+++ ") {
                break;
            }
            if line.is_empty() {
                body.push(" ");
                index += 1;
                continue;
            }
            match line.as_bytes()[0] {
                b' ' | b'+' | b'-' | b'\\' => {
                    body.push(line);
                    index += 1;
                }
                _ => break,
            }
        }
        apply_hunk(&mut lines, old_start, &body, &mut offset)?;
    }
    let mut out = lines.join("\n");
    if trailing_newline {
        out.push('\n');
    }
    Ok(out)
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

    #[tokio::test]
    async fn unified_patch_replaces_and_inserts() {
        let tmp = tempfile::tempdir().unwrap();
        let f = tmp.path().join("p.txt");
        write_file(&f, "a\nb\nc\n").await.unwrap();
        let patch = "@@ -1,3 +1,4 @@\n a\n-b\n+B\n c\n+d";
        let updated = FileEdit::patch(&f, patch).apply(None).await.unwrap();
        assert_eq!(updated, "a\nB\nc\nd\n");
    }

    #[tokio::test]
    async fn unified_patch_tolerates_shifted_context() {
        let tmp = tempfile::tempdir().unwrap();
        let f = tmp.path().join("shift.txt");
        write_file(&f, "intro\nkeep\nold\nkeep2\n").await.unwrap();
        let patch = "@@ -1,3 +1,3 @@\n keep\n-old\n+new\n keep2";
        let updated = FileEdit::patch(&f, patch).apply(None).await.unwrap();
        assert_eq!(updated, "intro\nkeep\nnew\nkeep2\n");
    }

    #[tokio::test]
    async fn unified_patch_rejects_missing_context() {
        let tmp = tempfile::tempdir().unwrap();
        let f = tmp.path().join("bad.txt");
        write_file(&f, "hello\n").await.unwrap();
        let patch = "@@ -1 +1 @@\n-missing\n+new";
        assert!(FileEdit::patch(&f, patch).apply(None).await.is_err());
    }
}
