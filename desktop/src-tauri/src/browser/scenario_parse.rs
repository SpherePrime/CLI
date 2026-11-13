use serde::Deserialize;
use serde_json::{json, Map, Value};

pub const MAX_STEPS: usize = 80;
pub const MAX_SCREENSHOTS: usize = 16;

#[derive(Debug)]
pub struct Plan {
    pub steps: Vec<Step>,
    pub continue_on_error: bool,
    pub notes: Vec<String>,
}

#[derive(Debug, Clone, Copy, Deserialize, PartialEq)]
pub struct Clip {
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
}

#[derive(Debug, Deserialize)]
#[serde(tag = "action", rename_all = "snake_case", rename_all_fields = "camelCase")]
pub enum Step {
    Navigate {
        url: String,
    },
    Resize {
        width: f64,
    },
    Click {
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
    },
    Hover {
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
    },
    Focus {
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
    },
    Type {
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
        value: String,
        #[serde(default)]
        submit: bool,
    },
    Select {
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
        option: String,
    },
    PressKey {
        key: String,
    },
    Scroll {
        #[serde(default)]
        to: Option<String>,
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
    },
    WaitFor {
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
        #[serde(default)]
        gone: bool,
        #[serde(default)]
        timeout_ms: Option<u64>,
    },
    Wait {
        ms: u64,
    },
    Assert {
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
        #[serde(default)]
        contains: Option<String>,
        #[serde(default)]
        absent: bool,
    },
    Evaluate {
        expression: String,
    },
    Screenshot {
        #[serde(default)]
        name: String,
        #[serde(default)]
        full_page: bool,
        #[serde(default)]
        tiles: bool,
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
        #[serde(default)]
        padding: Option<f64>,
        #[serde(default)]
        clip: Option<Clip>,
        #[serde(default)]
        zoom: Option<f64>,
    },
}

impl Step {
    pub fn name(&self) -> &'static str {
        match self {
            Self::Navigate { .. } => "navigate",
            Self::Resize { .. } => "resize",
            Self::Click { .. } => "click",
            Self::Hover { .. } => "hover",
            Self::Focus { .. } => "focus",
            Self::Type { .. } => "type",
            Self::Select { .. } => "select",
            Self::PressKey { .. } => "press_key",
            Self::Scroll { .. } => "scroll",
            Self::WaitFor { .. } => "wait_for",
            Self::Wait { .. } => "wait",
            Self::Assert { .. } => "assert",
            Self::Evaluate { .. } => "evaluate",
            Self::Screenshot { .. } => "screenshot",
        }
    }
}

const FIELDS: &[(&str, &[&str])] = &[
    ("navigate", &["url"]),
    ("resize", &["width"]),
    ("click", &["selector", "text"]),
    ("hover", &["selector", "text"]),
    ("focus", &["selector", "text"]),
    ("type", &["selector", "text", "value", "submit"]),
    ("select", &["selector", "text", "option"]),
    ("press_key", &["key"]),
    ("scroll", &["to", "selector", "text"]),
    ("wait_for", &["selector", "text", "gone", "timeoutMs"]),
    ("wait", &["ms"]),
    ("assert", &["selector", "text", "contains", "absent"]),
    ("evaluate", &["expression"]),
    ("screenshot", &["name", "fullPage", "tiles", "selector", "text", "padding", "clip", "zoom"]),
];

const NEEDS_TARGET: &[&str] = &["click", "hover", "focus", "type", "select"];

pub fn action_names() -> Vec<&'static str> {
    FIELDS.iter().map(|(action, _)| *action).collect()
}

