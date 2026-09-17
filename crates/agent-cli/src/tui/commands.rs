 use std::sync::Arc;
 
 use agent_config::{AgentConfig, ConfigLoader, ModelConfig};
 use agent_model::{
     build_from_model_config, ChatMessage, MessageContent, ModelProvider, ModelRequest, Role,
 };
 use agent_ui::{AgentApp, AppState, StatusType};
 
 pub fn default_model() -> ModelConfig {
     ModelConfig {
         provider: agent_config::ProviderKind::Mock,
         model: "mock-1".into(),
         base_url: None,
         api_key_env: None,
         temperature: None,
         max_tokens: None,
     }
 }
 
pub fn send_message(
    msg: String,
    app: &mut AgentApp,
    model: &ModelConfig,
    runtime: &tokio::runtime::Runtime,
) {
    app.push_user(&msg);

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
                content: MessageContent::Text(msg),
                tool_calls: None,
                tool_call_id: None,
            },
        ],
        tools: None,
        temperature: model.temperature,
        max_tokens: model.max_tokens,
    };
 
     let provider = match build_provider(model) {
         Ok(p) => p,
         Err(e) => {
             app.error(&format!("model init error: {e}"));
             return;
         }
     };
 
     match runtime.block_on(provider.chat(&request)) {
         Ok(resp) => {
             app.push_assistant(&resp.content.as_text());
             app.set_status(StatusType::Success);
         }
         Err(e) => {
             app.error(&format!("model error: {e}"));
         }
     }
 }
 
 fn build_provider(model: &ModelConfig) -> anyhow::Result<Arc<dyn ModelProvider>> {
     Ok(Arc::from(build_from_model_config(model)?))
 }
 
pub fn handle_slash_command(
    cmd: &str,
    cfg: &mut AgentConfig,
    model: &mut ModelConfig,
    app: &mut AgentApp,
) {
    let cmd = cmd.trim();
    let cmd = cmd.strip_prefix('/').unwrap_or(cmd);

    match cmd {
        "exit" | "quit" => {
            std::process::exit(0);
        }
        "help" | "?" => {
            app.set_state(AppState::Banner);
        }
        "model" => {
            let new = agent_ui::run_model_wizard(Some(model));
            *model = new;
            cfg.model = Some(model.clone());
            match ConfigLoader::save_global(cfg) {
                Ok(_) => app.set_status(StatusType::Success),
                Err(_) => app.error("failed to save config"),
            }
        }
        "config" | "set" => {
            let new_cfg = agent_ui::run_config_wizard(cfg);
            *cfg = new_cfg;
            *model = cfg.model.clone().unwrap_or_else(default_model);
            app.set_model_config(model.clone());
            app.set_status(StatusType::Success);
        }
        "status" | "session" | "tools" | "mcp" | "skills" | "plugins" => {
            app.set_status(StatusType::Success);
        }
        "clear" => {
            app.clear_messages();
        }
        "compact" | "undo" | "retry" | "diff" => {
            app.set_status(StatusType::Success);
        }
        _ => {}
    }
}
