use crate::{payload, processes, registry, status::InstallStatus, voice::assets, SetupState};
use std::{
    io::Write,
    path::{Path, PathBuf},
    sync::atomic::AtomicBool,
};
use tauri::{AppHandle, Emitter, Manager};

const EXECUTABLE: &str = "PrimeDesktop.exe";
const ENGINE: &str = "engine/prime_desktop_engine.dll";

pub fn default_directory() -> String {
    std::env::var_os("LOCALAPPDATA")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("C:\\"))
        .join("Programs")
        .join(registry::PRODUCT_KEY)
        .to_string_lossy()
        .into_owned()
}

pub fn executable(directory: &str) -> PathBuf {
    Path::new(directory).join(EXECUTABLE)
}

pub fn validate_directory(directory: &str) -> Result<(), String> {
    let path = Path::new(directory);
    if directory.len() > 240
        || !path.is_absolute()
        || directory.contains(['"', '\r', '\n', '\0'])
        || path.parent().is_none()
    {
        return Err("Выберите папку для приложения".into());
    }
    Ok(())
}

fn publish(app: &AppHandle, status: InstallStatus) -> InstallStatus {
    *app.state::<SetupState>().status.lock().unwrap() = status.clone();
    let _ = app.emit("setup:progress", status.clone());
    status
}

pub fn run(app: &AppHandle, directory: &str) -> InstallStatus {
    match install(app, directory) {
        Ok(warning) => publish(
            app,
            InstallStatus {
                warning,
                ..InstallStatus::phase("complete", directory)
            },
        ),
        Err(error) => publish(
            app,
            InstallStatus {
                error: Some(error),
                ..InstallStatus::phase("error", directory)
            },
        ),
    }
}

fn install(app: &AppHandle, directory: &str) -> Result<Option<String>, String> {
    publish(app, InstallStatus::phase("preparing", directory));
    payload::verify()?;
    if processes::app_running() {
        return Err(
            "Закройте Prime Desktop перед установкой, чтобы сохранить ваши задачи и изменения."
                .into(),
        );
    }
    publish(app, InstallStatus::phase("installing", directory));
    remove_legacy_installs()?;
    run_payload(directory)?;
    publish(app, InstallStatus::phase("verifying", directory));
    let root = Path::new(directory);
    if !root.join(EXECUTABLE).is_file() || !root.join(ENGINE).is_file() {
        return Err("Файлы приложения не найдены".into());
    }
    Ok(prepare_voice(app, directory))
}

fn remove_legacy_installs() -> Result<(), String> {
    for legacy in registry::legacy_installs() {
        let uninstaller = Path::new(&legacy.uninstaller);
        let Some(location) = uninstaller.parent() else {
            continue;
        };
        let arguments = format!(
            "{} _?={}",
            legacy.arguments.join(" "),
            location.to_string_lossy()
        );
        let code = processes::run_hidden(uninstaller, arguments.trim())
            .map_err(|error| format!("Не удалось удалить {}: {error}", legacy.name))?;
        if code != 0 {
            return Err(if code == 4 {
                "Закройте Prime Desktop перед установкой и попробуйте снова.".into()
            } else {
                format!("Не удалось удалить {} ({code})", legacy.name)
            });
        }
        let _ = std::fs::remove_file(uninstaller);
        let _ = std::fs::remove_dir(location);
    }
    Ok(())
}

fn run_payload(directory: &str) -> Result<(), String> {
    let mut file = tempfile::Builder::new()
        .prefix("prime-desktop-")
        .suffix(".exe")
        .tempfile()
        .map_err(|error| error.to_string())?;
    file.write_all(payload::BYTES)
        .and_then(|_| file.flush())
        .map_err(|error| error.to_string())?;
    let path = file.into_temp_path();
    let code = processes::run_hidden(&path, &format!("/S /D={directory}"))
        .map_err(|error| error.to_string())?;
    if code != 0 {
        return Err(format!(
            "Установка не завершена ({code}). Проверьте доступ к выбранной папке."
        ));
    }
    Ok(())
}

fn prepare_voice(app: &AppHandle, directory: &str) -> Option<String> {
    publish(
        app,
        InstallStatus {
            percent: Some(0.0),
            ..InstallStatus::phase("voice", directory)
        },
    );
    let mut reported = 0.0;
    let result = assets::ensure(
        &assets::model_directory(),
        &AtomicBool::new(false),
        |percent| {
            if percent - reported < 0.5 && percent < 100.0 {
                return;
            }
            reported = percent;
            publish(
                app,
                InstallStatus {
                    percent: Some(percent),
                    ..InstallStatus::phase("voice", directory)
                },
            );
        },
    );
    result.err().map(|_| {
        "Голосовая модель не скачалась. Prime докачает её при первом голосовом разговоре."
            .to_string()
    })
}
