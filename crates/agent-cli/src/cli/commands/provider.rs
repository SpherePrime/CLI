use std::process::ExitCode;

use crate::cmd::provider::ProviderAction;

use crate::cli::{load_config, save_config};

pub fn run(action: &ProviderAction) -> ExitCode {
    match action {
        ProviderAction::List => {
            let cfg = load_config();
            crate::cmd::provider::list_providers(&cfg);
            ExitCode::SUCCESS
        }
        ProviderAction::Add { id } => {
            let mut cfg = load_config();
            match crate::cmd::provider::add_provider(&mut cfg, id.clone()) {
                Ok(provider_id) => match save_config(&cfg) {
                    Ok(p) => {
                        println!("added provider '{provider_id}' to {}", p.display());
                        ExitCode::SUCCESS
                    }
                    Err(e) => {
                        eprintln!("failed to save config: {e}");
                        ExitCode::FAILURE
                    }
                },
                Err(e) => {
                    eprintln!("failed to add provider: {e}");
                    ExitCode::FAILURE
                }
            }
        }
        ProviderAction::Use { id } => {
            let mut cfg = load_config();
            match crate::cmd::provider::use_provider(&mut cfg, id) {
                Ok(()) => match save_config(&cfg) {
                    Ok(p) => {
                        println!("set active provider to '{id}' in {}", p.display());
                        ExitCode::SUCCESS
                    }
                    Err(e) => {
                        eprintln!("failed to save config: {e}");
                        ExitCode::FAILURE
                    }
                },
                Err(e) => {
                    eprintln!("failed: {e}");
                    ExitCode::FAILURE
                }
            }
        }
    }
}
