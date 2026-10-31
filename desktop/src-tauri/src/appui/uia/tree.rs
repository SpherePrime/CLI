use ::windows::core::{BOOL, Result as WindowsResult, BSTR};
use ::windows::Win32::Foundation::{HWND, POINT, RECT};
use ::windows::Win32::UI::Accessibility::{
    IUIAutomation, IUIAutomationCondition, IUIAutomationElement,
    IUIAutomationExpandCollapsePattern, IUIAutomationInvokePattern, IUIAutomationScrollPattern,
    IUIAutomationSelectionItemPattern, IUIAutomationTogglePattern, IUIAutomationValuePattern,
    TreeScope_Children, UIA_ExpandCollapsePatternId, UIA_InvokePatternId, UIA_ScrollPatternId,
    UIA_SelectionItemPatternId, UIA_TogglePatternId, UIA_ValuePatternId,
};
use ::windows::Win32::UI::WindowsAndMessaging::GetCursorPos;
use std::time::{SystemTime, UNIX_EPOCH};

use super::action::{MAX_DEPTH, MAX_NODES};
use super::roles;
use crate::appui::types::{Confidence, Provider, UiBounds, UiNode, UiSnapshot};

pub fn snapshot(
    automation: &IUIAutomation,
    window: HWND,
    max_nodes: usize,
    max_depth: usize,
) -> Result<UiSnapshot, String> {
    let root = unsafe { automation.ElementFromHandle(window) }.map_err(|error| error.to_string())?;
    let condition = unsafe { automation.CreateTrueCondition() }.map_err(|error| error.to_string())?;
    let title = text(unsafe { root.CurrentName() });
    let mut builder = Builder {
        condition,
        max_nodes,
        max_depth,
        count: 0,
        actionable: 0,
        truncated: false,
    };
    let node = builder.node(&root, None, "", 0)?;
    Ok(UiSnapshot {
        provider: Provider::Uia,
        hwnd: (window.0 as usize).to_string(),
        title,
        captured_at: now(),
        truncated: builder.truncated,
        warnings: warnings(builder.actionable, builder.truncated),
        count: builder.count,
        nodes: vec![node],
    })
}

pub fn describe(automation: &IUIAutomation, window: HWND) -> Result<serde_json::Value, String> {
    let mut point = POINT::default();
    unsafe { GetCursorPos(&mut point) }.map_err(|error| error.to_string())?;
    let element = unsafe { automation.ElementFromPoint(point) }.map_err(|error| error.to_string())?;
    let snapshot = snapshot(automation, window, MAX_NODES, MAX_DEPTH)?;
    let role = roles::role(unsafe { element.CurrentControlType() }.unwrap_or_default());
    let name = text(unsafe { element.CurrentName() });
    let automation_id = optional(unsafe { element.CurrentAutomationId() });
    let bounds = rect(unsafe { element.CurrentBoundingRectangle() });
    let mut candidates = Vec::new();
    for node in all(&snapshot.nodes) {
        if node.role == role
            && locator_same(&node.name, &name)
            && node.automation_id == automation_id
            && node.bounds == bounds
        {
            candidates.push(node.reference.clone());
        }
    }
    let (reference, confidence) = match candidates.len() {
        1 => (Some(candidates[0].clone()), "high"),
        0 => (None, "low"),
        _ => (None, "medium"),
    };
    let mut states = Vec::new();
    if flag(unsafe { element.CurrentIsEnabled() }) {
        states.push("enabled".to_owned());
    }
    if flag(unsafe { element.CurrentIsOffscreen() }) {
        states.push("offscreen".to_owned());
    }
    if flag(unsafe { element.CurrentIsPassword() }) {
        states.push("password".to_owned());
    }
    Ok(serde_json::json!({
        "element": {
            "reference": reference,
            "role": role,
            "name": name,
            "value": if flag(unsafe { element.CurrentIsPassword() }) { None } else { value(&element) },
            "bounds": bounds,
            "states": states,
            "actions": actions(&element),
            "automationId": automation_id,
            "className": optional(unsafe { element.CurrentClassName() }),
            "path": format!("{role}[{name}]"),
            "source": "uia",
            "confidence": confidence,
        },
        "reference": reference,
        "confidence": confidence,
        "candidates": candidates.len(),
        "window": snapshot.title,
    }))
}

fn all(nodes: &[UiNode]) -> Vec<&UiNode> {
    let mut collected = Vec::new();
    for node in nodes {
        collected.push(node);
        collected.extend(all(&node.children));
    }
    collected
}

fn locator_same(left: &str, right: &str) -> bool {
    left.trim().eq_ignore_ascii_case(right.trim())
}

pub fn warnings(actionable: usize, truncated: bool) -> Vec<String> {
    let mut messages = Vec::new();
    if actionable == 0 {
        messages.push("The application exposes no accessible elements".to_owned());
    }
    if truncated {
        messages.push("The snapshot was truncated at the node or depth limit".to_owned());
    }
    messages
}

struct Builder {
    condition: IUIAutomationCondition,
    max_nodes: usize,
    max_depth: usize,
    count: usize,
    actionable: usize,
    truncated: bool,
}

