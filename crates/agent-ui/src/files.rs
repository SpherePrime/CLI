use std::sync::RwLock;

static CACHE: RwLock<Option<Vec<String>>> = RwLock::new(None);

fn skip_dir(name: &str) -> bool {
    matches!(
        name,
        "target" | ".git" | "node_modules" | ".hg" | ".svn" | "sessions" | ".agent"
    )
}

pub fn file_candidates() -> Vec<String> {
    if let Ok(guard) = CACHE.read() {
        if let Some(cached) = &*guard {
            return cached.clone();
        }
    }
    build_candidates()
}

pub fn refresh_file_cache() {
    let files = build_candidates();
    if let Ok(mut guard) = CACHE.write() {
        *guard = Some(files);
    }
}

fn build_candidates() -> Vec<String> {
    let cwd = match std::env::current_dir() {
        Ok(d) => d,
        Err(_) => return Vec::new(),
    };
    let mut files = Vec::new();
    let mut stack = vec![(cwd.clone(), 0)];

    while let Some((dir, depth)) = stack.pop() {
        let entries = match std::fs::read_dir(&dir) {
            Ok(e) => e,
            Err(_) => continue,
        };
        for entry in entries.flatten() {
            let path = entry.path();
            let file_name = path.file_name().map(|n| n.to_string_lossy().to_string());
            let name = file_name.unwrap_or_default();
            if path.is_dir() {
                if !skip_dir(&name) && depth < 6 {
                    stack.push((path, depth + 1));
                }
            } else if path.is_file() {
                if let Ok(rel) = path.strip_prefix(&cwd) {
                    let text = rel.to_string_lossy().replace('\\', "/");
                    if !text.starts_with('.') {
                        files.push(text);
                    }
                }
            }
        }
    }

    files.sort();
    files.truncate(2000);
    files
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn skip_dirs_filtered() {
        assert!(skip_dir("target"));
        assert!(skip_dir(".git"));
        assert!(!skip_dir("src"));
    }

    #[test]
    fn candidates_non_empty_in_repo() {
        let files = file_candidates();
        assert!(!files.is_empty());
    }
}
