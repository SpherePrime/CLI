use std::io::Write;
use std::process::ExitCode;

use agent_config::{AgentMode, ConfigLoader};
use agent_core::{AgentEngine, EngineEvent};
use tokio::runtime::Runtime;
use tokio::sync::mpsc;

pub fn run_once(mode: &str, prompt: &str, storage: agent_storage::Storage, json: bool) -> ExitCode {
    let runtime = Runtime::new().expect("tokio runtime creation failed");
    runtime.block_on(async move {
        let mut config = ConfigLoader::new().load().unwrap_or_default();
        config.mode = Some(parse_mode(mode));
        if config.model.is_none() {
            eprintln!("no model configured; set [model] via 'agent model' or 'agent config'");
            return ExitCode::FAILURE;
        }

        let working_dir = std::env::current_dir().unwrap_or_default();
        let (tx, mut rx) = mpsc::channel::<EngineEvent>(256);
        let mut engine = match AgentEngine::new(config, working_dir, tx.clone()) {
            Ok(engine) => engine.with_storage(storage),
            Err(error) => {
                eprintln!("engine init error: {error}");
                return ExitCode::FAILURE;
            }
        };

        if let Err(error) = engine.run(prompt, &tx).await {
            eprintln!("error: {error}");
        }
        drop(tx);

        let mut exit_code = ExitCode::SUCCESS;
        while let Some(event) = rx.recv().await {
            emit(&event, json);
            if matches!(event, EngineEvent::Error { .. }) {
                exit_code = ExitCode::FAILURE;
            }
        }
        exit_code
    })
}

fn parse_mode(mode: &str) -> AgentMode {
    match mode {
        "readonly" => AgentMode::Readonly,
        "plan" => AgentMode::Plan,
        "debug" => AgentMode::Debug,
        "ci" => AgentMode::Ci,
        "auto" => AgentMode::Auto,
        _ => AgentMode::Auto,
    }
}

fn emit(event: &EngineEvent, json: bool) {
    if json {
        if let Ok(value) = serde_json::to_string(event) {
            println!("{value}");
        }
        return;
    }
    match event {
        EngineEvent::Started { model, .. } => {
            println!("= starting with model {model}");
        }
        EngineEvent::TextDelta { text } => {
            print!("{text}");
            let _ = std::io::stdout().flush();
        }
        EngineEvent::ToolCall { name, args, .. } => {
            println!("\n▶ {name} {args}");
        }
        EngineEvent::ToolResult {
            name, ok, error, ..
        } => {
            if *ok {
                println!("  ✓ {name}");
            } else {
                println!("  ✗ {name}: {}", error.as_deref().unwrap_or("tool failed"));
            }
        }
        EngineEvent::PermissionRequested { tool, reason, .. } => {
            println!("\n? permission required for {tool}: {reason}");
        }
        EngineEvent::Finished {
            stop_reason,
            input_tokens,
            output_tokens,
            ..
        } => {
            println!("\n= finished ({stop_reason}) tokens={input_tokens}+{output_tokens}");
        }
        EngineEvent::Error { message } => {
            println!("\nerror: {message}");
        }
        EngineEvent::ReasoningDelta { text } => {
            print!("\x1b[2m{text}\x1b[0m");
            let _ = std::io::stdout().flush();
        }
        EngineEvent::Usage { .. } | EngineEvent::PermissionResolved { .. } => {}
    }
}

#[test]
fn mode_parsing_covers_all_modes() {
    assert!(matches!(parse_mode("plan"), AgentMode::Plan));
    assert!(matches!(parse_mode("debug"), AgentMode::Debug));
    assert!(matches!(parse_mode("auto"), AgentMode::Auto));
    assert!(matches!(parse_mode("readonly"), AgentMode::Readonly));
    assert!(matches!(parse_mode("ci"), AgentMode::Ci));
    assert!(matches!(parse_mode("unknown"), AgentMode::Auto));
}
