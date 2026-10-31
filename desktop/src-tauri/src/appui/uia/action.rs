use std::collections::VecDeque;

use ::windows::core::BSTR;
use ::windows::Win32::Foundation::HWND;
use ::windows::Win32::UI::Accessibility::{
    IUIAutomation, IUIAutomationCondition, IUIAutomationElement,
    IUIAutomationExpandCollapsePattern, IUIAutomationInvokePattern, IUIAutomationScrollPattern,
    IUIAutomationSelectionItemPattern, IUIAutomationTogglePattern, IUIAutomationValuePattern,
    ScrollAmount, ScrollAmount_LargeDecrement, ScrollAmount_LargeIncrement,
    ScrollAmount_SmallDecrement, ScrollAmount_SmallIncrement, TreeScope_Children,
    UIA_ExpandCollapsePatternId, UIA_InvokePatternId, UIA_ScrollPatternId,
    UIA_SelectionItemPatternId, UIA_TogglePatternId, UIA_ValuePatternId,
};
use serde_json::{json, Value};

use super::{roles, tree};
use crate::appui::locator;
use crate::appui::types::{Locator, UiNode};

pub const MAX_NODES: usize = 4000;
pub const MAX_DEPTH: usize = 40;

pub enum UiAction {
    Invoke,
    SetValue(String),
    Toggle,
    Select,
    Focus,
    Expand,
    Collapse,
    Scroll(String, String),
}

impl UiAction {
    pub fn name(&self) -> &'static str {
        match self {
            Self::Invoke => "click",
            Self::SetValue(_) => "set_value",
            Self::Toggle => "toggle",
            Self::Select => "select",
            Self::Focus => "focus",
            Self::Expand => "expand",
            Self::Collapse => "collapse",
            Self::Scroll(_, _) => "scroll",
        }
    }
}

pub fn perform(
    automation: &IUIAutomation,
    window: HWND,
    locator: &Locator,
    action: UiAction,
) -> Result<Value, String> {
    let snapshot = tree::snapshot(automation, window, MAX_NODES, MAX_DEPTH)?;
    let reference = locator::resolve(&snapshot, locator)?;
    let expected = find(&snapshot.nodes, &reference)
        .ok_or_else(|| "The element is no longer available; take a new snapshot".to_owned())?
        .clone();
    let element = element_by_index(automation, window, index_of(&reference)?)?;
    verify(&element, &expected)?;
    let confidence = apply(&element, &action, window, &expected)?;
    Ok(json!({
        "status": "ok",
        "ref": reference,
        "role": expected.role,
        "name": expected.name,
        "action": action.name(),
        "confidence": confidence,
    }))
}

pub fn bounds_of(
    automation: &IUIAutomation,
    window: HWND,
    locator: &Locator,
) -> Result<(UiNode, String), String> {
    let snapshot = tree::snapshot(automation, window, MAX_NODES, MAX_DEPTH)?;
    let reference = locator::resolve(&snapshot, locator)?;
    let node = find(&snapshot.nodes, &reference)
        .ok_or_else(|| "The element is no longer available; take a new snapshot".to_owned())?
        .clone();
    Ok((node, reference))
}

fn apply(
    element: &IUIAutomationElement,
    action: &UiAction,
    window: HWND,
    node: &UiNode,
) -> Result<&'static str, String> {
    unsafe {
        match action {
            UiAction::Invoke => {
                if let Ok(pattern) =
                    element.GetCurrentPatternAs::<IUIAutomationInvokePattern>(UIA_InvokePatternId)
                {
                    pattern.Invoke().map_err(|error| error.to_string())?;
                    return Ok("high");
                }
                click_center(window, node)?;
                Ok("medium")
            }
            UiAction::SetValue(value) => {
                let pattern = element
                    .GetCurrentPatternAs::<IUIAutomationValuePattern>(UIA_ValuePatternId)
                    .map_err(|_| "The element does not accept a value".to_owned())?;
                pattern
                    .SetValue(&BSTR::from(value.as_str()))
                    .map_err(|error| error.to_string())?;
                Ok("high")
            }
            UiAction::Toggle => {
                let pattern = element
                    .GetCurrentPatternAs::<IUIAutomationTogglePattern>(UIA_TogglePatternId)
                    .map_err(|_| "The element cannot be toggled".to_owned())?;
                pattern.Toggle().map_err(|error| error.to_string())?;
                Ok("high")
            }
            UiAction::Select => {
                let pattern = element
                    .GetCurrentPatternAs::<IUIAutomationSelectionItemPattern>(
                        UIA_SelectionItemPatternId,
                    )
                    .map_err(|_| "The element cannot be selected".to_owned())?;
                pattern.Select().map_err(|error| error.to_string())?;
                Ok("high")
            }
            UiAction::Focus => {
                element.SetFocus().map_err(|error| error.to_string())?;
                Ok("high")
            }
            UiAction::Expand | UiAction::Collapse => {
                let pattern = element
                    .GetCurrentPatternAs::<IUIAutomationExpandCollapsePattern>(
                        UIA_ExpandCollapsePatternId,
                    )
                    .map_err(|_| "The element cannot be expanded or collapsed".to_owned())?;
                if matches!(action, UiAction::Expand) {
                    pattern.Expand().map_err(|error| error.to_string())?;
                } else {
                    pattern.Collapse().map_err(|error| error.to_string())?;
                }
                Ok("high")
            }
            UiAction::Scroll(direction, amount) => {
                let pattern = element
                    .GetCurrentPatternAs::<IUIAutomationScrollPattern>(UIA_ScrollPatternId)
                    .map_err(|_| "The element cannot be scrolled".to_owned())?;
                let step = scroll_amount(direction, amount)?;
                let (horizontal, vertical) = match direction.as_str() {
                    "left" | "right" => (step, ScrollAmount(2)),
                    _ => (ScrollAmount(2), step),
                };
                pattern
                    .Scroll(horizontal, vertical)
                    .map_err(|error| error.to_string())?;
                Ok("high")
            }
        }
    }
}

