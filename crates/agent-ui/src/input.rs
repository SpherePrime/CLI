use fuzzy_matcher::skim::SkimMatcherV2;
use fuzzy_matcher::FuzzyMatcher;
use std::collections::HashMap;

pub struct SlashCommand {
    pub name: String,
    pub description: String,
    pub handler: Option<String>,
}

pub struct SlashCommandPalette {
    pub commands: Vec<SlashCommand>,
}

impl SlashCommandPalette {
    pub fn new() -> Self {
        Self {
            commands: vec![
                SlashCommand {
                    name: "/help".into(),
                    description: "Show help".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/clear".into(),
                    description: "Clear messages".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/compact".into(),
                    description: "Compact context".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/context".into(),
                    description: "Show context".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/model".into(),
                    description: "Switch model".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/tools".into(),
                    description: "List tools".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/mcp".into(),
                    description: "Manage MCP".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/skills".into(),
                    description: "Manage skills".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/plugins".into(),
                    description: "Manage plugins".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/config".into(),
                    description: "Show config".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/status".into(),
                    description: "Show status".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/diff".into(),
                    description: "Show diff".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/undo".into(),
                    description: "Undo last change".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/retry".into(),
                    description: "Retry last step".into(),
                    handler: None,
                },
                SlashCommand {
                    name: "/exit".into(),
                    description: "Exit".into(),
                    handler: None,
                },
            ],
        }
    }

    pub fn add(&mut self, cmd: SlashCommand) {
        self.commands.push(cmd);
    }

    pub fn execute(&self, input: &str) -> Option<String> {
        let name = input.split_whitespace().next()?.to_string();
        let _desc = self.commands.iter().find(|c| c.name == name).map(|c| c.description.clone());
        Some(name)
    }

    pub fn complete(&self, prefix: &str) -> Vec<String> {
        if prefix.is_empty() {
            return Vec::new();
        }
        self.commands
            .iter()
            .filter(|c| c.name.starts_with(prefix))
            .map(|c| c.name.clone())
            .collect()
    }

    pub fn all_names(&self) -> Vec<String> {
        self.commands.iter().map(|c| c.name.clone()).collect()
    }
}

impl Default for SlashCommandPalette {
    fn default() -> Self {
        Self::new()
    }
}

pub struct Autocomplete {
    pub history: Vec<String>,
}

impl Autocomplete {
    pub fn new() -> Self {
        Self { history: Vec::new() }
    }

    pub fn push(&mut self, input: &str) {
        if !input.is_empty() {
            if self.history.last().map(|s| s.as_str()) != Some(input) {
                self.history.push(input.to_string());
            }
        }
    }

    pub fn suggest(&self, prefix: &str, max: usize) -> Vec<String> {
        self.history
            .iter()
            .filter(|s| s.starts_with(prefix))
            .map(|s| s.clone())
            .take(max)
            .collect()
    }
}

impl Default for Autocomplete {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn slash_completes() {
        let p = SlashCommandPalette::new();
        let candidates = p.complete("/c");
        assert!(candidates.iter().any(|s| s == "/clear"));
    }

    #[test]
    fn autocomplete_history() {
        let mut a = Autocomplete::new();
        a.push("hello");
        a.push("hello");
        a.push("hey");
        assert_eq!(a.suggest("hel", 5).len(), 1);
    }
}
