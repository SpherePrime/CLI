pub mod commands;

use std::process::ExitCode;

use agent_ui::{AgentApp, AppEvent};

use crossterm::event::{Event, KeyEventKind, KeyModifiers};

pub fn run_main_loop(storage: Option<agent_storage::Storage>) -> ExitCode {
    let mut app = AgentApp::new("AI Coding Agent", env!("CARGO_PKG_VERSION"));

    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("tokio runtime creation failed");

    let mut cfg = agent_config::ConfigLoader::new().load().unwrap_or_default();
    let mut model = cfg.model.clone().unwrap_or_else(commands::default_model);

    app.set_model_config(model.clone());

    agent_ui::files::refresh_file_cache();

    let mut terminal =
        ratatui::Terminal::new(ratatui::backend::CrosstermBackend::new(std::io::stdout()))
            .expect("failed to create terminal");

    let _ = crossterm::terminal::enable_raw_mode();
    let _ = crossterm::execute!(
        std::io::stdout(),
        crossterm::terminal::EnterAlternateScreen,
        crossterm::event::EnableMouseCapture,
        crossterm::terminal::Clear(crossterm::terminal::ClearType::All)
    );

    let _ = terminal.clear();

    loop {
        let _ = terminal.draw(|f| {
            app.render(f, ratatui::layout::Layout::default());
        });

        match crossterm::event::read() {
            Ok(Event::Key(key))
                if matches!(key.kind, KeyEventKind::Press | KeyEventKind::Repeat) =>
            {
                if key.code == crossterm::event::KeyCode::Char('c')
                    && key.modifiers.contains(KeyModifiers::CONTROL)
                {
                    continue;
                }

                if let Some(ev) = app.handle_event(&key) {
                    handle_app_event(
                        ev,
                        &mut app,
                        &mut cfg,
                        &mut model,
                        &runtime,
                        storage.as_ref(),
                    );
                }
            }
            Ok(Event::Mouse(mouse)) => {
                use crossterm::event::MouseEventKind;
                match mouse.kind {
                    MouseEventKind::ScrollUp => {
                        for _ in 0..3 {
                            app.scroll_from_bottom = app.scroll_from_bottom.saturating_add(1);
                        }
                        app.stick_to_bottom = false;
                    }
                    MouseEventKind::ScrollDown => {
                        for _ in 0..3 {
                            app.scroll_from_bottom = app.scroll_from_bottom.saturating_sub(1);
                        }
                        app.stick_to_bottom = app.scroll_from_bottom == 0;
                    }
                    _ => {}
                }
            }
            Ok(Event::Resize(_, _)) => {}
            _ => {}
        }
    }
}

fn handle_app_event(
    ev: AppEvent,
    app: &mut AgentApp,
    cfg: &mut agent_config::AgentConfig,
    model: &mut agent_config::ModelConfig,
    runtime: &tokio::runtime::Runtime,
    storage: Option<&agent_storage::Storage>,
) {
    match ev {
        AppEvent::Shutdown => {
            if let Some(s) = storage {
                let _ = s.write_session(
                    &agent_storage::SessionRecord::new(
                        Some(std::env::current_dir().unwrap_or_default()),
                        Some(model.model.clone()),
                    ),
                    &[],
                );
            }
            restore_terminal();
            std::process::exit(0);
        }
        AppEvent::SendMessage(msg) => {
            app.set_status(agent_ui::StatusType::Thinking("thinking".to_string()));
            commands::send_message(msg, app, model, runtime);
        }
        AppEvent::SlashCommand(cmd) => {
            commands::handle_slash_command(&cmd, cfg, model, app);
        }
        AppEvent::StateChanged(new_state) => {
            app.set_state(new_state);
        }
        AppEvent::Undo => {
            app.undo_last();
        }
        AppEvent::Clear => {
            app.clear_messages();
        }
        _ => {}
    }
}

pub fn restore_terminal() {
    let _ = crossterm::terminal::disable_raw_mode();
    let _ = crossterm::execute!(
        std::io::stdout(),
        crossterm::terminal::LeaveAlternateScreen,
        crossterm::event::DisableMouseCapture
    );
}