fn scroll_amount(direction: &str, amount: &str) -> Result<ScrollAmount, String> {
    let large = amount.eq_ignore_ascii_case("large");
    match direction.to_ascii_lowercase().as_str() {
        "up" | "left" => Ok(if large {
            ScrollAmount_LargeDecrement
        } else {
            ScrollAmount_SmallDecrement
        }),
        "down" | "right" => Ok(if large {
            ScrollAmount_LargeIncrement
        } else {
            ScrollAmount_SmallIncrement
        }),
        _ => Err("Scroll direction must be up, down, left or right".to_owned()),
    }
}

fn click_center(window: HWND, node: &UiNode) -> Result<(), String> {
    let hwnd = (window.0 as usize).to_string();
    let info = crate::computer::windows::info(&hwnd)?;
    let x = node.bounds.x - info.bounds.x + node.bounds.width / 2;
    let y = node.bounds.y - info.bounds.y + node.bounds.height / 2;
    crate::computer::input::perform(
        &hwnd,
        &crate::computer::types::Action::Click {
            x,
            y,
            button: None,
            count: None,
        },
        || Ok(()),
    )
}

fn verify(element: &IUIAutomationElement, expected: &UiNode) -> Result<(), String> {
    let role = roles::role(unsafe { element.CurrentControlType() }.unwrap_or_default());
    let name = unsafe { element.CurrentName() }
        .map(|value| value.to_string())
        .unwrap_or_default();
    let automation_id = unsafe { element.CurrentAutomationId() }
        .map(|value| value.to_string())
        .ok()
        .filter(|value| !value.is_empty());
    if role != expected.role
        || !locator::same(&name, &expected.name)
        || automation_id != expected.automation_id
    {
        return Err("The element changed; take a new snapshot".to_owned());
    }
    Ok(())
}

fn index_of(reference: &str) -> Result<usize, String> {
    reference
        .strip_prefix('n')
        .and_then(|value| value.parse::<usize>().ok())
        .and_then(|value| value.checked_sub(1))
        .ok_or_else(|| "Invalid element reference".to_owned())
}

fn find<'a>(nodes: &'a [UiNode], reference: &str) -> Option<&'a UiNode> {
    for node in nodes {
        if node.reference == reference {
            return Some(node);
        }
        if let Some(found) = find(&node.children, reference) {
            return Some(found);
        }
    }
    None
}

fn element_by_index(
    automation: &IUIAutomation,
    window: HWND,
    index: usize,
) -> Result<IUIAutomationElement, String> {
    let root = unsafe { automation.ElementFromHandle(window) }.map_err(|error| error.to_string())?;
    let condition: IUIAutomationCondition =
        unsafe { automation.CreateTrueCondition() }.map_err(|error| error.to_string())?;
    let mut queue = VecDeque::new();
    queue.push_back(root);
    let mut position = 0usize;
    while let Some(element) = queue.pop_front() {
        if position == index {
            return Ok(element);
        }
        position += 1;
        let found = unsafe { element.FindAll(TreeScope_Children, &condition) }
            .map_err(|error| error.to_string())?;
        let length = unsafe { found.Length() }.unwrap_or(0);
        for child in 0..length {
            if let Ok(element) = unsafe { found.GetElement(child) } {
                queue.push_back(element);
            }
        }
    }
    Err("The element is no longer available; take a new snapshot".to_owned())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn references_map_to_breadth_first_indexes() {
        assert_eq!(index_of("n1").unwrap(), 0);
        assert_eq!(index_of("n7").unwrap(), 6);
        assert!(index_of("7").is_err());
        assert!(index_of("n0").is_err());
        assert!(index_of("n").is_err());
    }

    #[test]
    fn scroll_directions_map_to_pattern_amounts() {
        assert_eq!(scroll_amount("down", "small").unwrap(), ScrollAmount_SmallIncrement);
        assert_eq!(scroll_amount("UP", "large").unwrap(), ScrollAmount_LargeDecrement);
        assert_eq!(scroll_amount("right", "large").unwrap(), ScrollAmount_LargeIncrement);
        assert_eq!(scroll_amount("left", "small").unwrap(), ScrollAmount_SmallDecrement);
        assert!(scroll_amount("sideways", "small").is_err());
    }

    #[test]
    fn action_names_are_stable() {
        assert_eq!(UiAction::Invoke.name(), "click");
        assert_eq!(UiAction::SetValue(String::new()).name(), "set_value");
        assert_eq!(UiAction::Scroll(String::new(), String::new()).name(), "scroll");
    }
}
