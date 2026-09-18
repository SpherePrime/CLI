use std::path::Path;

use anyhow::{bail, Result};
use async_trait::async_trait;
use serde_json::{json, Value};

use crate::builtin::exec::run_shell_command;
use crate::executor::{ToolExecutionContext, ToolExecutor, ToolOutput};

fn json_schema(properties: Value, required: &[&str]) -> Value {
    json!({
        "type": "object",
        "properties": properties,
        "required": required,
        "additionalProperties": false,
    })
}

fn package_manager(dir: &Path) -> &'static str {
    if dir.join("bun.lockb").exists() || dir.join("bun.lock").exists() {
        "bun"
    } else if dir.join("pnpm-lock.yaml").exists() {
        "pnpm"
    } else if dir.join("yarn.lock").exists() {
        "yarn"
    } else {
        "npm"
    }
}

fn package_scripts(dir: &Path) -> Vec<String> {
    let raw = match std::fs::read_to_string(dir.join("package.json")) {
        Ok(raw) => raw,
        Err(_) => return Vec::new(),
    };
    let value: Value = match serde_json::from_str(&raw) {
        Ok(value) => value,
        Err(_) => return Vec::new(),
    };
    value
        .get("scripts")
        .and_then(|scripts| scripts.as_object())
        .map(|scripts| scripts.keys().cloned().collect())
        .unwrap_or_default()
}

pub fn plan_commands(
    dir: &Path,
    kind: &str,
    override_command: Option<&str>,
) -> Result<Vec<String>> {
    if let Some(command) = override_command {
        return Ok(vec![command.to_string()]);
    }
    if dir.join("Cargo.toml").exists() {
        return Ok(match kind {
            "test" => vec!["cargo test --workspace --no-fail-fast".into()],
            "lint" => vec!["cargo clippy --workspace --all-targets -- -D warnings".into()],
            "format" => vec!["cargo fmt --all -- --check".into()],
            "all" => vec![
                "cargo fmt --all -- --check".into(),
                "cargo clippy --workspace --all-targets -- -D warnings".into(),
                "cargo test --workspace --no-fail-fast".into(),
            ],
            _ => vec!["cargo test --workspace --no-fail-fast".into()],
        });
    }
    if dir.join("package.json").exists() {
        let manager = package_manager(dir);
        let scripts = package_scripts(dir);
        let has = |name: &str| scripts.iter().any(|script| script == name);
        let run = |name: &str| match manager {
            "bun" => format!("bun run {name}"),
            "pnpm" => format!("pnpm run {name}"),
            "yarn" => format!("yarn {name}"),
            _ => format!("npm run {name}"),
        };
        let test_cmd = match manager {
            "bun" => "bun test".to_string(),
            _ => format!("{manager} test"),
        };
        return Ok(match kind {
            "test" => vec![test_cmd],
            "lint" if has("lint") => vec![run("lint")],
            "typecheck" if has("typecheck") => vec![run("typecheck")],
            "all" => {
                let mut commands = Vec::new();
                if has("typecheck") {
                    commands.push(run("typecheck"));
                }
                if has("lint") {
                    commands.push(run("lint"));
                }
                commands.push(test_cmd);
                commands
            }
            _ => vec![test_cmd],
        });
    }
    if dir.join("pyproject.toml").exists() || dir.join("pytest.ini").exists() {
        return Ok(vec!["pytest -q".into()]);
    }
    bail!("could not detect a project type (Cargo.toml, package.json or pyproject.toml)")
}

pub struct RunChecksTool;

#[async_trait]
impl ToolExecutor for RunChecksTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let kind = args
            .get("kind")
            .and_then(|v| v.as_str())
            .unwrap_or("all")
            .to_lowercase();
        let override_command = args.get("command").and_then(|v| v.as_str());
        let commands = plan_commands(&ctx.working_dir, &kind, override_command)?;

        let mut report = String::new();
        let mut failed: Option<i32> = None;
        for command in &commands {
            report.push_str(&format!("$ {command}\n"));
            let output = run_shell_command(ctx, command, ctx.working_dir.clone(), 600).await?;
            report.push_str(&output.content);
            report.push_str("\n\n");
            if !output.ok {
                failed = output.exit_code.or(Some(1));
                break;
            }
        }
        let report = report.trim_end().to_string();
        if let Some(code) = failed {
            return Ok(ToolOutput::failure(format!("checks failed (exit {code})"))
                .content(report)
                .exit_code(code));
        }
        Ok(ToolOutput::success(report).summary(format!("{} check(s) passed", commands.len())))
    }
}

pub fn run_checks_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "run_checks".into(),
        description: "Detect the project type and run its tests/lint/format/typecheck commands."
            .into(),
        input_schema: json_schema(
            json!({
                "kind": {
                    "type": "string",
                    "enum": ["test", "lint", "format", "typecheck", "all"],
                    "description": "Which checks to run (default all)"
                },
                "command": { "type": "string", "description": "Optional explicit command override" }
            }),
            &[],
        ),
        executor: std::sync::Arc::new(RunChecksTool),
        permissions: agent_permissions::PermissionScope::Execute,
        timeout_secs: 900,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn plans_cargo_all() {
        let dir = std::env::temp_dir().join(format!("agent-checks-{}", uuid::Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        std::fs::write(dir.join("Cargo.toml"), "[package]").unwrap();
        let commands = plan_commands(&dir, "all", None).unwrap();
        assert_eq!(commands.len(), 3);
        assert!(commands[0].contains("cargo fmt"));
        assert!(commands[2].contains("cargo test"));
    }

    #[test]
    fn override_wins_and_unknown_project_errors() {
        let dir = std::env::temp_dir().join(format!("agent-checks-{}", uuid::Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        assert!(plan_commands(&dir, "all", None).is_err());
        let commands = plan_commands(&dir, "all", Some("echo hi")).unwrap();
        assert_eq!(commands, vec!["echo hi".to_string()]);
    }
}
