use serde_json::Value;

pub struct Area {
    pub name: &'static str,
    pub prefixes: &'static [&'static str],
    pub summary: &'static str,
}

pub const AREAS: &[Area] = &[
    Area {
        name: "browser",
        prefixes: &["browser_"],
        summary: "Prime built-in browser: open, read and operate web pages. Page tools without tabId use this chat's own background tab; use browser_snapshot refs to click and type. Verify websites and web apps here, never with app_* tools.",
    },
    Area {
        name: "project",
        prefixes: &["desktop_", "context_", "launch_", "automation_"],
        summary: "Prime Desktop project features: capability discovery, persistent working context, saved app launch commands and scheduled automations.",
    },
    Area {
        name: "terminal",
        prefixes: &["terminal_"],
        summary: "Project terminals: terminal_create opens an agent-owned terminal; user-owned terminals need explicit user handover before writes.",
    },
    Area {
        name: "git",
        prefixes: &["git_", "github_"],
        summary: "Git history, commit inspection, undo/restore commits and GitHub pull requests and comments.",
    },
    Area {
        name: "ssh",
        prefixes: &["ssh_"],
        summary: "Saved SSH connections: remote commands, files, search, uploads, downloads and port forwards.",
    },
    Area {
        name: "windows",
        prefixes: &["window_"],
        summary: "Desktop windows: list windows, request access, view and operate only the window granted to this chat.",
    },
    Area {
        name: "apps",
        prefixes: &["app_"],
        summary: "Testing native desktop apps (Windows, Electron, Tauri), not websites: app_launch, app_find and app_* actions (changes come back as text), app_audit, app_check for each verified behaviour, app_close.",
    },
];

pub fn find(name: &str) -> Option<&'static Area> {
    AREAS.iter().find(|area| area.name == name)
}

pub fn names() -> String {
    AREAS
        .iter()
        .map(|area| area.name)
        .collect::<Vec<_>>()
        .join(",")
}

pub fn contains(area: &Area, tool: &str) -> bool {
    area.prefixes.iter().any(|prefix| tool.starts_with(prefix))
}

pub fn filter(area: Option<&Area>, definitions: Vec<Value>) -> Vec<Value> {
    match area {
        None => definitions,
        Some(area) => definitions
            .into_iter()
            .filter(|tool| {
                tool["name"]
                    .as_str()
                    .is_some_and(|name| contains(area, name))
            })
            .collect(),
    }
}

pub fn from_path(path: &str) -> Result<Option<&'static Area>, ()> {
    match path {
        "/mcp" => Ok(None),
        _ => path
            .strip_prefix("/mcp/")
            .and_then(find)
            .map(Some)
            .ok_or(()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn every_desktop_tool_belongs_to_exactly_one_area() {
        let mut definitions = super::super::tools::definitions();
        definitions.extend(super::super::workbench::definitions());
        for tool in definitions {
            let name = tool["name"].as_str().unwrap();
            let owners = AREAS.iter().filter(|area| contains(area, name)).count();
            assert_eq!(owners, 1, "{name}");
        }
    }

    #[test]
    fn paths_select_areas() {
        assert!(from_path("/mcp").unwrap().is_none());
        assert_eq!(from_path("/mcp/apps").unwrap().unwrap().name, "apps");
        assert!(from_path("/mcp/unknown").is_err());
        assert!(from_path("/other").is_err());
    }

    #[test]
    fn filter_keeps_only_area_tools() {
        let tools = vec![json!({"name":"browser_click"}), json!({"name":"app_find"})];
        let kept = filter(find("apps"), tools);
        assert_eq!(kept.len(), 1);
        assert_eq!(kept[0]["name"], "app_find");
    }

    #[test]
    fn names_are_comma_separated() {
        assert!(names().starts_with("browser,project,"));
    }
}
