 use std::process::ExitCode;
 
 use crate::cli::{PluginAction, plugins_dir};
 
 pub fn run(action: &PluginAction) -> ExitCode {
     let dir = agent_plugins::PluginManager::discover_from(&plugins_dir()).ok();
     match action {
         PluginAction::List => run_list(dir),
         PluginAction::Install { source } => run_install(source),
         _ => {
             println!("plugin action is a stub in this build");
             ExitCode::SUCCESS
         }
     }
 }
 
 fn run_list(dir: Option<Vec<std::path::PathBuf>>) -> ExitCode {
     for p in dir.iter().flatten() {
         println!("{}", p.display());
     }
     ExitCode::SUCCESS
 }
 
 fn run_install(source: &str) -> ExitCode {
     let target = plugins_dir().join(source);
     match std::fs::create_dir_all(&target) {
         Ok(_) => {
             println!("created plugin dir: {}", target.display());
             ExitCode::SUCCESS
         }
         Err(e) => {
             eprintln!("failed: {e}");
             ExitCode::FAILURE
         }
     }
 }
