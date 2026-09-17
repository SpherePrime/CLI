use agent_model::{ChatMessage, MessageContent, Role, ToolCall};
use anyhow::Result;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

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

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ContextManager {
    pub session_id: Uuid,
    pub messages: Vec<ChatMessage>,
    pub compacted_summaries: Vec<String>,
    pub project: Option<ProjectInfo>,
    pub open_files: Vec<String>,
    pub recent_edits: Vec<String>,
    pub user_instructions: Option<String>,
    pub max_messages: usize,
}

impl ContextManager {
    pub fn new(session_id: Uuid, max_messages: usize) -> Self {
        Self {
            session_id,
            messages: Vec::new(),
            compacted_summaries: Vec::new(),
            project: None,
            open_files: Vec::new(),
            recent_edits: Vec::new(),
            user_instructions: None,
            max_messages,
        }
    }

    pub fn detect_project(&mut self, root: &str) {
        self.project = Some(ProjectInfo::detect(root));
    }

    pub fn set_user_instructions(&mut self, instructions: &str) {
        self.user_instructions = Some(instructions.to_string());
    }

    pub fn push_user(&mut self, text: &str) {
        self.messages.push(ChatMessage {
            role: Role::User,
            content: MessageContent::Text(text.to_string()),
            tool_calls: None,
            tool_call_id: None,
        });
        self.maybe_compact();
    }

    pub fn push_assistant(&mut self, text: &str, tool_calls: Vec<ToolCall>) {
        self.messages.push(ChatMessage {
            role: Role::Assistant,
            content: MessageContent::Text(text.to_string()),
            tool_calls: (!tool_calls.is_empty()).then_some(tool_calls),
            tool_call_id: None,
        });
        self.maybe_compact();
    }

    pub fn push_tool_result(&mut self, tool_call_id: &str, text: &str) {
        self.messages.push(ChatMessage {
            role: Role::Tool,
            content: MessageContent::Text(text.to_string()),
            tool_calls: None,
            tool_call_id: Some(tool_call_id.to_string()),
        });
        self.maybe_compact();
    }

    fn maybe_compact(&mut self) {
        if self.max_messages == 0 || self.messages.len() <= self.max_messages {
            return;
        }
        let keep_head = 2usize;
        let keep_tail = 4usize;
        let start = keep_head.min(self.messages.len());
        let end = self.messages.len().saturating_sub(keep_tail).max(start);
        if start >= end {
            return;
        }
        let to_compress: Vec<ChatMessage> = self.messages[start..end].to_vec();
        let summary = to_compress
            .iter()
            .map(|m| {
                let role: &str = match m.role {
                    Role::System => "system",
                    Role::User => "user",
                    Role::Assistant => "assistant",
                    Role::Tool => "tool",
                };
                let preview: String = m.content.as_text().chars().take(120).collect();
                format!("[{role}]: {preview}")
            })
            .collect::<Vec<_>>()
            .join("\n");
        let keep_head_msgs: Vec<ChatMessage> = self.messages[..start].to_vec();
        let keep_tail_msgs: Vec<ChatMessage> = self.messages[end..].to_vec();
        let compacted = ChatMessage {
            role: Role::System,
            content: MessageContent::Text(format!(
                "<compaction>\n{}\n</compaction>",
                summary
            )),
            tool_calls: None,
            tool_call_id: None,
        };
        self.compacted_summaries.push(summary);
        self.messages = keep_head_msgs;
        self.messages.push(compacted);
        self.messages.extend(keep_tail_msgs);
    }

    pub fn render_for_model(&self) -> Vec<ChatMessage> {
        let mut out: Vec<ChatMessage> = Vec::new();
        if let Some(ins) = &self.user_instructions {
            out.push(ChatMessage {
                role: Role::System,
                content: MessageContent::Text(ins.clone()),
                tool_calls: None,
                tool_call_id: None,
            });
        }
        if let Some(proj) = &self.project {
            out.push(ChatMessage {
                role: Role::System,
                content: MessageContent::Text(
                    format!("Project context: {}", serde_json::to_string_pretty(&proj.to_json()).unwrap_or_default()),
                ),
                tool_calls: None,
                tool_call_id: None,
            });
        }
        out.extend(self.messages.clone());
        out
    }

    pub fn message_count(&self) -> usize {
        self.messages.len()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn project_detection_rust() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join("Cargo.toml"), "[package]\nname=test\n").unwrap();
        let info = ProjectInfo::detect(tmp.path().to_str().unwrap());
        assert_eq!(info.language, Language::Rust);
        assert_eq!(info.package_manager, PackageManager::Cargo);
    }

    #[test]
    fn compact_on_overflow() {
        let mut cm = ContextManager::new(Uuid::new_v4(), 6);
        for i in 0..10 {
            cm.push_user(&format!("msg{i}"));
        }
        assert!(cm.messages.len() <= 7);
    }
}
