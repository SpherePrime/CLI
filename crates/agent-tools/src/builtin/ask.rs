use anyhow::Result;
use async_trait::async_trait;
use serde_json::{json, Value};

use crate::executor::{ToolExecutionContext, ToolExecutor, ToolOutput, UserQuestion};

pub struct AskUserTool;

#[async_trait]
impl ToolExecutor for AskUserTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let question = args
            .get("question")
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .trim()
            .to_string();
        if question.is_empty() {
            return Ok(ToolOutput::failure("question is required".into()));
        }
        let options = args
            .get("options")
            .and_then(|v| v.as_array())
            .map(|items| {
                items
                    .iter()
                    .filter_map(|item| item.as_str().map(str::to_string))
                    .collect::<Vec<_>>()
            })
            .unwrap_or_default();
        let Some(prompter) = ctx.prompter.as_ref() else {
            return Ok(ToolOutput::failure(
                "no interactive user is available to answer".into(),
            ));
        };
        match prompter.ask(UserQuestion { question, options }).await {
            Some(answer) if !answer.trim().is_empty() => {
                Ok(ToolOutput::success(answer.clone()).summary(format!("answer: {answer}")))
            }
            _ => Ok(ToolOutput::failure("user did not answer".into())),
        }
    }
}

pub fn ask_user_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "ask_user".into(),
        description:
            "Ask the user a clarifying question and wait for the answer. Use options for a multiple-choice prompt, or omit them for free-form input."
                .into(),
        input_schema: json!({
            "type": "object",
            "properties": {
                "question": { "type": "string" },
                "options": {
                    "type": "array",
                    "items": { "type": "string" }
                }
            },
            "required": ["question"],
            "additionalProperties": false,
        }),
        executor: std::sync::Arc::new(AskUserTool),
        permissions: agent_permissions::PermissionScope::Read,
        timeout_secs: 600,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::executor::UserPrompter;
    use agent_config::{PermissionMode, PermissionsConfig};
    use agent_permissions::PermissionEngine;
    use uuid::Uuid;

    struct ScriptedPrompter(Option<String>);

    #[async_trait]
    impl UserPrompter for ScriptedPrompter {
        async fn ask(&self, _question: UserQuestion) -> Option<String> {
            self.0.clone()
        }
    }

    fn context(prompter: Option<ScriptedPrompter>) -> ToolExecutionContext {
        let engine = PermissionEngine::new(PermissionsConfig {
            mode: PermissionMode::Allow,
            ..Default::default()
        });
        let ctx =
            ToolExecutionContext::new(Uuid::new_v4(), std::env::current_dir().unwrap(), engine)
                .with_turn_id(Some("turn".into()));
        match prompter {
            Some(prompter) => ctx.with_prompter(std::sync::Arc::new(prompter)),
            None => ctx,
        }
    }

    #[tokio::test]
    async fn ask_user_returns_answer() {
        let tool = AskUserTool;
        let out = tool
            .execute(
                json!({ "question": "Which file?", "options": ["a", "b"] }),
                &context(Some(ScriptedPrompter(Some("a".into())))),
            )
            .await
            .unwrap();
        assert!(out.ok);
        assert_eq!(out.content, "a");
    }

    #[tokio::test]
    async fn ask_user_without_prompter_fails() {
        let tool = AskUserTool;
        let out = tool
            .execute(json!({ "question": "hi" }), &context(None))
            .await
            .unwrap();
        assert!(!out.ok);
    }

    #[tokio::test]
    async fn ask_user_rejects_empty_question() {
        let tool = AskUserTool;
        let out = tool
            .execute(json!({ "question": "  " }), &context(None))
            .await
            .unwrap();
        assert!(!out.ok);
        assert!(out.error.unwrap_or_default().contains("required"));
    }
}
