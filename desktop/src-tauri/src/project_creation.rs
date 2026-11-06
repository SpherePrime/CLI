use std::{io, path::{Path, PathBuf}};

pub fn validate_name(name: &str) -> io::Result<()> {
    let stem = name.split('.').next().unwrap_or("").to_uppercase();
    let reserved = matches!(stem.as_str(), "CON" | "PRN" | "AUX" | "NUL")
        || ["COM", "LPT"].iter().any(|prefix| stem.strip_prefix(prefix).map(|suffix| matches!(suffix, "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9" | "¹" | "²" | "³")).unwrap_or(false));
    if name.is_empty() || name != name.trim() || name.chars().count() > 80 || name.ends_with('.') || reserved
        || name.chars().any(|c| c.is_control() || r#"<>:"/\|?*"#.contains(c)) {
        return Err(io::Error::new(io::ErrorKind::InvalidInput, "INVALID_PROJECT_NAME"));
    }
    Ok(())
}

pub fn create_directory(documents: &Path, name: &str) -> io::Result<PathBuf> {
    validate_name(name)?;
    let base = documents.join("Prime");
    std::fs::create_dir_all(&base)?;
    let path = base.join(name);
    std::fs::create_dir(&path)?;
    Ok(path)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejects_unsafe_names() {
        for name in ["", " ", ".", "..", "../escape", "a/b", "a\\b", "C:demo", "a?", "a\0", "CON", "nul.txt", "COM1", "LPT9.log", "name.", "name ", " name"] {
            assert!(validate_name(name).is_err(), "{name:?}");
        }
        assert!(validate_name(&"a".repeat(81)).is_err());
        assert!(validate_name("Новый проект").is_ok());
    }

    #[test]
    fn concurrent_creation_never_reuses_an_existing_directory() {
        let root = std::env::temp_dir().join(format!("prime-create-race-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&root);
        let handles: Vec<_> = (0..8).map(|_| {
            let root = root.clone();
            std::thread::spawn(move || create_directory(&root, "race").is_ok())
        }).collect();
        assert_eq!(handles.into_iter().map(|handle| usize::from(handle.join().unwrap())).sum::<usize>(), 1);
        std::fs::write(root.join("Prime").join("file"), "preserved").unwrap();
        assert!(create_directory(&root, "file").is_err());
        assert_eq!(std::fs::read_to_string(root.join("Prime").join("file")).unwrap(), "preserved");
        std::fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn creates_only_new_directory_and_preserves_existing_content() {
        let root = std::env::temp_dir().join(format!("prime-create-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&root);
        let path = create_directory(&root, "Новый проект").unwrap();
        assert_eq!(path, root.join("Prime").join("Новый проект"));
        std::fs::write(path.join("keep.txt"), "keep").unwrap();
        assert!(create_directory(&root, "Новый проект").is_err());
        assert_eq!(std::fs::read_to_string(path.join("keep.txt")).unwrap(), "keep");
        assert!(create_directory(&root, "../escape").is_err());
        std::fs::remove_dir_all(root).unwrap();
    }
}
