use std::io::{self, Write};
use std::process::ExitCode;
use std::sync::Arc;

use agent_config::ConfigLoader;
use agent_model::{
    build_from_model_config, ChatMessage, MessageContent, ModelProvider, ModelRequest, Role,
};
use agent_ui::{AgentApp, SlashCommandPalette};

pub fn run_main_loop(storage: Option<agent_storage::Storage>) -> ExitCode {
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("tokio runtime creation failed");

    let cfg = ConfigLoader::new().load().unwrap_or_default();
    let model = cfg.model.clone().unwrap_or_else(default_model);
    let provider: Arc<dyn ModelProvider> = match build_from_model_config(&model) {
        Ok(p) => Arc::from(p),
        Err(e) => {
            eprintln!("model provider init failed: {e}");
            return ExitCode::FAILURE;
        }
    };

    let session_id = uuid::Uuid::new_v4();
    let mut app = AgentApp::new("AGENT", env!("CARGO_PKG_VERSION"));
    app.push_assistant(&format!(
        "Ready. Model: {} ({} provider). Type /help for commands.",
        model.model,
        provider_name(&model),
    ));

    let mut palette = SlashCommandPalette::new();
    let mut input = String::new();
    let cwd = std::env::current_dir().unwrap_or_default();

    loop {
        print!("> ");
        io::stdout().flush().ok();
        input.clear();
        match io::stdin().read_line(&mut input) {
            Ok(0) => break,
            Err(e) => {
                eprintln!("read error: {e}");
                break;
            }
            Ok(_) => {}
        }
        let user_input = input.trim().to_string();
        if user_input.is_empty() {
            continue;
        }

        app.push_user(&user_input);

        if let Some(cmd_name) = palette.execute(&user_input) {
            match cmd_name.as_str() {
                "/exit" | "/quit" => {
                    app.push_assistant("bye");
                    break;
                }
                "/clear" => {
                    app.messages.clear();
                    app.push_assistant("(cleared)");
                }
                "/compact" => {
                    app.compact();
                    app.push_assistant("(context compacted)");
                }
                "/help" => {
                    let names = palette.all_names();
                    app.push_assistant(&format!("available: {}", names.join(", ")));
                }
                "/tools" => {
                    app.push_assistant(
                        "tools: read_file, write_file, edit_file, delete_file, \
                         list_directory, search_files, find_files, shell, run_command",
                    );
                }
                "/status" => {
                    app.push_assistant(&format!(
                        "session={} model={} messages={}",
                        session_id,
                        model.model,
                        app.messages.len()
                    ));
                }
                "/undo" => {
                    app.undo_last();
                    app.push_assistant("(undo last tool call)");
                }
                "/retry" => {
                    app.push_assistant("(retry last step)");
                }
                "/diff" => {
                    app.push_assistant("(diff view)");
                }
                _ => {
                    app.push_assistant(&format!("handled {cmd_name}"));
                }
            }
        } else {
            let request = ModelRequest {
                model: model.model.clone(),
                messages: vec![
                    ChatMessage {
                        role: Role::System,
                        content: MessageContent::Text(
                            "You are a production-grade AI coding agent.".into(),
                        ),
                        tool_calls: None,
                        tool_call_id: None,
                    },
                    ChatMessage {
                        role: Role::User,
                        content: MessageContent::Text(user_input.clone()),
                        tool_calls: None,
                        tool_call_id: None,
                    },
                ],
                tools: None,
                temperature: model.temperature,
                max_tokens: model.max_tokens,
            };

            let response = runtime
                .block_on(provider.chat(&request))
                .unwrap_or_else(|e| {
                    agent_model::ModelResponse {
                        content: MessageContent::Text(format!(
                            "[model unavailable] {e}"
                        )),
                        tool_calls: vec![],
                        stop_reason: agent_model::StopReason::Error,
                        usage: Default::default(),
                    }
                });

            app.push_assistant(&response.content.as_text());
        }
    }

    if let Some(storage) = &storage {
        let _ = storage.write_session(
            &agent_storage::SessionRecord::new(Some(cwd), Some(model.model)),
            &[],
        );
    }

    app.push_assistant("session saved; goodbye.");
    println!(
        "\nsession {session_id} saved to {}",
        storage
            .as_ref()
            .map(|s| s.root().to_string_lossy().to_string())
            .unwrap_or_default()
    );

    ExitCode::SUCCESS
}

fn default_model() -> agent_config::ModelConfig {
    agent_config::ModelConfig {
        provider: agent_config::ProviderKind::Mock,
        model: "mock-1".into(),
        base_url: None,
        api_key_env: None,
        temperature: None,
        max_tokens: None,
    }
}

fn provider_name(mc: &agent_config::ModelConfig) -> &str {
    match mc.provider {
        agent_config::ProviderKind::OpenAi => "openai",
        agent_config::ProviderKind::Anthropic => "anthropic",
        agent_config::ProviderKind::Google => "google",
        agent_config::ProviderKind::OpenAiCompatible => "openai-compatible",
        agent_config::ProviderKind::Custom => "custom",
        agent_config::ProviderKind::Mock => "mock",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn provider_name_mock() {
        let mc = default_model();
        assert_eq!(provider_name(&mc), "mock");
    }
}
