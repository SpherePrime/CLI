use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

use agent_config::{AgentConfig, AgentMode, PermissionMode};
use agent_context::ContextManager;
use agent_model::{
    build_from_model_config, ChatMessage, ModelProvider, ModelRequest, ModelResponse, Role,
    StopReason, StreamChunk, Usage,
};
use agent_permissions::{PermissionEngine, PermissionScope};
use agent_storage::{SessionRecord, Storage};
use agent_tools::{builtin, ToolExecutionContext, ToolRegistry};
use anyhow::{Context, Result};
use chrono::Utc;
use futures::StreamExt;
use serde_json::{json, Value};
use tokio::sync::mpsc;
use uuid::Uuid;

use crate::engine::approval::EngineApprover;
use crate::engine::event::EngineEvent;
use crate::engine::prompt::build_system_prompt;

pub struct AgentEngine {
    pub session_id: Uuid,
    pub config: AgentConfig,
    pub working_dir: PathBuf,
    pub provider: Box<dyn ModelProvider>,
    pub tools: ToolRegistry,
    pub context: ContextManager,
    pub storage: Option<Storage>,
    pub approver: Arc<EngineApprover>,
    mode: AgentMode,
    session_ready: bool,
    cancel: Arc<AtomicBool>,
}

impl AgentEngine {
    pub fn new(
        config: AgentConfig,
        working_dir: PathBuf,
        event_tx: mpsc::Sender<EngineEvent>,
    ) -> Result<Self> {
        let model_cfg = config
            .model
            .clone()
            .context("no model configured; set [model] in the config")?;
        let provider = build_from_model_config(&model_cfg)?;
        Ok(Self::with_provider(config, working_dir, event_tx, provider))
    }

    pub fn with_provider(
        config: AgentConfig,
        working_dir: PathBuf,
        event_tx: mpsc::Sender<EngineEvent>,
        provider: Box<dyn ModelProvider>,
    ) -> Self {
        let mode = config.mode.unwrap_or_default();
        let auto_allow = matches!(mode, AgentMode::Auto | AgentMode::Debug | AgentMode::Ci);
        let approver = EngineApprover::new(event_tx, auto_allow);
        let session_id = Uuid::new_v4();
        let tools = builtin::register_builtin(ToolRegistry::new());
        let tool_names: Vec<String> = tools.tools.iter().map(|t| t.name.clone()).collect();
        let mut context = ContextManager::new(session_id, config.limits.max_context_messages);
        context.detect_project(&working_dir.to_string_lossy());
        context.set_user_instructions(&build_system_prompt(
            &working_dir.to_string_lossy(),
            &tool_names,
        ));
        Self {
            session_id,
            config,
            working_dir,
            provider,
            tools,
            context,
            storage: Storage::global().ok(),
            approver,
            mode,
            session_ready: false,
            cancel: Arc::new(AtomicBool::new(false)),
        }
    }

    pub fn with_storage(mut self, storage: Storage) -> Self {
        self.storage = Some(storage);
        self
    }

    pub fn with_session(mut self, session_id: Uuid) -> Self {
        self.session_id = session_id;
        self.context.session_id = session_id;
        self
    }

    pub fn cancel_handle(&self) -> Arc<AtomicBool> {
        Arc::clone(&self.cancel)
    }

    pub fn cancel(&self) {
        self.cancel.store(true, Ordering::SeqCst);
    }

    pub fn model_name(&self) -> String {
        self.config
            .model
            .as_ref()
            .map(|m| m.model.clone())
            .unwrap_or_default()
    }

    pub fn schemas(&self) -> Vec<agent_model::ToolSchema> {
        self.tools.schemas()
    }

