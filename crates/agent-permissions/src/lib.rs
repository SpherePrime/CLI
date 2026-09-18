use std::path::PathBuf;

use agent_config::{PermissionMode, PermissionsConfig};
use agent_storage::{AuditOutcome, AuditRecord};
use anyhow::anyhow;
use regex::Regex;
use uuid::Uuid;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
pub enum PermissionScope {
    #[default]
    Read,
    Write,
    Delete,
    Execute,
    Network,
    Install,
    External,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum PermissionDecision {
    Allow,
    Deny,
    Ask,
}

#[derive(Debug, Clone)]
pub struct Decision {
    pub decision: PermissionDecision,
    pub reason: String,
}

impl Decision {
    fn new(decision: PermissionDecision, reason: impl Into<String>) -> Self {
        Self {
            decision,
            reason: reason.into(),
        }
    }

    pub fn is_allowed(&self) -> bool {
        self.decision == PermissionDecision::Allow
    }
}

pub struct PermissionEngine {
    config: PermissionsConfig,
    audit: Option<agent_storage::Storage>,
}

impl PermissionEngine {
    pub fn new(config: PermissionsConfig) -> Self {
        Self {
            config,
            audit: None,
        }
    }

    pub fn with_storage(mut self, storage: agent_storage::Storage) -> Self {
        self.audit = Some(storage);
        self
    }

    pub fn set_mode(&mut self, mode: PermissionMode) {
        self.config.mode = mode;
    }

    pub fn mode(&self) -> PermissionMode {
        self.config.mode
    }

    pub fn check(&mut self, scope: PermissionScope, tool_name: &str, target: &str) -> Decision {
        let decision = self.evaluate(scope, tool_name, target);
        if self.config.secret_redaction {
            self.record_audit(tool_name, target, &decision, scope);
        }
        decision
    }

    fn evaluate(&self, scope: PermissionScope, tool_name: &str, target: &str) -> Decision {
        if self.config.denied_tools.iter().any(|d| d == tool_name) {
            return Decision::new(PermissionDecision::Deny, "tool is denied by config");
        }
        if self.config.allowed_tools.iter().any(|a| a == tool_name)
            && self.config.mode == PermissionMode::Allow
        {
            return Decision::new(PermissionDecision::Allow, "tool is explicitly allowed");
        }
        let path_ok = self.check_paths(target);
        let scope_ok = self.check_scope(scope, target);
        let decision = match self.config.mode {
            PermissionMode::Deny => PermissionDecision::Deny,
            PermissionMode::Allow if path_ok && scope_ok => PermissionDecision::Allow,
            PermissionMode::Allow => PermissionDecision::Deny,
            PermissionMode::AutoEdit if path_ok => match scope {
                PermissionScope::Read | PermissionScope::Write | PermissionScope::Delete => {
                    PermissionDecision::Allow
                }
                _ => PermissionDecision::Ask,
            },
            PermissionMode::AutoEdit => PermissionDecision::Deny,
            PermissionMode::Ask if !path_ok => PermissionDecision::Deny,
            _ => PermissionDecision::Ask,
        };
        Decision::new(decision, format!("tool={tool_name} target={target}"))
    }

    fn check_paths(&self, target: &str) -> bool {
        if self.config.allowed_paths.is_empty() {
            return true;
        }
        let t = PathBuf::from(target);
        for p in &self.config.allowed_paths {
            let allowed = PathBuf::from(p);
            if t.starts_with(&allowed) {
                return true;
            }
        }
        false
    }

    fn check_scope(&self, scope: PermissionScope, _target: &str) -> bool {
        match scope {
            PermissionScope::Read => true,
            _ => self.config.mode != PermissionMode::Deny,
        }
    }

    fn record_audit(
        &self,
        tool_name: &str,
        target: &str,
        decision: &Decision,
        scope: PermissionScope,
    ) {
        if let Some(storage) = &self.audit {
            let outcome = match decision.decision {
                PermissionDecision::Allow => AuditOutcome::Allowed,
                PermissionDecision::Deny => AuditOutcome::Denied,
                PermissionDecision::Ask => AuditOutcome::Asked,
            };
            let redacted_target = if self.config.secret_redaction {
                redact_secrets(target)
            } else {
                target.to_string()
            };
            let _ = storage.write_audit(&AuditRecord {
                id: Uuid::new_v4(),
                timestamp: chrono::Utc::now(),
                subject: "permissions".into(),
                action: tool_name.into(),
                detail: serde_json::json!({
                    "target": redacted_target,
                    "scope": format!("{scope:?}")
                }),
                outcome,
            });
        }
    }
}

pub fn redact_secrets(text: &str) -> String {
    let patterns: Vec<(&str, Regex)> = [
        (
            "OPENAI_API_KEY",
            Regex::new(r"sk-[a-zA-Z0-9]{20,}").unwrap(),
        ),
        (
            "ANTHROPIC_API_KEY",
            Regex::new(r"sk-ant-[a-zA-Z0-9]{20,}").unwrap(),
        ),
        ("PASSWORD", Regex::new(r"password[=:]\s*\S+").unwrap()),
        ("TOKEN", Regex::new(r"token[=:]\s*\S+").unwrap()),
    ]
    .iter()
    .map(|(name, pat)| (*name, pat.clone()))
    .collect();
    let mut out = text.to_string();
    for (name, pat) in &patterns {
        out = pat
            .replace_all(&out, format!("[{name}_REDACTED]"))
            .to_string();
    }
    out
}

pub fn validate_permissions(cfg: &PermissionsConfig) -> anyhow::Result<()> {
    if cfg.mode == PermissionMode::Deny && cfg.allowed_tools.is_empty() {
        return Err(anyhow!(
            "permissions.mode=deny with no allowed_tools is a hard block; verify this is intended"
        ));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn redact_basic() {
        let s = "key=sk-abcdef1234567890abcdef";
        let out = redact_secrets(s);
        assert!(!out.contains("sk-abcdef1234567890abcdef"));
    }

    #[test]
    fn engine_ask_mode() {
        let mut e = PermissionEngine::new(PermissionsConfig {
            mode: PermissionMode::Ask,
            ..Default::default()
        });
        let d = e.check(PermissionScope::Read, "read_file", "src/a.rs");
        assert!(d.decision == PermissionDecision::Ask);
    }

    #[test]
    fn engine_allow_mode() {
        let mut e = PermissionEngine::new(PermissionsConfig {
            mode: PermissionMode::Allow,
            allowed_tools: vec!["read_file".into()],
            ..Default::default()
        });
        let d = e.check(PermissionScope::Read, "read_file", "src/a.rs");
        assert!(d.decision == PermissionDecision::Allow);
    }

    #[test]
    fn engine_auto_edit_mode_allows_file_scopes() {
        let mut e = PermissionEngine::new(PermissionsConfig {
            mode: PermissionMode::AutoEdit,
            ..Default::default()
        });
        assert!(e
            .check(PermissionScope::Write, "write_file", "src/a.rs")
            .is_allowed());
        assert!(!e
            .check(PermissionScope::Execute, "shell", "ls")
            .is_allowed());
        assert!(
            e.check(PermissionScope::Execute, "shell", "ls").decision == PermissionDecision::Ask
        );
    }

    #[test]
    fn redact_password() {
        let s = "password=secret123";
        let out = redact_secrets(s);
        assert!(out.contains("[PASSWORD_REDACTED]"));
        assert!(!out.contains("secret123"));
    }
}
