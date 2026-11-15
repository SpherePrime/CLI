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

#[allow(dead_code)]
pub fn create_directory(documents: &Path, name: &str) -> io::Result<PathBuf> {
    validate_name(name)?;
    let base = documents.join("Prime");
    std::fs::create_dir_all(&base)?;
    let path = base.join(name);
    std::fs::create_dir(&path)?;
    Ok(path)
}

fn numbered_candidate(base_name: &str, index: usize) -> io::Result<String> {
    let suffix = format!(" {index}");
    let suffix_chars = suffix.chars().count();
    if suffix_chars >= 80 {
        return Err(io::Error::new(io::ErrorKind::InvalidInput, "INVALID_PROJECT_NAME"));
    }
    let max_prefix_chars = 80 - suffix_chars;
    let prefix: String = base_name.chars().take(max_prefix_chars).collect();
    let candidate = format!("{}{}", prefix.trim_end(), suffix);
    validate_name(&candidate)?;
    Ok(candidate)
}

pub fn next_available_name(documents: &Path, base_name: &str) -> io::Result<String> {
    validate_name(base_name)?;
    let base = documents.join("Prime");
    if !base.join(base_name).try_exists()? {
        return Ok(base_name.to_owned());
    }
    let mut index = 1usize;
    loop {
        let candidate = numbered_candidate(base_name, index)?;
        if !base.join(&candidate).try_exists()? {
            return Ok(candidate);
        }
        index = index.checked_add(1).ok_or_else(|| {
            io::Error::new(io::ErrorKind::AlreadyExists, "PROJECT_EXISTS")
        })?;
    }
}

pub fn create_directory_auto(documents: &Path, name: &str) -> io::Result<PathBuf> {
    validate_name(name)?;
    let base = documents.join("Prime");
    std::fs::create_dir_all(&base)?;
    let path = base.join(name);
    match std::fs::create_dir(&path) {
        Ok(()) => return Ok(path),
        Err(error) if error.kind() == io::ErrorKind::AlreadyExists => {}
        Err(error) => return Err(error),
    }
    let mut index = 1usize;
    loop {
        let candidate = numbered_candidate(name, index)?;
        let candidate_path = base.join(&candidate);
        match std::fs::create_dir(&candidate_path) {
            Ok(()) => return Ok(candidate_path),
            Err(error) if error.kind() == io::ErrorKind::AlreadyExists => {}
            Err(error) => return Err(error),
        }
        index = index.checked_add(1).ok_or_else(|| {
            io::Error::new(io::ErrorKind::AlreadyExists, "PROJECT_EXISTS")
        })?;
    }
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

    #[test]
    fn next_available_name_increments_and_skips_existing_gaps() {
        let root = tempfile::tempdir().unwrap();
        let prime = root.path().join("Prime");

        assert_eq!(next_available_name(root.path(), "Новый проект").unwrap(), "Новый проект");

        std::fs::create_dir_all(prime.join("Новый проект")).unwrap();
        assert_eq!(next_available_name(root.path(), "Новый проект").unwrap(), "Новый проект 1");

        std::fs::create_dir_all(prime.join("Новый проект 2")).unwrap();
        assert_eq!(next_available_name(root.path(), "Новый проект").unwrap(), "Новый проект 1");

        std::fs::create_dir_all(prime.join("Новый проект 1")).unwrap();
        assert_eq!(next_available_name(root.path(), "Новый проект").unwrap(), "Новый проект 3");

        let long_name = "a".repeat(80);
        std::fs::create_dir_all(prime.join(&long_name)).unwrap();
        let next_long = next_available_name(root.path(), &long_name).unwrap();
        assert_eq!(next_long.chars().count(), 80);
        assert!(next_long.ends_with(" 1"));
        assert!(validate_name(&next_long).is_ok());

        assert!(next_available_name(root.path(), "../escape").is_err());
    }

    #[test]
    fn create_directory_auto_increments_and_preserves_existing_contents() {
        let root = tempfile::tempdir().unwrap();
        let first = create_directory_auto(root.path(), "Новый проект").unwrap();
        assert_eq!(first, root.path().join("Prime").join("Новый проект"));
        std::fs::write(first.join("keep.txt"), "keep").unwrap();

        let second = create_directory_auto(root.path(), "Новый проект").unwrap();
        assert_eq!(second, root.path().join("Prime").join("Новый проект 1"));

        let third = create_directory_auto(root.path(), "Новый проект").unwrap();
        assert_eq!(third, root.path().join("Prime").join("Новый проект 2"));

        assert_eq!(std::fs::read_to_string(first.join("keep.txt")).unwrap(), "keep");

        std::fs::write(root.path().join("Prime").join("file"), "preserved").unwrap();
        let after_file = create_directory_auto(root.path(), "file").unwrap();
        assert_eq!(after_file, root.path().join("Prime").join("file 1"));
        assert_eq!(std::fs::read_to_string(root.path().join("Prime").join("file")).unwrap(), "preserved");

        assert!(create_directory_auto(root.path(), "../escape").is_err());
    }

    #[test]
    fn concurrent_auto_creation_creates_distinct_directories() {
        let root = tempfile::tempdir().unwrap();
        let root_path = root.path().to_path_buf();
        let handles: Vec<_> = (0..8)
            .map(|_| {
                let root_path = root_path.clone();
                std::thread::spawn(move || create_directory_auto(&root_path, "Новый проект").unwrap())
            })
            .collect();
        let mut created: Vec<String> = handles
            .into_iter()
            .map(|handle| {
                let path = handle.join().unwrap();
                assert!(path.is_dir());
                path.file_name().unwrap().to_string_lossy().into_owned()
            })
            .collect();
        created.sort();
        let mut expected = vec![
            "Новый проект".to_owned(),
            "Новый проект 1".to_owned(),
            "Новый проект 2".to_owned(),
            "Новый проект 3".to_owned(),
            "Новый проект 4".to_owned(),
            "Новый проект 5".to_owned(),
            "Новый проект 6".to_owned(),
            "Новый проект 7".to_owned(),
        ];
        expected.sort();
        assert_eq!(created, expected);
    }
}
