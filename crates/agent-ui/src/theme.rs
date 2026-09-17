use ratatui::style::Color;

#[derive(Debug, Clone, Default, PartialEq)]
pub struct Theme {
    pub background: Color,
    pub foreground: Color,
    pub accent: Color,
    pub error: Color,
    pub success: Color,
    pub warning: Color,
    pub info: Color,
}

impl Theme {
    pub fn default_light() -> Self {
        Self {
            background: Color::White,
            foreground: Color::Black,
            accent: Color::Blue,
            error: Color::Red,
            success: Color::Green,
            warning: Color::Yellow,
            info: Color::Cyan,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn theme_default() {
        let t = Theme::default();
        let _ = t;
    }
}
