pub struct InputEditor {
    pub content: String,
    pub cursor: usize,
    history: Vec<String>,
    history_index: Option<usize>,
}

impl InputEditor {
    pub fn new() -> Self {
        Self {
            content: String::new(),
            cursor: 0,
            history: Vec::new(),
            history_index: None,
        }
    }

    pub fn insert_char(&mut self, c: char) {
        self.content.insert(self.cursor, c);
        self.cursor += c.len_utf8();
    }

    pub fn insert_str(&mut self, s: &str) {
        self.content.insert_str(self.cursor, s);
        self.cursor += s.len();
    }

    pub fn backspace(&mut self) {
        if self.cursor == 0 {
            return;
        }
        let prev = self
            .content
            .char_indices()
            .take_while(|(i, _)| *i < self.cursor)
            .last();
        match prev {
            Some((start, c)) => {
                self.content.replace_range(start..self.cursor, "");
                self.cursor = start;
                let _ = c;
            }
            None => {
                self.content.clear();
                self.cursor = 0;
            }
        }
    }

    pub fn delete(&mut self) {
        if self.cursor >= self.content.len() {
            return;
        }
        let next = self.content[self.cursor..].chars().next().map(|c| c.len_utf8());
        if let Some(len) = next {
            self.content.replace_range(self.cursor..self.cursor + len, "");
        }
    }

    pub fn delete_word(&mut self) {
        if self.cursor == 0 {
            return;
        }
        let before = &self.content[..self.cursor];
        let bytes = before.as_bytes();
        let mut end = before.len();
        while end > 0 {
            let c = before[..end].chars().next_back().unwrap();
            if !c.is_alphanumeric() && c != '_' {
                end -= c.len_utf8();
                break;
            }
            end -= c.len_utf8();
            let _ = bytes;
        }
        self.content.replace_range(end..self.cursor, "");
        self.cursor = end;
    }

    pub fn clear_line(&mut self) {
        self.content.clear();
        self.cursor = 0;
    }

    pub fn move_left(&mut self) {
        if let Some((start, _)) = self
            .content
            .char_indices()
            .take_while(|(i, _)| *i < self.cursor)
            .last()
        {
            self.cursor = start;
        }
    }

    pub fn move_right(&mut self) {
        if self.cursor >= self.content.len() {
            return;
        }
        if let Some(c) = self.content[self.cursor..].chars().next() {
            self.cursor += c.len_utf8();
        }
    }

    pub fn move_home(&mut self) {
        let line_start = self.content[..self.cursor]
            .rfind('\n')
            .map(|i| i + 1)
            .unwrap_or(0);
        self.cursor = line_start;
    }

    pub fn move_end(&mut self) {
        let line_end = self.content[self.cursor..]
            .find('\n')
            .map(|i| self.cursor + i)
            .unwrap_or(self.content.len());
        self.cursor = line_end;
    }

    pub fn prev_history(&mut self) {
        if self.history.is_empty() {
            return;
        }
        let idx = match self.history_index {
            None => {
                self.collect_current();
                self.history.len().saturating_sub(1)
            }
            Some(i) if i == 0 => 0,
            Some(i) => i - 1,
        };
        self.apply_history(idx);
    }

    pub fn next_history(&mut self) {
        if self.history.is_empty() {
            return;
        }
        match self.history_index {
            None => {}
            Some(i) if i + 1 >= self.history.len() => {
                self.history_index = None;
                self.content.clear();
                self.cursor = 0;
            }
            Some(i) => {
                let next = i + 1;
                self.apply_history(next);
            }
        }
    }

    fn collect_current(&mut self) {
        if !self.content.trim().is_empty() {
            self.history.push(self.content.clone());
        }
    }

    fn apply_history(&mut self, idx: usize) {
        self.history_index = Some(idx);
        let value = self.history[idx].clone();
        self.content = value;
        self.cursor = self.content.len();
    }

    pub fn submit(&mut self) -> String {
        let value = self.content.trim().to_string();
        if !value.is_empty() {
            if self.history.last().map(|s| s.as_str()) != Some(value.as_str()) {
                self.history.push(value.clone());
            }
        }
        self.history_index = None;
        self.content.clear();
        self.cursor = 0;
        value
    }

    pub fn set_content(&mut self, content: &str) {
        self.content = content.to_string();
        self.cursor = self.content.len();
    }
}

impl Default for InputEditor {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn insert_at_cursor() {
        let mut ed = InputEditor::new();
        ed.insert_str("helo");
        ed.cursor = 3;
        ed.insert_char('l');
        assert_eq!(ed.content, "hello");
        assert_eq!(ed.cursor, 4);
    }

    #[test]
    fn backspace_removes_char() {
        let mut ed = InputEditor::new();
        ed.insert_str("abc");
        ed.backspace();
        assert_eq!(ed.content, "ab");
        assert_eq!(ed.cursor, 2);
    }

    #[test]
    fn delete_word_backwards() {
        let mut ed = InputEditor::new();
        ed.insert_str("fix bug now");
        ed.cursor = ed.content.len();
        ed.delete_word();
        assert_eq!(ed.content, "fix bug");
    }

    #[test]
    fn history_roundtrip() {
        let mut ed = InputEditor::new();
        let first = ed.submit();
        assert!(first.is_empty());
        ed.insert_str("echo hi");
        let val = ed.submit();
        assert_eq!(val, "echo hi");
        ed.prev_history();
        assert_eq!(ed.content, "echo hi");
        ed.next_history();
        assert_eq!(ed.content, "");
    }

    #[test]
    fn multiline_home_end() {
        let mut ed = InputEditor::new();
        ed.insert_str("line1\nline2 long");
        ed.cursor = 0;
        ed.move_end();
        assert_eq!(ed.cursor, "line1".len());
        ed.cursor = ed.content.len();
        ed.move_home();
        assert_eq!(ed.cursor, "line1\n".len());
    }
}