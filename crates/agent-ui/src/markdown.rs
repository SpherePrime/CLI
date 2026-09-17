use textwrap::fill;

pub fn render_markdown(text: &str, width: usize) -> String {
    let mut out = String::new();
    for raw in text.lines() {
        let line = raw.trim();
        if line.is_empty() {
            out.push('\n');
            continue;
        }
        if let Some(stripped) = line.strip_prefix("### ") {
            out.push_str(&format!("  ## {}\n", fill(stripped, width)));
        } else if let Some(stripped) = line.strip_prefix("## ") {
            out.push_str(&format!("   # {}\n", fill(stripped, width)));
        } else if let Some(stripped) = line.strip_prefix("# ") {
            out.push_str(&format!("   # {}  \n", fill(stripped, width)));
        } else if let Some(stripped) = line.strip_prefix("- ") {
            out.push_str(&format!("   • {}\n", fill(stripped, width)));
        } else if let Some(stripped) = line.strip_prefix("* ") {
            out.push_str(&format!("   • {}\n", fill(stripped, width)));
        } else if let Some(stripped) = line.strip_prefix("```") {
            out.push_str(&format!("   {stripped}\n"));
        } else {
            out.push_str(&format!("   {}\n", fill(line, width)));
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn markdown_headings() {
        let out = render_markdown("# Title\n## Sub", 80);
        assert!(out.contains("# Title"));
        assert!(out.contains("# Sub"));
    }

    #[test]
    fn markdown_bullets() {
        let out = render_markdown("- item1\n- item2", 80);
        assert!(out.contains("• item1"));
    }
}
