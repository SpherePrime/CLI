use std::sync::Arc;

use agent_model::{build_from_model_config, ChatMessage, MessageContent, ModelRequest, Role};

use super::state::AppState;

const MAX_TITLE_CHARS: usize = 60;

pub async fn generate_title(state: Arc<AppState>, text: &str) -> Option<String> {
    let config = state.config.read().unwrap().clone()?;
    let model = config.model?;
    let provider = build_from_model_config(&model).ok()?;

    let prompt = format!(
        "Write a short title for a coding session founded on this request.\n\
         Reply with only the title, at most 8 words, no quotes.\n\n\
         Request: {text}"
    );
    let request = ModelRequest {
        model: model.model.clone(),
        messages: vec![ChatMessage {
            role: Role::User,
            content: MessageContent::Text(prompt),
            tool_calls: None,
            tool_call_id: None,
        }],
        tools: None,
        temperature: Some(0.3),
        max_tokens: Some(32),
    };

    let response = provider.chat(&request).await.ok()?;
    let raw = response.content.as_text();
    let cleaned = raw
        .trim()
        .trim_matches('"')
        .trim()
        .lines()
        .next()
        .unwrap_or("")
        .trim()
        .to_string();
    if cleaned.is_empty() {
        return None;
    }
    Some(truncate(&cleaned).to_string())
}

fn truncate(value: &str) -> &str {
    if value.chars().count() <= MAX_TITLE_CHARS {
        return value;
    }
    let mut end = 0;
    for (index, (byte, _)) in value.char_indices().enumerate() {
        if index >= MAX_TITLE_CHARS {
            break;
        }
        end = byte;
    }
    &value[..end]
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn truncate_keeps_short_titles() {
        assert_eq!(truncate("hello"), "hello");
    }

    #[test]
    fn truncate_limits_long_titles() {
        let long = "a".repeat(200);
        let cut = truncate(&long);
        assert!(cut.chars().count() <= MAX_TITLE_CHARS);
    }

    #[test]
    fn truncate_preserves_multibyte_boundary() {
        let value = "тестирование-сессии-агента".repeat(10);
        let cut = truncate(&value);
        assert!(cut.chars().count() <= MAX_TITLE_CHARS);
        assert!(value.starts_with(cut));
    }
}