pub fn step_schema() -> Value {
    json!({
        "type":"object",
        "properties":{
            "action":{"type":"string","enum":action_names()},
            "url":{"type":"string","description":"navigate: URL to open"},
            "width":{"type":"number","minimum":320,"maximum":2560,"description":"resize: viewport width in CSS px"},
            "selector":{"type":"string","description":"CSS selector of the target element"},
            "text":{"type":"string","description":"Find the target by visible text, label, aria-label or placeholder instead of selector"},
            "value":{"type":"string","description":"type: text to enter"},
            "submit":{"type":"boolean","description":"type: press Enter after typing"},
            "option":{"type":"string","description":"select: option value or visible label"},
            "key":{"type":"string","description":"press_key: Enter, Tab, Escape, Backspace, Delete, Arrow keys, Home, End, PageUp, PageDown, Space or one character"},
            "to":{"type":"string","enum":["top","bottom"],"description":"scroll: page edge when no target is given"},
            "gone":{"type":"boolean","description":"wait_for: wait until the target disappears"},
            "timeoutMs":{"type":"integer","minimum":1,"maximum":15000,"description":"wait_for: timeout, default 15000"},
            "ms":{"type":"integer","minimum":0,"maximum":15000,"description":"wait: pause in milliseconds"},
            "contains":{"type":"string","description":"assert: expected text inside the target"},
            "absent":{"type":"boolean","description":"assert: the target must not exist"},
            "expression":{"type":"string","description":"evaluate: JavaScript expression, may be async"},
            "name":{"type":"string","description":"screenshot: short label"},
            "fullPage":{"type":"boolean","description":"screenshot: whole scrollable page"},
            "tiles":{"type":"boolean","description":"screenshot with fullPage: split a long page into readable sections"},
            "padding":{"type":"number","minimum":0,"maximum":400,"description":"screenshot of a target: extra CSS px around it to show its neighbours"},
            "clip":{"type":"object","properties":{"x":{"type":"number"},"y":{"type":"number"},"width":{"type":"number"},"height":{"type":"number"}},"required":["x","y","width","height"],"description":"screenshot: page region in CSS px (box from browser_snapshot)"},
            "zoom":{"type":"number","minimum":1,"maximum":3,"description":"screenshot: render 1-3x sharper for small text, icons, borders and focus rings"}
        },
        "required":["action"]
    })
}

pub fn parse(args: &Value) -> Result<Plan, String> {
    let steps = match &args["steps"] {
        Value::String(encoded) => serde_json::from_str::<Value>(encoded)
            .map_err(|error| format!("steps must be a JSON array of step objects: {error}"))?,
        other => other.clone(),
    };
    let items = steps
        .as_array()
        .ok_or("steps must be an array of step objects")?;
    if items.is_empty() {
        return Err("steps must contain at least one step".to_owned());
    }
    if items.len() > MAX_STEPS {
        return Err(format!("At most {MAX_STEPS} steps per run"));
    }
    let mut notes = Vec::new();
    let mut parsed = Vec::with_capacity(items.len());
    for (index, item) in items.iter().enumerate() {
        parsed.push(parse_step(index + 1, item, &mut notes)?);
    }
    let shots = parsed
        .iter()
        .filter(|step| matches!(step, Step::Screenshot { .. }))
        .count();
    if shots > MAX_SCREENSHOTS {
        return Err(format!("At most {MAX_SCREENSHOTS} screenshots per run"));
    }
    let continue_on_error = args
        .get("continueOnError")
        .or_else(|| args.get("continue_on_error"))
        .and_then(Value::as_bool)
        .unwrap_or(false);
    Ok(Plan {
        steps: parsed,
        continue_on_error,
        notes,
    })
}

fn parse_step(number: usize, item: &Value, notes: &mut Vec<String>) -> Result<Step, String> {
    let object = item
        .as_object()
        .ok_or_else(|| format!("step {number}: must be an object with an action field"))?;
    let (raw_action, action_from_type) = match object.get("action").and_then(Value::as_str) {
        Some(action) => (action, false),
        None => (
            object
                .get("type")
                .and_then(Value::as_str)
                .ok_or_else(|| format!("step {number}: missing action. Actions: {}", action_names().join(", ")))?,
            true,
        ),
    };
    let action = canonical_action(raw_action);
    let allowed = FIELDS
        .iter()
        .find(|(name, _)| *name == action)
        .map(|(_, fields)| *fields)
        .ok_or_else(|| format!("step {number}: unknown action {raw_action:?}. Actions: {}", action_names().join(", ")))?;
    let mut fields = Map::new();
    for (key, value) in object {
        if key == "action" || (action_from_type && key == "type") {
            continue;
        }
        fields.insert(camel_case(key), value.clone());
    }
    apply_aliases(&action, &mut fields, number, notes);
    let mut ignored = Vec::new();
    fields.retain(|key, _| {
        let keep = allowed.contains(&key.as_str());
        if !keep {
            ignored.push(key.clone());
        }
        keep
    });
    if !ignored.is_empty() {
        notes.push(format!("step {number} ({action}): ignored unknown field(s) {}", ignored.join(", ")));
    }
    if NEEDS_TARGET.contains(&action.as_str()) && !fields.contains_key("selector") && !fields.contains_key("text") {
        return Err(format!("step {number} ({action}): give selector, text or ref to find the element"));
    }
    fields.insert("action".to_owned(), Value::String(action.clone()));
    let mut step: Step = serde_json::from_value(Value::Object(fields)).map_err(|error| {
        format!("step {number} ({action}): {error}. Fields for {action}: {}", allowed.join(", "))
    })?;
    if let Step::Screenshot { name, .. } = &mut step {
        if name.trim().is_empty() {
            *name = format!("shot-{number}");
        }
    }
    Ok(step)
}

