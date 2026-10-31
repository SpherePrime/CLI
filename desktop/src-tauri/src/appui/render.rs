use super::types::{Confidence, Provider, UiNode, UiSnapshot};

pub fn snapshot_text(snapshot: &UiSnapshot) -> String {
    let mut lines = vec![format!(
        "{} [{}, {} nodes{}]",
        snapshot.title,
        provider_name(snapshot.provider),
        snapshot.count,
        if snapshot.truncated { ", truncated" } else { "" }
    )];
    for warning in &snapshot.warnings {
        lines.push(format!("warning: {warning}"));
    }
    for node in &snapshot.nodes {
        render_node(node, 0, &mut lines);
    }
    lines.join("\n")
}

fn render_node(node: &UiNode, depth: usize, lines: &mut Vec<String>) {
    let mut line = format!(
        "{}{} {} \"{}\" [{}]",
        "  ".repeat(depth),
        node.reference,
        node.role,
        node.name.replace('\n', " "),
        node.states.join(", ")
    );
    if let Some(value) = node.value.as_deref().filter(|value| !value.is_empty()) {
        line.push_str(&format!(" value=\"{}\"", value.replace('\n', " ")));
    }
    line.push_str(&format!(
        " ({},{},{},{})",
        node.bounds.x, node.bounds.y, node.bounds.width, node.bounds.height
    ));
    if !node.actions.is_empty() {
        line.push_str(&format!(" actions={}", node.actions.join(",")));
    }
    if node.confidence != Confidence::High {
        line.push_str(&format!(" confidence={}", confidence_name(node.confidence)));
    }
    lines.push(line);
    for child in &node.children {
        render_node(child, depth + 1, lines);
    }
}

fn provider_name(provider: Provider) -> &'static str {
    match provider {
        Provider::Uia => "uia",
        Provider::Atspi => "atspi",
        Provider::Ax => "ax",
        Provider::Dom => "dom",
        Provider::Vision => "vision",
    }
}

fn confidence_name(confidence: Confidence) -> &'static str {
    match confidence {
        Confidence::High => "high",
        Confidence::Medium => "medium",
        Confidence::Low => "low",
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::appui::types::{UiBounds, UiNode};

    fn node(reference: &str, role: &str, name: &str) -> UiNode {
        UiNode {
            reference: reference.to_owned(),
            role: role.to_owned(),
            name: name.to_owned(),
            path: format!("{role}[{name}]"),
            source: Provider::Uia,
            confidence: Confidence::High,
            states: vec!["enabled".to_owned()],
            ..Default::default()
        }
    }

    fn fixture_snapshot() -> UiSnapshot {
        let mut window = node("n1", "window", "Prime fixture");
        window.bounds = UiBounds { x: 0, y: 0, width: 420, height: 260 };
        let mut button = node("n3", "button", "Save");
        button.bounds = UiBounds { x: 120, y: 300, width: 50, height: 50 };
        button.actions = vec!["invoke".to_owned()];
        let mut edit = node("n4", "edit", "Name");
        edit.bounds = UiBounds { x: 10, y: 10, width: 200, height: 24 };
        edit.value = Some("Иван".to_owned());
        edit.actions = vec!["setValue".to_owned()];
        window.children = vec![button, edit];
        UiSnapshot {
            provider: Provider::Uia,
            hwnd: "42".to_owned(),
            title: "Prime fixture".to_owned(),
            captured_at: 0,
            truncated: false,
            warnings: Vec::new(),
            nodes: vec![window],
            count: 3,
        }
    }

    #[test]
    fn renders_nodes_with_role_name_states_bounds_and_actions() {
        let text = snapshot_text(&fixture_snapshot());
        assert!(text.contains("n3 button \"Save\" [enabled] (120,300,50,50) actions=invoke"));
        assert!(text.contains("n4 edit \"Name\" [enabled] value=\"Иван\""));
        assert!(text.contains("Prime fixture [uia, 3 nodes]"));
        assert!(text.contains("  n3 button"));
    }

    #[test]
    fn renders_warnings_truncation_and_low_confidence() {
        let mut snapshot = fixture_snapshot();
        snapshot.truncated = true;
        snapshot.warnings = vec!["The application exposes no accessible elements".to_owned()];
        snapshot.nodes[0].children[0].confidence = Confidence::Low;
        let text = snapshot_text(&snapshot);
        assert!(text.contains("[uia, 3 nodes, truncated]"));
        assert!(text.contains("warning: The application exposes no accessible elements"));
        assert!(text.contains("confidence=low"));
    }
}
