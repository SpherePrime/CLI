use std::process::ExitCode;

use dialoguer::Input;

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
        ProviderAction::SetKey { id, key } => {
            let key = match key {
                Some(key) => key.clone(),
                None => match Input::new()
                    .with_prompt(format!("Enter API key for '{id}'"))
                    .interact()
                {
                    Ok(key) => key,
                    Err(e) => {
                        eprintln!("failed to read key: {e}");
                        return ExitCode::FAILURE;
                    }
                },
            };
            match crate::server::credentials::set_key(id, &key) {
                Ok(()) => {
                    std::env::set_var(crate::server::credentials::env_name(id), &key);
                    println!("stored API key for '{id}'");
                    ExitCode::SUCCESS
                }
                Err(e) => {
                    eprintln!("failed to store key: {e}");
                    ExitCode::FAILURE
                }
            }
        }
        ProviderAction::RemoveKey { id } => match crate::server::credentials::remove_key(id) {
            Ok(()) => {
                println!("removed stored API key for '{id}'");
                ExitCode::SUCCESS
            }
            Err(e) => {
                eprintln!("failed to remove key: {e}");
                ExitCode::FAILURE
            }
        },
    }
}
