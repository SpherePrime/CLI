use std::sync::Arc;

use agent_permissions::{PermissionDecision, PermissionEngine, PermissionScope};
use anyhow::Result;
use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[async_trait]
pub trait ToolExecutor: Send + Sync + 'static {
    async fn execute(
        &self,
        args: serde_json::Value,
        ctx: &ToolExecutionContext,
    ) -> Result<ToolOutput>;
}

#[derive(Debug, Clone)]
pub struct PermissionRequest {
    pub session_id: Uuid,
    pub tool: String,
    pub scope: PermissionScope,
    pub target: String,
    pub reason: String,
}

#[async_trait]
pub trait PermissionApprover: Send + Sync + 'static {
    async fn approve(&self, request: PermissionRequest) -> PermissionDecision;
}

#[derive(Debug, Clone)]
pub struct UserQuestion {
    pub question: String,
    pub options: Vec<String>,
}

#[async_trait]
pub trait UserPrompter: Send + Sync + 'static {
    async fn ask(&self, question: UserQuestion) -> Option<String>;
}

#[derive(Clone)]
pub struct ToolExecutionContext {
    pub session_id: Uuid,
    pub working_dir: std::path::PathBuf,
    pub permission_engine: Arc<std::sync::Mutex<PermissionEngine>>,
    pub approver: Option<Arc<dyn PermissionApprover>>,
    pub prompter: Option<Arc<dyn UserPrompter>>,
    pub cancel: Arc<std::sync::atomic::AtomicBool>,
    pub turn_id: Option<String>,
}

impl ToolExecutionContext {
    pub fn new(
        session_id: Uuid,
        working_dir: std::path::PathBuf,
        engine: PermissionEngine,
    ) -> Self {
        Self {
            session_id,
            working_dir,
            permission_engine: Arc::new(std::sync::Mutex::new(engine)),
            approver: None,
            prompter: None,
            cancel: Arc::new(std::sync::atomic::AtomicBool::new(false)),
            turn_id: None,
        }
    }

    pub fn with_approver(mut self, approver: Arc<dyn PermissionApprover>) -> Self {
        self.approver = Some(approver);
        self
    }

    pub fn with_prompter(mut self, prompter: Arc<dyn UserPrompter>) -> Self {
        self.prompter = Some(prompter);
        self
    }

    pub fn with_permission_engine(
        mut self,
        engine: Arc<std::sync::Mutex<PermissionEngine>>,
    ) -> Self {
        self.permission_engine = engine;
        self
    }

    pub fn with_cancel(mut self, cancel: Arc<std::sync::atomic::AtomicBool>) -> Self {
        self.cancel = cancel;
        self
    }

    pub fn with_turn_id(mut self, turn_id: Option<String>) -> Self {
        self.turn_id = turn_id;
        self
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FileChange {
    pub path: String,
    pub change: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub diff: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub additions: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub deletions: Option<usize>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolArtifact {
    pub path: String,
    #[serde(rename = "type")]
    pub kind: ToolArtifactKind,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ToolArtifactKind {
    Created,
    Modified,
    Deleted,
    Read,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolOutput {
    pub ok: bool,
    pub content: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub summary: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub file_changes: Vec<FileChange>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub artifacts: Vec<ToolArtifact>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub exit_code: Option<i32>,
    #[serde(default)]
    pub truncated: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub duration_ms: Option<u64>,
}

impl ToolOutput {
    pub fn success(content: String) -> Self {
        Self {
            ok: true,
            content,
            error: None,
            summary: None,
            details: None,
            file_changes: Vec::new(),
            artifacts: Vec::new(),
            exit_code: None,
            truncated: false,
            duration_ms: None,
        }
    }

    pub fn failure(error: String) -> Self {
        Self {
            ok: false,
            content: String::new(),
            error: Some(error),
            summary: None,
            details: None,
            file_changes: Vec::new(),
            artifacts: Vec::new(),
            exit_code: None,
            truncated: false,
            duration_ms: None,
        }
    }

    pub fn summary(mut self, summary: impl Into<String>) -> Self {
        self.summary = Some(summary.into());
        self
    }

    pub fn content(mut self, content: impl Into<String>) -> Self {
        self.content = content.into();
        self
    }

    pub fn details(mut self, details: impl Into<String>) -> Self {
        self.details = Some(details.into());
        self
    }

    pub fn file_change(mut self, change: FileChange) -> Self {
        self.file_changes.push(change);
        self
    }

    pub fn file_changes(mut self, changes: impl IntoIterator<Item = FileChange>) -> Self {
        self.file_changes.extend(changes);
        self
    }

    pub fn artifact(mut self, artifact: ToolArtifact) -> Self {
        self.artifacts.push(artifact);
        self
    }

    pub fn artifacts(mut self, artifacts: impl IntoIterator<Item = ToolArtifact>) -> Self {
        self.artifacts.extend(artifacts);
        self
    }

    pub fn exit_code(mut self, code: i32) -> Self {
        self.exit_code = Some(code);
        self
    }

    pub fn truncated(mut self, truncated: bool) -> Self {
        self.truncated = truncated;
        self
    }

    pub fn duration(mut self, duration_ms: u64) -> Self {
        self.duration_ms = Some(duration_ms);
        self
    }
}

pub struct ToolDefinition {
    pub name: String,
    pub description: String,
    pub input_schema: serde_json::Value,
    pub executor: Arc<dyn ToolExecutor>,
    pub permissions: PermissionScope,
    pub timeout_secs: u64,
}

impl Clone for ToolDefinition {
    fn clone(&self) -> Self {
        Self {
            name: self.name.clone(),
            description: self.description.clone(),
            input_schema: self.input_schema.clone(),
            executor: Arc::clone(&self.executor),
            permissions: self.permissions,
            timeout_secs: self.timeout_secs,
        }
    }
}

impl ToolDefinition {
    pub fn to_schema(&self) -> agent_model::ToolSchema {
        agent_model::ToolSchema {
            name: self.name.clone(),
            description: self.description.clone(),
            input_schema: self.input_schema.clone(),
        }
    }

    pub fn schema(&self) -> agent_model::ToolSchema {
        self.to_schema()
    }
}
