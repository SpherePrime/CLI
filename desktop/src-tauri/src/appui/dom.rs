use std::net::TcpStream;
use std::time::Duration;

use serde_json::{json, Value};
use tungstenite::stream::MaybeTlsStream;
use tungstenite::{connect as ws_connect, Message, WebSocket};

use super::types::{Confidence, Provider, UiBounds, UiNode, UiSnapshot};
use super::uia::action::UiAction;

const HTTP_TIMEOUT: Duration = Duration::from_secs(5);

pub struct DomSession {
    socket: WebSocket<MaybeTlsStream<TcpStream>>,
    next: u64,
}

pub fn connect(port: u16) -> Result<DomSession, String> {
    let client = reqwest::blocking::Client::builder()
        .timeout(HTTP_TIMEOUT)
        .build()
        .map_err(|error| error.to_string())?;
    let targets: Value = client
        .get(format!("http://127.0.0.1:{port}/json/list"))
        .send()
        .map_err(|error| error.to_string())?
        .text()
        .map_err(|error| error.to_string())
        .and_then(|text| serde_json::from_str(&text).map_err(|error| error.to_string()))?;
    let pages = targets
        .as_array()
        .ok_or("The debug endpoint returned no targets")?;
    let page = pages
        .iter()
        .find(|target| target["type"] == "page" && target["webSocketDebuggerUrl"].is_string())
        .ok_or("The application exposes no debuggable page")?;
    let url = page["webSocketDebuggerUrl"]
        .as_str()
        .ok_or("The debug endpoint returned no socket address")?;
    let (socket, _) = ws_connect(url).map_err(|error| error.to_string())?;
    Ok(DomSession { socket, next: 0 })
}

impl DomSession {
    fn call(&mut self, method: &str, params: Value) -> Result<Value, String> {
        self.next += 1;
        let id = self.next;
        let message = json!({"id": id, "method": method, "params": params});
        self.socket
            .send(Message::Text(message.to_string().into()))
            .map_err(|error| error.to_string())?;
        loop {
            let incoming = self.socket.read().map_err(|error| error.to_string())?;
            let text = match incoming {
                Message::Text(text) => text.to_string(),
                Message::Binary(bytes) => String::from_utf8_lossy(&bytes).to_string(),
                _ => continue,
            };
            let value: Value = match serde_json::from_str(&text) {
                Ok(value) => value,
                Err(_) => continue,
            };
            if value["id"].as_u64() != Some(id) {
                continue;
            }
            if let Some(error) = value.get("error") {
                return Err(error.to_string());
            }
            return Ok(value["result"].clone());
        }
    }

    pub fn evaluate(&mut self, expression: &str) -> Result<Value, String> {
        let result = self.call(
            "Runtime.evaluate",
            json!({"expression": expression, "returnByValue": true, "awaitPromise": true}),
        )?;
        if let Some(exception) = result.get("exceptionDetails") {
            return Err(format!("The page rejected the script: {exception}"));
        }
        Ok(result["result"]["value"].clone())
    }

    pub fn snapshot(&mut self) -> Result<UiSnapshot, String> {
        let value = self.evaluate(include_str!("dom_snapshot.js"))?;
        from_dom(&value)
    }

    pub fn perform(
        &mut self,
        reference: &str,
        action: &UiAction,
        text: Option<&str>,
    ) -> Result<(), String> {
        let selector = format!("[data-prime-ref=\"{reference}\"]");
        let script = match action {
            UiAction::Invoke => format!(
                "(() => {{ const e = document.querySelector('{selector}'); if (!e) return 'missing'; e.scrollIntoView({{block:'center'}}); e.click(); return 'ok' }})()"
            ),
            UiAction::Focus => format!(
                "(() => {{ const e = document.querySelector('{selector}'); if (!e) return 'missing'; e.focus(); return 'ok' }})()"
            ),
            UiAction::Toggle => format!(
                "(() => {{ const e = document.querySelector('{selector}'); if (!e) return 'missing'; e.click(); return 'ok' }})()"
            ),
            UiAction::SetValue(_) => {
                let value = serde_json::to_string(text.unwrap_or_default())
                    .map_err(|error| error.to_string())?;
                format!(
                    "(() => {{ const e = document.querySelector('{selector}'); if (!e) return 'missing'; e.focus(); e.value = {value}; e.dispatchEvent(new Event('input', {{bubbles:true}})); e.dispatchEvent(new Event('change', {{bubbles:true}})); return 'ok' }})()"
                )
            }
            _ => return Err("This action is not available for a page element".to_owned()),
        };
        if self.evaluate(&script)?.as_str() == Some("missing") {
            return Err("The element is no longer available; take a new snapshot".to_owned());
        }
        Ok(())
    }
}

pub fn from_dom(value: &Value) -> Result<UiSnapshot, String> {
    let nodes = value["nodes"]
        .as_array()
        .ok_or("The page returned no elements")?;
    let mut collected = Vec::new();
    for node in nodes {
        collected.push(convert(node, None, "")?);
    }
    let count = count_nodes(&collected);
    let mut snapshot = UiSnapshot {
        provider: Provider::Dom,
        hwnd: value["url"].as_str().unwrap_or_default().to_owned(),
        title: value["title"].as_str().unwrap_or_default().to_owned(),
        captured_at: 0,
        truncated: value["truncated"].as_bool().unwrap_or(false),
        warnings: Vec::new(),
        nodes: collected,
        count,
    };
    if count <= 1 {
        snapshot
            .warnings
            .push("The page exposes no interactive elements".to_owned());
    }
    Ok(snapshot)
}