impl Builder {
    fn node(
        &mut self,
        element: &IUIAutomationElement,
        parent: Option<&str>,
        parent_path: &str,
        depth: usize,
    ) -> Result<UiNode, String> {
        self.count += 1;
        let reference = format!("n{}", self.count);
        let role = roles::role(unsafe { element.CurrentControlType() }.unwrap_or_default());
        let name = text(unsafe { element.CurrentName() });
        let password = flag(unsafe { element.CurrentIsPassword() });
        let mut states = Vec::new();
        if flag(unsafe { element.CurrentIsEnabled() }) {
            states.push("enabled".to_owned());
        }
        if flag(unsafe { element.CurrentIsOffscreen() }) {
            states.push("offscreen".to_owned());
        }
        if flag(unsafe { element.CurrentHasKeyboardFocus() }) {
            states.push("focused".to_owned());
        }
        if password {
            states.push("password".to_owned());
        }
        let actions = actions(element);
        if !actions.is_empty() {
            self.actionable += 1;
        }
        let path = if parent_path.is_empty() {
            format!("{role}[{name}]")
        } else {
            format!("{parent_path}/{role}[{name}]")
        };
        let mut children = Vec::new();
        if depth < self.max_depth {
            let found = unsafe { element.FindAll(TreeScope_Children, &self.condition) }
                .map_err(|error| error.to_string())?;
            let length = unsafe { found.Length() }.unwrap_or(0);
            for index in 0..length {
                if self.count >= self.max_nodes {
                    self.truncated = true;
                    break;
                }
                if let Ok(child) = unsafe { found.GetElement(index) } {
                    children.push(self.node(&child, Some(&reference), &path, depth + 1)?);
                }
            }
        } else if has_children(element, &self.condition) {
            self.truncated = true;
        }
        Ok(UiNode {
            reference,
            parent: parent.map(str::to_owned),
            role: role.to_owned(),
            name,
            value: if password { None } else { value(element) },
            bounds: rect(unsafe { element.CurrentBoundingRectangle() }),
            states,
            actions,
            automation_id: optional(unsafe { element.CurrentAutomationId() }),
            class_name: optional(unsafe { element.CurrentClassName() }),
            path,
            source: Provider::Uia,
            confidence: Confidence::High,
            children,
        })
    }
}

fn has_children(element: &IUIAutomationElement, condition: &IUIAutomationCondition) -> bool {
    unsafe { element.FindAll(TreeScope_Children, condition) }
        .map(|found| unsafe { found.Length() }.unwrap_or(0) > 0)
        .unwrap_or(false)
}

fn actions(element: &IUIAutomationElement) -> Vec<String> {
    let mut actions = Vec::new();
    unsafe {
        if element
            .GetCurrentPatternAs::<IUIAutomationInvokePattern>(UIA_InvokePatternId)
            .is_ok()
        {
            actions.push("invoke".to_owned());
        }
        if element
            .GetCurrentPatternAs::<IUIAutomationValuePattern>(UIA_ValuePatternId)
            .is_ok()
        {
            actions.push("setValue".to_owned());
        }
        if element
            .GetCurrentPatternAs::<IUIAutomationTogglePattern>(UIA_TogglePatternId)
            .is_ok()
        {
            actions.push("toggle".to_owned());
        }
        if element
            .GetCurrentPatternAs::<IUIAutomationSelectionItemPattern>(UIA_SelectionItemPatternId)
            .is_ok()
        {
            actions.push("select".to_owned());
        }
        if element
            .GetCurrentPatternAs::<IUIAutomationExpandCollapsePattern>(UIA_ExpandCollapsePatternId)
            .is_ok()
        {
            actions.push("expand".to_owned());
        }
        if element
            .GetCurrentPatternAs::<IUIAutomationScrollPattern>(UIA_ScrollPatternId)
            .is_ok()
        {
            actions.push("scroll".to_owned());
        }
    }
    actions
}

fn value(element: &IUIAutomationElement) -> Option<String> {
    let pattern = unsafe {
        element
            .GetCurrentPatternAs::<IUIAutomationValuePattern>(UIA_ValuePatternId)
            .ok()?
    };
    let value = unsafe { pattern.CurrentValue() }.ok()?.to_string();
    if value.is_empty() {
        None
    } else {
        Some(value)
    }
}

fn text(value: WindowsResult<BSTR>) -> String {
    value.map(|value| value.to_string()).unwrap_or_default()
}

fn optional(value: WindowsResult<BSTR>) -> Option<String> {
    let value = text(value);
    if value.is_empty() {
        None
    } else {
        Some(value)
    }
}

fn flag(value: WindowsResult<BOOL>) -> bool {
    value.map(|value| value.as_bool()).unwrap_or(false)
}

fn rect(value: WindowsResult<RECT>) -> UiBounds {
    match value {
        Ok(rect) => UiBounds {
            x: rect.left,
            y: rect.top,
            width: rect.right - rect.left,
            height: rect.bottom - rect.top,
        },
        Err(_) => UiBounds::default(),
    }
}

fn now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|value| value.as_secs())
        .unwrap_or(0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn an_application_without_accessible_elements_is_reported() {
        let messages = warnings(0, false);
        assert_eq!(messages.len(), 1);
        assert!(messages[0].contains("no accessible elements"), "{messages:?}");
    }

    #[test]
    fn a_populated_tree_has_no_warnings() {
        assert!(warnings(12, false).is_empty());
    }

    #[test]
    fn a_truncated_snapshot_is_reported() {
        let messages = warnings(12, true);
        assert_eq!(messages.len(), 1);
        assert!(messages[0].contains("truncated"), "{messages:?}");
    }
}