fn canonical_action(raw: &str) -> String {
    let snake = snake_case(raw);
    let action = match snake.as_str() {
        "goto" | "go_to" | "open" | "visit" | "load" | "navigate_to" | "open_url" => "navigate",
        "set_viewport" | "viewport" | "set_width" | "set_viewport_size" | "set_size" => "resize",
        "tap" | "press_button" => "click",
        "mouse_over" | "mouseover" | "move_mouse" | "mouse_move" => "hover",
        "fill" | "input" | "type_text" | "enter_text" | "fill_in" => "type",
        "select_option" | "choose" | "pick" => "select",
        "press" | "key" | "keypress" | "key_press" | "keyboard" => "press_key",
        "scroll_to" | "scroll_into_view" => "scroll",
        "wait_for_selector" | "wait_for_text" | "wait_until" | "wait_for_element" => "wait_for",
        "sleep" | "pause" | "delay" => "wait",
        "expect" | "check" | "verify" | "assert_text" | "assert_visible" | "assert_exists" => "assert",
        "eval" | "js" | "script" | "execute" | "run_script" | "evaluate_js" => "evaluate",
        "capture" | "snapshot" | "take_screenshot" | "shot" | "screen" => "screenshot",
        other => other,
    };
    action.to_owned()
}

fn apply_aliases(action: &str, fields: &mut Map<String, Value>, number: usize, notes: &mut Vec<String>) {
    if let Some(reference) = fields.remove("ref") {
        let reference = reference
            .as_u64()
            .or_else(|| reference.as_str().and_then(|text| text.trim().parse().ok()));
        if let (Some(reference), false) = (reference, fields.contains_key("selector")) {
            fields.insert("selector".to_owned(), Value::String(format!("[data-prime-ref=\"{reference}\"]")));
        }
    }
    let rename = |fields: &mut Map<String, Value>, from: &[&str], to: &str| {
        if fields.contains_key(to) {
            return;
        }
        if let Some(value) = from.iter().find_map(|key| fields.remove(*key)) {
            fields.insert(to.to_owned(), value);
        }
    };
    match action {
        "navigate" => rename(fields, &["href", "link", "address", "page"], "url"),
        "resize" => {
            rename(fields, &["w", "viewportWidth", "size"], "width");
            if fields.remove("height").is_some() {
                notes.push(format!("step {number} (resize): height is set by the panel; only width was applied"));
            }
        }
        "type" => {
            rename(fields, &["input", "content", "textValue", "fill", "keys"], "value");
            if !fields.contains_key("value") && fields.contains_key("selector") {
                if let Some(text) = fields.remove("text") {
                    fields.insert("value".to_owned(), text);
                }
            }
            rename(fields, &["enter", "pressEnter"], "submit");
        }
        "select" => {
            rename(fields, &["value", "label", "choice", "optionText"], "option");
        }
        "press_key" => {
            rename(fields, &["keys", "name", "value", "text"], "key");
            if let Some(Value::String(key)) = fields.get_mut("key") {
                *key = key_name(key);
            }
        }
        "scroll" => {
            rename(fields, &["position", "edge"], "to");
            if let Some(direction) = fields.remove("direction").and_then(|value| value.as_str().map(str::to_lowercase)) {
                fields.entry("to").or_insert_with(|| Value::String(if direction == "up" { "top" } else { "bottom" }.to_owned()));
            }
        }
        "wait_for" => {
            rename(fields, &["timeout", "waitMs"], "timeoutMs");
            if let Some(state) = fields.remove("state").and_then(|value| value.as_str().map(str::to_lowercase)) {
                fields.entry("gone").or_insert(Value::Bool(matches!(state.as_str(), "hidden" | "detached" | "gone")));
            }
            rename(fields, &["hidden", "disappear", "absent"], "gone");
        }
        "wait" => {
            rename(fields, &["duration", "time", "milliseconds", "timeout", "timeoutMs", "delay"], "ms");
            if !fields.contains_key("ms") {
                if let Some(seconds) = ["seconds", "sec", "s"].iter().find_map(|key| fields.remove(*key)).and_then(|value| number_of(&value)) {
                    fields.insert("ms".to_owned(), json!((seconds * 1000.0).round() as u64));
                }
            }
        }
        "assert" => {
            rename(fields, &["expected", "textContains", "includes", "equals", "toContain", "value"], "contains");
            rename(fields, &["notVisible", "notPresent", "missing", "hidden"], "absent");
            if !fields.contains_key("contains") && fields.contains_key("selector") {
                if let Some(text) = fields.remove("text") {
                    fields.insert("contains".to_owned(), text);
                }
            }
            fields.remove("visible");
            fields.remove("exists");
        }
        "evaluate" => rename(fields, &["script", "code", "js", "expr", "function"], "expression"),
        "screenshot" => {
            rename(fields, &["fullpage", "full", "wholePage"], "fullPage");
            rename(fields, &["label", "title", "id", "file", "filename"], "name");
            rename(fields, &["region", "area", "rect", "box"], "clip");
            rename(fields, &["scale", "zoomLevel", "magnify"], "zoom");
            rename(fields, &["margin", "pad", "around"], "padding");
            rename(fields, &["tile", "split", "sections"], "tiles");
        }
        _ => {}
    }
    for key in ["width", "ms", "timeoutMs", "padding", "zoom"] {
        if let Some(value) = fields.get(key).and_then(number_of) {
            let value = if matches!(key, "ms" | "timeoutMs") {
                json!(value.max(0.0).round() as u64)
            } else {
                json!(value)
            };
            fields.insert(key.to_owned(), value);
        }
    }
    for key in ["submit", "gone", "absent", "fullPage", "tiles"] {
        if let Some(Value::String(flag)) = fields.get(key) {
            let flag = matches!(flag.to_lowercase().as_str(), "true" | "yes" | "1");
            fields.insert(key.to_owned(), Value::Bool(flag));
        }
    }
}

