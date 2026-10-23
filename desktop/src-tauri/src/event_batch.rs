use serde_json::Value;
use std::time::{Duration, Instant};

const BYTE_LIMIT: usize = 8 * 1024 * 1024;
const DELAY: Duration = Duration::from_millis(75);

#[derive(Default)]
pub struct EventBatch {
    pending: Vec<(String, String, Value, usize)>,
    bytes: usize,
    started: Option<Instant>,
}

impl EventBatch {
    pub fn push(&mut self, kind: &str, payload: Value, bytes: usize, now: Instant) -> Vec<(String, Value)> {
        let id = if kind == "message" && payload["action"] == "updated" {
            payload["data"]["id"].as_str().filter(|id| !id.is_empty()).map(str::to_owned)
        } else { None };
        let mut output = self.due(now);
        let Some(id) = id else {
            output.extend(self.flush());
            output.push((kind.into(), payload));
            return output;
        };
        let previous = self.pending.iter().find(|entry| entry.0 == id).map(|entry| entry.3).unwrap_or(0);
        if self.bytes.saturating_sub(previous).saturating_add(bytes) > BYTE_LIMIT || self.pending.len() >= 64 {
            output.extend(self.flush());
        }
        if bytes > BYTE_LIMIT {
            output.push((kind.into(), payload));
            return output;
        }
        if let Some(entry) = self.pending.iter_mut().find(|entry| entry.0 == id) {
            self.bytes -= entry.3;
            *entry = (id, kind.into(), payload, bytes);
        } else {
            self.pending.push((id, kind.into(), payload, bytes));
        }
        self.bytes += bytes;
        self.started.get_or_insert(now);
        output
    }

    pub fn due(&mut self, now: Instant) -> Vec<(String, Value)> {
        if self.started.is_some_and(|start| now.saturating_duration_since(start) >= DELAY) { self.flush() } else { Vec::new() }
    }

    pub fn flush(&mut self) -> Vec<(String, Value)> {
        self.bytes = 0;
        self.started = None;
        std::mem::take(&mut self.pending).into_iter().map(|(_,kind,payload,_)|(kind,payload)).collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;
    #[test]
    fn replaces_stream_snapshots_and_flushes_before_other_events() {
        let mut batch=EventBatch::default();let now=Instant::now();
        assert_eq!(batch.push("message",json!({"action":"created","data":{"id":"one"}}),100,now).len(),1);
        for token in 0..10000 {assert!(batch.push("message",json!({"action":"updated","data":{"id":"one","token":token}}),200,now).is_empty());}
        assert_eq!(batch.pending.len(),1);assert_eq!(batch.bytes,200);
        let output=batch.push("permission",json!({"request":"approval"}),50,now);
        assert_eq!(output.len(),2);assert_eq!(output[0].1["data"]["token"],9999);assert_eq!(output[1].0,"permission");
    }
    #[test]
    fn flushes_idle_and_bounds_retained_bytes() {
        let mut batch=EventBatch::default();let now=Instant::now();
        batch.push("message",json!({"action":"updated","data":{"id":"one"}}),BYTE_LIMIT,now);
        assert_eq!(batch.push("message",json!({"action":"updated","data":{"id":"two"}}),1,now).len(),1);
        assert_eq!(batch.due(now+DELAY).len(),1);assert_eq!(batch.bytes,0);
    }
}
