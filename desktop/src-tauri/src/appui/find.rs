use super::render::node_line;
use super::types::{UiNode, UiSnapshot};

pub const MAX_MATCHES: usize = 30;

pub struct Query {
    pub text: Option<String>,
    pub role: Option<String>,
}

impl Query {
    pub fn is_empty(&self) -> bool {
        self.text.is_none() && self.role.is_none()
    }

    fn matches(&self, node: &UiNode) -> bool {
        if let Some(role) = &self.role {
            if !node.role.eq_ignore_ascii_case(role) {
                return false;
            }
        }
        let Some(text) = &self.text else {
            return true;
        };
        let needle = text.to_lowercase();
        [
            Some(node.name.as_str()),
            node.value.as_deref(),
            node.automation_id.as_deref(),
        ]
        .into_iter()
        .flatten()
        .any(|field| field.to_lowercase().contains(&needle))
    }
}

pub fn matches<'a>(snapshot: &'a UiSnapshot, query: &Query) -> Vec<&'a UiNode> {
    let mut found = Vec::new();
    for node in &snapshot.nodes {
        collect(node, query, &mut found);
    }
    found
}

fn collect<'a>(node: &'a UiNode, query: &Query, found: &mut Vec<&'a UiNode>) {
    if query.matches(node) {
        found.push(node);
    }
    for child in &node.children {
        collect(child, query, found);
    }
}

pub fn render(nodes: &[&UiNode]) -> String {
    if nodes.is_empty() {
        return "No matching elements".to_owned();
    }
    let mut lines: Vec<String> = nodes
        .iter()
        .take(MAX_MATCHES)
        .map(|node| format!("{}  path={}", node_line(node), node.path))
        .collect();
    if nodes.len() > MAX_MATCHES {
        lines.push(format!(
            "... {} more matches. Narrow the query.",
            nodes.len() - MAX_MATCHES
        ));
    }
    lines.join("\n")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::appui::types::Provider;

    fn node(reference: &str, role: &str, name: &str) -> UiNode {
        UiNode {
            reference: reference.to_owned(),
            role: role.to_owned(),
            name: name.to_owned(),
            path: format!("{role}[{name}]"),
            ..Default::default()
        }
    }

    fn snapshot() -> UiSnapshot {
        let mut window = node("n1", "window", "App");
        let mut edit = node("n3", "edit", "Email");
        edit.value = Some("user@example.test".to_owned());
        window.children = vec![node("n2", "button", "Save changes"), edit];
        UiSnapshot {
            provider: Provider::Uia,
            hwnd: "1".to_owned(),
            title: "App".to_owned(),
            captured_at: 0,
            truncated: false,
            warnings: Vec::new(),
            nodes: vec![window],
            count: 3,
        }
    }

    #[test]
    fn finds_by_text_case_insensitively_including_values() {
        let snapshot = snapshot();
        let query = Query {
            text: Some("SAVE".to_owned()),
            role: None,
        };
        let found = matches(&snapshot, &query);
        assert_eq!(found.len(), 1);
        assert_eq!(found[0].reference, "n2");
        let query = Query {
            text: Some("example.test".to_owned()),
            role: None,
        };
        assert_eq!(matches(&snapshot, &query)[0].reference, "n3");
    }

    #[test]
    fn filters_by_role() {
        let snapshot = snapshot();
        let query = Query {
            text: None,
            role: Some("Edit".to_owned()),
        };
        let found = matches(&snapshot, &query);
        assert_eq!(found.len(), 1);
        assert!(render(&found).contains("path=edit[Email]"));
    }
}
