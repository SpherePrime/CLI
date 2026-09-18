use ratatui::style::{Color, Style};

#[derive(Debug, Clone, Default, PartialEq)]
pub struct Theme {
    pub background: Color,
    pub foreground: Color,
    pub accent: Color,
    pub accent_light: Color,
    pub accent_dark: Color,
    pub border: Color,
    pub error: Color,
    pub success: Color,
    pub warning: Color,
    pub info: Color,
    pub message_user: Color,
    pub message_assistant: Color,
    pub message_tool: Color,
    pub status_ready: Color,
    pub status_thinking: Color,
    pub status_error: Color,
    pub status_success: Color,
}

impl Theme {
    pub fn opencode_dark() -> Self {
        Self {
            background: Color::Rgb(20, 22, 27),
            foreground: Color::Rgb(205, 214, 229),
            accent: Color::Rgb(79, 193, 255),
            accent_light: Color::Rgb(148, 163, 184),
            accent_dark: Color::Rgb(55, 65, 81),
            border: Color::Rgb(55, 65, 81),
            error: Color::Rgb(248, 113, 113),
            success: Color::Rgb(74, 222, 128),
            warning: Color::Rgb(245, 176, 65),
            info: Color::Rgb(125, 214, 199),
            message_user: Color::Rgb(129, 142, 178),
            message_assistant: Color::Rgb(166, 183, 218),
            message_tool: Color::Rgb(196, 161, 251),
            status_ready: Color::Rgb(79, 193, 255),
            status_thinking: Color::Rgb(245, 176, 65),
            status_error: Color::Rgb(248, 113, 113),
            status_success: Color::Rgb(74, 222, 128),
        }
    }

    pub fn dracula() -> Self {
        Self {
            background: Color::Rgb(24, 24, 24),
            foreground: Color::Rgb(248, 248, 242),
            accent: Color::Rgb(189, 147, 255),
            accent_light: Color::Rgb(156, 163, 175),
            accent_dark: Color::Rgb(45, 45, 45),
            border: Color::Rgb(63, 63, 70),
            error: Color::Rgb(248, 113, 113),
            success: Color::Rgb(145, 223, 187),
            warning: Color::Rgb(255, 184, 108),
            info: Color::Rgb(114, 182, 255),
            message_user: Color::Rgb(145, 223, 187),
            message_assistant: Color::Rgb(189, 147, 255),
            message_tool: Color::Rgb(255, 121, 178),
            status_ready: Color::Rgb(114, 182, 255),
            status_thinking: Color::Rgb(255, 184, 108),
            status_error: Color::Rgb(248, 113, 113),
            status_success: Color::Rgb(145, 223, 187),
        }
    }

    pub fn github_dark() -> Self {
        Self {
            background: Color::Rgb(13, 17, 24),
            foreground: Color::Rgb(225, 228, 236),
            accent: Color::Rgb(93, 221, 255),
            accent_light: Color::Rgb(158, 178, 207),
            accent_dark: Color::Rgb(30, 36, 48),
            border: Color::Rgb(48, 54, 61),
            error: Color::Rgb(220, 38, 38),
            success: Color::Rgb(34, 197, 94),
            warning: Color::Rgb(245, 176, 65),
            info: Color::Rgb(125, 214, 199),
            message_user: Color::Rgb(166, 183, 218),
            message_assistant: Color::Rgb(199, 146, 222),
            message_tool: Color::Rgb(251, 188, 5),
            status_ready: Color::Rgb(93, 221, 255),
            status_thinking: Color::Rgb(251, 188, 5),
            status_error: Color::Rgb(220, 38, 38),
            status_success: Color::Rgb(34, 197, 94),
        }
    }

    pub fn default_theme() -> Self {
        Self::opencode_dark()
    }

    pub fn header_style(&self) -> Style {
        Style::default().fg(self.accent)
    }

    pub fn subheader_style(&self) -> Style {
        Style::default()
            .fg(self.accent_light)
            .add_modifier(ratatui::style::Modifier::DIM)
    }

    pub fn prompt_style(&self) -> Style {
        Style::default()
            .fg(self.accent)
            .add_modifier(ratatui::style::Modifier::BOLD)
    }

    pub fn status_ready_style(&self) -> Style {
        Style::default().fg(self.status_ready)
    }

    pub fn status_thinking_style(&self) -> Style {
        Style::default().fg(self.status_thinking)
    }

    pub fn error_style(&self) -> Style {
        Style::default().fg(self.error)
    }

    pub fn success_style(&self) -> Style {
        Style::default().fg(self.success)
    }

    pub fn muted_style(&self) -> Style {
        Style::default()
            .fg(self.accent_light)
            .add_modifier(ratatui::style::Modifier::DIM)
    }

    pub fn border_style(&self) -> Style {
        Style::default().fg(self.border)
    }

    pub fn message_user_style(&self) -> Style {
        Style::default().fg(self.message_user)
    }

    pub fn message_assistant_style(&self) -> Style {
        Style::default().fg(self.message_assistant)
    }

    pub fn message_tool_style(&self) -> Style {
        Style::default().fg(self.message_tool)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_opencode_dark_background() {
        let theme = Theme::opencode_dark();
        assert_eq!(theme.background, Color::Rgb(20, 22, 27));
    }

    #[test]
    fn test_dracula_background() {
        let theme = Theme::dracula();
        assert_eq!(theme.background, Color::Rgb(24, 24, 24));
    }

    #[test]
    fn test_github_dark_background() {
        let theme = Theme::github_dark();
        assert_eq!(theme.background, Color::Rgb(13, 17, 24));
    }

    #[test]
    fn test_default_theme() {
        let theme = Theme::default_theme();
        assert_eq!(theme.background, Color::Rgb(20, 22, 27));
    }
}
