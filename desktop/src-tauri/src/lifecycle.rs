use std::sync::atomic::Ordering;
use tauri::Manager;

pub fn setup(app: &mut tauri::App) -> Result<(), Box<dyn std::error::Error>> {
    let directory = if cfg!(debug_assertions) {
        std::env::var_os("PRIME_NATIVE_TEST_DATA")
            .map(std::path::PathBuf::from)
            .unwrap_or(app.path().app_data_dir()?)
    } else {
        app.path().app_data_dir()?
    };
    if !directory.is_absolute() {
        return Err("Native preview data directory must be absolute".into());
    }
    std::fs::create_dir_all(&directory)?;
    let directory = dunce::canonicalize(directory)?;
    if !crate::state::isolated_profile() {
        crate::migration::import_legacy_state(&directory);
    }
    let state = crate::state::AppState::new(directory).map_err(|error| error.message)?;
    state
        .teams
        .set_adapter(std::sync::Arc::new(crate::teams::adapter::NativeTeamAdapter::new(
            app.handle().clone(),
        )));
    let browser_server = crate::browser::mcp::start(app.handle(), state.browser.clone())?;
    std::env::set_var("PRIME_DESKTOP_BROWSER_URL", &browser_server.url);
    std::env::set_var("PRIME_DESKTOP_BROWSER_TOKEN", &browser_server.token);
    app.manage(browser_server);
    app.manage(state);
    if let Err(error) = crate::tray::create(app.handle()) {
        eprintln!("Tray unavailable: {error}");
    }
    if let Err(error) = app.state::<crate::state::AppState>().windows.start(app.handle()) {
        eprintln!("Window control unavailable: {error}");
    }
    crate::sudo::publish_environment();
    let handle = app.handle().clone();
    std::thread::spawn(move || {
        let state = handle.state::<crate::state::AppState>();
        crate::migration::run_once(&state);
    });
    let resources = app.path().resource_dir()?;
    let handle = app.handle().clone();
    std::thread::spawn(move || {
        let state = handle.state::<crate::state::AppState>();
        let result = crate::engine::Engine::load_bundled(
            &state.directory,
            Some(&resources),
            state.isolated,
        );
        if state.closing.load(Ordering::Acquire) {
            if let Ok(engine) = result {
                engine.close();
            }
            return;
        }
        let mut status = state.engine.lock().unwrap();
        match result {
            Ok(engine) => status.instance = Some(engine),
            Err(error) => status.error = Some(error.message),
        }
        status.starting = false;
        let payload = serde_json::json!({"status":if status.instance.is_some(){"ready"}else{"error"},"error":status.error,"version":{"version":"Go ABI 1","transport":"in-process"}});
        drop(status);
        crate::events::emit(&handle, "", "runtime", payload);
        if state.require_engine().is_ok() {
            state.automations.start(handle.clone());
            state.teams.start(handle.clone());
        }
    });
    Ok(())
}

pub fn window_event(window: &tauri::Window, event: &tauri::WindowEvent) {
    if window.label() != "main" {
        return;
    }
    if let tauri::WindowEvent::CloseRequested { api, .. } = event {
        let state = window.state::<crate::state::AppState>();
        if state.editor_dirty.load(Ordering::Relaxed) {
            api.prevent_close();
            let window = window.clone();
            std::thread::spawn(move || {
                let result = rfd::MessageDialog::new()
                    .set_title("Несохранённые изменения")
                    .set_description("Закрыть приложение и потерять несохранённые изменения?")
                    .set_buttons(rfd::MessageButtons::YesNo)
                    .show();
                if result == rfd::MessageDialogResult::Yes {
                    window
                        .state::<crate::state::AppState>()
                        .editor_dirty
                        .store(false, Ordering::Relaxed);
                    let _ = window.close();
                }
            });
        } else if !state.close_ready.swap(false, Ordering::AcqRel) {
            api.prevent_close();
            crate::events::emit(window.app_handle(), "", "beforeClose", serde_json::Value::Null);
        } else if !state.quitting.load(Ordering::Acquire) && crate::tray::keep_running(&state) {
            api.prevent_close();
            crate::tray::hide_to_tray(window.app_handle());
        }
    }
}

pub fn run_event(app: &tauri::AppHandle, event: tauri::RunEvent) {
    if let tauri::RunEvent::ExitRequested { api, code, .. } = event {
        let Some(state) = app.try_state::<crate::state::AppState>() else {
            return;
        };
        if state.closing.swap(true, Ordering::AcqRel) {
            return;
        }
        api.prevent_exit();
        let app = app.clone();
        std::thread::spawn(move || {
            let state = app.state::<crate::state::AppState>();
            shutdown(&app, &state);
            app.exit(code.unwrap_or(0));
        });
    }
}

pub fn shutdown(app: &tauri::AppHandle, state: &crate::state::AppState) {
    state.automations.stop();
    state.teams.stop();
    state.windows.shutdown(&app);
    state.speech.shutdown();
    let engine = state.engine.lock().unwrap().instance.clone();
    if let Some(engine) = engine {
        engine.close();
    }
    state.automations.shutdown();
    state.teams.shutdown();
    let _ = state.terminals.shutdown();
    state.remotes.shutdown();
    if let Some(server) = app.try_state::<crate::browser::mcp::Server>() {
        server.shutdown();
    }
}
