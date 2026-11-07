use serde::Serialize;

use super::types::{UiBounds, UiNode, UiSnapshot};

const MIN_GAP: i32 = 4;
const MIN_TARGET: i32 = 24;
const AVERAGE_CHAR_WIDTH: f32 = 6.5;
const MAX_ISSUES: usize = 80;
const INTERACTIVE_ROLES: &[&str] = &[
    "button", "checkbox", "radio", "link", "menuitem", "tab", "combobox", "edit", "slider",
    "listitem", "treeitem", "switch", "spinner",
];

#[derive(Clone, Copy, Debug, Serialize, PartialEq, Eq, PartialOrd, Ord)]
#[serde(rename_all = "lowercase")]
pub enum Severity {
    Error,
    Warning,
    Info,
}

#[derive(Clone, Debug, Serialize)]
pub struct Issue {
    pub kind: &'static str,
    pub severity: Severity,
    pub refs: Vec<String>,
    pub message: String,
}

#[derive(Clone, Debug, Serialize)]
pub struct Audit {
    pub checked: usize,
    pub errors: usize,
    pub warnings: usize,
    pub issues: Vec<Issue>,
}

impl Audit {
    pub fn passed(&self) -> bool {
        self.errors == 0
    }

    pub fn text(&self) -> String {
        let mut lines = vec![format!(
            "Layout audit: {} elements, {} errors, {} warnings",
            self.checked, self.errors, self.warnings
        )];
        for issue in &self.issues {
            lines.push(format!(
                "{} {} [{}] {}",
                match issue.severity {
                    Severity::Error => "E",
                    Severity::Warning => "W",
                    Severity::Info => "I",
                },
                issue.kind,
                issue.refs.join(", "),
                issue.message
            ));
        }
        lines.join("\n")
    }
}

fn visible(node: &UiNode) -> bool {
    node.bounds.width > 0
        && node.bounds.height > 0
        && !node.states.iter().any(|state| state == "offscreen")
}

fn interactive(node: &UiNode) -> bool {
    !node.actions.is_empty() || INTERACTIVE_ROLES.contains(&node.role.as_str())
}

fn describe(node: &UiNode) -> String {
    if node.name.trim().is_empty() {
        format!("{} {}", node.role, node.reference)
    } else {
        format!("{} \"{}\"", node.role, node.name.replace('\n', " "))
    }
}

fn contains(outer: &UiBounds, inner: &UiBounds) -> bool {
    inner.x >= outer.x
        && inner.y >= outer.y
        && inner.x + inner.width <= outer.x + outer.width
        && inner.y + inner.height <= outer.y + outer.height
}

fn intersection(a: &UiBounds, b: &UiBounds) -> i64 {
    let width = (a.x + a.width).min(b.x + b.width) - a.x.max(b.x);
    let height = (a.y + a.height).min(b.y + b.height) - a.y.max(b.y);
    if width <= 0 || height <= 0 {
        0
    } else {
        width as i64 * height as i64
    }
}

fn area(bounds: &UiBounds) -> i64 {
    bounds.width as i64 * bounds.height as i64
}

fn gap(a: &UiBounds, b: &UiBounds) -> Option<(i32, &'static str)> {
    let vertical_overlap = (a.y + a.height).min(b.y + b.height) - a.y.max(b.y);
    let horizontal_overlap = (a.x + a.width).min(b.x + b.width) - a.x.max(b.x);
    if vertical_overlap > 0 {
        let distance = if a.x + a.width <= b.x {
            b.x - (a.x + a.width)
        } else if b.x + b.width <= a.x {
            a.x - (b.x + b.width)
        } else {
            return None;
        };
        return Some((distance, "horizontal"));
    }
    if horizontal_overlap > 0 {
        let distance = if a.y + a.height <= b.y {
            b.y - (a.y + a.height)
        } else if b.y + b.height <= a.y {
            a.y - (b.y + b.height)
        } else {
            return None;
        };
        return Some((distance, "vertical"));
    }
    None
}

