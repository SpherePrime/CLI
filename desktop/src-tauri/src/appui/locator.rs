use super::types::{Locator, UiNode, UiSnapshot};

pub fn resolve(snapshot: &UiSnapshot, locator: &Locator) -> Result<String, String> {
    if let Some(reference) = locator.reference.as_deref() {
        return if all(snapshot).iter().any(|node| node.reference == reference) {
            Ok(reference.to_owned())
        } else {
            Err("Stale reference; take a new snapshot".to_owned())
        };
    }
    if let Some(automation_id) = locator.automation_id.as_deref() {
        return unique(snapshot, "automationId", |node| {
            node.automation_id.as_deref() == Some(automation_id)
        });
    }
    if let Some(path) = locator.path.as_deref() {
        return unique(snapshot, "path", |node| node.path == path);
    }
    if let Some(role) = locator.role.as_deref() {
        let name = locator.name.as_deref();
        return unique(snapshot, "role", |node| {
            node.role.eq_ignore_ascii_case(role.trim())
                && name.is_none_or(|name| same(&node.name, name))
        });
    }
    if let Some(name) = locator.name.as_deref() {
        return unique(snapshot, "name", |node| same(&node.name, name));
    }
    Err("Provide reference, automationId, path, role or name".to_owned())
}

pub fn same(left: &str, right: &str) -> bool {
    left.trim().eq_ignore_ascii_case(right.trim())
}

fn unique(
    snapshot: &UiSnapshot,
    key: &str,
    matches: impl Fn(&UiNode) -> bool,
) -> Result<String, String> {
    let found: Vec<&UiNode> = all(snapshot).into_iter().filter(|node| matches(node)).collect();
    match found.len() {
        0 => Err(format!("No element matches {key}")),
        1 => Ok(found[0].reference.clone()),
        _ => Err(format!(
            "Ambiguous {key}: {} elements match ({})",
            found.len(),
            found
                .iter()
                .map(|node| format!("{} \"{}\" ref={}", node.role, node.name, node.reference))
                .collect::<Vec<_>>()
                .join(", ")
        )),
    }
}

fn all(snapshot: &UiSnapshot) -> Vec<&UiNode> {
    let mut nodes = Vec::new();
    for node in &snapshot.nodes {
        collect(node, &mut nodes);
    }
    nodes
}

fn collect<'a>(node: &'a UiNode, nodes: &mut Vec<&'a UiNode>) {
    nodes.push(node);
    for child in &node.children {
        collect(child, nodes);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::appui::types::{Confidence, Provider, UiNode, UiSnapshot};

    fn node(reference: &str, role: &str, name: &str) -> UiNode {
        UiNode {
            reference: reference.to_owned(),
            role: role.to_owned(),
            name: name.to_owned(),
            path: format!("{role}[{name}]"),
            source: Provider::Uia,
            confidence: Confidence::High,
            ..Default::default()
        }
    }

    fn snapshot_with_two_save_buttons() -> UiSnapshot {
        let mut window = node("n1", "window", "Prime fixture");
        let mut first = node("n3", "button", "Save");
        first.automation_id = Some("save-primary".to_owned());
        let mut second = node("n7", "button", "Save");
        second.automation_id = Some("save-secondary".to_owned());
        window.children = vec![first, second];
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
    fn ambiguous_name_lists_candidates_instead_of_choosing_one() {
        let snapshot = snapshot_with_two_save_buttons();
        let error = resolve(
            &snapshot,
            &Locator { name: Some("Save".to_owned()), ..Default::default() },
        )
        .unwrap_err();
        assert!(error.contains("n3") && error.contains("n7"), "{error}");
        assert!(error.contains("Ambiguous name"), "{error}");
    }

    #[test]
    fn stale_reference_is_rejected() {
        let snapshot = snapshot_with_two_save_buttons();
        let error = resolve(
            &snapshot,
            &Locator { reference: Some("n99".to_owned()), ..Default::default() },
        )
        .unwrap_err();
        assert!(error.contains("Stale reference"), "{error}");
    }

    #[test]
    fn empty_locator_is_rejected() {
        let error = resolve(&snapshot_with_two_save_buttons(), &Locator::default()).unwrap_err();
        assert!(error.contains("Provide reference"), "{error}");
    }

    #[test]
    fn automation_id_and_role_name_resolve_uniquely() {
        let snapshot = snapshot_with_two_save_buttons();
        assert_eq!(
            resolve(
                &snapshot,
                &Locator { automation_id: Some("save-secondary".to_owned()), ..Default::default() }
            )
            .unwrap(),
            "n7"
        );
        assert_eq!(
            resolve(
                &snapshot,
                &Locator { role: Some("BUTTON".to_owned()), name: Some(" save ".to_owned()), ..Default::default() },
            )
            .unwrap_err()
            .contains("Ambiguous role"),
            true
        );
        assert_eq!(
            resolve(
                &snapshot,
                &Locator { role: Some("window".to_owned()), name: Some("prime FIXTURE".to_owned()), ..Default::default() },
            )
            .unwrap(),
            "n1"
        );
    }

    #[test]
    fn missing_element_is_reported_without_a_candidate() {
        let snapshot = snapshot_with_two_save_buttons();
        let error = resolve(
            &snapshot,
            &Locator { name: Some("Delete".to_owned()), ..Default::default() },
        )
        .unwrap_err();
        assert!(error.contains("No element matches"), "{error}");
    }
}
