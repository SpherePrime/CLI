use ratatui::style::{Color, Style, Stylize};

#[derive(Debug, Clone, Default)]
pub struct Theme;

impl Theme {
    pub fn brand() -> Style {
        Style::default().fg(Color::Rgb(0x4f, 0xc1, 0xff)).bold()
    }

    pub fn grey() -> Style {
        Style::default().fg(Color::Rgb(0x6b, 0x72, 0x80))
    }

    pub fn green() -> Style {
        Style::default().fg(Color::Rgb(0x3e, 0xcf, 0x8e))
    }

    pub fn red() -> Style {
        Style::default().fg(Color::Rgb(0xff, 0x6b, 0x6b))
    }

    pub fn text() -> Style {
        Style::default().fg(Color::Rgb(0xe8, 0xec, 0xf1))
    }
}
