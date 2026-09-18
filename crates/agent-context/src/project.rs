use serde::{Deserialize, Serialize};

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
