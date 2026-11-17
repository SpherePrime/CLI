use crate::{
    error::{HostError, HostResult},
    file_index::{build_gitignore, search_uncached, should_fast_skip, FileCache},
};
use base64::Engine;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    fs,
    io::Write,
    path::{Component, Path, PathBuf},
};

fn validate_relative(relative: &str) -> HostResult<&Path> {
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
    Ok(relative)
}

pub fn scoped_path(root: &Path, relative: &str, create: bool) -> HostResult<PathBuf> {
    let relative = validate_relative(relative)?;
    let root = dunce::canonicalize(root)?;
    let target = root.join(relative);
    let resolved = if create && !target.exists() {
        target
            .parent()
            .ok_or_else(|| HostError::new("PATH_DENIED", "Invalid path"))
            .and_then(|parent| Ok(dunce::canonicalize(parent)?))?
            .join(target.file_name().unwrap())
    } else {
        dunce::canonicalize(&target)?
    };
    if !resolved.starts_with(&root) {
        return Err(HostError::new(
            "PATH_DENIED",
            "Path leaves the project through a link",
        ));
    }
    Ok(resolved)
}

pub fn trash(root: &Path, relative: &str) -> HostResult<()> {
    let target = scoped_path(root, relative, false)?;
    if target == dunce::canonicalize(root)? {
        return Err(HostError::new(
            "PATH_DENIED",
            "The project root cannot be deleted",
        ));
    }
    trash::delete(&target).map_err(|error| HostError::new("TRASH_FAILED", error.to_string()))
}

pub fn editor_url(target: &Path) -> HostResult<String> {
    let url = url::Url::from_file_path(target)
        .map_err(|_| HostError::new("INVALID_PATH", "Path cannot be opened in the editor"))?;
    Ok(format!("vscode://file{}", url.path()))
}

pub fn list(root: &Path, relative: &str) -> HostResult<Value> {
    list_inner(root, relative, None)
}

pub fn list_cached(cache: &FileCache, root: &Path, relative: &str) -> HostResult<Value> {
    list_inner(root, relative, Some(cache))
}

pub fn search_files(root: &Path, query: &str, limit: usize) -> HostResult<Value> {
    search_uncached(root, query, limit)
}

pub fn search_cached(
    cache: &FileCache,
    root: &Path,
    query: &str,
    limit: usize,
) -> HostResult<Value> {
    cache.search(root, query, limit)
}

