use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum EngineEvent {
    Started {
        session_id: Uuid,
        model: String,
    },
    TextDelta {
        text: String,
    },
    ReasoningDelta {
        text: String,
    },
    ToolCall {
        id: String,
        name: String,
        args: serde_json::Value,
    },
    ToolResult {
        id: String,
        name: String,
        ok: bool,
        content: String,
        #[serde(skip_serializing_if = "Option::is_none")]
        error: Option<String>,
        ms: u128,
    },
    PermissionRequested {
        id: Uuid,
        tool: String,
        scope: String,
        target: String,
        reason: String,
    },
    PermissionResolved {
        id: Uuid,
        decision: String,
    },
    Usage {
        input_tokens: u64,
        output_tokens: u64,
    },
    Finished {
        stop_reason: String,
        input_tokens: u64,
        output_tokens: u64,
        iterations: usize,
    },
    Error {
        message: String,
    },
}

impl EngineEvent {
    pub fn error(message: impl Into<String>) -> Self {
        Self::Error {
            message: message.into(),
        }
    }
}
