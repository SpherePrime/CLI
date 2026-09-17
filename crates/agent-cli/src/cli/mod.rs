 pub mod actions;
 pub mod util;
 
 pub use util::{list_session_files, load_config, plugins_dir, save_config, skills_dir};
 
 use clap::Parser;
 use std::process::ExitCode;
 
 use crate::cli::actions::{mcp, plugin, session, skill};
 
 #[derive(Debug, Parser)]
 #[command(name = "agent", version, about = "Production-ready CLI AI Coding Agent")]
 pub struct Cli {
     #[arg(long, short = 'd', help = "Enable debug logging")]
     pub debug: bool,
     #[arg(long, short = 'v', action = clap::ArgAction::Count, help = "Increase verbosity")]
     pub verbose: u8,
 
     #[command(subcommand)]
     pub command: Option<Command>,
 }
 
 #[derive(Debug, clap::Subcommand)]
 pub enum Command {
     /// Run the agent interactively
     Run {
         #[arg(long, default_value = "interactive")]
         mode: String,
     },
     /// Initialize a project for the agent
     Init,
     /// Diagnose configuration
     Doctor,
     /// Manage MCP servers
     Mcp {
         #[command(subcommand)]
         action: McpAction,
     },
     /// Manage plugins
     Plugin {
         #[command(subcommand)]
         action: PluginAction,
     },
     /// Manage skills
     Skill {
         #[command(subcommand)]
         action: SkillAction,
     },
     /// Manage sessions
     Session {
         #[command(subcommand)]
         action: SessionAction,
     },
     /// Manage the model
     Model {
         #[command(subcommand)]
         action: crate::cmd::model::ModelAction,
     },
     /// Manage providers
     Provider {
         #[command(subcommand)]
         action: crate::cmd::provider::ProviderAction,
     },
     /// Show available tools
     Tools,
     /// Show configuration
     Config,
     /// Generate shell completion
     #[command(hide = true)]
     Completion {
         #[arg(long, default_value = "bash")]
         shell: String,
     },
     /// Rollback last change
     Undo,
     /// Update the agent
     Update,
 }
 
 #[derive(Debug, clap::Subcommand)]
 pub enum McpAction {
     List,
     Add {
         name: String,
         command: String,
         #[arg(short, long)]
         args: Vec<String>,
     },
     Remove {
         name: String,
     },
     Enable {
         name: String,
     },
     Disable {
         name: String,
     },
     Inspect {
         name: String,
     },
 }
 
 #[derive(Debug, clap::Subcommand)]
 pub enum PluginAction {
     List,
     Install {
         source: String,
     },
     Remove {
         name: String,
     },
     Enable {
         name: String,
     },
     Disable {
         name: String,
     },
     Inspect {
         name: String,
     },
 }
 
 #[derive(Debug, clap::Subcommand)]
 pub enum SkillAction {
     List,
     Install {
         source: String,
     },
     Remove {
         name: String,
     },
     Enable {
         name: String,
     },
     Disable {
         name: String,
     },
     Inspect {
         name: String,
     },
 }
 
 #[derive(Debug, clap::Subcommand)]
 pub enum SessionAction {
     List,
     Resume {
         id: Option<String>,
     },
 }
 
 pub fn dispatch(cli: &Cli, storage: &agent_storage::Storage) -> ExitCode {
     match &cli.command {
         None => crate::tui::run_main_loop(Some(storage.clone())),
         Some(Command::Run { mode }) => {
             let mode = mode.as_str();
             if mode == "ci" || mode == "readonly" {
                 eprintln!("non-interactive mode '{mode}' is not yet implemented in this build");
                 ExitCode::FAILURE
             } else {
                 crate::tui::run_main_loop(Some(storage.clone()))
             }
         }
         Some(Command::Init) => crate::init::run_init(),
         Some(Command::Doctor) => crate::doctor::run_doctor(storage),
         Some(Command::Mcp { action }) => mcp::run(action),
         Some(Command::Plugin { action }) => plugin::run(action),
         Some(Command::Skill { action }) => skill::run(action),
         Some(Command::Session { action }) => session::run(action, storage.clone()),
         Some(Command::Model { action }) => run_model(action),
         Some(Command::Provider { action }) => run_provider(action),
         Some(Command::Tools) => run_tools(),
         Some(Command::Config) => run_config(),
         Some(Command::Completion { shell }) => run_completion(shell),
         Some(Command::Undo) => {
             println!("undo is stub in this build; use /undo in the interactive TUI");
             ExitCode::SUCCESS
         }
         Some(Command::Update) => {
             println!("update is stub in this build");
             ExitCode::SUCCESS
         }
     }
 }
 
 fn run_model(action: &crate::cmd::model::ModelAction) -> ExitCode {
     use crate::cmd::model;
     match action {
         model::ModelAction::List => {
             let cfg = load_config();
             model::list_models(&cfg);
             ExitCode::SUCCESS
         }
         model::ModelAction::Add { spec: _ } => {
             let mut cfg = load_config();
             match model::add_model(&mut cfg) {
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
         model::ModelAction::Use { spec } => {
             let mut cfg = load_config();
             match model::use_model(&mut cfg, spec) {
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
 
 fn run_provider(action: &crate::cmd::provider::ProviderAction) -> ExitCode {
     use crate::cmd::provider;
     match action {
         provider::ProviderAction::List => {
             let cfg = load_config();
             provider::list_providers(&cfg);
             ExitCode::SUCCESS
         }
         provider::ProviderAction::Add { id } => {
             let mut cfg = load_config();
             match provider::add_provider(&mut cfg, id.clone()) {
                 Ok(provider_id) => {
                     match save_config(&cfg) {
                         Ok(p) => {
                             println!("added provider '{provider_id}' to {}", p.display());
                             ExitCode::SUCCESS
                         }
                         Err(e) => {
                             eprintln!("failed to save config: {e}");
                             ExitCode::FAILURE
                         }
                     }
                 }
                 Err(e) => {
                     eprintln!("failed to add provider: {e}");
                     ExitCode::FAILURE
                 }
             }
         }
         provider::ProviderAction::Use { id } => {
             let mut cfg = load_config();
             match provider::use_provider(&mut cfg, id) {
                 Ok(()) => {
                     match save_config(&cfg) {
                         Ok(p) => {
                             println!("set active provider to '{id}' in {}", p.display());
                             ExitCode::SUCCESS
                         }
                         Err(e) => {
                             eprintln!("failed to save config: {e}");
                             ExitCode::FAILURE
                         }
                     }
                 }
                 Err(e) => {
                     eprintln!("failed: {e}");
                     ExitCode::FAILURE
                 }
             }
         }
     }
 }
 
 fn run_tools() -> ExitCode {
     println!("Available tools:");
     println!("  read_file       read a file");
     println!("  write_file      write a file");
     println!("  edit_file       edit a file with search/replace or line-range");
     println!("  delete_file     delete a file");
     println!("  list_directory  list a directory");
     println!("  search_files    search files by glob");
     println!("  find_files      find files by glob");
     println!("  shell           run a shell command");
     println!("  run_command     run a command");
     ExitCode::SUCCESS
 }
 
 fn run_config() -> ExitCode {
     let cfg = load_config();
     println!("{}", toml::to_string_pretty(&cfg).unwrap_or_else(|_| "(no config)".into()));
     ExitCode::SUCCESS
 }
 
 fn run_completion(shell: &str) -> ExitCode {
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
