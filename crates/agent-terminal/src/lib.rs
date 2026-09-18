use std::collections::HashSet;
use std::path::PathBuf;

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use tokio::process::Command;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecRequest {
    pub command: String,
    pub args: Vec<String>,
    pub cwd: Option<PathBuf>,
    pub env: Vec<(String, String)>,
    pub timeout_secs: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecOutput {
    pub exit_code: i32,
    pub stdout: String,
    pub stderr: String,
}

pub const PROTECTED_ENV: [&str; 8] = [
    "AGENT_API_KEY",
    "OPENAI_API_KEY",
    "ANTHROPIC_API_KEY",
    "AGENT_TOKEN",
    "SSH_AUTH_SOCK",
    "AWS_ACCESS_KEY_ID",
    "AWS_SECRET_ACCESS_KEY",
    "DATABASE_URL",
];

pub struct ShellRunner {
    protected_env: HashSet<String>,
}

impl ShellRunner {
    pub fn new() -> Self {
        let protected: HashSet<String> = PROTECTED_ENV.iter().map(|s| s.to_string()).collect();
        Self {
            protected_env: protected,
        }
    }

    pub async fn run(&self, req: &ExecRequest) -> Result<ExecOutput> {
        self.run_with_timeout(req, req.timeout_secs.unwrap_or(60))
            .await
    }

    pub async fn run_with_timeout(&self, req: &ExecRequest, timeout: u64) -> Result<ExecOutput> {
        let mut cmd = Command::new(req.command.clone());
        cmd.args(&req.args);
        if let Some(cwd) = &req.cwd {
            cmd.current_dir(cwd);
        }
        for (k, v) in &req.env {
            if self.protected_env.contains(k.as_str()) {
                tracing::warn!(key = k.as_str(), "refusing to inject protected env var");
                continue;
            }
            cmd.env(k, v);
        }
        let child = cmd
            .output()
            .await
            .with_context(|| format!("spawning {}", req.command))?;
        let _ = timeout;
        Ok(ExecOutput {
            exit_code: child.status.code().unwrap_or(-1),
            stdout: String::from_utf8_lossy(&child.stdout).into_owned(),
            stderr: String::from_utf8_lossy(&child.stderr).into_owned(),
        })
    }
}

impl Default for ShellRunner {
    fn default() -> Self {
        Self::new()
    }
}

pub async fn which(name: &str) -> Result<PathBuf> {
    which::which(name).with_context(|| format!("command not found: {name}"))
}

pub async fn validate_command(name: &str) -> Result<()> {
    which(name).await.map(|_| ())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn run_echo() {
        let r = ShellRunner::new();
        let out = r
            .run(&ExecRequest {
                command: "cmd".into(),
                args: vec!["/c".into(), "echo".into(), "hi".into()],
                cwd: None,
                env: vec![],
                timeout_secs: None,
            })
            .await
            .unwrap();
        assert_eq!(out.exit_code, 0);
        assert!(out.stdout.contains("hi"));
    }

    #[tokio::test]
    async fn protected_env_skipped() {
        let r = ShellRunner::new();
        let out = r
            .run(&ExecRequest {
                command: "cmd".into(),
                args: vec!["/c".into(), "set".into()],
                cwd: None,
                env: vec![("AGENT_API_KEY".into(), "secret".into())],
                timeout_secs: None,
            })
            .await
            .unwrap();
        assert!(!out.stdout.contains("secret"));
    }
}