    pub async fn run(&mut self, user_text: &str, tx: &mpsc::Sender<EngineEvent>) -> Result<()> {
        self.ensure_session()?;
        self.persist_message(&ChatMessage {
            role: Role::User,
            content: agent_model::MessageContent::Text(user_text.to_string()),
            tool_calls: None,
            tool_call_id: None,
        })?;
        self.context.push_user(user_text);
        let _ = tx
            .send(EngineEvent::Started {
                session_id: self.session_id,
                model: self.model_name(),
            })
            .await;

        let started = std::time::Instant::now();
        let max_iterations = self.config.limits.max_tool_calls.max(1);
        let mut iterations = 0usize;
        let mut usage = Usage::default();

        loop {
            if self.cancel.load(Ordering::SeqCst) {
                let _ = tx.send(EngineEvent::error("generation cancelled")).await;
                break;
            }
            if started.elapsed().as_secs() > self.config.limits.max_execution_time_secs {
                let _ = tx
                    .send(EngineEvent::error("execution time limit reached"))
                    .await;
                break;
            }
            if iterations >= max_iterations {
                let _ = tx.send(EngineEvent::error("tool call limit reached")).await;
                break;
            }

            let request = self.build_request();
            let mut stream = match self.provider.chat_stream(&request).await {
                Ok(stream) => stream,
                Err(error) => {
                    let _ = tx.send(EngineEvent::error(error.to_string())).await;
                    self.touch_session();
                    return Err(error);
                }
            };

            let mut done: Option<ModelResponse> = None;
            while let Some(item) = stream.next().await {
                match item {
                    Ok(StreamChunk::TextDelta(text)) => {
                        let _ = tx.send(EngineEvent::TextDelta { text }).await;
                    }
                    Ok(StreamChunk::Usage(partial)) => {
                        let _ = tx
                            .send(EngineEvent::Usage {
                                input_tokens: partial.input_tokens,
                                output_tokens: partial.output_tokens,
                            })
                            .await;
                    }
                    Ok(StreamChunk::Done { response }) => done = Some(response),
                    Ok(StreamChunk::Error(message)) => {
                        let _ = tx.send(EngineEvent::error(message)).await;
                    }
                    Ok(StreamChunk::ToolCallDelta { .. }) => {}
                    Err(error) => {
                        let _ = tx.send(EngineEvent::error(error.to_string())).await;
                    }
                }
            }

            let response = match done {
                Some(response) => response,
                None => {
                    let message = "model stream ended without a response";
                    let _ = tx.send(EngineEvent::error(message)).await;
                    self.touch_session();
                    anyhow::bail!(message);
                }
            };

            usage.input_tokens += response.usage.input_tokens;
            usage.output_tokens += response.usage.output_tokens;
            let assistant = ChatMessage {
                role: Role::Assistant,
                content: response.content.clone(),
                tool_calls: (!response.tool_calls.is_empty()).then(|| response.tool_calls.clone()),
                tool_call_id: None,
            };
            self.persist_message(&assistant)?;
            self.context
                .push_assistant(&response.content.as_text(), response.tool_calls.clone());
            let _ = tx
                .send(EngineEvent::Usage {
                    input_tokens: usage.input_tokens,
                    output_tokens: usage.output_tokens,
                })
                .await;

            if response.tool_calls.is_empty() {
                let _ = tx
                    .send(EngineEvent::Finished {
                        stop_reason: reason_name(response.stop_reason).to_string(),
                        input_tokens: usage.input_tokens,
                        output_tokens: usage.output_tokens,
                        iterations,
                    })
                    .await;
                break;
            }

            for call in &response.tool_calls {
                let _ = tx
                    .send(EngineEvent::ToolCall {
                        id: call.id.clone(),
                        name: call.name.clone(),
                        args: call.args.clone(),
                    })
                    .await;
            }

            let mut planned = Vec::new();
            for call in &response.tool_calls {
                let scope = self
                    .tools
                    .find(&call.name)
                    .map(|tool| tool.permissions)
                    .unwrap_or(PermissionScope::Read);
                if !self.mode_allows(scope) {
                    let payload = json!({
                        "ok": false,
                        "error": format!("tool {} is not permitted in {:?} mode", call.name, self.mode),
                    });
                    self.context
                        .push_tool_result(&call.id, &payload.to_string());
                    self.persist_tool_result(&call.id, &payload.to_string())?;
                    let _ = tx
                        .send(EngineEvent::ToolResult {
                            id: call.id.clone(),
                            name: call.name.clone(),
                            ok: false,
                            content: String::new(),
                            error: Some(payload["error"].as_str().unwrap_or_default().to_string()),
                            ms: 0,
                        })
                        .await;
                    continue;
                }
                planned.push(call.clone());
            }

            if !planned.is_empty() {
                let ctx = self.tool_context();
                let calls: Vec<(String, Value)> = planned
                    .iter()
                    .map(|call| (call.name.clone(), call.args.clone()))
                    .collect();
                let batch_started = std::time::Instant::now();
                let results = self
                    .tools
                    .call_parallel(&calls, &ctx, self.config.limits.max_parallel_tools)
                    .await?;
                let elapsed = batch_started.elapsed().as_millis();
                for (call, (_, output)) in planned.iter().zip(results) {
                    let payload = if output.ok {
                        json!({ "ok": true, "content": output.content })
                    } else {
                        json!({
                            "ok": false,
                            "content": output.content,
                            "error": output.error.clone().unwrap_or_else(|| "tool failed".into()),
                        })
                    };
                    let text = payload.to_string();
                    self.context.push_tool_result(&call.id, &text);
                    self.persist_tool_result(&call.id, &text)?;
                    let _ = tx
                        .send(EngineEvent::ToolResult {
                            id: call.id.clone(),
                            name: call.name.clone(),
                            ok: output.ok,
                            content: output.content,
                            error: output.error,
                            ms: elapsed,
                        })
                        .await;
                    iterations += 1;
                }
            }

            if iterations >= max_iterations {
                let _ = tx.send(EngineEvent::error("tool call limit reached")).await;
                break;
            }
        }

        self.touch_session();
        Ok(())
    }

