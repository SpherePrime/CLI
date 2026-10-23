use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Serialize)]
pub struct Bounds {
    pub x: i32,
    pub y: i32,
    pub width: i32,
    pub height: i32,
}

#[derive(Clone, Debug, Serialize)]
pub struct WindowInfo {
    pub hwnd: String,
    pub title: String,
    pub pid: u32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub identity: Option<String>,
    pub bounds: Bounds,
    #[serde(skip)]
    pub process_created: u64,
}

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum AccessMode {
    View,
    Control,
}

#[derive(Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Share {
    pub workspace_id: String,
    pub session_id: String,
    pub hwnd: String,
    pub title: String,
    pub pid: u32,
    pub identity: String,
    pub mode: AccessMode,
    pub shared_at: u64,
    #[serde(skip)]
    pub process_created: u64,
    #[serde(skip)]
    pub generation: String,
}

#[derive(Clone, Serialize)]
pub struct Log {
    pub time: u64,
    pub title: String,
    pub source: String,
    pub ok: bool,
    pub action: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

#[derive(Default)]
pub struct Workspace {
    pub share: Option<Share>,
    pub logs: Vec<Log>,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(tag = "action", rename_all = "snake_case", deny_unknown_fields)]
pub enum Action {
    Focus {},
    Click {
        x: i32,
        y: i32,
        button: Option<String>,
        count: Option<u8>,
    },
    Type {
        text: String,
    },
    PressKey {
        key: String,
    },
    Scroll {
        x: i32,
        y: i32,
        delta: i32,
    },
}

impl Action {
    pub fn name(&self) -> &'static str {
        match self {
            Self::Focus {} => "focus",
            Self::Click { .. } => "click",
            Self::Type { .. } => "type",
            Self::PressKey { .. } => "press_key",
            Self::Scroll { .. } => "scroll",
        }
    }
    pub fn validate(&self) -> Result<(), String> {
        match self {
            Self::Click {
                x,
                y,
                button,
                count,
            } => {
                if !(0..=100000).contains(x)
                    || !(0..=100000).contains(y)
                    || button
                        .as_deref()
                        .is_some_and(|button| !["left", "right"].contains(&button))
                    || count.is_some_and(|count| !(1..=2).contains(&count))
                {
                    return Err("Invalid window click".to_owned());
                }
            }
            Self::Type { text } if text.is_empty() || text.encode_utf16().count() > 1000 => {
                return Err("Window text must contain 1–1000 UTF-16 units".to_owned())
            }
            Self::PressKey { key } => {
                if key.is_empty() || key.len() > 60 {
                    return Err("Invalid window key combination".to_owned());
                }
                key_codes(key)?;
            }
            Self::Scroll { x, y, delta }
                if !(0..=100000).contains(x)
                    || !(0..=100000).contains(y)
                    || *delta == 0
                    || !(-2400..=2400).contains(delta) =>
            {
                return Err("Invalid window scroll".to_owned())
            }
            _ => {}
        }
        Ok(())
    }
}

pub fn key_codes(value: &str) -> Result<Vec<u16>, String> {
    let uppercase = value.to_ascii_uppercase();
    let keys: Vec<&str> = uppercase.split('+').map(str::trim).collect();
    if keys.len() > 4 || keys.iter().any(|key| key.is_empty()) {
        return Err("Invalid key combination".to_owned());
    }
    let contains = |key| keys.contains(&key);
    if (contains("ALT") && (contains("TAB") || contains("ESC") || contains("ESCAPE")))
        || ((contains("CTRL") || contains("CONTROL"))
            && (contains("ESC") || contains("ESCAPE") || (contains("ALT") && contains("DELETE"))))
    {
        return Err("Desktop-wide shortcuts are not allowed for window control".to_owned());
    }
    keys.into_iter()
        .map(|key| match key {
            "ENTER" | "RETURN" => Ok(13),
            "TAB" => Ok(9),
            "ESC" | "ESCAPE" => Ok(27),
            "BACKSPACE" => Ok(8),
            "DELETE" => Ok(46),
            "SPACE" => Ok(32),
            "HOME" => Ok(36),
            "END" => Ok(35),
            "PAGEUP" => Ok(33),
            "PAGEDOWN" => Ok(34),
            "LEFT" => Ok(37),
            "UP" => Ok(38),
            "RIGHT" => Ok(39),
            "DOWN" => Ok(40),
            "CTRL" | "CONTROL" => Ok(17),
            "SHIFT" => Ok(16),
            "ALT" => Ok(18),
            key if key.len() == 1 && key.as_bytes()[0].is_ascii_alphanumeric() => {
                Ok(key.as_bytes()[0] as u16)
            }
            key if key.starts_with('F') => key[1..]
                .parse::<u16>()
                .ok()
                .filter(|number| (1..=12).contains(number))
                .map(|number| 111 + number)
                .ok_or_else(|| format!("Unsupported key: {key}")),
            _ => Err(format!("Unsupported key: {key}")),
        })
        .collect()
}
