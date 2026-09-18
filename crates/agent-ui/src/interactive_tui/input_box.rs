use crossterm::terminal::ClearType;

pub struct Welcome;

impl Welcome {
    pub fn banner(model: &str, provider: &str, version: &str) {
        let _ = crossterm::execute!(
            std::io::stdout(),
            crossterm::terminal::Clear(ClearType::All)
        );

        let line = "══════════════════════════════════════════════";
        println!("\x1b[1;38;2;79;193;255m{line}\x1b[0m");
        println!("\x1b[1;38;2;79;193;255m  AGENT — AI Coding Agent  v{version}\x1b[0m");
        println!("\x1b[38;2;107;114;128m  model: {model}  ·  provider: {provider}\x1b[0m");
        println!(
            "\x1b[38;2;107;114;128m  /help · /model · /config · /status · /tools · /exit\x1b[0m"
        );
        println!("\x1b[1;38;2;79;193;255m{line}\x1b[0m");
    }
}
