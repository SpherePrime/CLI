use anyhow::Result;
use async_trait::async_trait;
use serde_json::{json, Value};

use crate::executor::{ToolExecutionContext, ToolExecutor, ToolOutput};

pub struct UpdatePlanTool;

#[async_trait]
impl ToolExecutor for UpdatePlanTool {
    async fn execute(&self, args: Value, _ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let count = args
            .get("steps")
            .and_then(|v| v.as_array())
            .map(|steps| steps.len())
            .unwrap_or(0);
        Ok(ToolOutput::success(format!("plan updated ({count} steps)"))
            .summary(format!("{count} step(s)")))
    }
}

pub fn update_plan_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "update_plan".into(),
        description:
            "Publish or update the task plan. Each step has a title and status (pending, in_progress, completed)."
                .into(),
        input_schema: json!({
            "type": "object",
            "properties": {
                "steps": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "properties": {
                            "title": { "type": "string" },
                            "status": { "type": "string", "enum": ["pending", "in_progress", "completed"] }
                        },
                        "required": ["title"]
                    }
                }
            },
            "required": ["steps"],
            "additionalProperties": false,
        }),
        executor: std::sync::Arc::new(UpdatePlanTool),
        permissions: agent_permissions::PermissionScope::Read,
        timeout_secs: 15,
    }
}
