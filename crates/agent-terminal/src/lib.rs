use std::collections::HashSet;
use std::path::PathBuf;

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use tokio::io::AsyncReadExt;
use tokio::process::{Child, Command};
use tokio::time::{timeout, Duration};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecRequest {
    pub command: String,
    pub args: Vec<String>,
    pub cwd: Option<PathBuf>,
    pub env: Vec<(String, String)>,
    pub timeout_secs: Option<u64>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ExecOutput {
    pub exit_code: i32,
    pub stdout: String,
    pub stderr: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum ExecOutcome {
    Completed(ExecOutput),
    TimedOut,
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
        let limit = req.timeout_secs.unwrap_or(60);
        match self.run_with_timeout(req, limit).await? {
            ExecOutcome::Completed(output) => Ok(output),
            ExecOutcome::TimedOut => anyhow::bail!("command timed out after {limit}s"),
        }
    }

    pub async fn run_with_timeout(
        &self,
        req: &ExecRequest,
        timeout_secs: u64,
    ) -> Result<ExecOutcome> {
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
        cmd.stdout(std::process::Stdio::piped());
        cmd.stderr(std::process::Stdio::piped());
        let mut child = cmd
            .spawn()
            .with_context(|| format!("spawning {}", req.command))?;

        let run = async {
            let stdout = child.stdout.take();
            let stderr = child.stderr.take();
            let mut out_bytes = Vec::new();
            let mut err_bytes = Vec::new();
            if let Some(mut reader) = stdout {
                let _ = reader.read_to_end(&mut out_bytes).await;
            }
            if let Some(mut reader) = stderr {
                let _ = reader.read_to_end(&mut err_bytes).await;
            }
            let status = child.wait().await;
            (out_bytes, err_bytes, status)
        };

        match timeout(Duration::from_secs(timeout_secs.max(1)), run).await {
            Ok((out_bytes, err_bytes, status)) => {
                let exit_code = status
                    .map(|status| status.code().unwrap_or(-1))
                    .unwrap_or(-1);
                Ok(ExecOutcome::Completed(ExecOutput {
                    exit_code,
                    stdout: String::from_utf8_lossy(&out_bytes).into_owned(),
                    stderr: String::from_utf8_lossy(&err_bytes).into_owned(),
                }))
            }
            Err(_) => {
                kill_process_tree(&mut child).await;
                Ok(ExecOutcome::TimedOut)
            }
        }
    }
}

async fn kill_process_tree(child: &mut Child) {
    child.start_kill().ok();
    #[cfg(windows)]
    {
        if let Some(pid) = child.id() {
            let _ = Command::new("taskkill")
                .args(["/PID", &pid.to_string(), "/T", "/F"])
                .output()
                .await;
        }
    }
    let _ = child.wait().await;
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

    #[tokio::test]
    async fn timeout_kills_long_running_command() {
        let r = ShellRunner::new();
        let outcome = r
            .run_with_timeout(
                &ExecRequest {
                    command: "cmd".into(),
                    args: vec!["/c".into(), "ping -n 8 127.0.0.1 > nul".into()],
                    cwd: None,
                    env: vec![],
                    timeout_secs: Some(1),
                },
                1,
            )
            .await
            .unwrap();
        assert_eq!(outcome, ExecOutcome::TimedOut);
    }

    #[tokio::test]
    async fn completes_before_timeout() {
        let r = ShellRunner::new();
        let outcome = r
            .run_with_timeout(
                &ExecRequest {
                    command: "cmd".into(),
                    args: vec!["/c".into(), "echo done".into()],
                    cwd: None,
                    env: vec![],
                    timeout_secs: Some(1),
                },
                1,
            )
            .await
            .unwrap();
        match outcome {
            ExecOutcome::Completed(output) => {
                assert_eq!(output.exit_code, 0);
                assert!(output.stdout.contains("done"));
            }
            ExecOutcome::TimedOut => panic!("fast command must not time out"),
        }
    }
}
