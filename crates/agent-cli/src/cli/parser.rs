use clap::Parser;

#[derive(Debug, Parser)]
#[command(
    name = "agent",
    version,
    about = "Production-ready CLI AI Coding Agent"
)]
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
    /// Serve the HTTP API for the TypeScript TUI
    Serve {
        #[arg(long, default_value = "0.0.0.0")]
        host: String,
        #[arg(long, default_value_t = 0)]
        port: u16,
    },
    /// Launch the TypeScript TUI with an embedded server
    Tui {
        #[arg(long, default_value_t = 40123)]
        port: u16,
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
