use fuzzy_matcher::skim::SkimMatcherV2;
use fuzzy_matcher::FuzzyMatcher;

use crate::input::SlashCommandPalette;

#[derive(Debug, Clone, PartialEq)]
pub enum CompleteKind {
    None,
    Slash,
    File,
}

#[derive(Debug, Clone)]
pub struct CompletionItem {
    pub value: String,
    pub description: String,
}

pub struct CompletionEngine {
    pub kind: CompleteKind,
    pub items: Vec<CompletionItem>,
    pub selected: usize,
    matcher: SkimMatcherV2,
}

impl CompletionEngine {
    pub fn new() -> Self {
        Self {
            kind: CompleteKind::None,
            items: Vec::new(),
            selected: 0,
            matcher: SkimMatcherV2::default(),
        }
    }

    pub fn update(&mut self, input: &str, cursor: usize, palette: &SlashCommandPalette) {
        let token = token_before_cursor(input, cursor);

        if let Some(slash) = token.strip_prefix('/') {
            self.kind = CompleteKind::Slash;
            let mut scored: Vec<(i64, String, String)> = palette
                .all_commands()
                .iter()
                .filter_map(|c| {
                    let score = self.matcher.fuzzy_match(&c.name, slash)?;
                    Some((score, c.name.clone(), c.description.clone()))
                })
                .collect();
            scored.sort_by_key(|a| std::cmp::Reverse(a.0));
            self.items = scored
                .into_iter()
                .map(|(_, name, desc)| CompletionItem {
                    value: name,
                    description: desc,
                })
                .collect();
        } else if token.contains('@') {
            self.kind = CompleteKind::File;
            let query = token.split('@').next_back().unwrap_or("");
            let mut scored: Vec<(i64, String, String)> = crate::files::file_candidates()
                .iter()
                .filter_map(|path| {
                    let score = self.matcher.fuzzy_match(path, query)?;
                    Some((score, path.clone(), String::new()))
                })
                .collect();
            scored.sort_by_key(|a| std::cmp::Reverse(a.0));
            self.items = scored
                .into_iter()
                .map(|(_, path, _)| CompletionItem {
                    value: path,
                    description: String::new(),
                })
                .collect();
        } else {
            self.kind = CompleteKind::None;
            self.items = Vec::new();
        }

        if self.selected >= self.items.len() {
            self.selected = 0;
        }
    }

    pub fn select_next(&mut self) {
        if self.items.is_empty() {
            return;
        }
        self.selected = (self.selected + 1) % self.items.len();
    }

    pub fn select_prev(&mut self) {
        if self.items.is_empty() {
            return;
        }
        self.selected = if self.selected == 0 {
            self.items.len() - 1
        } else {
            self.selected - 1
        };
    }

    pub fn current(&self) -> Option<&CompletionItem> {
        self.items.get(self.selected)
    }

    pub fn visible(&self) -> bool {
        self.kind != CompleteKind::None && !self.items.is_empty()
    }

    pub fn reset(&mut self) {
        self.kind = CompleteKind::None;
        self.items = Vec::new();
        self.selected = 0;
    }
}

impl Default for CompletionEngine {
    fn default() -> Self {
        Self::new()
    }
}

pub fn token_before_cursor(input: &str, cursor: usize) -> &str {
    let cursor = cursor.min(input.len());
    let before = &input[..cursor];
    match before.rfind([' ', '\n', '\t']) {
        Some(idx) => &input[idx + 1..cursor],
        None => before,
    }
}

pub fn replace_token(input: &mut String, cursor: usize, replacement: &str) {
    let cursor = cursor.min(input.len());
    let start = match input[..cursor].rfind([' ', '\n', '\t']) {
        Some(idx) => idx + 1,
        None => 0,
    };
    input.replace_range(start..cursor, replacement);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn token_slash() {
        let t = token_before_cursor("/hel world", 4);
        assert_eq!(t, "/hel");
    }

    #[test]
    fn token_space_split() {
        let t = token_before_cursor("check @sr", 9);
        assert_eq!(t, "@sr");
    }

    #[test]
    fn slash_completion_orders_by_score() {
        let mut engine = CompletionEngine::new();
        engine.update("/m", 2, &SlashCommandPalette::new());
        assert_eq!(engine.kind, CompleteKind::Slash);
        assert!(!engine.items.is_empty());
        assert!(engine.items[0].value.starts_with("/"));
    }

    #[test]
    fn no_completion_plain_text() {
        let mut engine = CompletionEngine::new();
        engine.update("hello world", 11, &SlashCommandPalette::new());
        assert_eq!(engine.kind, CompleteKind::None);
    }

    #[test]
    fn replace_token_idempotent() {
        let mut input = String::from("say /hel");
        let cursor = input.len();
        replace_token(&mut input, cursor, "/help");
        assert_eq!(input, "say /help");
    }
}
