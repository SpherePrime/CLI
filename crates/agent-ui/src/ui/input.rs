use crate::input::SlashCommandPalette;
use crossterm::event::{KeyCode, KeyModifiers};
use fuzzy_matcher::skim::SkimMatcherV2;
use fuzzy_matcher::FuzzyMatcher;
use ratatui::layout::Rect;
use ratatui::widgets::{Block, BorderType, List, ListItem, Paragraph};
use ratatui::Frame;

use super::Component;

pub struct InputScreen {
    pub input: String,
    pub cursor_pos: usize,
    pub suggestions: Vec<String>,
    pub selected_suggestion: usize,
    pub palette: SlashCommandPalette,
    matcher: SkimMatcherV2,
}

impl InputScreen {
    pub fn new() -> Self {
        Self {
            input: String::new(),
            cursor_pos: 0,
            suggestions: Vec::new(),
            selected_suggestion: 0,
            palette: SlashCommandPalette::new(),
            matcher: SkimMatcherV2::default(),
        }
    }

    pub fn set_input(&mut self, text: &str) {
        self.input = text.to_string();
        self.cursor_pos = self.byte_pos_for_char_pos(self.input.len());
        self.update_suggestions();
    }

    pub fn complete(&mut self, prefix: &str) -> Vec<String> {
        let candidates = self.palette.complete(prefix);
        self.suggestions = candidates;
        self.selected_suggestion = 0;
        self.suggestions.clone()
    }

    fn update_suggestions(&mut self) {
        let prefix = if self.input.starts_with('/') {
            self.input[1..].to_string()
        } else {
            self.input.clone()
        };

        if prefix.is_empty() {
            self.suggestions = self.palette.all_names();
            self.selected_suggestion = 0;
            return;
        }

        let mut matches: Vec<(i64, String)> = self
            .palette
            .all_names()
            .iter()
            .filter_map(|name| {
                self.matcher
                    .fuzzy_match(name, &prefix)
                    .map(|score| (score, name.clone()))
            })
            .filter(|(_, name)| name.starts_with(&prefix))
            .collect();

        matches.sort_by(|a, b| b.0.cmp(&a.0));

        self.suggestions = matches.into_iter().map(|(_, name)| name).collect();
        self.selected_suggestion = 0;
    }

    pub fn handle_input(&mut self, key: KeyCode, modifiers: KeyModifiers) -> Option<&str> {
        match key {
            KeyCode::Char(c) => {
                if modifiers.contains(KeyModifiers::CONTROL) {
                    match c {
                        'a' | 'A' => {
                            self.cursor_pos = 0;
                            return None;
                        }
                        'e' | 'E' => {
                            self.cursor_pos = self.input.len();
                            return None;
                        }
                        'u' | 'U' => {
                            let byte_pos = self.byte_pos_for_char_pos(self.cursor_pos);
                            self.input = format!("{}{}", &self.input[..byte_pos], &self.input[byte_pos..].chars().skip(1).collect::<String>());
                            self.update_suggestions();
                            return None;
                        }
                        _ => {}
                    }
                }

                let byte_pos = self.byte_pos_for_char_pos(self.cursor_pos);
                self.input.insert(byte_pos, c);
                self.cursor_pos = self.char_pos_for_byte_pos(byte_pos + c.len_utf8());
                self.update_suggestions();
                None
            }
            KeyCode::Backspace => {
                if self.input.is_empty() || self.cursor_pos == 0 {
                    return None;
                }
                
                let char_pos = self.cursor_pos.saturating_sub(1);
                let byte_pos = self.byte_pos_for_char_pos(char_pos);
                let next_byte_pos = self.byte_pos_for_char_pos(char_pos + 1).min(self.input.len());
                self.input.replace_range(byte_pos..next_byte_pos, "");
                self.cursor_pos = char_pos;
                self.update_suggestions();
                None
            }
            KeyCode::Delete => {
                if self.cursor_pos >= self.input.len() {
                    return None;
                }
                
                let byte_pos = self.byte_pos_for_char_pos(self.cursor_pos);
                let next_byte_pos = self.byte_pos_for_char_pos(self.cursor_pos + 1).min(self.input.len());
                self.input.replace_range(byte_pos..next_byte_pos, "");
                self.update_suggestions();
                None
            }
            KeyCode::Left => {
                if self.cursor_pos > 0 {
                    self.cursor_pos = self.cursor_pos.saturating_sub(1);
                }
                None
            }
            KeyCode::Right => {
                if self.cursor_pos < self.input.len() {
                    self.cursor_pos = self.cursor_pos + 1;
                }
                None
            }
            KeyCode::Home => {
                self.cursor_pos = 0;
                None
            }
            KeyCode::End => {
                self.cursor_pos = self.input.len();
                None
            }
            KeyCode::Up => {
                if !self.suggestions.is_empty() {
                    if self.selected_suggestion > 0 {
                        self.selected_suggestion -= 1;
                    }
                }
                None
            }
            KeyCode::Down => {
                if !self.suggestions.is_empty() {
                    if self.selected_suggestion < self.suggestions.len() - 1 {
                        self.selected_suggestion += 1;
                    }
                }
                None
            }
            KeyCode::Tab => {
                if !self.suggestions.is_empty() && self.selected_suggestion < self.suggestions.len() {
                    let prefix = self.suggestions[self.selected_suggestion].clone();
                    self.input = prefix;
                    self.cursor_pos = self.input.len();
                    self.suggestions.clear();
                    self.selected_suggestion = 0;
                }
                None
            }
            KeyCode::Enter => {
                Some(Box::leak(self.input.clone().into_boxed_str()))
            }
            KeyCode::Esc => {
                self.input.clear();
                self.cursor_pos = 0;
                self.suggestions.clear();
                None
            }
            _ => None,
        }
    }

