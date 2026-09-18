use std::path::PathBuf;

use agent_terminal::{ExecRequest, ShellRunner};
use anyhow::{anyhow, Context, Result};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GitStatus {
    pub branch: String,
    pub staged: Vec<String>,
    pub unstaged: Vec<String>,
    pub untracked: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GitLogEntry {
    pub hash: String,
    pub author: String,
    pub date: String,
    pub subject: String,
}

pub struct GitRepo {
    runner: ShellRunner,
    pub root: PathBuf,
}

impl GitRepo {
    pub fn new(root: PathBuf) -> Self {
        Self {
            runner: ShellRunner::new(),
            root,
        }
    }

    async fn git(&self, args: &[&str]) -> Result<String> {
        let req = ExecRequest {
            command: "git".into(),
            args: args.iter().map(|s| s.to_string()).collect(),
            cwd: Some(self.root.clone()),
            env: vec![],
            timeout_secs: Some(30),
        };
        let out = self
            .runner
            .run(&req)
            .await
            .with_context(|| format!("git {}", args.join(" ")))?;
        if out.exit_code != 0 {
            return Err(anyhow!(
                "git {} failed: {}",
                args.join(" "),
                out.stderr.trim()
            ));
        }
        Ok(out.stdout)
    }

    pub async fn command(&self, args: &[&str]) -> Result<String> {
        self.git(args).await
    }

    pub async fn status(&self) -> Result<GitStatus> {
        let raw = self.git(&["status", "--porcelain"]).await?;
        let mut status = GitStatus {
            branch: self
                .git(&["rev-parse", "--abbrev-ref", "HEAD"])
                .await?
                .trim()
                .to_string(),
            staged: vec![],
            unstaged: vec![],
            untracked: vec![],
        };
        for line in raw.lines() {
            if line.is_empty() {
                continue;
            }
            let idx = line.chars().take(2).collect::<String>();
            let path = line.chars().skip(3).collect::<String>();
            match idx.as_str() {
                "A " | "M " | "D " | "R " | "C " => status.staged.push(path),
                " M" | " D" => status.unstaged.push(path),
                "?? " => status.untracked.push(path),
                _ => status.unstaged.push(path),
            }
        }
        Ok(status)
    }

    pub async fn diff(&self, staged: bool) -> Result<String> {
        let mut args = vec!["diff".to_string()];
        if staged {
            args.push("--staged".to_string());
        }
        let arg_refs: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
        self.git(&arg_refs).await
    }

    pub async fn log(&self, n: usize) -> Result<Vec<GitLogEntry>> {
        let out = self
            .git(&["log", "--pretty=format:%H|%an|%ad|%s", "-n", &n.to_string()])
            .await?;
        let mut entries = Vec::new();
        for line in out.lines() {
            let parts: Vec<&str> = line.splitn(4, '|').collect();
            if parts.len() == 4 {
                entries.push(GitLogEntry {
                    hash: parts[0].to_string(),
                    author: parts[1].to_string(),
                    date: parts[2].to_string(),
                    subject: parts[3].to_string(),
                });
            }
        }
        Ok(entries)
    }

    pub async fn branches(&self) -> Result<Vec<String>> {
        let out = self.git(&["branch", "-a"]).await?;
        Ok(out
            .lines()
            .map(|l| l.trim().trim_start_matches("* ").to_string())
            .collect())
    }

    pub async fn checkout(&self, branch: &str) -> Result<()> {
        self.git(&["checkout", branch]).await?;
        Ok(())
    }

    pub async fn add(&self, paths: &[&str]) -> Result<()> {
        let mut args: Vec<&str> = vec!["add"];
        args.extend_from_slice(paths);
        self.git(&args).await?;
        Ok(())
    }

    pub async fn commit(&self, message: &str, allow_empty: bool) -> Result<()> {
        let mut args = vec!["commit", "-m", message];
        if allow_empty {
            args.push("--allow-empty");
        }
        let refs: Vec<&str> = args.iter().map(|s| s.as_ref()).collect();
        self.git(&refs).await?;
        Ok(())
    }

    pub async fn is_repo(&self) -> bool {
        let out = self
            .runner
            .run(&ExecRequest {
                command: "git".into(),
                args: vec!["rev-parse".to_string(), "--is-inside-work-tree".to_string()],
                cwd: Some(self.root.clone()),
                env: vec![],
                timeout_secs: Some(5),
            })
            .await;
        matches!(out, Ok(o) if o.exit_code == 0)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn git_repo_check() {
        let tmp = tempfile::tempdir().unwrap();
        let repo = GitRepo::new(tmp.path().to_path_buf());
        assert!(!repo.is_repo().await);
    }
}
