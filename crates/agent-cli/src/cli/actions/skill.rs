 use std::process::ExitCode;
 
 use crate::cli::{SkillAction, skills_dir};
 
 pub fn run(action: &SkillAction) -> ExitCode {
     match action {
         SkillAction::List => run_list(),
         SkillAction::Install { source } => run_install(source),
         _ => {
             println!("skill action is a stub in this build");
             ExitCode::SUCCESS
         }
     }
 }
 
 fn run_list() -> ExitCode {
     let skills_dir_path = skills_dir();
     if let Ok(mut entries) = std::fs::read_dir(&skills_dir_path) {
         for e in entries.flatten() {
             println!("{}", e.file_name().to_string_lossy());
         }
     } else {
         println!("(no skills installed at {})", skills_dir_path.display());
     }
     ExitCode::SUCCESS
 }
 
 fn run_install(source: &str) -> ExitCode {
     let target = skills_dir().join(source);
     match std::fs::create_dir_all(&target) {
         Ok(_) => {
             let _ = std::fs::write(target.join("SKILL.md"), "# TODO: write instructions\n");
             println!("installed skill at {}", target.display());
             ExitCode::SUCCESS
         }
         Err(e) => {
             eprintln!("failed: {e}");
             ExitCode::FAILURE
         }
     }
 }
