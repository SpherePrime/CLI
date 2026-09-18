use std::collections::HashSet;

use serde_json::Value;
use uuid::Uuid;

pub fn events(storage: &agent_storage::Storage, id: Uuid) -> Vec<Value> {
    let events = storage.read_session_events(&id);
    if !events.is_empty() {
        return events;
    }
    let messages = agent_sessions::SessionManager::resume(storage, id).unwrap_or_default();
    migrate_messages_to_events(&messages)
}

pub fn interrupted(storage: &agent_storage::Storage, id: Uuid) -> bool {
    let events = events(storage, id);
    let has_persisted_events = !storage.read_session_events(&id).is_empty();
    let mut open_turns: HashSet<String> = HashSet::new();
    let mut closed: HashSet<String> = HashSet::new();
    let mut found_turn = false;
    for event in &events {
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
    if !has_persisted_events && open_turns.len() == 1 && open_turns.contains(&String::new()) {
        return false;
    }
    open_turns.difference(&closed).count() > 0
}

fn merge_meta(base: Value, extra: Value) -> Value {
    match (base, extra) {
        (Value::Object(mut map), Value::Object(other)) => {
            map.extend(other);
            Value::Object(map)
        }
        (base, _) => base,
    }
}

fn migrate_messages_to_events(messages: &[Value]) -> Vec<Value> {
    let mut out: Vec<Value> = Vec::new();
    if messages.is_empty() {
        return out;
    }
    let turn_id = String::new();
    let protocol: u32 = 1;
    let ts = chrono::Utc::now().to_rfc3339();
    let mut sequence = 0u64;

    let push =
        |out: &mut Vec<Value>, sequence: &mut u64, kind: &str, meta_extra: serde_json::Value| {
            *sequence += 1;
            let base = serde_json::json!({
                "protocol": protocol,
                "sequence": *sequence,
                "turn_id": turn_id,
                "item_id": format!("{kind}_{sequence}"),
                "ts": ts,
            });
            out.push(serde_json::json!({
                "type": kind,
                "meta": merge_meta(base, meta_extra),
            }));
        };

    push(
        &mut out,
        &mut sequence,
        "turn_started",
        serde_json::json!({}),
    );
    for (index, msg) in messages.iter().enumerate() {
        let role = msg
            .get("role")
            .and_then(|r| r.as_str())
            .unwrap_or("assistant");
        let content = msg
            .get("content")
            .and_then(|c| c.as_str())
            .unwrap_or("")
            .to_string();
        if role == "user" {
            push(
                &mut out,
                &mut sequence,
                "user_message",
                serde_json::json!({ "text": content }),
            );
        } else if role == "assistant" {
            let item_id = format!("assistant_migrated_{index}");
            push(
                &mut out,
                &mut sequence,
                "assistant_message_started",
                serde_json::json!({ "item_id": item_id }),
            );
            if !content.is_empty() {
                push(
                    &mut out,
                    &mut sequence,
                    "text_delta",
                    serde_json::json!({ "item_id": item_id, "text": content }),
                );
            }
            push(
                &mut out,
                &mut sequence,
                "assistant_message_completed",
                serde_json::json!({ "item_id": item_id }),
            );
        }
    }
    push(
        &mut out,
        &mut sequence,
        "turn_completed",
        serde_json::json!({}),
    );
    out
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

    #[test]
    fn legacy_messages_are_migrated_to_events() {
        let (storage, id) = storage();
        let messages = vec![
            serde_json::json!({ "role": "user", "content": "hello" }),
            serde_json::json!({ "role": "assistant", "content": "hi there" }),
        ];
        let mut record = agent_storage::SessionRecord::new(None, None);
        record.id = id;
        let _ = storage.write_session(&record, &messages);
        let evts = events(&storage, id);
        let kinds: Vec<String> = evts
            .iter()
            .filter_map(|e| e.get("type").and_then(|t| t.as_str()).map(String::from))
            .collect();
        assert!(
            kinds.contains(&"turn_started".to_string()),
            "migrated events missing turn_started: {kinds:?}"
        );
        assert!(
            kinds.contains(&"user_message".to_string()),
            "migrated events missing user_message: {kinds:?}"
        );
        assert!(
            kinds.contains(&"assistant_message_started".to_string()),
            "migrated events missing assistant_message_started: {kinds:?}"
        );
        assert!(
            kinds.contains(&"text_delta".to_string()),
            "migrated events missing text_delta: {kinds:?}"
        );
        assert!(
            kinds.contains(&"turn_completed".to_string()),
            "migrated events missing turn_completed: {kinds:?}"
        );
    }

    #[test]
    fn migrated_legacy_session_is_not_interrupted() {
        let (storage, id) = storage();
        let messages = vec![
            serde_json::json!({ "role": "user", "content": "hello" }),
            serde_json::json!({ "role": "assistant", "content": "hi there" }),
        ];
        let mut record = agent_storage::SessionRecord::new(None, None);
        record.id = id;
        let _ = storage.write_session(&record, &messages);
        assert!(
            !interrupted(&storage, id),
            "migrated legacy session must not be marked interrupted"
        );
    }
}
