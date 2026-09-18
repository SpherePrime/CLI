use serde::{Deserialize, Serialize};

pub fn git_root(start: &std::path::Path) -> Option<std::path::PathBuf> {
    let mut dir = start;
    loop {
        if dir.join(".git").exists() {
            return Some(dir.to_path_buf());
        }
        dir = dir.parent()?;
    }
}

pub fn canonical_project_root(path: &std::path::Path) -> std::path::PathBuf {
    let canonical = std::fs::canonicalize(path).unwrap_or_else(|_| path.to_path_buf());
    let canonical = strip_windows_prefix(canonical);
    git_root(&canonical).unwrap_or(canonical)
}

fn strip_windows_prefix(path: std::path::PathBuf) -> std::path::PathBuf {
    let raw = path.to_string_lossy();
    if cfg!(windows) {
        if let Some(rest) = raw.strip_prefix(r"\\?\UNC\") {
            return std::path::PathBuf::from(format!(r"\\{rest}"));
        }
        if let Some(rest) = raw.strip_prefix(r"\\?\") {
            return std::path::PathBuf::from(rest.to_string());
        }
    }
    path
}

pub fn git_remote_origin(root: &std::path::Path) -> Option<String> {
    let output = std::process::Command::new("git")
        .args(["config", "--get", "remote.origin.url"])
        .current_dir(root)
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    let remote = String::from_utf8_lossy(&output.stdout).trim().to_string();
    (!remote.is_empty()).then_some(remote)
}

pub fn git_branch(root: &std::path::Path) -> Option<String> {
    let output = std::process::Command::new("git")
        .args(["rev-parse", "--abbrev-ref", "HEAD"])
        .current_dir(root)
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    let branch = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if branch.is_empty() || branch == "HEAD" {
        return None;
    }
    Some(branch)
}

pub fn project_id(path: &std::path::Path) -> String {
    let bytes = path.to_string_lossy().as_bytes().to_vec();
    let mut hash: u64 = 0xcbf29ce484222325;
    for byte in bytes {
        hash ^= u64::from(byte);
        hash = hash.wrapping_mul(0x100000001b3);
    }
    format!("{hash:016x}")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn git_root_found_at_repo_boundary() {
        let tmp = tempfile::tempdir().unwrap();
        let sub = tmp.path().join("src").join("deep");
        std::fs::create_dir_all(&sub).unwrap();
        std::fs::create_dir(tmp.path().join(".git")).unwrap();
        assert_eq!(git_root(&sub).unwrap(), tmp.path());
    }

    #[test]
    fn canonical_root_falls_back_to_path_outside_git() {
        let tmp = tempfile::tempdir().unwrap();
        let root = canonical_project_root(tmp.path());
        let canonical = std::fs::canonicalize(tmp.path()).unwrap();
        let expected = canonical.to_string_lossy().into_owned();
        let expected = expected.trim_start_matches(r"\\?\");
        assert_eq!(root.to_string_lossy().as_ref(), expected);
    }

    #[test]
    fn project_id_is_stable_and_distinct() {
        let a = std::path::Path::new("C:/projects/one");
        let b = std::path::Path::new("C:/projects/two");
        assert_eq!(project_id(a), project_id(a));
        assert_ne!(project_id(a), project_id(b));
        assert_eq!(project_id(a).len(), 16);
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
pub enum Language {
    #[default]
    Unknown,
    Rust,
    TypeScript,
    JavaScript,
    Python,
    Go,
    Java,
    CSharp,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
pub enum PackageManager {
    #[default]
    None,
    Npm,
    Pnpm,
    Yarn,
    Cargo,
    Pip,
    Poetry,
    Go,
    Maven,
    Gradle,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ProjectInfo {
    pub root: String,
    pub language: Language,
    pub package_manager: PackageManager,
    pub framework: Option<String>,
    pub is_git_repo: bool,
    pub test_framework: Option<String>,
}

impl ProjectInfo {
    pub fn detect(root: &str) -> Self {
        let p = std::path::Path::new(root);
        let mut info = Self {
            root: root.to_string(),
            language: Language::Unknown,
            package_manager: PackageManager::None,
            framework: None,
            is_git_repo: p.join(".git").exists(),
            test_framework: None,
        };

        if p.join("Cargo.toml").exists() {
            info.language = Language::Rust;
            info.package_manager = PackageManager::Cargo;
            info.test_framework = Some("cargo-test".to_string());
        }
        if p.join("package.json").exists() {
            if p.join("pnpm-lock.yaml").exists() {
                info.package_manager = PackageManager::Pnpm;
            } else if p.join("yarn.lock").exists() {
                info.package_manager = PackageManager::Yarn;
            } else {
                info.package_manager = PackageManager::Npm;
            }
            if p.join("tsconfig.json").exists() {
                info.language = Language::TypeScript;
            } else {
                info.language = Language::JavaScript;
            }
            if p.join("vitest.config.ts").exists() {
                info.test_framework = Some("vitest".to_string());
            }
        }
        if p.join("pyproject.toml").exists() {
            info.language = Language::Python;
            info.package_manager = if p.join("poetry.lock").exists() {
                PackageManager::Poetry
            } else {
                PackageManager::Pip
            };
            if p.join("pytest.ini").exists() || p.join("conftest.py").exists() {
                info.test_framework = Some("pytest".to_string());
            }
        }
        if p.join("go.mod").exists() {
            info.language = Language::Go;
            info.package_manager = PackageManager::Go;
            info.test_framework = Some("go-test".to_string());
        }
        if p.join("pom.xml").exists() {
            info.language = Language::Java;
            info.package_manager = PackageManager::Maven;
        }
        if p.join("build.gradle").exists() || p.join("build.gradle.kts").exists() {
            info.language = Language::Java;
            info.package_manager = PackageManager::Gradle;
        }
        if p.join("csproj").exists() {
            info.language = Language::CSharp;
        }
        info
    }

    pub fn to_json(&self) -> serde_json::Value {
        serde_json::json!({
            "root": self.root,
            "language": format!("{:?}", self.language),
            "package_manager": format!("{:?}", self.package_manager),
            "framework": self.framework,
            "is_git_repo": self.is_git_repo,
            "test_framework": self.test_framework,
        })
    }
}