    fn build_request(&self) -> ModelRequest {
        let model = self.config.model.as_ref();
        ModelRequest {
            model: model.map(|m| m.model.clone()).unwrap_or_default(),
            messages: self.context.render_for_model(),
            tools: Some(self.tools.schemas()),
            temperature: model.and_then(|m| m.temperature),
            max_tokens: model.and_then(|m| m.max_tokens),
        }
    }

    fn mode_allows(&self, scope: PermissionScope) -> bool {
        match self.mode {
            AgentMode::Readonly | AgentMode::Plan => scope == PermissionScope::Read,
            _ => true,
        }
    }

    fn tool_context(&self) -> ToolExecutionContext {
        let mut permissions = self.config.permissions.clone();
        permissions.mode = match self.mode {
            AgentMode::Auto | AgentMode::Debug | AgentMode::Ci => PermissionMode::Allow,
            AgentMode::Plan | AgentMode::Readonly => PermissionMode::Deny,
            AgentMode::Interactive => permissions.mode,
        };
        let engine = PermissionEngine::new(permissions);
        ToolExecutionContext::new(self.session_id, self.working_dir.clone(), engine)
            .with_approver(self.approver.clone())
    }

    fn ensure_session(&mut self) -> Result<()> {
        if self.session_ready {
            return Ok(());
        }
        if let Some(storage) = &self.storage {
            if !storage.session_path(&self.session_id).exists() {
                let record = SessionRecord {
                    id: self.session_id,
                    created_at: Utc::now(),
                    updated_at: Utc::now(),
                    project_path: Some(self.working_dir.clone()),
                    model: Some(self.model_name()),
                    metadata: json!({ "title": Value::Null, "mode": format!("{:?}", self.mode) }),
                };
                storage.write_session(&record, &[])?;
            }
        }
        self.session_ready = true;
        Ok(())
    }

    fn persist_message(&self, message: &ChatMessage) -> Result<()> {
        if let Some(storage) = &self.storage {
            let value = serde_json::to_value(message)?;
            storage.append_session_message(&self.session_id, &value)?;
        }
        Ok(())
    }

    fn persist_tool_result(&self, tool_call_id: &str, content: &str) -> Result<()> {
        self.persist_message(&ChatMessage {
            role: Role::Tool,
            content: agent_model::MessageContent::Text(content.to_string()),
            tool_calls: None,
            tool_call_id: Some(tool_call_id.to_string()),
        })
    }

    fn touch_session(&self) {
        let Some(storage) = &self.storage else {
            return;
        };
        if let Ok(mut record) = storage.read_session(&self.session_id) {
            record.touch();
            let _ = storage.replace_session_meta(&record);
        }
    }
}

