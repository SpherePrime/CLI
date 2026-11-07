use std::collections::HashMap;

use super::types::{UiNode, UiSnapshot};

const MAX_LINES: usize = 60;
pub const NO_CHANGES: &str = "No interface changes detected";

struct Entry<'a> {
    node: &'a UiNode,
}

fn index(snapshot: &UiSnapshot) -> Vec<(String, Entry<'_>)> {
    let mut seen: HashMap<String, usize> = HashMap::new();
    let mut entries = Vec::new();
    for node in &snapshot.nodes {
        collect(node, &mut seen, &mut entries);
    }
    entries
}

fn collect<'a>(
    node: &'a UiNode,
    seen: &mut HashMap<String, usize>,
    entries: &mut Vec<(String, Entry<'a>)>,
) {
    let count = seen.entry(node.path.clone()).or_insert(0);
    let key = format!("{}#{}", node.path, count);
    *count += 1;
    entries.push((key, Entry { node }));
    for child in &node.children {
        collect(child, seen, entries);
    }
}

fn label(node: &UiNode) -> String {
    let name = node.name.replace('\n', " ");
    if name.is_empty() {
        format!("{} {}", node.reference, node.role)
    } else {
        format!("{} {} \"{}\"", node.reference, node.role, name)
    }
}

fn value(node: &UiNode) -> String {
    node.value.clone().unwrap_or_default().replace('\n', " ")
}

pub fn describe(before: &UiSnapshot, after: &UiSnapshot) -> String {
    let previous = index(before);
    let current = index(after);
    let previous_map: HashMap<&str, &Entry> = previous
        .iter()
        .map(|(key, entry)| (key.as_str(), entry))
        .collect();
    let current_keys: std::collections::HashSet<&str> =
        current.iter().map(|(key, _)| key.as_str()).collect();
    let mut lines = Vec::new();
    if before.title != after.title {
        lines.push(format!(
            "~ window title \"{}\" -> \"{}\"",
            before.title, after.title
        ));
    }
    for (key, entry) in &current {
        let node = entry.node;
        match previous_map.get(key.as_str()) {
            None => lines.push(format!("+ {}{}", label(node), suffix(node))),
            Some(old) => {
                let mut details = Vec::new();
                if value(old.node) != value(node) {
                    details.push(format!(
                        "value \"{}\" -> \"{}\"",
                        value(old.node),
                        value(node)
                    ));
                }
                if old.node.states != node.states {
                    details.push(format!(
                        "states [{}] -> [{}]",
                        old.node.states.join(", "),
                        node.states.join(", ")
                    ));
                }
                if !details.is_empty() {
                    lines.push(format!("~ {}: {}", label(node), details.join("; ")));
                }
            }
        }
    }
    for (key, entry) in &previous {
        if !current_keys.contains(key.as_str()) {
            let node = entry.node;
            let name = node.name.replace('\n', " ");
            lines.push(if name.is_empty() {
                format!("- {}", node.role)
            } else {
                format!("- {} \"{}\"", node.role, name)
            });
        }
    }
    if lines.is_empty() {
        return NO_CHANGES.to_owned();
    }
    let total = lines.len();
    if total > MAX_LINES {
        lines.truncate(MAX_LINES);
        lines.push(format!(
            "... {} more changes. Use app_find or app_snapshot.",
            total - MAX_LINES
        ));
    }
    lines.join("\n")
}

fn suffix(node: &UiNode) -> String {
    let value = value(node);
    let mut parts = Vec::new();
    if !value.is_empty() {
        parts.push(format!(" value=\"{value}\""));
    }
    if !node.states.is_empty() {
        parts.push(format!(" [{}]", node.states.join(", ")));
    }
    parts.concat()
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
            path: format!("window[App]/{role}[{name}]"),
            states: vec!["enabled".to_owned()],
            ..Default::default()
        }
    }

    fn snapshot(nodes: Vec<UiNode>) -> UiSnapshot {
        UiSnapshot {
            provider: Provider::Uia,
            hwnd: "1".to_owned(),
            title: "App".to_owned(),
            captured_at: 0,
            truncated: false,
            warnings: Vec::new(),
            count: nodes.len(),
            nodes,
        }
    }

    #[test]
    fn identical_snapshots_report_no_changes() {
        let before = snapshot(vec![node("n1", "button", "Save")]);
        assert_eq!(describe(&before, &before.clone()), NO_CHANGES);
    }

    #[test]
    fn added_removed_and_changed_elements_are_listed() {
        let mut edit = node("n2", "edit", "Name");
        edit.value = Some(String::new());
        let before = snapshot(vec![node("n1", "button", "Save"), edit.clone()]);
        edit.value = Some("Ivan".to_owned());
        let after = snapshot(vec![edit, node("n3", "dialog", "Saved")]);
        let text = describe(&before, &after);
        assert!(text.contains("+ n3 dialog \"Saved\""), "{text}");
        assert!(text.contains("- button \"Save\""), "{text}");
        assert!(
            text.contains("~ n2 edit \"Name\": value \"\" -> \"Ivan\""),
            "{text}"
        );
    }

    #[test]
    fn long_change_lists_are_truncated() {
        let before = snapshot(Vec::new());
        let nodes = (0..100)
            .map(|index| node(&format!("n{index}"), "text", &format!("Row {index}")))
            .collect();
        let text = describe(&before, &snapshot(nodes));
        assert!(text.contains("40 more changes"), "{text}");
    }
}
