use agent_storage::SessionRecord;
use anyhow::{Context, Result};
use uuid::Uuid;

pub struct SessionManager {
    pub id: Uuid,
    pub storage: agent_storage::Storage,
}

impl SessionManager {
    pub fn new(storage: agent_storage::Storage, id: Uuid) -> Self {
        Self { id, storage }
    }

    pub fn latest(storage: &agent_storage::Storage) -> Option<Uuid> {
        let dir = storage.sessions_dir();
        let mut entries: Vec<(chrono::DateTime<chrono::Utc>, Uuid)> = Vec::new();
        if let Ok(rd) = std::fs::read_dir(&dir) {
            for entry in rd.flatten() {
                let fname = entry.file_name();
                let fname_str = match fname.to_str() {
                    Some(s) => s,
                    None => continue,
                };
                let dot = match fname_str.rfind('.') {
                    Some(d) => d,
                    None => continue,
                };
                let id = match Uuid::parse_str(&fname_str[..dot]) {
                    Ok(id) => id,
                    Err(_) => continue,
                };
                let modified = entry.metadata().ok()?.modified().ok()?;
                let ts: chrono::DateTime<chrono::Utc> = modified.into();
                entries.push((ts, id));
            }
        }
        entries.sort();
        entries.pop().map(|(_, id)| id)
    }

    pub fn resume(storage: &agent_storage::Storage, id: Uuid) -> Result<Vec<serde_json::Value>> {
        let p = storage.sessions_dir().join(format!("{}.jsonl", id));
        let content = std::fs::read_to_string(&p)
            .with_context(|| format!("reading session at {}", p.display()))?;
        let mut messages = Vec::new();
        for line in content.lines() {
            let trimmed = line.trim();
            if trimmed.is_empty() {
                continue;
            }
            let v: serde_json::Value =
                serde_json::from_str(trimmed).context("malformed JSONL line")?;
            if v.get("type").and_then(|t| t.as_str()) == Some("message") {
                if let Some(msg) = v.get("message") {
                    messages.push(msg.clone());
                }
            }
        }
        Ok(messages)
    }

    pub fn save(&self, record: &SessionRecord, messages: &[serde_json::Value]) -> Result<()> {
        self.storage.write_session(record, messages)
    }

    pub fn list(&self) -> Result<Vec<Uuid>> {
        let dir = self.storage.sessions_dir();
        let mut ids = Vec::new();
        for entry in std::fs::read_dir(&dir)?.flatten() {
            let fname = entry.file_name();
            let s = fname.to_str().unwrap_or_default();
            if let Some(stem) = s.strip_suffix(".jsonl") {
                if let Ok(id) = Uuid::parse_str(stem) {
                    ids.push(id);
                }
            }
        }
        Ok(ids)
    }
}

impl Clone for SessionManager {
    fn clone(&self) -> Self {
        Self {
            id: self.id,
            storage: self.storage.clone(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn session_save_and_list() {
        let tmp = tempfile::tempdir().unwrap();
        let s = agent_storage::Storage::new(tmp.path().to_path_buf());
        let rec = SessionRecord::new(None, None);
        let mgr = SessionManager::new(s.clone(), rec.id);
        mgr.save(&rec, &[]).unwrap();
        let ids = mgr.list().unwrap();
        assert!(ids.contains(&rec.id));
    }
}
