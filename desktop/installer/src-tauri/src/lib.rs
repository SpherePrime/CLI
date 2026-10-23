#[allow(dead_code)]
#[path = "../../../src-tauri/src/error.rs"]
mod error;
mod install;
mod payload;
mod processes;
mod registry;
mod status;
mod voice;

use status::{InstallStatus, SetupInfo};
use std::sync::Mutex;
use tauri::{AppHandle, Manager, State};

pub struct SetupState {
    status: Mutex<InstallStatus>,
}

fn active(state: &SetupState) -> bool {
    state.status.lock().unwrap().active()
}

#[tauri::command]
fn setup_info() -> Result<SetupInfo, String> {
    let existing = registry::current_install();
    Ok(SetupInfo::new(
        env!("CARGO_PKG_VERSION"),
        existing
            .clone()
            .unwrap_or_else(install::default_directory),
        payload::INSTALLED_SIZE,
        existing.is_some(),
    ))
}

#[tauri::command]
async fn setup_choose_directory(
    state: State<'_, SetupState>,
    current: String,
) -> Result<Option<String>, String> {
    if active(&state) || registry::current_install().is_some() {
        return Ok(None);
    }
    let picked = rfd::AsyncFileDialog::new()
        .set_title("Папка для Prime Desktop")
        .set_directory(&current)
        .pick_folder()
        .await;
    Ok(picked.map(|folder| folder.path().to_string_lossy().into_owned()))
}

#[tauri::command]
async fn setup_install(app: AppHandle, directory: String) -> Result<InstallStatus, String> {
    let directory = match registry::current_install() {
        Some(existing) => existing,
        None => directory,
    };
    install::validate_directory(&directory)?;
    {
        let state = app.state::<SetupState>();
        let mut status = state.status.lock().unwrap();
        if status.active() {
            return Err("Установка уже выполняется".into());
        }
        *status = InstallStatus::phase("preparing", &directory);
    }
    let worker = app.clone();
    tauri::async_runtime::spawn_blocking(move || install::run(&worker, &directory))
        .await
        .map_err(|error| error.to_string())
}

#[tauri::command]
fn setup_open(app: AppHandle, state: State<'_, SetupState>) -> Result<(), String> {
    let status = state.status.lock().unwrap().clone();
    let directory = match (status.phase.as_str(), status.directory) {
        ("complete", Some(directory)) => directory,
        _ => return Err("Установка не завершена".into()),
    };
    processes::launch(&install::executable(&directory))
        .map_err(|_| "Не удалось открыть приложение".to_string())?;
    app.exit(0);
    Ok(())
}

#[tauri::command]
fn setup_close(app: AppHandle, state: State<'_, SetupState>) {
    if !active(&state) {
        app.exit(0);
    }
}

pub fn run() {
    tauri::Builder::default()
        .manage(SetupState {
            status: Mutex::new(InstallStatus::ready()),
        })
        .invoke_handler(tauri::generate_handler![
            setup_info,
            setup_choose_directory,
            setup_install,
            setup_open,
            setup_close
        ])
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                if active(&window.state::<SetupState>()) {
                    api.prevent_close();
                }
            }
        })
        .run(tauri::generate_context!())
        .expect("failed to start Prime Desktop setup");
}