fn number_of(value: &Value) -> Option<f64> {
    value.as_f64().or_else(|| {
        value
            .as_str()
            .map(|text| text.trim().trim_end_matches("px").trim_end_matches("ms").trim())
            .and_then(|text| text.parse().ok())
    })
}

fn key_name(key: &str) -> String {
    let named = match key.trim().to_lowercase().as_str() {
        "enter" | "return" => "Enter",
        "tab" => "Tab",
        "esc" | "escape" => "Escape",
        "backspace" => "Backspace",
        "delete" | "del" => "Delete",
        "arrowleft" | "left" => "ArrowLeft",
        "arrowright" | "right" => "ArrowRight",
        "arrowup" | "up" => "ArrowUp",
        "arrowdown" | "down" => "ArrowDown",
        "home" => "Home",
        "end" => "End",
        "pageup" => "PageUp",
        "pagedown" => "PageDown",
        "space" | "spacebar" => "Space",
        _ => return key.to_owned(),
    };
    named.to_owned()
}

fn snake_case(raw: &str) -> String {
    let mut output = String::new();
    for (index, character) in raw.trim().chars().enumerate() {
        if character.is_ascii_uppercase() {
            if index > 0 && !output.ends_with('_') {
                output.push('_');
            }
            output.push(character.to_ascii_lowercase());
        } else if character == '-' || character == ' ' || character == '.' {
            output.push('_');
        } else {
            output.push(character);
        }
    }
    output
}

