use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatMessage {
    pub role: Role,
    pub content: MessageContent,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_calls: Option<Vec<ToolCall>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_call_id: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Role {
    System,
    User,
    Assistant,
    Tool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum MessageContent {
    Text(String),
    Multi { parts: Vec<ContentPart> },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum ContentPart {
    Text {
        text: String,
    },
    ToolCall {
        id: String,
        name: String,
        args: serde_json::Value,
    },
}

impl MessageContent {
    pub fn as_text(&self) -> String {
        match self {
            MessageContent::Text(s) => s.clone(),
            MessageContent::Multi { parts } => parts
                .iter()
                .filter_map(|p| match p {
                    ContentPart::Text { text } => Some(text.as_str()),
                    ContentPart::ToolCall { .. } => None,
                })
                .collect::<Vec<_>>()
                .join(""),
        }
    }

    pub fn role_name(role: &Role) -> &'static str {
        match role {
            Role::System => "system",
            Role::User => "user",
            Role::Assistant => "assistant",
            Role::Tool => "tool",
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolCall {
    pub id: String,
    pub name: String,
    pub args: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelRequest {
    pub model: String,
    pub messages: Vec<ChatMessage>,
    pub tools: Option<Vec<ToolSchema>>,
    pub temperature: Option<f32>,
    pub max_tokens: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolSchema {
    pub name: String,
    pub description: String,
    pub input_schema: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelResponse {
    pub content: MessageContent,
    pub tool_calls: Vec<ToolCall>,
    pub stop_reason: StopReason,
    pub usage: Usage,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum StopReason {
    EndTurn,
    ToolUse,
    MaxTokens,
    Error,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct Usage {
    pub input_tokens: u64,
    pub output_tokens: u64,
}

impl std::fmt::Display for Usage {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "in={} out={}", self.input_tokens, self.output_tokens)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum StreamChunk {
    TextDelta(String),
    ToolCallDelta {
        index: usize,
        id: Option<String>,
        name: Option<String>,
        args_delta: String,
    },
    Usage(Usage),
    Done {
        response: ModelResponse,
    },
    Error(String),
}
