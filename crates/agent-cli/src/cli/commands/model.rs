 use std::process::ExitCode;
 
 use crate::cmd::model::ModelAction;
 
 use crate::cli::{load_config, save_config};
 
 pub fn run(action: &ModelAction) -> ExitCode {
     match action {
         ModelAction::List => {
             let cfg = load_config();
             crate::cmd::model::list_models(&cfg);
             ExitCode::SUCCESS
         }
         ModelAction::Add { spec: _ } => {
             let mut cfg = load_config();
             match crate::cmd::model::add_model(&mut cfg) {
                 Ok(model_config) => {
                     match save_config(&cfg) {
                         Ok(p) => {
                             println!(
                                 "model set to '{}' (provider: {:?}) in {}",
                                 model_config.model, model_config.provider,
                                 p.display()
                             );
                             ExitCode::SUCCESS
                         }
                         Err(e) => {
                             eprintln!("failed to save config: {e}");
                             ExitCode::FAILURE
                         }
                     }
                 }
                 Err(e) => {
                     eprintln!("failed to add model: {e}");
                     ExitCode::FAILURE
                 }
             }
         }
         ModelAction::Use { spec } => {
             let mut cfg = load_config();
             match crate::cmd::model::use_model(&mut cfg, spec) {
                 Ok(()) => {
                     match save_config(&cfg) {
                         Ok(p) => {
                             println!("model set to '{spec}' in {}", p.display());
                             ExitCode::SUCCESS
                         }
                         Err(e) => {
                             eprintln!("failed to save config: {e}");
                             ExitCode::FAILURE
                         }
                     }
                 }
                 Err(e) => {
                     eprintln!("failed to use model: {e}");
                     ExitCode::FAILURE
                 }
             }
         }
     }
 }