pub fn run(snapshot: &UiSnapshot) -> Audit {
    let mut issues = Vec::new();
    let mut checked = 0;
    let window = snapshot.nodes.first().map(|node| node.bounds);
    for node in &snapshot.nodes {
        walk(node, window.as_ref(), &mut issues, &mut checked);
    }
    issues.sort_by(|a: &Issue, b: &Issue| a.severity.cmp(&b.severity));
    let errors = issues
        .iter()
        .filter(|issue| issue.severity == Severity::Error)
        .count();
    let warnings = issues
        .iter()
        .filter(|issue| issue.severity == Severity::Warning)
        .count();
    issues.truncate(MAX_ISSUES);
    Audit {
        checked,
        errors,
        warnings,
        issues,
    }
}

fn walk(node: &UiNode, window: Option<&UiBounds>, issues: &mut Vec<Issue>, checked: &mut usize) {
    if visible(node) {
        *checked += 1;
        check_node(node, window, issues);
    }
    check_siblings(&node.children, issues);
    for child in &node.children {
        walk(child, window, issues, checked);
    }
}

fn check_node(node: &UiNode, window: Option<&UiBounds>, issues: &mut Vec<Issue>) {
    if let Some(window) = window {
        if !contains(window, &node.bounds) && intersection(window, &node.bounds) > 0 {
            issues.push(Issue {
                kind: "clipped",
                severity: Severity::Warning,
                refs: vec![node.reference.clone()],
                message: format!("{} extends beyond the window edge", describe(node)),
            });
        }
    }
    if !interactive(node) {
        if node.role == "text" {
            check_truncation(node, issues);
        }
        return;
    }
    if node.name.trim().is_empty() && node.automation_id.is_none() && node.role != "edit" {
        issues.push(Issue {
            kind: "unnamed",
            severity: Severity::Error,
            refs: vec![node.reference.clone()],
            message: format!("{} has no accessible name", node.role),
        });
    }
    if node.bounds.width < MIN_TARGET || node.bounds.height < MIN_TARGET {
        issues.push(Issue {
            kind: "small-target",
            severity: Severity::Warning,
            refs: vec![node.reference.clone()],
            message: format!(
                "{} is {}x{} px, smaller than {MIN_TARGET}x{MIN_TARGET}",
                describe(node),
                node.bounds.width,
                node.bounds.height
            ),
        });
    }
    if matches!(node.role.as_str(), "button" | "tab" | "menuitem" | "link") {
        check_truncation(node, issues);
    }
}

fn check_truncation(node: &UiNode, issues: &mut Vec<Issue>) {
    let characters = node
        .name
        .lines()
        .map(|line| line.chars().count())
        .max()
        .unwrap_or(0);
    let lines = ((node.bounds.height as f32) / 16.0).floor().max(1.0);
    let needed = characters as f32 * AVERAGE_CHAR_WIDTH / lines;
    if characters > 3 && needed > node.bounds.width as f32 * 1.15 {
        issues.push(Issue {
            kind: "maybe-truncated",
            severity: Severity::Info,
            refs: vec![node.reference.clone()],
            message: format!(
                "{} needs about {} px but is {} px wide",
                describe(node),
                needed.round() as i32,
                node.bounds.width
            ),
        });
    }
}

