 use ratatui::layout::Rect;
 use ratatui::prelude::*;
 use ratatui::widgets::{Block, BorderType, List, Paragraph};
 use ratatui::Frame;
 
 use crate::theme::Theme;
 
 pub use widgets::*;
 
 mod widgets;
 mod render;
 
 pub use render::{
     format_path, render_empty_state, render_header, render_help, render_prompt, render_status_line,
     shorten_path,
 };
 
 #[cfg(test)]
 mod tests {
     use super::*;
 
     #[test]
     fn test_shorten_path() {
         let result = shorten_path("C:\\Users\\dwert\\OneDrive\\GitHub\\CLI", 30);
         assert!(result.contains('~'));
     }
 }
