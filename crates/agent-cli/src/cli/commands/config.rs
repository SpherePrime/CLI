 use std::process::ExitCode;
 
 use crate::cli::load_config;
 
 pub fn run() -> ExitCode {
     let cfg = load_config();
     println!("{}", toml::to_string_pretty(&cfg).unwrap_or_else(|_| "(no config)".into()));
     ExitCode::SUCCESS
 }
