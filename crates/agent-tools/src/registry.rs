use agent_events::{AgentEvent, AgentEventPayload, AgentScope, EventBus};
use agent_permissions::PermissionDecision;
use anyhow::{anyhow, Result};

use crate::executor::{ToolDefinition, ToolExecutionContext, ToolOutput};

pub struct ToolRegistry {
    pub tools: Vec<ToolDefinition>,
    pub events: EventBus,
}

impl Clone for ToolRegistry {
    fn clone(&self) -> Self {
        Self {
            tools: self.tools.clone(),
            events: self.events.clone(),
        }
    }
}

impl Default for ToolRegistry {
    fn default() -> Self {
        Self::new()
    }
}

impl ToolRegistry {
    pub fn new() -> Self {
        Self {
            tools: Vec::new(),
            events: EventBus::new(),
        }
    }

    pub fn with_events(mut self, events: EventBus) -> Self {
        self.events = events;
        self
    }

    pub fn register(mut self, tool: ToolDefinition) -> Self {
        tracing::debug!(name = tool.name.as_str(), "registering tool");
        self.tools.push(tool);
        self
    }

    pub fn schemas(&self) -> Vec<agent_model::ToolSchema> {
        self.tools.iter().map(|t| t.schema()).collect()
    }

    pub fn find(&self, name: &str) -> Option<&ToolDefinition> {
        self.tools.iter().find(|t| t.name == name)
    }

    pub async fn call(
        &self,
        name: &str,
        args: serde_json::Value,
        ctx: &ToolExecutionContext,
    ) -> Result<ToolOutput> {
        let tool = self
            .find(name)
            .ok_or_else(|| anyhow!("tool not found: {name}"))?;

        self.events.dispatch(
            AgentEventPayload::new(AgentEvent::ToolBefore)
                .with_scope(AgentScope::Tool)
                .with_detail(serde_json::json!({ "tool": name, "args": args.clone() })),
        );

        let target = args
            .get("path")
            .or(args.get("command"))
            .map(|v| v.as_str().unwrap_or_default())
            .unwrap_or_default();
        let decision = {
            let mut engine = ctx.permission_engine.lock().unwrap();
            engine.check(tool.permissions, name, target)
        };
        let resolved = match decision.decision {
            PermissionDecision::Allow => PermissionDecision::Allow,
            PermissionDecision::Deny => PermissionDecision::Deny,
            PermissionDecision::Ask => match &ctx.approver {
                Some(approver) => {
                    approver
                        .approve(crate::executor::PermissionRequest {
                            session_id: ctx.session_id,
                            tool: name.to_string(),
                            scope: tool.permissions,
                            target: target.to_string(),
                            reason: decision.reason.clone(),
                        })
                        .await
                }
                None => PermissionDecision::Deny,
            },
        };
        if resolved != PermissionDecision::Allow {
            return Ok(ToolOutput::failure(format!(
                "permission denied for {name} ({})",
                decision.reason
            )));
        }

        let started = std::time::Instant::now();
        let result = tokio::time::timeout(
            std::time::Duration::from_secs(tool.timeout_secs),
            self.invoke(tool, args, ctx),
        )
        .await;
        let elapsed_ms = started.elapsed().as_millis();

        let out = match result {
            Ok(Ok(output)) => output,
            Ok(Err(error)) => {
                self.events.dispatch(
                    AgentEventPayload::new(AgentEvent::ToolError)
                        .with_scope(AgentScope::Tool)
                        .with_detail(
                            serde_json::json!({ "tool": name, "error": error.to_string() }),
                        ),
                );
                ToolOutput::failure(error.to_string())
            }
            Err(_) => {
                self.events.dispatch(
                    AgentEventPayload::new(AgentEvent::ToolError)
                        .with_scope(AgentScope::Tool)
                        .with_detail(serde_json::json!({ "tool": name, "error": "timed out" })),
                );
                ToolOutput::failure(format!("tool {name} timed out"))
            }
        };

        self.events.dispatch(
            AgentEventPayload::new(AgentEvent::ToolAfter)
                .with_scope(AgentScope::Tool)
                .with_detail(serde_json::json!({
                    "tool": name,
                    "ms": elapsed_ms,
                    "ok": out.ok,
                })),
        );
        Ok(out)
    }

    async fn invoke(
        &self,
        tool: &ToolDefinition,
        args: serde_json::Value,
        ctx: &ToolExecutionContext,
    ) -> Result<ToolOutput> {
        tool.executor.execute(args, ctx).await
    }

    pub async fn call_parallel(
        &self,
        calls: &[(String, serde_json::Value)],
        ctx: &ToolExecutionContext,
        max_concurrent: usize,
    ) -> Result<Vec<(String, ToolOutput)>> {
        let mut results: Vec<(String, ToolOutput)> = Vec::new();
        for chunk in calls.chunks(max_concurrent.max(1)) {
            let futures: Vec<_> = chunk
                .iter()
                .map(|(n, a)| self.call(n, a.clone(), ctx))
                .collect();
            let joined = futures::future::join_all(futures).await;
            for (item, (n, _)) in joined.into_iter().zip(chunk.iter()) {
                match item {
                    Ok(out) => results.push((n.clone(), out)),
                    Err(e) => return Err(e),
                }
            }
        }
        Ok(results)
    }
}
