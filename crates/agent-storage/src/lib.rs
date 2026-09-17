use std::path::PathBuf;

use anyhow::{Context, Result};
use chrono::Utc;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum StorageScope {
    Global,
    Project,
    Session,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionRecord {
    pub id: Uuid,
    pub created_at: chrono::DateTime<Utc>,
    pub updated_at: chrono::DateTime<Utc>,
    pub project_path: Option<PathBuf>,
    pub model: Option<String>,
    pub metadata: serde_json::Value,
}

impl SessionRecord {
    pub fn new(project_path: Option<PathBuf>, model: Option<String>) -> Self {
        let now = Utc::now();
        Self {
            id: Uuid::new_v4(),
            created_at: now,
            updated_at: now,
            project_path,
            model,
            metadata: serde_json::json!({}),
        }
    }

    pub fn touch(&mut self) {
        self.updated_at = Utc::now();
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuditRecord {
    pub id: Uuid,
    pub timestamp: chrono::DateTime<Utc>,
    pub subject: String,
    pub action: String,
    pub detail: serde_json::Value,
    pub outcome: AuditOutcome,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum AuditOutcome {
    Allowed,
    Denied,
    Asked,
    TimedOut,
}

pub struct Storage {
    root: PathBuf,
}

impl Clone for Storage {
    fn clone(&self) -> Self {
        Self { root: self.root.clone() }
    }
}

impl std::fmt::Debug for Storage {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Storage").field("root", &self.root).finish()
    }
}

impl serde::Serialize for Storage {
    fn serialize<S: serde::Serializer>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error> {
        serializer.serialize_str(&self.root.to_string_lossy())
    }
}

impl<'de> serde::Deserialize<'de> for Storage {
    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> std::result::Result<Self, D::Error> {
        let s = String::deserialize(deserializer)?;
        Ok(Self::new(std::path::PathBuf::from(s)))
    }
}

impl Storage {
    pub fn new(root: PathBuf) -> Self {
        let _ = std::fs::create_dir_all(&root);
        Self { root }
    }

    pub fn global() -> Result<Self> {
        let dir = dirs::home_dir()
            .context("cannot determine home dir")?
            .join(".agent");
        Ok(Self::new(dir))
    }

    pub fn root(&self) -> &PathBuf {
        &self.root
    }

    pub fn sessions_dir(&self) -> PathBuf {
        let p = self.root.join("sessions");
        let _ = std::fs::create_dir_all(&p);
        p
    }

    pub fn audit_dir(&self) -> PathBuf {
        let p = self.root.join("audit");
        let _ = std::fs::create_dir_all(&p);
        p
    }

    pub fn snapshots_dir(&self, session_id: &Uuid) -> PathBuf {
        let p = self.sessions_dir().join(session_id.to_string()).join("snapshots");
        let _ = std::fs::create_dir_all(&p);
        p
    }

    pub fn write_audit(&self, record: &AuditRecord) -> Result<()> {
        let p = self.audit_dir().join(format!("{}.jsonl", record.timestamp.format("%Y-%m-%d")));
        let line = serde_json::json!({ "record": record });
        let mut raw = std::fs::read_to_string(&p).unwrap_or_default();
        raw.push_str(&serde_json::to_string_pretty(&line)?);
        raw.push('\n');
        std::fs::write(&p, raw).with_context(|| format!("writing audit to {}", p.display()))?;
        Ok(())
    }

    pub fn write_session(&self, record: &SessionRecord, messages: &[serde_json::Value]) -> Result<()> {
        let p = self.sessions_dir().join(format!("{}.jsonl", record.id));
        let meta = serde_json::json!({ "type": "meta", "session": record });
        let lines = messages
            .iter()
            .map(|m| serde_json::json!({ "type": "message", "message": m }))
            .collect::<Vec<_>>();
        let mut out = String::new();
        out.push_str(&serde_json::to_string(&meta)?);
        out.push('\n');
        for l in lines {
            out.push_str(&serde_json::to_string(&l)?);
            out.push('\n');
        }
        std::fs::write(&p, out).with_context(|| format!("writing session {}", p.display()))?;
        Ok(())
    }

    pub fn read_session(&self, id: &Uuid) -> Result<SessionRecord> {
        let p = self.sessions_dir().join(format!("{}.jsonl", id));
        let content = std::fs::read_to_string(&p)
            .with_context(|| format!("reading session {}", p.display()))?;
        let first = content
            .lines()
            .find(|l| !l.trim().is_empty())
            .context("empty session file")?;
        let v: serde_json::Value = serde_json::from_str(first).context("malformed session")?;
        let record: SessionRecord =
            serde_json::from_value(v.get("session").cloned().unwrap_or_default())
                .context("invalid session record")?;
        Ok(record)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sessions_round_trip() {
        let tmp = tempfile::tempdir().unwrap();
        let s = Storage::new(tmp.path().to_path_buf());
        let mut rec = SessionRecord::new(None, Some(String::from("gpt-4")));
        rec.touch();
        let msgs = vec![serde_json::json!({"role": "user", "content": "hi"})];
        s.write_session(&rec, &msgs).unwrap();
        let read = s.read_session(&rec.id).unwrap();
        assert_eq!(read.id, rec.id);
    }
}
