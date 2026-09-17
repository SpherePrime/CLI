use ratatui::layout::Rect;
use ratatui::widgets::{Block, BorderType, Paragraph};
use ratatui::Frame;

pub trait Component {
    fn render(&self, frame: &mut Frame, area: Rect) -> Rect;
}

#[derive(Debug, Clone)]
pub struct InputField {
    pub label: String,
    pub placeholder: String,
    pub value: String,
}

impl InputField {
    pub fn new(label: &str) -> Self {
        Self {
            label: label.to_string(),
            placeholder: String::new(),
            value: String::new(),
        }
    }

    pub fn placeholder(mut self, placeholder: &str) -> Self {
        self.placeholder = placeholder.to_string();
        self
    }

    pub fn default(mut self, value: &str) -> Self {
        self.value = value.to_string();
        self
    }

    pub fn set_value(&mut self, value: &str) {
        self.value = value.to_string();
    }
}

impl Component for InputField {
    fn render(&self, frame: &mut Frame, area: Rect) -> Rect {
        let display_text = if self.value.is_empty() {
            self.placeholder.clone()
        } else {
            self.value.clone()
        };

        let block = Block::bordered()
            .title(self.label.as_str())
            .border_type(BorderType::Rounded);

        let inner_area = block.inner(area);
        frame.render_widget(block, area);

        let paragraph = Paragraph::new(display_text);
        frame.render_widget(paragraph, inner_area);

        inner_area
    }
}

#[derive(Debug, Clone)]
pub struct SelectList {
    pub label: String,
    pub options: Vec<String>,
    pub selected_index: usize,
}

impl SelectList {
    pub fn new(label: &str) -> Self {
        Self {
            label: label.to_string(),
            options: Vec::new(),
            selected_index: 0,
        }
    }

    pub fn options(mut self, options: Vec<&str>) -> Self {
        self.options = options.into_iter().map(|s| s.to_string()).collect();
        self
    }

    pub fn default(mut self, value: &str) -> Self {
        if let Some(idx) = self.options.iter().position(|o| o == value) {
            self.selected_index = idx;
        }
        self
    }

    pub fn selected(&self) -> Option<&str> {
        self.options.get(self.selected_index).map(|s| s.as_str())
    }

    pub fn set_selected(&mut self, index: usize) {
        if index < self.options.len() {
            self.selected_index = index;
        }
    }

    pub fn previous(&mut self) {
        if self.selected_index > 0 {
            self.selected_index -= 1;
        }
    }

    pub fn next(&mut self) {
        if self.selected_index < self.options.len().saturating_sub(1) {
            self.selected_index += 1;
        }
    }
}

impl Component for SelectList {
    fn render(&self, frame: &mut Frame, area: Rect) -> Rect {
        let selected = self.options.get(self.selected_index).map(|s| s.as_str()).unwrap_or("");

        let items: Vec<_> = self
            .options
            .iter()
            .enumerate()
            .map(|(i, opt)| {
                if i == self.selected_index {
                    format!("▶ {}", opt)
                } else {
                    format!("  {}", opt)
                }
            })
            .collect();

        let block = Block::bordered()
            .title(self.label.as_str())
            .border_type(BorderType::Rounded);

        let inner_area = block.inner(area);

        frame.render_widget(block, area);

        let list = ratatui::widgets::List::new(items).block(Block::bordered().title("Options"));
        frame.render_widget(list, inner_area);

        inner_area
    }
}

#[derive(Debug, Clone)]
pub struct FormField {
    pub input: InputField,
    pub select: SelectList,
}

impl FormField {
    pub fn new(input_label: &str, select_label: &str) -> Self {
        Self {
            input: InputField::new(input_label),
            select: SelectList::new(select_label),
        }
    }

    pub fn input(mut self, placeholder: &str, default: &str) -> Self {
        self.input = self.input.placeholder(placeholder).default(default);
        self
    }

    pub fn select(mut self, options: Vec<&str>, default: &str) -> Self {
        self.select = self.select.options(options).default(default);
        self
    }
}

impl Component for FormField {
    fn render(&self, frame: &mut Frame, area: Rect) -> Rect {
        let height = 3 + 5; // Input + Select borders
        let block = Block::bordered()
            .title("Form")
            .border_type(BorderType::Rounded);

        let area = block.inner(area);
        frame.render_widget(block, area);

        let input_height = 3;
        let select_area = Rect {
            x: area.x,
            y: area.y + input_height,
            width: area.width,
            height: area.height - input_height,
        };

        self.select.render(frame, select_area);

        area
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_input_field_new() {
        let input = InputField::new("prompt");
        assert_eq!(input.label, "prompt");
        assert_eq!(input.value, "");
        assert_eq!(input.placeholder, "");
    }

    #[test]
    fn test_input_field_with_default() {
        let input = InputField::new("prompt").default("value");
        assert_eq!(input.value, "value");
    }

    #[test]
    fn test_input_field_with_placeholder() {
        let input = InputField::new("prompt").placeholder("Enter text...");
        assert_eq!(input.placeholder, "Enter text...");
    }

    #[test]
    fn test_select_list_new() {
        let select = SelectList::new("model");
        assert_eq!(select.label, "model");
        assert!(select.options.is_empty());
    }

    #[test]
    fn test_select_list_with_options() {
        let select = SelectList::new("model").options(vec!["gpt-4", "gpt-3.5"]);
        assert_eq!(select.options.len(), 2);
        assert_eq!(select.selected_index, 0);
    }

    #[test]
    fn test_select_list_default() {
        let select = SelectList::new("model")
            .options(vec!["gpt-4", "gpt-3.5"])
            .default("gpt-3.5");
        assert_eq!(select.selected_index, 1);
    }

    #[test]
    fn test_select_list_navigation() {
        let mut select = SelectList::new("model").options(vec!["a", "b", "c"]);
        assert_eq!(select.selected_index, 0);

        select.next();
        assert_eq!(select.selected_index, 1);

        select.next();
        assert_eq!(select.selected_index, 2);

        select.next();
        assert_eq!(select.selected_index, 2); // Ступічка на максимумі

        select.previous();
        assert_eq!(select.selected_index, 1);
    }

    #[test]
    fn test_form_field_new() {
        let form = FormField::new("name", "type");
        assert_eq!(form.input.label, "name");
        assert_eq!(form.select.label, "type");
    }

    #[test]
    fn test_form_field_with_inputs() {
        let form = FormField::new("name", "type")
            .input("Enter name", "default")
            .select(vec!["a", "b"], "b");

        assert_eq!(form.input.placeholder, "Enter name");
        assert_eq!(form.input.value, "default");
        assert_eq!(form.select.selected_index, 1);
    }
}