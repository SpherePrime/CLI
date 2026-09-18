pub mod actions;
pub mod commands;
pub mod parser;
pub mod util;

pub use parser::{Cli, McpAction, PluginAction, SessionAction, SkillAction};
pub use util::{list_session_files, load_config, plugins_dir, save_config, skills_dir};

use parser::Command;
use std::process::ExitCode;

use crate::cli::actions::{mcp, plugin, session, skill};

pub fn dispatch(cli: &Cli, storage: &agent_storage::Storage) -> ExitCode {
    match &cli.command {
        None => crate::launcher::run(40123),
        Some(Command::Run { .. }) if cli.agent => crate::launcher::run(40123),
        Some(Command::Run { mode, prompt, json }) => {
            let prompt_text = prompt.join(" ");
            if mode != "interactive" || !prompt_text.is_empty() {
                crate::run::run_once(mode, &prompt_text, storage.clone(), *json)
            } else {
                crate::launcher::run(40123)
            }
        }
        Some(Command::Serve {
            host,
            port,
            workspace,
            token,
        }) => crate::server::serve_with(
            host.as_str(),
            *port,
            storage.clone(),
            workspace.as_deref(),
            token.as_deref(),
        ),
        Some(Command::Tui { port }) => crate::launcher::run(*port),
        Some(Command::Init) => crate::init::run_init(),
        Some(Command::Doctor) => crate::doctor::run_doctor(storage),
        Some(Command::Mcp { action }) => mcp::run(action),
        Some(Command::Plugin { action }) => plugin::run(action),
        Some(Command::Skill { action }) => skill::run(action),
        Some(Command::Session { action }) => session::run(action, storage.clone()),
        Some(Command::Model { action }) => commands::model::run(action),
        Some(Command::Provider { action }) => commands::provider::run(action),
        Some(Command::Tools) => commands::tools::run(),
        Some(Command::Config) => commands::config::run(),
        Some(Command::Completion { shell }) => commands::completion::run(shell),
        Some(Command::Undo) => commands::undo::run(),
        Some(Command::Update) => commands::update::run(),
    }
}
