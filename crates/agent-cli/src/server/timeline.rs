use std::collections::HashSet;

use serde_json::Value;
use uuid::Uuid;

pub fn events(storage: &agent_storage::Storage, id: Uuid) -> Vec<Value> {
    storage.read_session_events(&id)
}

pub fn interrupted(storage: &agent_storage::Storage, id: Uuid) -> bool {
    let mut open_turns: HashSet<String> = HashSet::new();
    let mut closed: HashSet<String> = HashSet::new();
    let mut found_turn = false;
    for event in events(storage, id) {
        let kind = event.get("type").and_then(|t| t.as_str()).unwrap_or("");
        let turn_id = event.get("meta").and_then(|m| m.get("turn_id"));
        let turn = turn_id
            .and_then(|t| t.as_str())
            .unwrap_or_default()
            .to_string();
        match kind {
            "turn_started" => {
                found_turn = true;
                open_turns.insert(turn);
            }
            "turn_completed" | "turn_cancelled" => {
                closed.insert(turn);
            }
            _ => {}
        }
    }
    if !found_turn {
        return false;
    }
    open_turns.difference(&closed).count() > 0
}

#[cfg(test)]
mod tests {
    use super::*;
    use agent_storage::Storage;
    use uuid::Uuid;

    fn storage() -> (Storage, Uuid) {
        let tmp = tempfile::tempdir().unwrap();
        let storage = Storage::new(tmp.path().join(".agent"));
        let id = Uuid::new_v4();
        let mut record = agent_storage::SessionRecord::new(None, None);
        record.id = id;
        let _ = storage.write_session(&record, &[]);
        (storage, id)
    }

    fn push(storage: &Storage, id: &Uuid, kind: &str) {
        let event = serde_json::json!({
            "type": kind,
            "meta": { "turn_id": "turn_0", "sequence": 1 }
        });
        let _ = storage.append_session_event(id, &event);
    }

    #[test]
    fn empty_session_is_not_interrupted() {
        let (storage, id) = storage();
        assert!(!interrupted(&storage, id));
    }

    #[test]
    fn closed_turn_is_not_interrupted() {
        let (storage, id) = storage();
        push(&storage, &id, "turn_started");
        push(&storage, &id, "assistant_message_started");
        push(&storage, &id, "turn_completed");
        assert!(!interrupted(&storage, id));
    }

    #[test]
    fn unclosed_turn_is_interrupted() {
        let (storage, id) = storage();
        push(&storage, &id, "turn_started");
        push(&storage, &id, "assistant_message_started");
        assert!(interrupted(&storage, id));
    }

    #[test]
    fn timeline_returns_persisted_events() {
        let (storage, id) = storage();
        push(&storage, &id, "turn_started");
        push(&storage, &id, "turn_completed");
        let kinds: Vec<String> = events(&storage, id)
            .iter()
            .filter_map(|e| e.get("type").and_then(|t| t.as_str()).map(String::from))
            .collect();
        assert_eq!(
            kinds,
            vec!["turn_started".to_string(), "turn_completed".to_string()]
        );
    }
}