fn camel_case(raw: &str) -> String {
    let mut output = String::new();
    let mut upper = false;
    for character in raw.chars() {
        if character == '_' || character == '-' {
            upper = !output.is_empty();
        } else if upper {
            output.push(character.to_ascii_uppercase());
            upper = false;
        } else {
            output.push(character);
        }
    }
    output
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_a_full_scenario() {
        let plan = parse(&json!({"steps":[
            {"action":"navigate","url":"http://localhost:5173"},
            {"action":"resize","width":390},
            {"action":"click","text":"Записаться"},
            {"action":"type","selector":"#email","value":"a@b.c","submit":true},
            {"action":"wait_for","text":"Готово","timeoutMs":3000},
            {"action":"assert","selector":"h1","contains":"Prime"},
            {"action":"scroll","to":"bottom"},
            {"action":"screenshot","name":"mobile","fullPage":true},
            {"action":"screenshot","name":"form","selector":"form"}
        ]}))
        .unwrap();
        assert_eq!(plan.steps.len(), 9);
        assert!(!plan.continue_on_error);
        assert!(plan.notes.is_empty());
        assert!(matches!(&plan.steps[2], Step::Click { text, .. } if text.as_deref() == Some("Записаться")));
        assert!(matches!(&plan.steps[7], Step::Screenshot { full_page: true, .. }));
    }

    #[test]
    fn accepts_the_shapes_models_send() {
        let plan = parse(&json!({"continue_on_error":true,"steps":"[
            {\"action\":\"goto\",\"href\":\"http://localhost:3000\"},
            {\"action\":\"setViewport\",\"width\":\"390px\",\"height\":844},
            {\"action\":\"fill\",\"selector\":\"#email\",\"text\":\"a@b.c\"},
            {\"action\":\"press\",\"key\":\"enter\"},
            {\"action\":\"waitForSelector\",\"selector\":\".done\",\"timeout\":2000},
            {\"action\":\"sleep\",\"seconds\":1},
            {\"action\":\"expect\",\"selector\":\"h1\",\"text\":\"Prime\"},
            {\"action\":\"hover\",\"ref\":4},
            {\"action\":\"select_option\",\"selector\":\"select\",\"value\":\"ru\"},
            {\"action\":\"screenshot\",\"full_page\":true,\"tiles\":\"true\"},
            {\"action\":\"screenshot\",\"selector\":\".card\",\"padding\":24,\"zoom\":2},
            {\"action\":\"screenshot\",\"region\":{\"x\":0,\"y\":0,\"width\":300,\"height\":200}}
        ]"}))
        .unwrap();
        assert!(plan.continue_on_error);
        assert!(matches!(&plan.steps[0], Step::Navigate { url } if url == "http://localhost:3000"));
        assert!(matches!(&plan.steps[1], Step::Resize { width } if *width == 390.0));
        assert!(matches!(&plan.steps[2], Step::Type { value, text: None, .. } if value == "a@b.c"));
        assert!(matches!(&plan.steps[3], Step::PressKey { key } if key == "Enter"));
        assert!(matches!(&plan.steps[4], Step::WaitFor { timeout_ms: Some(2000), .. }));
        assert!(matches!(&plan.steps[5], Step::Wait { ms: 1000 }));
        assert!(matches!(&plan.steps[6], Step::Assert { contains: Some(text), text: None, .. } if text == "Prime"));
        assert!(matches!(&plan.steps[7], Step::Hover { selector: Some(selector), .. } if selector == "[data-prime-ref=\"4\"]"));
        assert!(matches!(&plan.steps[8], Step::Select { option, .. } if option == "ru"));
        assert!(matches!(&plan.steps[9], Step::Screenshot { full_page: true, tiles: true, name, .. } if name == "shot-10"));
        assert!(matches!(&plan.steps[10], Step::Screenshot { padding: Some(padding), zoom: Some(zoom), .. } if *padding == 24.0 && *zoom == 2.0));
        assert!(matches!(&plan.steps[11], Step::Screenshot { clip: Some(Clip { width, .. }), .. } if *width == 300.0));
        assert_eq!(plan.notes.len(), 1);
    }

    #[test]
    fn unknown_fields_are_reported_not_fatal() {
        let plan = parse(&json!({"steps":[{"action":"click","selector":"a","force":true}]})).unwrap();
        assert_eq!(plan.notes, vec!["step 1 (click): ignored unknown field(s) force".to_owned()]);
    }

    #[test]
    fn errors_name_the_step_and_its_fields() {
        assert!(parse(&json!({"steps":[]})).is_err());
        let unknown = parse(&json!({"steps":[{"action":"fly"}]})).unwrap_err();
        assert!(unknown.contains("step 1") && unknown.contains("navigate"));
        let missing = parse(&json!({"steps":[{"action":"navigate","url":"x"},{"action":"navigate"}]})).unwrap_err();
        assert!(missing.starts_with("step 2 (navigate)") && missing.contains("Fields for navigate: url"));
        let target = parse(&json!({"steps":[{"action":"click"}]})).unwrap_err();
        assert!(target.contains("give selector, text or ref"));
        let shots: Vec<Value> = (0..=MAX_SCREENSHOTS).map(|index| json!({"action":"screenshot","name":index.to_string()})).collect();
        assert!(parse(&json!({"steps":shots})).is_err());
    }

    #[test]
    fn schema_lists_every_field() {
        let schema = step_schema();
        for (_, fields) in FIELDS {
            for field in *fields {
                assert!(schema["properties"].get(*field).is_some(), "{field} missing from schema");
            }
        }
    }
}