fn list_inner(root: &Path, relative: &str, cache: Option<&FileCache>) -> HostResult<Value> {
    let canonical_root = dunce::canonicalize(root)?;
    let relative_path = validate_relative(relative)?;
    let directory = dunce::canonicalize(canonical_root.join(relative_path))?;
    if !directory.starts_with(&canonical_root) {
        return Err(HostError::new(
            "PATH_DENIED",
            "Path leaves the project through a link",
        ));
    }
    let rules = match cache {
        Some(cache) => cache.gitignore(&canonical_root)?,
        None => build_gitignore(&canonical_root)?,
    };
    let mut items = Vec::new();
    for entry in fs::read_dir(&directory)? {
        let entry = entry?;
        let name = entry.file_name().to_string_lossy().into_owned();
        let file_type = entry.file_type()?;
        let is_dir = if file_type.is_symlink() {
            let Ok(resolved) = dunce::canonicalize(entry.path()) else {
                continue;
            };
            if !resolved.starts_with(&canonical_root) {
                continue;
            }
            resolved.is_dir()
        } else {
            file_type.is_dir()
        };
        if should_fast_skip(&name, is_dir) {
            continue;
        }
        let entry_path = entry.path();
        if entry_path.starts_with(&canonical_root)
            && rules
                .matched_path_or_any_parents(&entry_path, is_dir)
                .is_ignore()
        {
            continue;
        }
        items.push(json!({
            "name": name,
            "path": relative_path.join(&name).to_string_lossy(),
            "directory": is_dir
        }));
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

pub use crate::media::video_mime_for_path;

pub fn read(root: &Path, relative: &str) -> HostResult<Value> {
    let path = scoped_path(root, relative, false)?;
    if !path.is_file() {
        return Err(HostError::new("INVALID_FILE", "Select a file"));
    }
    let name = path.file_name().unwrap().to_string_lossy();
    if let Some(mime) = video_mime_for_path(&path) {
        return Ok(json!({
            "path": relative,
            "name": name,
            "video": {
                "path": relative,
                "mime": mime
            }
        }));
    }
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
    if mime.is_none() && path.metadata()?.len() > 2 * 1024 * 1024 {
        return Err(HostError::new("FILE_TOO_LARGE", "File exceeds 2 MiB"));
    }
    let bytes = fs::read(&path)?;
    if let Some(mime) = mime {
        return Ok(
            json!({"path":relative,"name":name,"image":format!("data:{mime};base64,{}",base64::engine::general_purpose::STANDARD.encode(&bytes))}),
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
    fn listing_applies_ignore_rules_for_uncanonical_roots() {
        let root = tempfile::tempdir().unwrap();
        fs::write(root.path().join(".gitignore"), "build/\n*.log\n").unwrap();
        fs::create_dir_all(root.path().join("build")).unwrap();
        fs::create_dir_all(root.path().join("src")).unwrap();
        fs::write(root.path().join("debug.log"), "").unwrap();
        fs::write(root.path().join("src").join("main.rs"), "").unwrap();
        let uncanonical = root.path().join("src").join("..");
        let names: Vec<String> = list(&uncanonical, "")
            .unwrap()
            .as_array()
            .unwrap()
            .iter()
            .map(|item| item["name"].as_str().unwrap().to_owned())
            .collect();
        assert_eq!(names, vec!["src", ".gitignore"]);
        let nested = list(&uncanonical, "src").unwrap();
        assert_eq!(nested[0]["name"], "main.rs");
    }

    #[test]
    fn trash_refuses_the_project_root_and_moves_entries() {
        let root = tempfile::tempdir().unwrap();
        fs::write(root.path().join("old.txt"), "x").unwrap();
        assert_eq!(trash(root.path(), "").unwrap_err().code, "PATH_DENIED");
        trash(root.path(), "old.txt").unwrap();
        assert!(!root.path().join("old.txt").exists());
    }

    #[test]
    fn editor_url_uses_the_vscode_file_scheme() {
        let root = tempfile::tempdir().unwrap();
        let url = editor_url(&dunce::canonicalize(root.path()).unwrap()).unwrap();
        assert!(url.starts_with("vscode://file/"));
        assert!(!url.contains('\\'));
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

    #[test]
    fn large_image_files_can_be_read_without_size_limit() {
        let root = tempfile::tempdir().unwrap();
        let data = vec![0x89u8; 2 * 1024 * 1024 + 1024];
        fs::write(root.path().join("large.png"), &data).unwrap();
        let result = read(root.path(), "large.png").unwrap();
        assert_eq!(result["name"], "large.png");
        assert!(result["image"]
            .as_str()
            .unwrap()
            .starts_with("data:image/png;base64,"));
    }

    #[test]
    fn large_text_files_still_exceed_limit() {
        let root = tempfile::tempdir().unwrap();
        let data = vec![b'a'; 2 * 1024 * 1024 + 1024];
        fs::write(root.path().join("large.txt"), &data).unwrap();
        let error = read(root.path(), "large.txt").unwrap_err();
        assert_eq!(error.code, "FILE_TOO_LARGE");
    }

    #[test]
    fn video_files_return_metadata_without_size_or_binary_errors() {
        let root = tempfile::tempdir().unwrap();
        let mut data = vec![0u8; 2 * 1024 * 1024 + 1024];
        data[4..12].copy_from_slice(b"ftypmp42");
        fs::write(root.path().join("render.mp4"), &data).unwrap();
        let result = read(root.path(), "render.mp4").unwrap();
        assert_eq!(result["path"], "render.mp4");
        assert_eq!(result["name"], "render.mp4");
        assert_eq!(result["video"]["path"], "render.mp4");
        assert_eq!(result["video"]["mime"], "video/mp4");
        assert!(result.get("content").is_none());
    }

    #[test]
    fn listing_skips_fast_ignore_directories_and_secret_files() {
        let root = tempfile::tempdir().unwrap();
        for dir in [
            "dist",
            "build",
            "target",
            ".prime",
            "__pycache__",
            ".next",
            ".nuxt",
            ".turbo",
            ".venv",
            ".cache",
            "vendor",
            ".git",
            "node_modules",
            ".superpowers",
            "src",
        ] {
            fs::create_dir_all(root.path().join(dir)).unwrap();
        }
        fs::write(root.path().join(".env"), "SECRET=1").unwrap();
        fs::write(root.path().join(".env.local"), "SECRET=2").unwrap();
        fs::write(root.path().join("README.md"), "# demo").unwrap();

        let names: Vec<String> = list(root.path(), "")
            .unwrap()
            .as_array()
            .unwrap()
            .iter()
            .map(|item| item["name"].as_str().unwrap().to_owned())
            .collect();
        assert_eq!(names, vec!["src", "README.md"]);
    }

    #[cfg(unix)]
    #[test]
    fn listing_filters_escaping_symlinks_and_keeps_internal_symlinks() {
        let outside = tempfile::tempdir().unwrap();
        fs::write(outside.path().join("secret.txt"), "secret").unwrap();
        let root = tempfile::tempdir().unwrap();
        fs::create_dir_all(root.path().join("src")).unwrap();
        fs::write(root.path().join("src").join("main.rs"), "fn main() {}").unwrap();
        std::os::unix::fs::symlink(
            outside.path().join("secret.txt"),
            root.path().join("escape.txt"),
        )
        .unwrap();
        std::os::unix::fs::symlink(
            root.path().join("src").join("main.rs"),
            root.path().join("linked.rs"),
        )
        .unwrap();

        let names: Vec<String> = list(root.path(), "")
            .unwrap()
            .as_array()
            .unwrap()
            .iter()
            .map(|item| item["name"].as_str().unwrap().to_owned())
            .collect();
        assert_eq!(names, vec!["src", "linked.rs"]);
    }

    #[test]
    fn cached_gitignore_invalidates_when_ignore_file_changes() {
        let root = tempfile::tempdir().unwrap();
        let cache = FileCache::new();
        fs::write(root.path().join("keep.txt"), "").unwrap();
        fs::write(root.path().join("ignored.log"), "").unwrap();

        let initial: Vec<String> = list_cached(&cache, root.path(), "")
            .unwrap()
            .as_array()
            .unwrap()
            .iter()
            .map(|item| item["name"].as_str().unwrap().to_owned())
            .collect();
        assert_eq!(initial, vec!["ignored.log", "keep.txt"]);

        fs::write(root.path().join(".gitignore"), "*.log\n").unwrap();
        cache.invalidate(root.path());

        let after: Vec<String> = list_cached(&cache, root.path(), "")
            .unwrap()
            .as_array()
            .unwrap()
            .iter()
            .map(|item| item["name"].as_str().unwrap().to_owned())
            .collect();
        assert_eq!(after, vec![".gitignore", "keep.txt"]);
    }

    #[test]
    fn search_files_walks_subdirectories_respects_ignores_and_limits() {
        let root = tempfile::tempdir().unwrap();
        fs::write(root.path().join(".gitignore"), "*.tmp\n").unwrap();
        fs::write(root.path().join(".primeignore"), "secret/\n").unwrap();
        fs::create_dir_all(root.path().join("src").join("app")).unwrap();
        fs::create_dir_all(root.path().join("src").join("components")).unwrap();
        fs::create_dir_all(root.path().join("node_modules").join("pkg")).unwrap();
        fs::create_dir_all(root.path().join("target").join("debug")).unwrap();
        fs::create_dir_all(root.path().join("secret")).unwrap();

        fs::write(root.path().join("README.md"), "readme").unwrap();
        fs::write(root.path().join("src").join("app").join("App.tsx"), "").unwrap();
        fs::write(
            root.path().join("src").join("components").join("AppHeader.tsx"),
            "",
        )
        .unwrap();
        fs::write(root.path().join("scratch.tmp"), "").unwrap();
        fs::write(root.path().join("secret").join("keys.txt"), "").unwrap();
        fs::write(
            root.path().join("node_modules").join("pkg").join("App.js"),
            "",
        )
        .unwrap();
        fs::write(root.path().join("target").join("debug").join("App"), "").unwrap();

        let results = search_files(root.path(), "app", 10)
            .unwrap()
            .as_array()
            .unwrap()
            .clone();
        let paths: Vec<String> = results
            .iter()
            .map(|item| item["path"].as_str().unwrap().to_owned())
            .collect();
        assert!(paths.contains(&"src/app/App.tsx".to_string()));
        assert!(paths.contains(&"src/components/AppHeader.tsx".to_string()));
        assert!(!paths.iter().any(|path| path.contains("node_modules")));
        assert!(!paths.iter().any(|path| path.contains("target")));
        assert!(!paths.iter().any(|path| path.contains("secret")));
        assert!(!paths.iter().any(|path| path.ends_with(".tmp")));

        let limited = search_files(root.path(), "", 2).unwrap();
        assert_eq!(limited.as_array().unwrap().len(), 2);

        let cache = FileCache::new();
        let cached_before = search_cached(&cache, root.path(), "brandnew", 10).unwrap();
        assert!(cached_before.as_array().unwrap().is_empty());
        fs::write(root.path().join("src").join("brandnew.ts"), "").unwrap();
        cache.invalidate(root.path());
        let cached_after = search_cached(&cache, root.path(), "brandnew", 10).unwrap();
        assert_eq!(cached_after[0]["path"], "src/brandnew.ts");
    }
}
