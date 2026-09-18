use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;

use serde::{Deserialize, Serialize};
use uuid::Uuid;

pub const PROTOCOL_VERSION: u32 = 2;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EventMeta {
    pub protocol: u32,
    pub sequence: u64,
    pub turn_id: Uuid,
    pub item_id: String,
    pub ts: i64,
}

#[derive(Debug, Clone)]
pub struct EventClock {
    sequence: Arc<AtomicU64>,
    pub turn_id: Uuid,
}

impl EventClock {
    pub fn new(turn_id: Uuid) -> Self {
        Self::with_base(turn_id, 0)
    }

    pub fn with_base(turn_id: Uuid, base: u64) -> Self {
        Self {
            sequence: Arc::new(AtomicU64::new(base)),
            turn_id,
        }
    }

    pub fn last_sequence(&self) -> u64 {
        self.sequence.load(Ordering::SeqCst)
    }

    pub fn meta(&self, item_id: impl Into<String>) -> EventMeta {
        let sequence = self
            .sequence
            .fetch_add(1, Ordering::SeqCst)
            .saturating_add(1);
        EventMeta {
            protocol: PROTOCOL_VERSION,
            sequence,
            turn_id: self.turn_id,
            item_id: item_id.into(),
            ts: chrono::Utc::now().timestamp_millis(),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PlanStep {
    pub title: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub status: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum EngineEvent {
    Started {
        #[serde(flatten)]
        meta: EventMeta,
        model: String,
    },
    TurnStarted {
        #[serde(flatten)]
        meta: EventMeta,
        model: String,
    },
    TurnCompleted {
        #[serde(flatten)]
        meta: EventMeta,
    },
    TurnCancelled {
        #[serde(flatten)]
        meta: EventMeta,
        reason: Option<String>,
    },
    AssistantMessageStarted {
        #[serde(flatten)]
        meta: EventMeta,
        id: String,
    },
    TextDelta {
        #[serde(flatten)]
        meta: EventMeta,
        text: String,
    },
    AssistantMessageCompleted {
        #[serde(flatten)]
        meta: EventMeta,
    },
    ReasoningStarted {
        #[serde(flatten)]
        meta: EventMeta,
    },
    ReasoningDelta {
        #[serde(flatten)]
        meta: EventMeta,
        text: String,
    },
    ReasoningCompleted {
        #[serde(flatten)]
        meta: EventMeta,
    },
    ToolCallStarted {
        #[serde(flatten)]
        meta: EventMeta,
        id: String,
        name: String,
        args: serde_json::Value,
    },
    ToolCallDelta {
        #[serde(flatten)]
        meta: EventMeta,
        index: usize,
        id: Option<String>,
        name: Option<String>,
        args_delta: String,
    },
    ToolCallCompleted {
        #[serde(flatten)]
        meta: EventMeta,
        id: String,
        name: String,
    },
    ToolResult {
        #[serde(flatten)]
        meta: EventMeta,
        id: String,
        name: String,
        ok: bool,
        content: String,
        #[serde(skip_serializing_if = "Option::is_none")]
        error: Option<String>,
        duration_ms: u128,
        #[serde(skip_serializing_if = "Option::is_none")]
        summary: Option<String>,
        #[serde(skip_serializing_if = "Option::is_none")]
        details: Option<String>,
        #[serde(skip_serializing_if = "Option::is_none")]
        exit_code: Option<i32>,
        #[serde(default)]
        truncated: bool,
        #[serde(default)]
        file_changes: Vec<serde_json::Value>,
    },
    ActivityChanged {
        #[serde(flatten)]
        meta: EventMeta,
        activity: String,
        #[serde(skip_serializing_if = "Option::is_none")]
        kind: Option<String>,
    },
    PermissionRequested {
        #[serde(flatten)]
        meta: EventMeta,
        id: Uuid,
        tool: String,
        scope: String,
        target: String,
        reason: String,
    },
    PermissionResolved {
        #[serde(flatten)]
        meta: EventMeta,
        id: Uuid,
        decision: String,
    },
    PermissionModeChanged {
        #[serde(flatten)]
        meta: EventMeta,
        mode: String,
    },
    QuestionAsked {
        #[serde(flatten)]
        meta: EventMeta,
        id: Uuid,
        question: String,
        #[serde(default)]
        options: Vec<String>,
    },
    QuestionAnswered {
        #[serde(flatten)]
        meta: EventMeta,
        id: Uuid,
        #[serde(skip_serializing_if = "Option::is_none")]
        answer: Option<String>,
    },
    Usage {
        #[serde(flatten)]
        meta: EventMeta,
        input_tokens: u64,
        output_tokens: u64,
    },
    Finished {
        #[serde(flatten)]
        meta: EventMeta,
        stop_reason: String,
        input_tokens: u64,
        output_tokens: u64,
        iterations: usize,
    },
    Error {
        #[serde(flatten)]
        meta: EventMeta,
        message: String,
    },
    SessionTitleChanged {
        #[serde(flatten)]
        meta: EventMeta,
        title: String,
    },
    PlanUpdated {
        #[serde(flatten)]
        meta: EventMeta,
        steps: Vec<PlanStep>,
    },
}

pub fn error_event(
    clock: &EventClock,
    item_id: impl Into<String>,
    message: impl Into<String>,
) -> EngineEvent {
    EngineEvent::Error {
        meta: clock.meta(item_id),
        message: message.into(),
    }
}
