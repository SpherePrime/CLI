use std::path::PathBuf;
use std::process::ExitCode;

pub fn run() -> ExitCode {
    let cwd = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
    match agent_filesystem::snapshot::restore_latest(&cwd) {
        Ok(Some(path)) => {
            println!("restored {path}");
            ExitCode::SUCCESS
        }
        Ok(None) => {
            eprintln!("nothing to undo in this directory");
            ExitCode::FAILURE
        }
        Err(error) => {
            eprintln!("undo failed: {error}");
            ExitCode::FAILURE
        }
    }
}