    fn byte_pos_for_char_pos(&self, char_pos: usize) -> usize {
        if self.input.is_empty() || char_pos == 0 {
            return 0;
        }
        
        self.input.char_indices()
            .nth(char_pos.saturating_sub(1))
            .map(|(byte_idx, c)| byte_idx + c.len_utf8())
            .unwrap_or(self.input.len())
    }

    fn char_pos_for_byte_pos(&self, byte_pos: usize) -> usize {
        self.input.char_indices()
            .take_while(|(b, _)| *b < byte_pos)
            .count()
    }

    pub fn selected_value(&self) -> &str {
        if self.suggestions.is_empty() {
            &self.input
        } else if self.selected_suggestion < self.suggestions.len() {
            &self.suggestions[self.selected_suggestion]
        } else {
            &self.input
        }
    }
}

impl Default for InputScreen {
    fn default() -> Self {
        Self::new()
    }
}

impl Component for InputScreen {
    fn render(&self, frame: &mut Frame, area: Rect) -> Rect {
        let block = Block::default()
            .title("Input")
            .border_type(BorderType::Rounded);

        let inner_area = block.inner(area);
        frame.render_widget(block, area);

        let display_text = if self.input.is_empty() {
            "Type a message...".to_string()
        } else {
            self.input.clone()
        };

        let paragraph = Paragraph::new(display_text);
        frame.render_widget(paragraph, inner_area);

        let list_area = Rect {
            x: inner_area.x,
            y: inner_area.y.saturating_add(inner_area.height.saturating_sub(5)),
            width: inner_area.width,
            height: 5,
        };

        if !self.suggestions.is_empty() {
            let items: Vec<ListItem> = self
                .suggestions
                .iter()
                .enumerate()
                .map(|(i, s)| {
                    let content = if i == self.selected_suggestion {
                        format!("▶ {}", s)
                    } else {
                        format!("  {}", s)
                    };
                    ListItem::new(content)
                })
                .collect();

            let list = List::new(items).block(
                Block::default()
                    .title("Suggestions")
                    .border_type(BorderType::Rounded),
            );
            frame.render_widget(list, list_area);
        }

        inner_area
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_input_screen_new() {
        let input = InputScreen::new();
        assert!(input.input.is_empty());
        assert_eq!(input.cursor_pos, 0);
        assert!(input.suggestions.is_empty());
    }

    #[test]
    fn test_set_input() {
        let mut input = InputScreen::new();
        input.set_input("/hel");
        assert_eq!(input.input, "/hel");
        assert!(input.cursor_pos <= input.input.len());
    }

    #[test]
    fn test_complete_returns_suggestions() {
        let mut input = InputScreen::new();
        let suggestions = input.complete("/h");
        assert!(!suggestions.is_empty());
        assert!(suggestions.contains(&"/help".to_string()));
    }

    #[test]
    fn test_fuzzy_match() {
        let mut input = InputScreen::new();
        let suggestions = input.complete("/h");
        assert!(suggestions.iter().any(|s| s == "/help"));
    }
}