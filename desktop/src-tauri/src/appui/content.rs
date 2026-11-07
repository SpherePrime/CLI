use serde_json::{json, Map, Value};

const SECTION_KEYS: [&str; 2] = ["text", "changes"];
const IMAGE_PREFIX: &str = "data:image/png;base64,";

pub fn mcp_content(value: Value) -> Value {
    match value {
        Value::Array(items) => Value::Array(items),
        Value::String(text) => json!([{ "type": "text", "text": text }]),
        Value::Object(object) => object_content(object),
        other => json!([{ "type": "text", "text": other.to_string() }]),
    }
}

fn object_content(mut object: Map<String, Value>) -> Value {
    let image = match object.get("image").and_then(Value::as_str) {
        Some(data) if data.starts_with(IMAGE_PREFIX) => {
            let encoded = data[IMAGE_PREFIX.len()..].to_owned();
            object.remove("image");
            Some(encoded)
        }
        _ => None,
    };
    let mut sections = Vec::new();
    for key in SECTION_KEYS {
        if let Some(Value::String(section)) = object.remove(key) {
            if !section.is_empty() {
                sections.push((key, section));
            }
        }
    }
    let mut parts = Vec::new();
    if !object.is_empty() {
        parts.push(Value::Object(object).to_string());
    }
    for (key, section) in sections {
        if key == "changes" {
            parts.push(format!("Changes:\n{section}"));
        } else {
            parts.push(section);
        }
    }
    let mut content = vec![json!({ "type": "text", "text": parts.join("\n\n") })];
    if let Some(data) = image {
        content.push(json!({ "type": "image", "mimeType": "image/png", "data": data }));
    }
    Value::Array(content)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn objects_become_text_content() {
        let content = mcp_content(json!({"count": 3, "text": "group[A]\n  button[Save]"}));
        let items = content.as_array().unwrap();
        assert_eq!(items.len(), 1);
        assert_eq!(items[0]["type"], "text");
        assert_eq!(
            items[0]["text"],
            "{\"count\":3}\n\ngroup[A]\n  button[Save]"
        );
    }

    #[test]
    fn changes_and_images_are_kept_readable() {
        let content = mcp_content(
            json!({"ok": true, "changes": "button[Save] appeared", "image": "data:image/png;base64,AAAA"}),
        );
        let items = content.as_array().unwrap();
        assert_eq!(
            items[0]["text"],
            "{\"ok\":true}\n\nChanges:\nbutton[Save] appeared"
        );
        assert_eq!(items[1]["type"], "image");
        assert_eq!(items[1]["data"], "AAAA");
    }

    #[test]
    fn content_arrays_and_strings_pass_through() {
        let existing = json!([{ "type": "text", "text": "done" }]);
        assert_eq!(mcp_content(existing.clone()), existing);
        assert_eq!(
            mcp_content(json!("plain")),
            json!([{ "type": "text", "text": "plain" }])
        );
        assert_eq!(mcp_content(json!(null))[0]["text"], "null");
    }
}
