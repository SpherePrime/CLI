use std::path::PathBuf;
use std::process::ExitCode;

pub fn run() -> ExitCode {
    let cwd = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
    match agent_filesystem::snapshot::restore_turn(&cwd) {
        Ok(paths) if paths.is_empty() => {
            eprintln!("nothing to undo in this directory");
            ExitCode::FAILURE
        }
        Ok(paths) => {
            for path in &paths {
                println!("restored {path}");
            }
            ExitCode::SUCCESS
        }
        Err(error) => {
            eprintln!("undo failed: {error}");
            ExitCode::FAILURE
        }
    }
}
