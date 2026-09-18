pub mod error;
pub mod provider;
pub mod providers;
pub mod types;

pub use error::ModelError;
pub use provider::{build_from_model_config, ModelProvider, ProviderConfig};
pub use providers::mock::MockProvider;
pub use types::{
    ChatMessage, ContentPart, MessageContent, ModelRequest, ModelResponse, Role, StopReason,
    StreamChunk, ToolCall, ToolSchema, Usage,
};

#[cfg(test)]
mod tests {
    use crate::provider::build_from_model_config;
    use crate::provider::ModelProvider;
    use crate::providers::mock::MockProvider;
    use crate::types::{ChatMessage, MessageContent, ModelRequest, Role, StopReason};
    use agent_config::{ModelConfig, ProviderKind};

    #[tokio::test]
    async fn mock_provider_echo() {
        let p = MockProvider::new();
        let resp = p
            .chat(&ModelRequest {
                model: "mock".into(),
                messages: vec![ChatMessage {
                    role: Role::User,
                    content: MessageContent::Text("hello".into()),
                    tool_calls: None,
                    tool_call_id: None,
                }],
                tools: None,
                temperature: None,
                max_tokens: None,
            })
            .await
            .unwrap();
        assert!(resp.content.as_text().contains("hello"));
        assert_eq!(resp.stop_reason, StopReason::EndTurn);
    }

    #[tokio::test]
    async fn build_mock_provider() {
        let mc = ModelConfig {
            provider: ProviderKind::Mock,
            model: "mock".into(),
            base_url: None,
            api_key_env: None,
            temperature: None,
            max_tokens: None,
        };
        let p = build_from_model_config(&mc).unwrap();
        let name = p.name().await;
        assert_eq!(name, "mock");
    }
}
