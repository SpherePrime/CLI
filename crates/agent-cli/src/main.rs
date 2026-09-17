mod completion;
mod doctor;
mod init;
mod main_loop;
mod runtime;

pub mod cmd;

use std::process::ExitCode;

use clap::Parser;
use uuid::Uuid;

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
        action: ModelAction,
    },
    /// Manage providers
    Provider {
        #[command(subcommand)]
        action: ProviderAction,
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
    Add { name: String, command: String, #[arg(short, long)] args: Vec<String> },
    Remove { name: String },
    Enable { name: String },
    Disable { name: String },
    Inspect { name: String },
}

#[derive(Debug, clap::Subcommand)]
pub enum PluginAction {
    List,
    Install { source: String },
    Remove { name: String },
    Enable { name: String },
    Disable { name: String },
    Inspect { name: String },
}

#[derive(Debug, clap::Subcommand)]
pub enum SkillAction {
    List,
    Install { source: String },
    Remove { name: String },
    Enable { name: String },
    Disable { name: String },
    Inspect { name: String },
}

#[derive(Debug, clap::Subcommand)]
pub enum SessionAction {
    List,
    Resume { id: Option<String> },
}

use cmd::model::ModelAction;
use cmd::provider::ProviderAction;

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

    match &cli.command {
        None => main_loop::run_main_loop(Some(storage.clone())),
        Some(Command::Run { mode }) => {
            let mode = mode.as_str();
            if mode == "ci" || mode == "readonly" {
                eprintln!("non-interactive mode '{mode}' is not yet implemented in this build");
                ExitCode::FAILURE
            } else {
                main_loop::run_main_loop(Some(storage.clone()))
            }
        }
        Some(Command::Init) => init::run_init(),
        Some(Command::Doctor) => doctor::run_doctor(&storage),
        Some(Command::Mcp { action }) => match action {
            McpAction::List => {
                let cfg = load_config();
                for (name, server) in &cfg.mcp.servers {
                    println!(
                        "{name} {}",
                        if server.enabled { "[enabled]" } else { "[disabled]" }
                    );
                }
                ExitCode::SUCCESS
            }
            McpAction::Add { name, command, args } => {
                let mut cfg = load_config();
                cfg.mcp.servers.insert(
                    name.clone(),
                    agent_config::McpServerConfig {
                        name: name.clone(),
                        command: Some(command.clone()),
                        args: args.clone(),
                        url: None,
                        env: Default::default(),
                        enabled: true,
                    },
                );
                match save_config(&cfg) {
                    Ok(p) => {
                        println!("added MCP server '{name}' to {}", p.display());
                        ExitCode::SUCCESS
                    }
                    Err(e) => {
                        eprintln!("failed to save config: {e}");
                        ExitCode::FAILURE
                    }
                }
            }
            McpAction::Remove { name } => {
                let mut cfg = load_config();
                cfg.mcp.servers.remove(name);
                match save_config(&cfg) {
                    Ok(p) => {
                        println!("removed MCP server '{name}' from {}", p.display());
                        ExitCode::SUCCESS
                    }
                    Err(e) => {
                        eprintln!("failed to save config: {e}");
                        ExitCode::FAILURE
                    }
                }
            }
            McpAction::Enable { name } => {
                let mut cfg = load_config();
                if let Some(s) = cfg.mcp.servers.get_mut(name) {
                    s.enabled = true;
                    println!("enabled '{name}'");
                } else {
                    eprintln!("MCP server '{name}' not found");
                    return ExitCode::FAILURE;
                }
                save_config(&cfg).map(|_| ExitCode::SUCCESS).unwrap_or_else(|_| ExitCode::FAILURE)
            }
            McpAction::Disable { name } => {
                let mut cfg = load_config();
                if let Some(s) = cfg.mcp.servers.get_mut(name) {
                    s.enabled = false;
                    println!("disabled '{name}'");
                } else {
                    eprintln!("MCP server '{name}' not found");
                    return ExitCode::FAILURE;
                }
                save_config(&cfg).map(|_| ExitCode::SUCCESS).unwrap_or_else(|_| ExitCode::FAILURE)
            }
            McpAction::Inspect { name } => {
                let cfg = load_config();
                match cfg.mcp.servers.get(name) {
                    Some(s) => {
                        println!("{}", serde_json::to_string_pretty(s).unwrap_or_default());
                        ExitCode::SUCCESS
                    }
                    None => {
                        eprintln!("MCP server '{name}' not found");
                        ExitCode::FAILURE
                    }
                }
            }
        },
        Some(Command::Plugin { action }) => {
            let dir = agent_plugins::PluginManager::discover_from(&plugins_dir()).ok();
            match action {
                PluginAction::List => {
                    for p in dir.iter().flatten() {
                        println!("{}", p.display());
                    }
                    ExitCode::SUCCESS
                }
                PluginAction::Install { source } => {
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
                _ => {
                    println!("plugin action is a stub in this build");
                    ExitCode::SUCCESS
                }
            }
        }
        Some(Command::Skill { action }) => match action {
            SkillAction::List => {
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
            SkillAction::Install { source } => {
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
            _ => {
                println!("skill action is a stub in this build");
                ExitCode::SUCCESS
            }
        },
        Some(Command::Session { action }) => match action {
            SessionAction::List => {
                let mgr = agent_sessions::SessionManager::new(storage.clone(), Uuid::new_v4());
                let _ = mgr;
                let ids = list_session_files();
                if ids.is_empty() {
                    println!("(no sessions)");
                }
                for id in ids {
                    println!("{id}");
                }
                ExitCode::SUCCESS
            }
            SessionAction::Resume { id } => match id {
                Some(id_str) => match Uuid::parse_str(id_str) {
                    Ok(uuid) => {
                        let msgs = agent_sessions::SessionManager::resume(&storage, uuid)
                            .unwrap_or_default();
                        println!("resumed session {uuid} with {} messages", msgs.len());
                        ExitCode::SUCCESS
                    }
                    Err(_) => {
                        eprintln!("invalid session id: {id_str}");
                        ExitCode::FAILURE
                    }
                },
                None => {
                    let latest = agent_sessions::SessionManager::latest(&storage);
                    match latest {
                        Some(id) => {
                            println!("latest session: {id}");
                            ExitCode::SUCCESS
                        }
                        None => {
                            println!("(no sessions)");
                            ExitCode::SUCCESS
                        }
                    }
                }
            },
        },
        Some(Command::Model { action }) => match action {
            cmd::model::ModelAction::List => {
                let cfg = load_config();
                cmd::model::list_models(&cfg);
                ExitCode::SUCCESS
            }
            cmd::model::ModelAction::Add { spec: _ } => {
                let mut cfg = load_config();
                match cmd::model::add_model(&mut cfg) {
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
            cmd::model::ModelAction::Use { spec } => {
                let mut cfg = load_config();
                match cmd::model::use_model(&mut cfg, &spec) {
                    Ok(()) => {
                        match save_config(&cfg) {
                            Ok(p) => {
                                println!("model set to '{}' in {}", spec, p.display());
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
        },
        Some(Command::Provider { action }) => match action {
            cmd::provider::ProviderAction::List => {
                let cfg = load_config();
                cmd::provider::list_providers(&cfg);
                ExitCode::SUCCESS
            }
            cmd::provider::ProviderAction::Add { id } => {
                let mut cfg = load_config();
                match cmd::provider::add_provider(&mut cfg, id.clone()) {
                    Ok(provider_id) => {
                        match save_config(&cfg) {
                            Ok(p) => {
                                println!("added provider '{}' to {}", provider_id, p.display());
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
            cmd::provider::ProviderAction::Use { id } => {
                let mut cfg = load_config();
                match cmd::provider::use_provider(&mut cfg, id) {
                    Ok(()) => {
                        match save_config(&cfg) {
                            Ok(p) => {
                                println!("set active provider to '{}' in {}", id, p.display());
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
        },
        Some(Command::Tools) => {
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
        Some(Command::Config) => {
            let cfg = load_config();
            println!("{}", toml::to_string_pretty(&cfg).unwrap_or_else(|_| "(no config)".into()));
            ExitCode::SUCCESS
        }
        Some(Command::Completion { shell }) => {
            let mut cmd = {
                use clap::CommandFactory;
                Cli::command()
            };
            match completion::generate_completion(shell, &mut cmd) {
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

fn load_config() -> agent_config::AgentConfig {
    let loader = agent_config::ConfigLoader::new();
    loader.load().unwrap_or_default()
}

fn save_config(
    cfg: &agent_config::AgentConfig,
) -> std::result::Result<std::path::PathBuf, anyhow::Error> {
    agent_config::ConfigLoader::save_global(cfg)
}

fn plugins_dir() -> std::path::PathBuf {
    let base = agent_storage::Storage::global()
        .ok()
        .map(|s| s.root().clone())
        .unwrap_or_else(|| std::path::PathBuf::from("agent"));
    base.join("plugins")
}

fn skills_dir() -> std::path::PathBuf {
    let base = agent_storage::Storage::global()
        .ok()
        .map(|s| s.root().clone())
        .unwrap_or_else(|| std::path::PathBuf::from("agent"));
    base.join("skills")
}

fn list_session_files() -> Vec<String> {
    let dir = agent_storage::Storage::global()
        .ok()
        .map(|s| s.sessions_dir())
        .unwrap_or_else(|| std::path::PathBuf::from("sessions"));
    std::fs::read_dir(&dir)
        .ok()
        .into_iter()
        .flatten()
        .filter_map(|e| e.ok())
        .filter(|e| e.path().extension().and_then(|s| s.to_str()) == Some("jsonl"))
        .map(|e| {
            e.file_name()
                .to_string_lossy()
                .trim_end_matches(".jsonl")
                .to_string()
        })
        .collect()
}
