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
    state.teams.set_adapter(std::sync::Arc::new(
        crate::teams::adapter::NativeTeamAdapter::new(app.handle().clone()),
    ));
    let browser_server = crate::browser::mcp::start(app.handle(), state.browser.clone())?;
    std::env::set_var("PRIME_DESKTOP_BROWSER_URL", &browser_server.url);
    std::env::set_var("PRIME_DESKTOP_BROWSER_TOKEN", &browser_server.token);
    std::env::set_var("PRIME_DESKTOP_MCP_AREAS", crate::browser::areas::names());
    app.manage(browser_server);
    app.manage(state);
    if let Err(error) = crate::tray::create(app.handle()) {
        eprintln!("Tray unavailable: {error}");
    }
    if let Err(error) = app
        .state::<crate::state::AppState>()
        .windows
        .start(app.handle())
    {
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
        let result =
            crate::engine::Engine::load_bundled(&state.directory, Some(&resources), state.isolated);
        let Some(payload) = install_engine_result(&state.closing, &state.engine, result, |engine| {
            engine.close();
        }) else {
            return;
        };
        crate::events::emit(&handle, "", "runtime", payload);
        if state.require_engine().is_ok() {
            state.automations.start(handle.clone());
            state.teams.start(handle.clone());
        }
    });
    Ok(())
}

fn install_engine_result<T, F>(
    closing: &std::sync::atomic::AtomicBool,
    engine: &std::sync::Mutex<crate::state::EngineSlot<T>>,
    result: crate::error::HostResult<T>,
    close_engine: F,
) -> Option<serde_json::Value>
where
    F: FnOnce(T),
{
    let mut status = engine.lock().unwrap();
    if closing.load(Ordering::Acquire) {
        status.starting = false;
        drop(status);
        if let Ok(instance) = result {
            close_engine(instance);
        }
        return None;
    }
    match result {
        Ok(instance) => status.instance = Some(instance),
        Err(error) => status.error = Some(error.message),
    }
    status.starting = false;
    Some(serde_json::json!({
        "status": if status.instance.is_some() { "ready" } else { "error" },
        "error": status.error,
        "version": { "version": "Go ABI 1", "transport": "in-process" }
    }))
}

fn take_engine_for_shutdown<T, F>(
    closing: &std::sync::atomic::AtomicBool,
    engine: &std::sync::Mutex<crate::state::EngineSlot<T>>,
    close_engine: F,
) where
    F: FnOnce(T),
{
    closing.store(true, Ordering::Release);
    let instance = engine.lock().unwrap().instance.take();
    if let Some(instance) = instance {
        close_engine(instance);
    }
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
                if matches!(result, rfd::MessageDialogResult::Yes | rfd::MessageDialogResult::Ok) {
                    let state = window.state::<crate::state::AppState>();
                    state.editor_dirty.store(false, Ordering::Relaxed);
                    state.quitting.store(true, Ordering::Release);
                    state.close_ready.store(true, Ordering::Release);
                    state.close_in_progress.store(false, Ordering::Release);
                    let _ = window.close();
                }
            });
        } else if !state.quitting.load(Ordering::Acquire)
            && !state.close_ready.swap(false, Ordering::AcqRel)
        {
            api.prevent_close();
            if state.close_in_progress.swap(true, Ordering::AcqRel) {
                return;
            }
            crate::events::emit(
                window.app_handle(),
                "",
                "beforeClose",
                serde_json::Value::Null,
            );
            let window_clone = window.clone();
            let generation = state.close_generation.fetch_add(1, Ordering::SeqCst) + 1;
            std::thread::spawn(move || {
                std::thread::sleep(std::time::Duration::from_millis(600));
                let state = window_clone.state::<crate::state::AppState>();
                if state.close_generation.load(Ordering::Acquire) == generation {
                    state.close_in_progress.store(false, Ordering::Release);
                    state.close_ready.store(true, Ordering::Release);
                    let _ = window_clone.close();
                }
            });
        } else if crate::tray::should_hide_to_tray(state.quitting.load(Ordering::Acquire)) {
            api.prevent_close();
            state.close_in_progress.store(false, Ordering::Release);
            state.close_generation.fetch_add(1, Ordering::SeqCst);
            let _ = window.hide();
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
    take_engine_for_shutdown(&state.closing, &state.engine, |engine| {
        engine.close();
    });
    state.automations.shutdown();
    state.teams.shutdown();
    let _ = state.terminals.shutdown();
    state.remotes.shutdown();
    if let Some(server) = app.try_state::<crate::browser::mcp::Server>() {
        server.shutdown();
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::state::EngineSlot;
    use std::sync::{
        atomic::{AtomicBool, AtomicUsize},
        Arc, Barrier, Mutex,
    };

    #[test]
    fn engine_load_closes_instance_when_closing_set_before_lock() {
        let closing = AtomicBool::new(false);
        let engine = Mutex::new(EngineSlot::<Arc<AtomicBool>> {
            instance: None,
            error: None,
            starting: true,
        });
        let loaded = Arc::new(AtomicBool::new(false));
        closing.store(true, Ordering::Release);
        let payload = install_engine_result(&closing, &engine, Ok(loaded.clone()), |instance| {
            instance.store(true, Ordering::Release);
        });
        assert!(payload.is_none());
        assert!(loaded.load(Ordering::Acquire));
        let status = engine.lock().unwrap();
        assert!(status.instance.is_none());
        assert!(!status.starting);
    }

    #[test]
    fn engine_load_and_shutdown_race_closes_exactly_once() {
        for _ in 0..64 {
            let closing = Arc::new(AtomicBool::new(false));
            let engine = Arc::new(Mutex::new(EngineSlot::<Arc<AtomicUsize>> {
                instance: None,
                error: None,
                starting: true,
            }));
            let closes = Arc::new(AtomicUsize::new(0));
            let barrier = Arc::new(Barrier::new(2));

            let loader_closing = closing.clone();
            let loader_engine = engine.clone();
            let loader_closes = closes.clone();
            let loader_barrier = barrier.clone();
            let loader = std::thread::spawn(move || {
                loader_barrier.wait();
                install_engine_result(&loader_closing, &loader_engine, Ok(loader_closes), |instance| {
                    instance.fetch_add(1, Ordering::AcqRel);
                });
            });

            let shutdown_closing = closing.clone();
            let shutdown_engine = engine.clone();
            let shutdown_barrier = barrier.clone();
            let stopper = std::thread::spawn(move || {
                shutdown_barrier.wait();
                take_engine_for_shutdown(&shutdown_closing, &shutdown_engine, |instance| {
                    instance.fetch_add(1, Ordering::AcqRel);
                });
            });

            loader.join().unwrap();
            stopper.join().unwrap();
            assert_eq!(closes.load(Ordering::Acquire), 1);
            assert!(engine.lock().unwrap().instance.is_none());
        }
    }
}
