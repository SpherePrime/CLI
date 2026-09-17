 use std::process::ExitCode;
 
 use crate::cli::parser::Cli;
 
 pub fn run(shell: &str) -> ExitCode {
     let mut cmd = {
         use clap::CommandFactory;
         Cli::command()
     };
     match crate::completion::generate_completion(shell, &mut cmd) {
         Some(code) => {
             print!("{code}");
             ExitCode::SUCCESS
         }
         None => {
             eprintln!("unsupported shell '{shell}'");
             ExitCode::FAILURE
         }
     }
 }
