use std::process::ExitCode;
use uuid::Uuid;

use crate::cli::{list_session_files, SessionAction};

pub fn run(action: &SessionAction, storage: agent_storage::Storage) -> ExitCode {
    match action {
        SessionAction::List => run_list(),
        SessionAction::Resume { id } => run_resume(id, &storage),
    }
}

fn run_list() -> ExitCode {
    let ids = list_session_files();
    if ids.is_empty() {
        println!("(no sessions)");
    }
    for id in ids {
        println!("{id}");
    }
    ExitCode::SUCCESS
}

fn run_resume(id: &Option<String>, storage: &agent_storage::Storage) -> ExitCode {
    match id {
        Some(id_str) => match Uuid::parse_str(id_str) {
            Ok(uuid) => {
                let msgs =
                    agent_sessions::SessionManager::resume(storage, uuid).unwrap_or_default();
                println!("resumed session {uuid} with {} messages", msgs.len());
                ExitCode::SUCCESS
            }
            Err(_) => {
                eprintln!("invalid session id: {id_str}");
                ExitCode::FAILURE
            }
        },
        None => match agent_sessions::SessionManager::latest(storage) {
            Some(id) => {
                println!("latest session: {id}");
                ExitCode::SUCCESS
            }
            None => {
                println!("(no sessions)");
                ExitCode::SUCCESS
            }
        },
    }
}
