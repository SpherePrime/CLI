use serde::{Deserialize, Serialize};

#[derive(Clone, Copy, Debug, Default, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct UiBounds {
    pub x: i32,
    pub y: i32,
    pub width: i32,
    pub height: i32,
}

#[derive(Clone, Copy, Debug, Default, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum Confidence {
    #[default]
    High,
    Medium,
    Low,
}

#[derive(Clone, Copy, Debug, Default, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum Provider {
    #[default]
    Uia,
    Atspi,
    Ax,
    Dom,
    Vision,
}

#[derive(Clone, Debug, Default, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct Locator {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reference: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub automation_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub role: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path: Option<String>,
}

#[derive(Clone, Debug, Default, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct UiNode {
    #[serde(rename = "ref")]
    pub reference: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub parent: Option<String>,
    pub role: String,
    pub name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub value: Option<String>,
    pub bounds: UiBounds,
    #[serde(default)]
    pub states: Vec<String>,
    #[serde(default)]
    pub actions: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub automation_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub class_name: Option<String>,
    pub path: String,
    pub source: Provider,
    pub confidence: Confidence,
    #[serde(default)]
    pub children: Vec<UiNode>,
}

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct UiSnapshot {
    pub provider: Provider,
    pub hwnd: String,
    pub title: String,
    pub captured_at: u64,
    pub truncated: bool,
    #[serde(default)]
    pub warnings: Vec<String>,
    pub nodes: Vec<UiNode>,
    pub count: usize,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn locator_rejects_unknown_fields_and_accepts_camel_case() {
        let locator: Locator = serde_json::from_str(r#"{"automationId":"save","name":"Save"}"#).unwrap();
        assert_eq!(locator.automation_id.as_deref(), Some("save"));
        assert_eq!(locator.name.as_deref(), Some("Save"));
        assert!(serde_json::from_str::<Locator>(r#"{"workspaceId":"other"}"#).is_err());
    }

    #[test]
    fn node_serializes_reference_as_ref() {
        let node = UiNode {
            reference: "n3".to_owned(),
            role: "button".to_owned(),
            name: "Save".to_owned(),
            path: "window/button[Save]".to_owned(),
            source: Provider::Uia,
            confidence: Confidence::High,
            ..Default::default()
        };
        let value = serde_json::to_value(node).unwrap();
        assert_eq!(value["ref"], "n3");
        assert_eq!(value["source"], "uia");
        assert_eq!(value["confidence"], "high");
        assert!(value.get("reference").is_none());
    }
}
