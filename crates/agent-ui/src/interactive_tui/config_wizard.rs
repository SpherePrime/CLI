use agent_config::{AgentConfig, PermissionMode};
use dialoguer::theme::ColorfulTheme;
use dialoguer::{Input, Select};

use super::super::ui::model_wizard;

pub fn run_config_wizard(cfg: &AgentConfig) -> AgentConfig {
    let theme = ColorfulTheme::default();
    let items: Vec<String> = vec![
        "Model & Provider — wizard".into(),
        "Permission mode".into(),
        "Resource limits".into(),
        "Save config to ~/.agent/config.toml".into(),
        "Exit config menu".into(),
    ];

    let mut out = cfg.clone();

    let choice = match Select::with_theme(&theme)
        .with_prompt(" config section")
        .items(&items)
        .default(0)
        .interact()
    {
        Ok(i) => i,
        Err(_) => return out,
    };

    match choice {
        0 => {
            out.model = Some(model_wizard::run_model_wizard(out.model.as_ref()));
        }
        1 => {
            let modes: Vec<String> = vec!["ask".into(), "allow".into(), "deny".into()];
            if let Ok(i) = Select::with_theme(&theme)
                .with_prompt(" permission mode")
                .items(&modes)
                .default(0)
                .interact()
            {
                out.permissions.mode = match i {
                    0 => PermissionMode::Ask,
                    1 => PermissionMode::Allow,
                    _ => PermissionMode::Deny,
                };
            }
        }
        2 => {
            let limits = &mut out.limits;
            let t: String = Input::with_theme(&theme)
                .with_prompt(" max tool calls")
                .default(limits.max_tool_calls.to_string())
                .interact_text()
                .unwrap_or_default();
            limits.max_tool_calls = t.trim().parse().unwrap_or(limits.max_tool_calls);

            let t: String = Input::with_theme(&theme)
                .with_prompt(" max parallel tools")
                .default(limits.max_parallel_tools.to_string())
                .interact_text()
                .unwrap_or_default();
            limits.max_parallel_tools = t.trim().parse().unwrap_or(limits.max_parallel_tools);

            let t: String = Input::with_theme(&theme)
                .with_prompt(" max context messages")
                .default(limits.max_context_messages.to_string())
                .interact_text()
                .unwrap_or_default();
            limits.max_context_messages = t.trim().parse().unwrap_or(limits.max_context_messages);
        }
        3 => match agent_config::ConfigLoader::save_global(&out) {
            Ok(p) => println!("\x1b[32m✓ saved to {}\x1b[0m", p.display()),
            Err(e) => eprintln!("\x1b[31msave failed: {e}\x1b[0m"),
        },
        _ => {}
    }

    out
}