fn check_siblings(children: &[UiNode], issues: &mut Vec<Issue>) {
    let items: Vec<&UiNode> = children.iter().filter(|child| visible(child)).collect();
    for (index, first) in items.iter().enumerate() {
        for second in items.iter().skip(index + 1) {
            let a = &first.bounds;
            let b = &second.bounds;
            let shared = intersection(a, b);
            if shared > 0 {
                if contains(a, b) || contains(b, a) {
                    continue;
                }
                let smaller = area(a).min(area(b)).max(1);
                if shared * 10 >= smaller {
                    issues.push(Issue {
                        kind: "overlap",
                        severity: Severity::Error,
                        refs: vec![first.reference.clone(), second.reference.clone()],
                        message: format!(
                            "{} overlaps {} by {}%",
                            describe(first),
                            describe(second),
                            shared * 100 / smaller
                        ),
                    });
                }
                continue;
            }
            if !(interactive(first) && interactive(second)) {
                continue;
            }
            if let Some((distance, axis)) = gap(a, b) {
                if distance < MIN_GAP {
                    issues.push(Issue {
                        kind: "too-close",
                        severity: Severity::Warning,
                        refs: vec![first.reference.clone(), second.reference.clone()],
                        message: format!(
                            "{} and {} are {distance} px apart ({axis}), less than {MIN_GAP}",
                            describe(first),
                            describe(second)
                        ),
                    });
                }
            }
            if first.role == second.role
                && !first.name.trim().is_empty()
                && first.name == second.name
            {
                issues.push(Issue {
                    kind: "duplicate",
                    severity: Severity::Info,
                    refs: vec![first.reference.clone(), second.reference.clone()],
                    message: format!(
                        "Two {} elements share the name \"{}\"",
                        first.role, first.name
                    ),
                });
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::appui::types::Provider;

    fn node(reference: &str, role: &str, name: &str, bounds: (i32, i32, i32, i32)) -> UiNode {
        UiNode {
            reference: reference.to_owned(),
            role: role.to_owned(),
            name: name.to_owned(),
            path: format!("{role}[{name}]"),
            bounds: UiBounds {
                x: bounds.0,
                y: bounds.1,
                width: bounds.2,
                height: bounds.3,
            },
            actions: if role == "button" {
                vec!["invoke".to_owned()]
            } else {
                Vec::new()
            },
            ..Default::default()
        }
    }

    fn snapshot(children: Vec<UiNode>) -> UiSnapshot {
        let mut window = node("n1", "window", "App", (0, 0, 400, 300));
        window.children = children;
        UiSnapshot {
            provider: Provider::Uia,
            hwnd: "1".to_owned(),
            title: "App".to_owned(),
            captured_at: 0,
            truncated: false,
            warnings: Vec::new(),
            nodes: vec![window],
            count: 1,
        }
    }

    fn kinds(audit: &Audit) -> Vec<&'static str> {
        audit.issues.iter().map(|issue| issue.kind).collect()
    }

    #[test]
    fn a_clean_layout_passes() {
        let audit = run(&snapshot(vec![
            node("n2", "button", "Save", (10, 10, 80, 32)),
            node("n3", "button", "Cancel", (110, 10, 80, 32)),
        ]));
        assert!(audit.passed());
        assert!(audit.issues.is_empty(), "{}", audit.text());
    }

    #[test]
    fn close_buttons_and_overlaps_are_reported() {
        let audit = run(&snapshot(vec![
            node("n2", "button", "Save", (10, 10, 80, 32)),
            node("n3", "button", "Cancel", (92, 10, 80, 32)),
            node("n4", "button", "Help", (180, 10, 80, 32)),
            node("n5", "button", "Close", (200, 20, 80, 32)),
        ]));
        let found = kinds(&audit);
        assert!(found.contains(&"too-close"), "{}", audit.text());
        assert!(found.contains(&"overlap"), "{}", audit.text());
        assert!(!audit.passed());
    }

    #[test]
    fn unnamed_small_and_clipped_elements_are_reported() {
        let audit = run(&snapshot(vec![
            node("n2", "button", "", (10, 10, 16, 16)),
            node("n3", "button", "Next", (360, 10, 80, 32)),
        ]));
        let found = kinds(&audit);
        assert!(found.contains(&"unnamed"));
        assert!(found.contains(&"small-target"));
        assert!(found.contains(&"clipped"));
    }

    #[test]
    fn long_labels_in_narrow_buttons_are_flagged() {
        let audit = run(&snapshot(vec![node(
            "n2",
            "button",
            "Save all changes and close the window",
            (10, 10, 60, 28),
        )]));
        assert!(kinds(&audit).contains(&"maybe-truncated"));
    }
}
