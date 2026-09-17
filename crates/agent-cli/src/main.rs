mod completion;
mod doctor;
mod init;
mod runtime;
mod server;

pub mod cli;
pub mod cmd;
pub mod tui;

use std::process::ExitCode;

use clap::Parser;

use cli::Cli;

fn main() -> ExitCode {
    let cli = Cli::parse();

    if cli.debug || cli.verbose > 0 {
        let filter = if cli.debug {
            "debug"
        } else {
            "trace"
        };
        let _ = tracing_subscriber::fmt()
            .with_env_filter(
                tracing_subscriber::EnvFilter::from_default_env(),
            )
            .with_max_level(tracing::Level::TRACE)
            .try_init();
        let _ = filter;
    }

    let storage = match agent_storage::Storage::global() {
        Ok(s) => s,
        Err(e) => {
            eprintln!("cannot initialize storage: {e}");
            return ExitCode::FAILURE;
        }
    };

    cli::dispatch(&cli, &storage)
}