fn reason_name(reason: StopReason) -> &'static str {
    match reason {
        StopReason::EndTurn => "end_turn",
        StopReason::ToolUse => "tool_use",
        StopReason::MaxTokens => "max_tokens",
        StopReason::Error => "error",
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use agent_config::{ModelConfig, ProviderKind};
    use agent_model::{MessageContent, ToolCall};
    use async_trait::async_trait;
    use std::sync::atomic::{AtomicUsize, Ordering};

    struct ScriptedProvider {
        calls: AtomicUsize,
    }

    #[async_trait]
    impl ModelProvider for ScriptedProvider {
        async fn name(&self) -> &'static str {
            "scripted"
        }

        async fn list_models(&self) -> Result<Vec<String>> {
            Ok(vec!["scripted".into()])
        }

        async fn chat(&self, _request: &ModelRequest) -> Result<ModelResponse> {
            anyhow::bail!("chat is not used in this test")
        }

        async fn chat_stream(
            &self,
            _request: &ModelRequest,
        ) -> Result<
            futures::stream::BoxStream<
                'static,
                std::result::Result<StreamChunk, agent_model::ModelError>,
            >,
        > {
            use futures::stream::StreamExt;
            let call = self.calls.fetch_add(1, Ordering::SeqCst);
            let response = if call == 0 {
                ModelResponse {
                    content: MessageContent::Text(String::new()),
                    tool_calls: vec![ToolCall {
                        id: "call_1".into(),
                        name: "read_file".into(),
                        args: serde_json::json!({ "path": "note.txt" }),
                    }],
                    stop_reason: StopReason::ToolUse,
                    usage: Usage {
                        input_tokens: 10,
                        output_tokens: 5,
                    },
                }
            } else {
                ModelResponse {
                    content: MessageContent::Text("done".into()),
                    tool_calls: vec![],
                    stop_reason: StopReason::EndTurn,
                    usage: Usage {
                        input_tokens: 20,
                        output_tokens: 7,
                    },
                }
            };
            Ok(futures::stream::iter(vec![Ok(StreamChunk::Done { response })]).boxed())
        }
    }

    fn test_config() -> AgentConfig {
        AgentConfig {
            mode: Some(AgentMode::Auto),
            model: Some(ModelConfig {
                provider: ProviderKind::Mock,
                model: "scripted".into(),
                base_url: None,
                api_key_env: None,
                temperature: None,
                max_tokens: None,
            }),
            ..Default::default()
        }
    }

    #[tokio::test]
    async fn engine_executes_tool_loop_and_persists() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join("note.txt"), "hello world").unwrap();
        let storage = Storage::new(tmp.path().join(".agent"));
        let (tx, mut rx) = mpsc::channel(64);
        let mut engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx.clone(),
            Box::new(ScriptedProvider {
                calls: AtomicUsize::new(0),
            }),
        )
        .with_storage(storage.clone());
        let session_id = engine.session_id;

        engine.run("please read note.txt", &tx).await.unwrap();

        let mut saw_tool_result = false;
        while let Ok(event) = rx.try_recv() {
            if matches!(event, EngineEvent::ToolResult { ok: true, .. }) {
                saw_tool_result = true;
            }
        }
        assert!(saw_tool_result, "expected a successful tool result event");

        let messages = agent_sessions::SessionManager::resume(&storage, session_id).unwrap();
        let roles: Vec<String> = messages
            .iter()
            .map(|m| {
                m.get("role")
                    .and_then(|r| r.as_str())
                    .unwrap_or("?")
                    .to_string()
            })
            .collect();
        assert_eq!(roles[0], "user");
        assert!(roles.contains(&"assistant".to_string()));
        assert!(roles.contains(&"tool".to_string()));
        assert_eq!(roles.last().map(|s| s.as_str()), Some("assistant"));
    }

    #[tokio::test]
    async fn path_traversal_is_rejected() {
        let tmp = tempfile::tempdir().unwrap();
        let storage = Storage::new(tmp.path().join(".agent"));
        let (tx, _rx) = mpsc::channel(8);
        let engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx,
            Box::new(ScriptedProvider {
                calls: AtomicUsize::new(0),
            }),
        )
        .with_storage(storage);
        let ctx = engine.tool_context();
        let output = engine
            .tools
            .call(
                "read_file",
                serde_json::json!({ "path": "../secret.txt" }),
                &ctx,
            )
            .await
            .unwrap();
        assert!(!output.ok);
    }
}
