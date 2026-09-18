use clap_complete::{generate, Shell};

pub fn generate_completion(shell: &str, app: &mut clap::Command) -> Option<String> {
    let shell_enum = match shell.to_lowercase().as_str() {
        "bash" => Shell::Bash,
        "zsh" => Shell::Zsh,
        "fish" => Shell::Fish,
        "powershell" | "pwsh" => Shell::PowerShell,
        _ => return None,
    };
    let mut buf: Vec<u8> = Vec::new();
    generate(shell_enum, app, "agent", &mut buf);
    String::from_utf8(buf).ok()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bash_completion() {
        let mut app = clap::Command::new("agent")
            .version("0.1.0")
            .about("CLI AI Coding Agent");
        let out = generate_completion("bash", &mut app).unwrap();
        assert!(!out.is_empty());
    }

    #[test]
    fn unsupported_shell() {
        let mut app = clap::Command::new("agent");
        let out = generate_completion("ksh", &mut app);
        assert!(out.is_none());
    }
}
