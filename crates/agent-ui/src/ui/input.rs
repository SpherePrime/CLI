use crate::input::SlashCommandPalette;
use crossterm::event::{KeyCode, KeyModifiers};
use ratatui::layout::Rect;
use ratatui::widgets::{List, ListItem};
use ratatui::Frame;

use super::components::Component;

pub struct InputScreen {
    pub input: String,
    pub cursor_pos: usize,
    pub suggestions: Vec<String>,
    pub selected_suggestion: usize,
    pub palette: SlashCommandPalette,
}

impl InputScreen {
    pub fn new() -> Self {
        Self {
            input: String::new(),
            cursor_pos: 0,
            suggestions: Vec::new(),
            selected_suggestion: 0,
            palette: SlashCommandPalette::new(),
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
                // Simple prefix matching
                if name.starts_with(&prefix) {
                    Some((100, name.clone()))
                } else {
                    None
                }
            })
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
    fn render(&self, _frame: &mut Frame, area: Rect) -> Rect {
        // Render suggestions below the input area
        if !self.suggestions.is_empty() {
            let input_height = 1u16;
            let suggestions_area = Rect {
                x: area.x,
                y: area.y.saturating_add(input_height + 1),
                width: area.width,
                height: area.height.saturating_sub(input_height + 1).max(5),
            };

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

            let list = List::new(items);
            _frame.render_widget(list, suggestions_area);
        }

        area
    }
}