fn convert(value: &Value, parent: Option<&str>, parent_path: &str) -> Result<UiNode, String> {
    let role = value["role"].as_str().unwrap_or("group").to_owned();
    let name = value["name"].as_str().unwrap_or_default().to_owned();
    let path = if parent_path.is_empty() {
        format!("{role}[{name}]")
    } else {
        format!("{parent_path}/{role}[{name}]")
    };
    let reference = value["ref"]
        .as_str()
        .map(str::to_owned)
        .unwrap_or_else(|| format!("{path}"));
    let mut children = Vec::new();
    for child in value["children"].as_array().into_iter().flatten() {
        children.push(convert(child, Some(&reference), &path)?);
    }
    Ok(UiNode {
        reference,
        parent: parent.map(str::to_owned),
        role,
        name,
        value: value["value"].as_str().map(str::to_owned),
        bounds: UiBounds {
            x: value["bounds"]["x"].as_i64().unwrap_or(0) as i32,
            y: value["bounds"]["y"].as_i64().unwrap_or(0) as i32,
            width: value["bounds"]["width"].as_i64().unwrap_or(0) as i32,
            height: value["bounds"]["height"].as_i64().unwrap_or(0) as i32,
        },
        states: value["states"]
            .as_array()
            .into_iter()
            .flatten()
            .filter_map(|state| state.as_str().map(str::to_owned))
            .collect(),
        actions: value["actions"]
            .as_array()
            .into_iter()
            .flatten()
            .filter_map(|action| action.as_str().map(str::to_owned))
            .collect(),
        automation_id: value["automationId"].as_str().map(str::to_owned),
        class_name: value["className"].as_str().map(str::to_owned),
        path,
        source: Provider::Dom,
        confidence: Confidence::High,
        children,
    })
}

fn count_nodes(nodes: &[UiNode]) -> usize {
    nodes
        .iter()
        .map(|node| 1 + count_nodes(&node.children))
        .sum()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn page() -> Value {
        json!({
            "title": "Fixture page",
            "url": "http://127.0.0.1/fixture",
            "truncated": false,
            "nodes": [{
                "ref": null,
                "role": "group",
                "name": "",
                "value": null,
                "bounds": {"x": 0, "y": 0, "width": 400, "height": 300},
                "states": [],
                "actions": [],
                "automationId": null,
                "className": "app",
                "path": "group[]",
                "children": [
                    {
                        "ref": "n1",
                        "role": "button",
                        "name": "Save",
                        "value": null,
                        "bounds": {"x": 10, "y": 20, "width": 80, "height": 24},
                        "states": ["enabled"],
                        "actions": ["invoke"],
                        "automationId": "save",
                        "className": "primary",
                        "path": "button[Save]",
                        "children": []
                    },
                    {
                        "ref": "n2",
                        "role": "edit",
                        "name": "Name",
                        "value": "Иван",
                        "bounds": {"x": 10, "y": 60, "width": 200, "height": 24},
                        "states": ["enabled", "focused"],
                        "actions": ["setValue"],
                        "automationId": null,
                        "className": null,
                        "path": "edit[Name]",
                        "children": []
                    }
                ]
            }]
        })
    }

    #[test]
    fn converts_a_page_into_the_shared_graph() {
        let snapshot = from_dom(&page()).unwrap();
        assert_eq!(snapshot.provider, Provider::Dom);
        assert_eq!(snapshot.title, "Fixture page");
        assert_eq!(snapshot.count, 3);
        assert!(snapshot.warnings.is_empty());
        let group = &snapshot.nodes[0];
        assert_eq!(group.children.len(), 2);
        let button = &group.children[0];
        assert_eq!(button.reference, "n1");
        assert_eq!(button.role, "button");
        assert_eq!(button.actions, vec!["invoke"]);
        assert_eq!(button.parent.as_deref(), Some("group[]"));
        assert_eq!(button.path, "group[]/button[Save]");
        assert_eq!(button.bounds.width, 80);
        let edit = &group.children[1];
        assert_eq!(edit.value.as_deref(), Some("Иван"));
        assert_eq!(edit.states, vec!["enabled", "focused"]);
    }

    #[test]
    fn an_empty_page_is_reported_instead_of_succeeding_silently() {
        let empty = json!({"title": "Empty", "url": "http://127.0.0.1/empty", "nodes": []});
        let snapshot = from_dom(&empty).unwrap();
        assert_eq!(snapshot.count, 0);
        assert!(!snapshot.warnings.is_empty());
    }

    #[test]
    fn the_shared_renderer_accepts_a_page_snapshot() {
        let snapshot = from_dom(&page()).unwrap();
        let text = crate::appui::render::snapshot_text(&snapshot);
        assert!(text.contains("n1 button \"Save\" [enabled]"), "{text}");
        assert!(text.contains("actions=invoke"), "{text}");
        assert!(text.contains("[dom,"), "{text}");
    }
}
