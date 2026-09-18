use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

use agent_config::{AgentConfig, AgentMode, PermissionMode};
use agent_context::project::{canonical_project_root, git_remote_origin, project_id};
use agent_context::ContextManager;
use agent_model::{
    build_from_model_config, ChatMessage, ModelProvider, ModelRequest, ModelResponse, Role,
    StopReason, StreamChunk, Usage,
};
use agent_permissions::{PermissionEngine, PermissionScope};
use agent_plugins::native::NativePlugin;
use agent_skills::SkillRegistry;
use agent_storage::{ProjectRef, SessionRecord, Storage};
use agent_tools::executor::{
    FileChange, ToolDefinition, ToolExecutionContext, ToolExecutor, ToolOutput,
};
use agent_tools::{builtin, ToolRegistry};
use anyhow::{Context, Result};
use async_trait::async_trait;
use chrono::Utc;
use futures::StreamExt;
use serde_json::Value;
use std::sync::Mutex;
use tokio::sync::mpsc;
use uuid::Uuid;

use crate::engine::approval::EngineApprover;
use crate::engine::event::{error_event, EngineEvent, EventClock, PlanStep};
use crate::engine::mcp;
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
    permissions: Arc<Mutex<PermissionEngine>>,
    clock: EventClock,
    mode: AgentMode,
    session_ready: bool,
    cancel: Arc<AtomicBool>,
    mcp: Option<Arc<tokio::sync::Mutex<agent_mcp::McpClient>>>,
    mcp_loaded: bool,
    plugins: Vec<Arc<NativePlugin>>,
    plugins_loaded: bool,
    current_turn: Option<String>,
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
        let clock = EventClock::new(Uuid::new_v4());
        let approver = EngineApprover::new(event_tx, clock.clone(), auto_allow);
        let session_id = Uuid::new_v4();
        let tools = builtin::register_builtin(ToolRegistry::new());
        let tool_names: Vec<String> = tools.tools.iter().map(|t| t.name.clone()).collect();
        let mut context = ContextManager::new(session_id, config.limits.max_context_messages);
        context.detect_project(&working_dir.to_string_lossy());
        let skills = Self::load_skills(&config, &working_dir);
        let mut system = build_system_prompt(&working_dir.to_string_lossy(), &tool_names);
        let skill_context = skills.context_block();
        if !skill_context.is_empty() {
            system = format!("{system}\n\n{skill_context}");
        }
        context.set_user_instructions(&system);
        let mcp = mcp::build_client(&config.mcp);
        let mut permissions = config.permissions.clone();
        permissions.mode = match mode {
            AgentMode::Auto | AgentMode::Debug | AgentMode::Ci => PermissionMode::Allow,
            AgentMode::Plan | AgentMode::Readonly => PermissionMode::Deny,
            AgentMode::Interactive => permissions.mode,
        };
        let permission_engine = Arc::new(Mutex::new(PermissionEngine::new(permissions)));
        Self {
            session_id,
            config,
            working_dir,
            provider,
            tools,
            context,
            storage: Storage::global().ok(),
            approver,
            permissions: permission_engine,
            clock,
            mode,
            session_ready: false,
            cancel: Arc::new(AtomicBool::new(false)),
            mcp,
            mcp_loaded: false,
            plugins: Vec::new(),
            plugins_loaded: false,
            current_turn: None,
        }
    }

    fn load_skills(config: &AgentConfig, working_dir: &Path) -> SkillRegistry {
        let mut skills =
            SkillRegistry::new().with_project(working_dir.join(".agent").join("skills"));
        if let Ok(storage) = Storage::global() {
            skills = skills.with_global(storage.root().join("skills"));
        }
        let _ = skills.discover_sync();
        for name in &config.skills_disabled {
            let _ = skills.disable(name);
        }
        skills
    }

    pub fn with_storage(mut self, storage: Storage) -> Self {
        self.storage = Some(storage);
        self
    }

    pub fn with_session(mut self, session_id: Uuid) -> Self {
        self.session_id = session_id;
        self.context.session_id = session_id;
        if let Some(storage) = &self.storage {
            if let Ok(record) = storage.read_session(&session_id) {
                if self.mode == AgentMode::Interactive {
                    if let Some(mode_name) =
                        record.metadata.get("perms_mode").and_then(|v| v.as_str())
                    {
                        if let Ok(mode) = serde_json::from_str::<PermissionMode>(mode_name) {
                            self.set_permission_mode(mode);
                        }
                    } else {
                        let project_path =
                            record.project_path.as_deref().or(Some(&self.working_dir));
                        if let Some(dir) = project_path.filter(|p| p.exists()) {
                            let mode_file = dir.join(".agent").join("permission-mode.json");
                            if let Ok(raw) = std::fs::read_to_string(&mode_file) {
                                if let Ok(value) = serde_json::from_str::<Value>(&raw) {
                                    if let Some(name) = value.get("mode").and_then(|v| v.as_str()) {
                                        if let Ok(mode) =
                                            serde_json::from_str::<PermissionMode>(name)
                                        {
                                            self.set_permission_mode(mode);
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
                if let Some(project_path) = record.project_path.as_deref() {
                    if project_path.exists() {
                        self.working_dir = project_path.to_path_buf();
                    }
                }
                let messages =
                    agent_sessions::SessionManager::resume(storage, session_id).unwrap_or_default();
                let history: Vec<ChatMessage> = messages
                    .iter()
                    .filter_map(|value| serde_json::from_value(value.clone()).ok())
                    .collect();
                self.context.push_history(history);
                let base = self
                    .clock
                    .last_sequence()
                    .max(event_log_max_sequence(storage, &session_id));
                self.clock = EventClock::with_base(self.clock.turn_id, base);
            }
        }
        self.rebuild_prompt();
        self
    }

    fn rebuild_prompt(&mut self) {
        let tool_names: Vec<String> = self.tools.tools.iter().map(|t| t.name.clone()).collect();
        let mut system = build_system_prompt(&self.working_dir.to_string_lossy(), &tool_names);
        let skills = Self::load_skills(&self.config, &self.working_dir);
        let skill_context = skills.context_block();
        if !skill_context.is_empty() {
            system = format!("{system}\n\n{skill_context}");
        }
        self.context
            .detect_project(&self.working_dir.to_string_lossy());
        self.context.set_user_instructions(&system);
    }

    pub fn cancel_handle(&self) -> Arc<AtomicBool> {
        Arc::clone(&self.cancel)
    }

    pub fn cancel(&self) {
        self.cancel.store(true, Ordering::SeqCst);
    }

    pub fn clock(&self) -> &EventClock {
        &self.clock
    }

    pub fn set_permission_mode(&self, mode: PermissionMode) {
        self.permissions.lock().unwrap().set_mode(mode);
    }

    pub fn permission_mode(&self) -> PermissionMode {
        self.permissions.lock().unwrap().mode()
    }

    pub fn permission_engine(&self) -> Arc<Mutex<PermissionEngine>> {
        Arc::clone(&self.permissions)
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
        self.ensure_mcp_tools().await;
        self.ensure_plugins().await;
        self.persist_message(&ChatMessage {
            role: Role::User,
            content: agent_model::MessageContent::Text(user_text.to_string()),
            tool_calls: None,
            tool_call_id: None,
        })?;
        self.context.push_user(user_text);

        let clock = self.clock.clone();
        let turn_item = clock.turn_id.to_string();
        self.current_turn = Some(turn_item.clone());
        self.emit(
            tx,
            EngineEvent::Started {
                meta: clock.meta(&turn_item),
                model: self.model_name(),
            },
        )
        .await;
        self.emit(
            tx,
            EngineEvent::TurnStarted {
                meta: clock.meta(&turn_item),
                model: self.model_name(),
            },
        )
        .await;

        let started = std::time::Instant::now();
        let max_iterations = self.config.limits.max_tool_calls.max(1);
        let mut iterations = 0usize;
        let mut usage = Usage::default();

        let result = self
            .run_loop(
                tx,
                &turn_item,
                &mut iterations,
                &mut usage,
                max_iterations,
                started,
            )
            .await;

        self.current_turn = None;

        if self.cancel.load(Ordering::SeqCst) {
            self.emit(
                tx,
                EngineEvent::TurnCancelled {
                    meta: clock.meta(&turn_item),
                    reason: Some("cancelled".into()),
                },
            )
            .await;
        } else {
            self.emit(
                tx,
                EngineEvent::TurnCompleted {
                    meta: clock.meta(&turn_item),
                },
            )
            .await;
        }

        if let Err(error) = &result {
            self.emit(tx, error_event(&clock, &turn_item, error.to_string()))
                .await;
        }

        self.touch_session();
        result
    }

    #[allow(clippy::too_many_arguments)]
    async fn run_loop(
        &mut self,
        tx: &mpsc::Sender<EngineEvent>,
        turn_item: &str,
        iterations: &mut usize,
        usage: &mut Usage,
        max_iterations: usize,
        started: std::time::Instant,
    ) -> Result<()> {
        let clock = self.clock.clone();
        loop {
            if self.cancel.load(Ordering::SeqCst) {
                break;
            }
            if started.elapsed().as_secs() > self.config.limits.max_execution_time_secs {
                self.emit(
                    tx,
                    error_event(&clock, turn_item, "execution time limit reached"),
                )
                .await;
                break;
            }
            if *iterations >= max_iterations {
                self.emit(
                    tx,
                    error_event(&clock, turn_item, "tool call limit reached"),
                )
                .await;
                break;
            }

            let request = self.build_request();
            let mut stream = match self.provider.chat_stream(&request).await {
                Ok(stream) => stream,
                Err(error) => {
                    return Err(error);
                }
            };

            let item_id = format!("assistant_{}_{}", clock.turn_id, iterations);
            self.emit(
                tx,
                EngineEvent::AssistantMessageStarted {
                    meta: clock.meta(&item_id),
                    id: item_id.clone(),
                },
            )
            .await;
            self.emit(
                tx,
                EngineEvent::ActivityChanged {
                    meta: clock.meta(format!("activity_{item_id}")),
                    activity: "Thinking…".into(),
                    kind: Some("thinking".into()),
                },
            )
            .await;
            let mut reasoning_open = false;
            let mut streamed_text = false;
            let mut done: Option<ModelResponse> = None;
            while let Some(item) = stream.next().await {
                match item {
                    Ok(StreamChunk::TextDelta(text)) => {
                        streamed_text = true;
                        self.emit(
                            tx,
                            EngineEvent::TextDelta {
                                meta: clock.meta(&item_id),
                                text,
                            },
                        )
                        .await;
                    }
                    Ok(StreamChunk::ReasoningDelta(text)) => {
                        if !reasoning_open {
                            self.emit(
                                tx,
                                EngineEvent::ReasoningStarted {
                                    meta: clock.meta(&item_id),
                                },
                            )
                            .await;
                            reasoning_open = true;
                        }
                        self.emit(
                            tx,
                            EngineEvent::ReasoningDelta {
                                meta: clock.meta(&item_id),
                                text,
                            },
                        )
                        .await;
                    }
                    Ok(StreamChunk::Usage(partial)) => {
                        self.emit(
                            tx,
                            EngineEvent::Usage {
                                meta: clock.meta(&item_id),
                                input_tokens: partial.input_tokens,
                                output_tokens: partial.output_tokens,
                            },
                        )
                        .await;
                    }
                    Ok(StreamChunk::Done { response }) => done = Some(response),
                    Ok(StreamChunk::Error(message)) => {
                        self.emit(tx, error_event(&clock, &item_id, message)).await;
                    }
                    Ok(StreamChunk::ToolCallDelta {
                        index,
                        id,
                        name,
                        args_delta,
                    }) => {
                        self.emit(
                            tx,
                            EngineEvent::ToolCallDelta {
                                meta: clock.meta(format!("tool_prep_{index}")),
                                index,
                                id,
                                name,
                                args_delta,
                            },
                        )
                        .await;
                    }
                    Err(error) => {
                        self.emit(tx, error_event(&clock, &item_id, error.to_string()))
                            .await;
                    }
                }
            }
            if reasoning_open {
                self.emit(
                    tx,
                    EngineEvent::ReasoningCompleted {
                        meta: clock.meta(&item_id),
                    },
                )
                .await;
            }

            let response = match done {
                Some(response) => response,
                None => anyhow::bail!("model stream ended without a response"),
            };

            let final_text = response.content.as_text();
            if !streamed_text && !final_text.is_empty() {
                self.emit(
                    tx,
                    EngineEvent::TextDelta {
                        meta: clock.meta(&item_id),
                        text: final_text.clone(),
                    },
                )
                .await;
            }
            self.emit(
                tx,
                EngineEvent::AssistantMessageCompleted {
                    meta: clock.meta(&item_id),
                },
            )
            .await;

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
            self.emit(
                tx,
                EngineEvent::Usage {
                    meta: clock.meta(&item_id),
                    input_tokens: usage.input_tokens,
                    output_tokens: usage.output_tokens,
                },
            )
            .await;

            if response.tool_calls.is_empty() {
                self.emit(
                    tx,
                    EngineEvent::Finished {
                        meta: clock.meta(turn_item),
                        stop_reason: reason_name(response.stop_reason).to_string(),
                        input_tokens: usage.input_tokens,
                        output_tokens: usage.output_tokens,
                        iterations: *iterations,
                    },
                )
                .await;
                return Ok(());
            }

            for call in &response.tool_calls {
                self.emit(
                    tx,
                    EngineEvent::ToolCallStarted {
                        meta: clock.meta(format!("tool_{}", call.id)),
                        id: call.id.clone(),
                        name: call.name.clone(),
                        args: call.args.clone(),
                    },
                )
                .await;
            }

            let mut planned: Vec<agent_model::ToolCall> = Vec::new();
            for call in &response.tool_calls {
                let scope = self
                    .tools
                    .find(&call.name)
                    .map(|tool| tool.permissions)
                    .unwrap_or(PermissionScope::Read);
                if !self.mode_allows(scope) {
                    let message = format!(
                        "tool {} is not permitted in {:?} mode",
                        call.name, self.mode
                    );
                    self.emit(
                        tx,
                        EngineEvent::ToolResult {
                            meta: clock.meta(format!("tool_{}", call.id)),
                            id: call.id.clone(),
                            name: call.name.clone(),
                            ok: false,
                            content: String::new(),
                            error: Some(message.clone()),
                            duration_ms: 0,
                            summary: Some("denied by mode".into()),
                            details: None,
                            exit_code: None,
                            truncated: false,
                            file_changes: Vec::new(),
                        },
                    )
                    .await;
                    let payload = serde_json::json!({ "ok": false, "error": message });
                    self.context
                        .push_tool_result(&call.id, &payload.to_string());
                    self.persist_tool_result(&call.id, &payload.to_string())?;
                } else {
                    planned.push(call.clone());
                }
            }

            if !planned.is_empty() {
                let ctx = Arc::new(self.tool_context());
                let tools = self.tools.clone();
                let mut tasks = Vec::new();
                for call in &planned {
                    let tools = tools.clone();
                    let ctx = Arc::clone(&ctx);
                    let tx = tx.clone();
                    let clock = clock.clone();
                    let call = call.clone();
                    let tool_item = format!("tool_{}", call.id);
                    let announced = {
                        let tx = tx.clone();
                        let clock = clock.clone();
                        let item = tool_item.clone();
                        let activity = tool_activity(&call);
                        async move {
                            let _ = tx
                                .send(EngineEvent::ActivityChanged {
                                    meta: clock.meta(item),
                                    activity,
                                    kind: Some("tool".into()),
                                })
                                .await;
                        }
                    };
                    let task_item = tool_item.clone();
                    tasks.push(async move {
                        announced.await;
                        let started = std::time::Instant::now();
                        let out = if call.name == "update_plan" {
                            let _ = tx
                                .send(EngineEvent::PlanUpdated {
                                    meta: clock.meta(task_item),
                                    steps: plan_steps(&call.args),
                                })
                                .await;
                            ToolOutput::success("plan updated".to_string())
                        } else {
                            tools
                                .call(&call.name, call.args.clone(), &ctx)
                                .await
                                .unwrap_or_else(|error| ToolOutput::failure(error.to_string()))
                        };
                        let duration_ms = started.elapsed().as_millis();
                        (call, out, duration_ms)
                    });
                }
                let mut unordered = futures::stream::FuturesUnordered::new();
                for task in tasks {
                    unordered.push(task);
                }
                while let Some((call, out, duration_ms)) = unordered.next().await {
                    self.handle_tool_finished(tx, &clock, &call, out, duration_ms)
                        .await;
                    *iterations += 1;
                }
            }

            if *iterations >= max_iterations {
                self.emit(
                    tx,
                    error_event(&clock, turn_item, "tool call limit reached"),
                )
                .await;
                break;
            }
        }
        Ok(())
    }

    async fn handle_tool_finished(
        &mut self,
        tx: &mpsc::Sender<EngineEvent>,
        clock: &EventClock,
        call: &agent_model::ToolCall,
        output: ToolOutput,
        duration_ms: u128,
    ) {
        let ToolOutput {
            ok,
            content,
            error,
            summary,
            details,
            file_changes,
            exit_code,
            truncated,
        } = output;
        let file_changes = tool_file_changes(file_changes);
        let tool_item = format!("tool_{}", call.id);
        self.emit(
            tx,
            EngineEvent::ToolCallCompleted {
                meta: clock.meta(&tool_item),
                id: call.id.clone(),
                name: call.name.clone(),
            },
        )
        .await;

        let payload = if ok {
            serde_json::json!({ "ok": true, "content": content })
        } else {
            serde_json::json!({
                "ok": false,
                "content": content,
                "error": error.clone().unwrap_or_else(|| "tool failed".into()),
            })
        };
        let text = payload.to_string();
        self.context.push_tool_result(&call.id, &text);
        let _ = self.persist_tool_result(&call.id, &text);

        self.emit(
            tx,
            EngineEvent::ToolResult {
                meta: clock.meta(&tool_item),
                id: call.id.clone(),
                name: call.name.clone(),
                ok,
                content: self.redact(&content),
                error: error.as_deref().map(|value| self.redact(value)),
                duration_ms,
                summary: summary.as_deref().map(|value| self.redact(value)),
                details: details.as_deref().map(|value| self.redact(value)),
                exit_code,
                truncated,
                file_changes: self.redact_file_changes(file_changes),
            },
        )
        .await;
    }

    fn redact(&self, text: &str) -> String {
        if self.config.permissions.secret_redaction {
            agent_permissions::redact_secrets(text)
        } else {
            text.to_string()
        }
    }

    fn redact_file_changes(&self, changes: Vec<Value>) -> Vec<Value> {
        if !self.config.permissions.secret_redaction {
            return changes;
        }
        changes
            .into_iter()
            .map(|mut value| {
                if let Some(diff) = value.get_mut("diff") {
                    if let Some(text) = diff.as_str() {
                        *diff = Value::String(self.redact(text));
                    }
                }
                value
            })
            .collect()
    }

    async fn emit(&self, tx: &mpsc::Sender<EngineEvent>, event: EngineEvent) {
        if let Some(storage) = &self.storage {
            let value = serde_json::to_value(&event).unwrap_or_default();
            let _ = storage.append_session_event(&self.session_id, &value);
        }
        let _ = tx.send(event).await;
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
        ToolExecutionContext::new(
            self.session_id,
            self.working_dir.clone(),
            PermissionEngine::new(self.config.permissions.clone()),
        )
        .with_permission_engine(self.permissions.clone())
        .with_approver(self.approver.clone())
        .with_prompter(self.approver.clone())
        .with_cancel(Arc::clone(&self.cancel))
        .with_turn_id(self.current_turn.clone())
    }

    async fn ensure_mcp_tools(&mut self) {
        if self.mcp_loaded {
            return;
        }
        self.mcp_loaded = true;
        let Some(client) = self.mcp.clone() else {
            return;
        };
        mcp::register_server_tools(&mut self.tools, client).await;
    }

    async fn ensure_plugins(&mut self) {
        if self.plugins_loaded {
            return;
        }
        self.plugins_loaded = true;
        let Some(root) = self.storage.as_ref().map(|storage| storage.root()) else {
            return;
        };
        let dir = root.join("plugins");
        let Ok(entries) = std::fs::read_dir(&dir) else {
            return;
        };
        let enabled_map = self.config.plugins.clone();
        for entry in entries.flatten() {
            let path = entry.path();
            if !path.is_dir() {
                continue;
            }
            let Some(name) = path.file_name().and_then(|value| value.to_str()) else {
                continue;
            };
            let Some(library_path) = agent_plugins::native::find_library(&path, name) else {
                continue;
            };
            let plugin = match NativePlugin::load(&library_path) {
                Ok(plugin) => Arc::new(plugin),
                Err(error) => {
                    tracing::warn!("skipping plugin {}: {error}", path.display());
                    continue;
                }
            };
            if !plugin.enabled(&enabled_map) {
                continue;
            }
            if let Err(error) = plugin.initialize().await {
                tracing::warn!("plugin '{}' failed to initialize: {error}", plugin.name());
                continue;
            }
            for tool_name in &plugin.tool_names {
                let full_name = format!("plugin_{}_{}", plugin.name(), tool_name);
                let executor = Arc::new(NativeToolExecutor {
                    plugin: Arc::clone(&plugin),
                    tool: tool_name.clone(),
                });
                self.tools.add(ToolDefinition {
                    name: full_name,
                    description: format!(
                        "Native plugin tool '{tool_name}' provided by '{}'",
                        plugin.name()
                    ),
                    input_schema: serde_json::json!({
                        "type": "object",
                        "properties": {},
                        "additionalProperties": true
                    }),
                    executor,
                    permissions: PermissionScope::Write,
                    timeout_secs: 60,
                });
            }
            self.plugins.push(plugin);
        }
    }

    fn ensure_session(&mut self) -> Result<()> {
        if self.session_ready {
            return Ok(());
        }
        if let Some(storage) = &self.storage {
            if !storage.session_path(&self.session_id).exists() {
                let mut record = SessionRecord {
                    id: self.session_id,
                    created_at: Utc::now(),
                    updated_at: Utc::now(),
                    project_path: None,
                    project_id: None,
                    project_name: None,
                    remote_url: None,
                    model: Some(self.model_name()),
                    metadata: serde_json::json!({
                        "title": Value::Null,
                        "mode": format!("{:?}", self.mode)
                    }),
                };
                record.attach_project(&project_ref(&self.working_dir));
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

pub fn project_ref(working_dir: &Path) -> ProjectRef {
    let root = canonical_project_root(working_dir);
    let name = root
        .file_name()
        .map(|s| s.to_string_lossy().into_owned())
        .unwrap_or_else(|| root.to_string_lossy().into_owned());
    ProjectRef {
        id: project_id(&root),
        name,
        path: root.clone(),
        remote_url: git_remote_origin(&root),
    }
}

fn tool_file_changes(changes: Vec<FileChange>) -> Vec<Value> {
    changes
        .into_iter()
        .map(|change| serde_json::to_value(change).unwrap_or(Value::Null))
        .collect()
}

fn tool_activity(call: &agent_model::ToolCall) -> String {
    let name = call.name.to_lowercase();
    let args = &call.args;
    let file = ["path", "file", "directory"]
        .iter()
        .filter_map(|key| args.get(*key).and_then(Value::as_str))
        .next()
        .map(|path| path.rsplit(['/', '\\']).next().unwrap_or(path).to_string());
    let command = args
        .get("command")
        .and_then(Value::as_str)
        .unwrap_or_default()
        .trim();
    match name.as_str() {
        "read_file" | "read_many_files" | "list_dir" => file
            .map(|f| format!("Reading {f}"))
            .unwrap_or_else(|| "Reading files…".into()),
        "write_file" | "create_file" | "edit_file" | "patch_file" | "apply_patch" => file
            .map(|f| format!("Editing {f}"))
            .unwrap_or_else(|| "Editing files…".into()),
        "delete_path" | "remove" => file
            .map(|f| format!("Deleting {f}"))
            .unwrap_or_else(|| "Deleting…".into()),
        "shell" | "terminal" | "run" => {
            let preview = command
                .split_whitespace()
                .take(4)
                .collect::<Vec<_>>()
                .join(" ");
            format!("Running {preview}")
        }
        "git" => format!("Running git {command}"),
        "test" | "run_tests" | "run_checks" => "Running tests…".into(),
        "glob" | "grep" | "find" => {
            let query = args
                .get("query")
                .or_else(|| args.get("pattern"))
                .and_then(Value::as_str)
                .unwrap_or_default()
                .trim();
            format!("Searching {query}")
        }
        "ask_user" => "Asking you…".into(),
        "view_image" => file
            .map(|f| format!("Viewing {f}"))
            .unwrap_or_else(|| "Viewing image…".into()),
        "http_fetch" | "fetch" => {
            let url = args.get("url").and_then(Value::as_str).unwrap_or_default();
            format!("Fetching {url}")
        }
        "lsp" => "Analyzing code…".into(),
        "process" => "Managing processes…".into(),
        "dependency" => "Checking dependencies…".into(),
        "update_plan" => "Updating plan…".into(),
        _ => format!("Preparing {name}"),
    }
}

fn plan_steps(args: &Value) -> Vec<PlanStep> {
    args.get("steps")
        .and_then(Value::as_array)
        .map(|steps| {
            steps
                .iter()
                .filter_map(|step| match step {
                    Value::String(title) => Some(PlanStep {
                        title: title.clone(),
                        status: None,
                    }),
                    Value::Object(map) => {
                        let title = map.get("title").and_then(Value::as_str)?.to_string();
                        let status = map
                            .get("status")
                            .and_then(Value::as_str)
                            .map(str::to_string);
                        Some(PlanStep { title, status })
                    }
                    _ => None,
                })
                .collect()
        })
        .unwrap_or_default()
}

fn event_log_max_sequence(storage: &Storage, session_id: &Uuid) -> u64 {
    storage
        .read_session_events(session_id)
        .iter()
        .filter_map(|event| event.get("meta").and_then(|m| m.get("sequence")))
        .filter_map(|sequence| sequence.as_u64())
        .max()
        .unwrap_or(0)
}

struct NativeToolExecutor {
    plugin: Arc<NativePlugin>,
    tool: String,
}

#[async_trait]
impl ToolExecutor for NativeToolExecutor {
    async fn execute(&self, args: Value, _ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let args_json = serde_json::to_string(&args).unwrap_or_else(|_| "{}".to_string());
        match self.plugin.run_tool(&self.tool, &args_json).await {
            Ok(content) => Ok(ToolOutput::success(content)),
            Err(error) => Ok(ToolOutput::failure(error.to_string())),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use agent_config::{ModelConfig, ProviderKind};
    use agent_model::{MessageContent, ToolCall};
    use async_trait::async_trait;
    use std::collections::VecDeque;
    use std::sync::Mutex;

    struct StepProvider {
        steps: Mutex<VecDeque<Vec<StreamChunk>>>,
    }

    impl StepProvider {
        fn new(steps: Vec<Vec<StreamChunk>>) -> Self {
            Self {
                steps: Mutex::new(steps.into()),
            }
        }
    }

    #[async_trait]
    impl ModelProvider for StepProvider {
        async fn name(&self) -> &'static str {
            "step"
        }

        async fn list_models(&self) -> Result<Vec<String>> {
            Ok(vec!["step".into()])
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
            let mut steps = self.steps.lock().unwrap();
            let chunks = steps.pop_front().unwrap_or_default();
            Ok(futures::stream::iter(chunks.into_iter().map(Ok)).boxed())
        }
    }

    fn done_response(content: &str, tool_calls: Vec<ToolCall>) -> StreamChunk {
        let has_tools = !tool_calls.is_empty();
        StreamChunk::Done {
            response: ModelResponse {
                content: MessageContent::Text(content.to_string()),
                tool_calls,
                stop_reason: if has_tools {
                    StopReason::ToolUse
                } else {
                    StopReason::EndTurn
                },
                usage: Usage {
                    input_tokens: 10,
                    output_tokens: 5,
                },
            },
        }
    }

    fn test_config() -> AgentConfig {
        AgentConfig {
            mode: Some(AgentMode::Auto),
            model: Some(ModelConfig {
                provider: ProviderKind::Mock,
                model: "step".into(),
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
        let (tx, mut rx) = mpsc::channel(256);
        let mut engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx.clone(),
            Box::new(StepProvider::new(vec![
                vec![done_response(
                    "",
                    vec![ToolCall {
                        id: "call_1".into(),
                        name: "read_file".into(),
                        args: serde_json::json!({ "path": "note.txt" }),
                    }],
                )],
                vec![done_response("done", vec![])],
            ])),
        )
        .with_storage(storage.clone());
        let session_id = engine.session_id;

        engine.run("please read note.txt", &tx).await.unwrap();
        drop(tx);

        let mut saw_tool_result = false;
        let mut saw_final_text = false;
        while let Ok(event) = rx.try_recv() {
            if matches!(event, EngineEvent::ToolResult { ok: true, .. }) {
                saw_tool_result = true;
            }
            if matches!(event, EngineEvent::TextDelta { text, .. } if text == "done") {
                saw_final_text = true;
            }
        }
        assert!(saw_tool_result, "expected a successful tool result event");
        assert!(saw_final_text, "expected the final text delta");

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
    async fn tool_call_precedes_tool_result() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join("note.txt"), "hello world").unwrap();
        let storage = Storage::new(tmp.path().join(".agent"));
        let (tx, mut rx) = mpsc::channel(256);
        let mut engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx.clone(),
            Box::new(StepProvider::new(vec![
                vec![done_response(
                    "",
                    vec![ToolCall {
                        id: "call_1".into(),
                        name: "read_file".into(),
                        args: serde_json::json!({ "path": "note.txt" }),
                    }],
                )],
                vec![done_response("done", vec![])],
            ])),
        )
        .with_storage(storage);

        engine.run("read the note", &tx).await.unwrap();
        drop(tx);

        let mut events = Vec::new();
        while let Ok(event) = rx.try_recv() {
            events.push(event);
        }
        let call_index = events
            .iter()
            .position(|event| matches!(event, EngineEvent::ToolCallStarted { name, .. } if name == "read_file"))
            .expect("expected a tool_call started event");
        let result_index = events
            .iter()
            .position(|event| matches!(event, EngineEvent::ToolResult { name, .. } if name == "read_file"))
            .expect("expected a tool_result event");
        assert!(
            call_index < result_index,
            "tool_call must arrive before tool_result"
        );
    }

    #[tokio::test]
    async fn tool_call_delta_is_forwarded() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join("note.txt"), "hello world").unwrap();
        let (tx, mut rx) = mpsc::channel(256);
        let mut engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx.clone(),
            Box::new(StepProvider::new(vec![
                vec![
                    StreamChunk::ToolCallDelta {
                        index: 0,
                        id: Some("call_1".into()),
                        name: Some("read_file".into()),
                        args_delta: "{\"path\":\"note".into(),
                    },
                    StreamChunk::ToolCallDelta {
                        index: 0,
                        id: None,
                        name: None,
                        args_delta: ".txt\"}".into(),
                    },
                    done_response(
                        "",
                        vec![ToolCall {
                            id: "call_1".into(),
                            name: "read_file".into(),
                            args: serde_json::json!({ "path": "note.txt" }),
                        }],
                    ),
                ],
                vec![done_response("done", vec![])],
            ])),
        );

        engine.run("read the note", &tx).await.unwrap();
        drop(tx);

        let mut deltas = 0;
        while let Ok(event) = rx.try_recv() {
            if matches!(event, EngineEvent::ToolCallDelta { .. }) {
                deltas += 1;
            }
        }
        assert!(
            deltas >= 2,
            "tool call deltas must be forwarded, got {deltas}"
        );
    }

    #[tokio::test]
    async fn reasoning_events_are_bracketed() {
        let tmp = tempfile::tempdir().unwrap();
        let (tx, mut rx) = mpsc::channel(256);
        let mut engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx.clone(),
            Box::new(StepProvider::new(vec![vec![
                StreamChunk::ReasoningDelta("thinking hard".into()),
                StreamChunk::TextDelta("step one".into()),
                done_response("step one", vec![]),
            ]])),
        );
        engine.run("hi", &tx).await.unwrap();
        drop(tx);

        let mut events = Vec::new();
        while let Ok(event) = rx.try_recv() {
            events.push(event);
        }
        let started = events
            .iter()
            .position(|e| matches!(e, EngineEvent::ReasoningStarted { .. }))
            .expect("reasoning_started expected");
        let delta = events
            .iter()
            .position(|e| matches!(e, EngineEvent::ReasoningDelta { .. }))
            .expect("reasoning_delta expected");
        let completed = events
            .iter()
            .position(|e| matches!(e, EngineEvent::ReasoningCompleted { .. }))
            .expect("reasoning_completed expected");
        assert!(
            started < delta && delta < completed,
            "reasoning must be bracketed"
        );
    }

    #[tokio::test]
    async fn final_assistant_text_is_separate_item_after_tools() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join("a.txt"), "a").unwrap();
        std::fs::write(tmp.path().join("b.txt"), "b").unwrap();
        let (tx, mut rx) = mpsc::channel(256);
        let mut engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx.clone(),
            Box::new(StepProvider::new(vec![
                vec![
                    StreamChunk::ReasoningDelta("I will read both files".into()),
                    StreamChunk::TextDelta("I will check the files".into()),
                    done_response(
                        "I will check the files",
                        vec![
                            ToolCall {
                                id: "call_a".into(),
                                name: "read_file".into(),
                                args: serde_json::json!({ "path": "a.txt" }),
                            },
                            ToolCall {
                                id: "call_b".into(),
                                name: "read_file".into(),
                                args: serde_json::json!({ "path": "b.txt" }),
                            },
                        ],
                    ),
                ],
                vec![
                    StreamChunk::ReasoningDelta("now I summarize".into()),
                    StreamChunk::TextDelta("Both files are ready. Done!".into()),
                    done_response("Both files are ready. Done!", vec![]),
                ],
            ])),
        );
        engine.run("read both files", &tx).await.unwrap();
        drop(tx);

        let mut events = Vec::new();
        while let Ok(event) = rx.try_recv() {
            events.push(event);
        }

        let assistant_items: Vec<String> = events
            .iter()
            .filter_map(|event| match event {
                EngineEvent::AssistantMessageStarted { id, .. } => Some(id.clone()),
                _ => None,
            })
            .collect();
        assert_eq!(
            assistant_items.len(),
            2,
            "two model iterations must create two assistant items"
        );

        let last_tool_result = events
            .iter()
            .rposition(|e| matches!(e, EngineEvent::ToolResult { .. }))
            .expect("expected tool results");
        let second_assistant = events
            .iter()
            .position(|e| matches!(e, EngineEvent::AssistantMessageStarted { id, .. } if *id == assistant_items[1]))
            .expect("expected second assistant item");
        assert!(
            last_tool_result < second_assistant,
            "final assistant item must start after all tool results"
        );

        let final_text_index = events
            .iter()
            .position(|e| matches!(e, EngineEvent::TextDelta { text, .. } if text == "Both files are ready. Done!"))
            .expect("final text expected");
        let final_text_item = match &events[final_text_index] {
            EngineEvent::TextDelta { meta, .. } => meta.item_id.clone(),
            _ => unreachable!(),
        };
        assert!(
            final_text_index > last_tool_result,
            "final text must be after tools"
        );
        assert_eq!(
            final_text_item, assistant_items[1],
            "final text must belong to the second assistant item, not the first"
        );
    }

    #[tokio::test]
    async fn parallel_tools_report_completion_order_and_individual_duration() {
        use std::time::Duration;
        use tokio::time::sleep;

        struct SleepTool {
            ms: u64,
        }

        #[async_trait]
        impl ToolExecutor for SleepTool {
            async fn execute(
                &self,
                _args: Value,
                _ctx: &ToolExecutionContext,
            ) -> Result<ToolOutput> {
                sleep(Duration::from_millis(self.ms)).await;
                Ok(ToolOutput::success(format!("slept {}ms", self.ms)))
            }
        }

        let tmp = tempfile::tempdir().unwrap();
        let (tx, mut rx) = mpsc::channel(256);
        let mut engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx.clone(),
            Box::new(StepProvider::new(vec![
                vec![done_response(
                    "",
                    vec![
                        ToolCall {
                            id: "call_slow".into(),
                            name: "sleep_80".into(),
                            args: serde_json::json!({}),
                        },
                        ToolCall {
                            id: "call_fast".into(),
                            name: "sleep_15".into(),
                            args: serde_json::json!({}),
                        },
                    ],
                )],
                vec![done_response("done", vec![])],
            ])),
        );
        engine.tools.add(ToolDefinition {
            name: "sleep_80".into(),
            description: "sleep for 80ms".into(),
            input_schema: serde_json::json!({ "type": "object" }),
            executor: Arc::new(SleepTool { ms: 80 }),
            permissions: PermissionScope::Read,
            timeout_secs: 10,
        });
        engine.tools.add(ToolDefinition {
            name: "sleep_15".into(),
            description: "sleep for 15ms".into(),
            input_schema: serde_json::json!({ "type": "object" }),
            executor: Arc::new(SleepTool { ms: 15 }),
            permissions: PermissionScope::Read,
            timeout_secs: 10,
        });

        engine.run("run the sleeps", &tx).await.unwrap();
        drop(tx);

        let mut orders = Vec::new();
        let mut durations = std::collections::HashMap::new();
        while let Ok(event) = rx.try_recv() {
            if let EngineEvent::ToolResult {
                id, duration_ms, ..
            } = event
            {
                orders.push(id.clone());
                durations.insert(id, duration_ms);
            }
        }
        assert_eq!(orders.len(), 2);
        assert_eq!(orders[0], "call_fast", "fast tool must finish first");
        assert_eq!(orders[1], "call_slow", "slow tool must finish second");
        let fast = durations.get("call_fast").copied().unwrap_or(0);
        let slow = durations.get("call_slow").copied().unwrap_or(0);
        assert!(
            fast > 0 && fast < slow,
            "individual durations expected, fast={fast} slow={slow}"
        );
    }

    #[tokio::test]
    async fn tool_secrets_redacted_in_event_log() {
        struct LeakTool;
        #[async_trait]
        impl ToolExecutor for LeakTool {
            async fn execute(
                &self,
                _args: Value,
                _ctx: &ToolExecutionContext,
            ) -> Result<ToolOutput> {
                Ok(
                    ToolOutput::success("api key: sk-abcdef1234567890abcdef".into())
                        .summary("discloses sk-11112222333344445555")
                        .details("detail password=secret123")
                        .file_change(FileChange {
                            path: "note.txt".into(),
                            change: "modified".into(),
                            diff: Some("+set token=abc12345secret".into()),
                            additions: Some(1),
                            deletions: Some(0),
                        }),
                )
            }
        }

        let tmp = tempfile::tempdir().unwrap();
        let storage = Storage::new(tmp.path().join(".agent"));
        let (tx, mut rx) = mpsc::channel(256);
        let mut engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx.clone(),
            Box::new(StepProvider::new(vec![
                vec![done_response(
                    "",
                    vec![ToolCall {
                        id: "call_l".into(),
                        name: "leak".into(),
                        args: serde_json::json!({}),
                    }],
                )],
                vec![done_response("done", vec![])],
            ])),
        )
        .with_storage(storage.clone());
        engine.tools.add(ToolDefinition {
            name: "leak".into(),
            description: "leak".into(),
            input_schema: serde_json::json!({ "type": "object" }),
            executor: Arc::new(LeakTool),
            permissions: PermissionScope::Read,
            timeout_secs: 10,
        });

        engine.run("find the key", &tx).await.unwrap();
        drop(tx);

        let mut saw_redacted = false;
        while let Ok(event) = rx.try_recv() {
            if let EngineEvent::ToolResult {
                name,
                content,
                file_changes,
                ..
            } = event
            {
                if name == "leak" {
                    saw_redacted = true;
                    assert!(!content.contains("sk-abcdef1234567890abcdef"));
                    assert!(content.contains("[OPENAI_API_KEY_REDACTED]"));
                    let diff = file_changes
                        .first()
                        .and_then(|c| c.get("diff"))
                        .and_then(|d| d.as_str())
                        .unwrap_or("");
                    assert!(!diff.contains("abc12345secret"));
                }
            }
        }
        assert!(saw_redacted, "expected the leak tool result");

        let events = storage.read_session_events(&engine.session_id);
        let blob = serde_json::to_string(&events).unwrap();
        assert!(!blob.contains("sk-abcdef1234567890abcdef"));
        assert!(!blob.contains("sk-11112222333344445555"));
        assert!(!blob.contains("password=secret123"));
    }

    #[tokio::test]
    async fn event_log_reload_restores_exact_timeline() {
        let tmp = tempfile::tempdir().unwrap();
        let storage = Storage::new(tmp.path().join(".agent"));
        let (tx, mut rx) = mpsc::channel(256);
        let mut engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx.clone(),
            Box::new(StepProvider::new(vec![
                vec![done_response(
                    "",
                    vec![
                        ToolCall {
                            id: "call_a".into(),
                            name: "read_file".into(),
                            args: serde_json::json!({ "path": "a.txt" }),
                        },
                        ToolCall {
                            id: "call_b".into(),
                            name: "read_file".into(),
                            args: serde_json::json!({ "path": "b.txt" }),
                        },
                    ],
                )],
                vec![done_response("final answer here", vec![])],
            ])),
        )
        .with_storage(storage.clone());
        let session_id = engine.session_id;
        std::fs::write(tmp.path().join("a.txt"), "alpha").unwrap();
        std::fs::write(tmp.path().join("b.txt"), "beta").unwrap();

        engine.run("read both files", &tx).await.unwrap();
        drop(tx);

        let mut captured: Vec<String> = Vec::new();
        while let Ok(event) = rx.try_recv() {
            let entry = match &event {
                EngineEvent::TurnStarted { meta, .. } => format!("turn_started:{}", meta.item_id),
                EngineEvent::AssistantMessageStarted { meta, .. } => {
                    format!("assistant_message_started:{}", meta.item_id)
                }
                EngineEvent::TextDelta { meta, text } => {
                    format!("text_delta:{}:{text}", meta.item_id)
                }
                EngineEvent::ToolResult { meta, id, .. } => {
                    format!("tool_result:{}:{}", meta.item_id, id)
                }
                EngineEvent::AssistantMessageCompleted { meta } => {
                    format!("assistant_message_completed:{}", meta.item_id)
                }
                EngineEvent::TurnCompleted { meta } => format!("turn_completed:{}", meta.item_id),
                EngineEvent::Usage { meta, .. } => format!("usage:{}", meta.item_id),
                _ => continue,
            };
            captured.push(entry);
        }
        assert!(
            captured.len() > 1,
            "expected a captured timeline, got {}",
            captured.len()
        );

        let stored: Vec<String> = storage
            .read_session_events(&session_id)
            .iter()
            .filter_map(|event| {
                let kind = event.get("type").and_then(|t| t.as_str())?;
                match kind {
                    "turn_started"
                    | "assistant_message_started"
                    | "assistant_message_completed"
                    | "turn_completed"
                    | "usage" => {}
                    "text_delta" | "tool_result" => {}
                    _ => return None,
                }
                let item = event
                    .get("item_id")
                    .and_then(|v| v.as_str())
                    .unwrap_or_default();
                if kind == "text_delta" {
                    let text = event.get("text").and_then(|t| t.as_str()).unwrap_or("");
                    return Some(format!("text_delta:{item}:{text}"));
                }
                if kind == "tool_result" {
                    let id = event.get("id").and_then(|v| v.as_str()).unwrap_or("");
                    return Some(format!("tool_result:{item}:{id}"));
                }
                Some(format!("{kind}:{item}"))
            })
            .collect();
        assert_eq!(
            stored, captured,
            "persisted event log must reproduce the exact streamed timeline"
        );

        let tool_last = stored
            .iter()
            .rposition(|entry| entry.starts_with("tool_result:"))
            .expect("expected tool results");
        let final_text = stored
            .iter()
            .position(|entry| {
                entry.starts_with("text_delta:") && entry.ends_with("final answer here")
            })
            .expect("expected final text");
        assert!(
            tool_last < final_text,
            "final answer must be reloaded after the tools"
        );
    }

    #[tokio::test]
    async fn permission_mode_switch_changes_decisions() {
        use agent_permissions::PermissionDecision;
        let tmp = tempfile::tempdir().unwrap();
        let (tx, _rx) = mpsc::channel(8);
        let mut config = test_config();
        config.mode = Some(AgentMode::Interactive);
        config.permissions.mode = PermissionMode::Ask;
        let engine = AgentEngine::with_provider(
            config,
            tmp.path().to_path_buf(),
            tx,
            Box::new(StepProvider::new(vec![])),
        );
        let check = |engine: &AgentEngine, scope: PermissionScope, tool: &str| {
            engine
                .tool_context()
                .permission_engine
                .lock()
                .unwrap()
                .check(scope, tool, "a.txt")
                .decision
        };
        assert_eq!(
            check(&engine, PermissionScope::Write, "write_file"),
            PermissionDecision::Ask
        );
        engine.set_permission_mode(PermissionMode::Allow);
        assert_eq!(
            check(&engine, PermissionScope::Write, "write_file"),
            PermissionDecision::Allow
        );
        assert_eq!(
            check(&engine, PermissionScope::Execute, "shell"),
            PermissionDecision::Allow
        );
        engine.set_permission_mode(PermissionMode::AutoEdit);
        assert_eq!(
            check(&engine, PermissionScope::Write, "write_file"),
            PermissionDecision::Allow
        );
        assert_eq!(
            check(&engine, PermissionScope::Delete, "delete_path"),
            PermissionDecision::Allow
        );
        assert_eq!(
            check(&engine, PermissionScope::Execute, "shell"),
            PermissionDecision::Ask
        );
    }

    #[tokio::test]
    async fn turn_events_are_persisted_to_event_log() {
        let tmp = tempfile::tempdir().unwrap();
        let storage = Storage::new(tmp.path().join(".agent"));
        let (tx, mut rx) = mpsc::channel(256);
        let mut engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx.clone(),
            Box::new(StepProvider::new(vec![vec![done_response(
                "hi there",
                vec![],
            )]])),
        )
        .with_storage(storage.clone());
        let session_id = engine.session_id;
        engine.run("say hi", &tx).await.unwrap();
        drop(tx);
        while rx.try_recv().is_ok() {}

        let events = storage.read_session_events(&session_id);
        let types: Vec<String> = events
            .iter()
            .filter_map(|e| {
                e.get("type")
                    .and_then(|t| t.as_str())
                    .map(|s| s.to_string())
            })
            .collect();
        assert!(
            types.contains(&"turn_started".to_string()),
            "event log must contain turn_started"
        );
        assert!(
            types.contains(&"turn_completed".to_string()),
            "event log must contain turn_completed"
        );
        assert!(
            types.contains(&"assistant_message_started".to_string()),
            "event log must contain assistant start"
        );
    }

    #[tokio::test]
    async fn skill_instructions_are_injected() {
        let tmp = tempfile::tempdir().unwrap();
        let skill_dir = tmp.path().join(".agent").join("skills").join("my-skill");
        std::fs::create_dir_all(&skill_dir).unwrap();
        std::fs::write(
            skill_dir.join("SKILL.md"),
            "---\ntools: read_file\n---\n# My Skill\nAlways greet the user when asked.",
        )
        .unwrap();
        let (tx, _rx) = mpsc::channel(8);
        let engine = AgentEngine::with_provider(
            test_config(),
            tmp.path().to_path_buf(),
            tx,
            Box::new(StepProvider::new(vec![])),
        );
        let messages = engine.context.render_for_model();
        let content = messages[0].content.as_text();
        assert!(content.contains("Always greet the user"));
    }

    #[tokio::test]
    async fn resumed_session_sends_history_to_model() {
        let tmp = tempfile::tempdir().unwrap();
        let storage = Storage::new(tmp.path().join(".agent"));
        let session_id = Uuid::new_v4();
        let requests: Arc<std::sync::Mutex<Vec<ModelRequest>>> = Arc::new(Mutex::new(Vec::new()));

        {
            let (tx, _rx) = mpsc::channel(8);
            let recording = Arc::clone(&requests);
            let mut engine = AgentEngine::with_provider(
                test_config(),
                tmp.path().to_path_buf(),
                tx.clone(),
                Box::new(RecordingProvider {
                    requests: recording,
                    first_reply: MessageContent::Text(String::new()),
                }),
            )
            .with_storage(storage.clone());
            engine.session_id = session_id;
            engine
                .run("remember the secret token: 42-alpha-77", &tx)
                .await
                .unwrap();
        }

        {
            let (tx, _rx) = mpsc::channel(8);
            let recording = Arc::clone(&requests);
            let engine = AgentEngine::with_provider(
                test_config(),
                tmp.path().to_path_buf(),
                tx.clone(),
                Box::new(RecordingProvider {
                    requests: recording,
                    first_reply: MessageContent::Text(String::new()),
                }),
            )
            .with_storage(storage.clone());
            let engine = engine.with_session(session_id);
            let messages = engine.context.render_for_model();
            let all: Vec<String> = messages
                .iter()
                .map(|m| {
                    let role = match m.role {
                        Role::User => "user",
                        Role::Assistant => "assistant",
                        Role::Tool => "tool",
                        Role::System => "system",
                    };
                    format!("[{role}] {}", m.content.as_text())
                })
                .collect();
            let joined = all.join("\n");
            assert!(
                joined.contains("42-alpha-77"),
                "resumed context must contain the first user message, got: {joined}"
            );
        }
    }

    struct RecordingProvider {
        requests: Arc<std::sync::Mutex<Vec<ModelRequest>>>,
        first_reply: MessageContent,
    }

    #[async_trait]
    impl ModelProvider for RecordingProvider {
        async fn name(&self) -> &'static str {
            "recording"
        }

        async fn list_models(&self) -> Result<Vec<String>> {
            Ok(vec!["recording".into()])
        }

        async fn chat(&self, _request: &ModelRequest) -> Result<ModelResponse> {
            anyhow::bail!("chat is not used in this test")
        }

        async fn chat_stream(
            &self,
            request: &ModelRequest,
        ) -> Result<
            futures::stream::BoxStream<
                'static,
                std::result::Result<StreamChunk, agent_model::ModelError>,
            >,
        > {
            use futures::stream::StreamExt;
            self.requests.lock().unwrap().push(request.clone());
            let response = ModelResponse {
                content: self.first_reply.clone(),
                tool_calls: vec![],
                stop_reason: StopReason::EndTurn,
                usage: Usage {
                    input_tokens: 1,
                    output_tokens: 1,
                },
            };
            Ok(futures::stream::iter(vec![Ok(StreamChunk::Done { response })]).boxed())
        }
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
            Box::new(StepProvider::new(vec![])),
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
