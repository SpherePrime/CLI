#![cfg_attr(not(windows), allow(dead_code))]

use crate::appui::types::Locator;

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

pub enum UiaRequest {
    Snapshot {
        hwnd: String,
        max_nodes: usize,
        max_depth: usize,
    },
    Act {
        hwnd: String,
        locator: Locator,
        action: UiAction,
    },
    ElementPng {
        hwnd: String,
        locator: Locator,
    },
    ElementAtPoint {
        hwnd: String,
        x: i32,
        y: i32,
    },
    BoundsAtPoint {
        x: i32,
        y: i32,
    },
}
