use chrono::Local;
use std::{
    fs::{self, File, OpenOptions},
    io::Write,
    path::{Path, PathBuf},
    sync::{Mutex, OnceLock},
};

const MAX_LOG_BYTES: u64 = 10 * 1024 * 1024;
const DESKTOP_LOG_NAME: &str = "desktop.log";
const DESKTOP_LOG_BACKUP: &str = "desktop.log.1";

static LOGGER: OnceLock<Mutex<LogSink>> = OnceLock::new();
static ACTIVE_DIR: OnceLock<PathBuf> = OnceLock::new();

struct LogSink {
    path: PathBuf,
    file: Option<File>,
}

fn home_directory() -> Option<PathBuf> {
    #[cfg(windows)]
    {
        if let Some(profile) = std::env::var_os("USERPROFILE").filter(|v| !v.is_empty()) {
            return Some(PathBuf::from(profile));
        }
        let drive = std::env::var_os("HOMEDRIVE").unwrap_or_default();
        let path = std::env::var_os("HOMEPATH").unwrap_or_default();
        if !drive.is_empty() && !path.is_empty() {
            let mut combined = PathBuf::from(drive);
            combined.push(path);
            return Some(combined);
        }
    }
    std::env::var_os("HOME")
        .filter(|v| !v.is_empty())
        .map(PathBuf::from)
}

pub fn resolve_log_dir(app_data_dir: Option<&Path>, isolated: bool) -> PathBuf {
    if let Some(override_dir) = std::env::var_os("PRIME_DESKTOP_LOG_DIR").filter(|v| !v.is_empty())
    {
        return PathBuf::from(override_dir);
    }
    if isolated {
        if let Some(dir) = app_data_dir {
            return dir.join("logs");
        }
    }
    if let Some(home) = home_directory() {
        return home.join(".prime").join("logs");
    }
    app_data_dir
        .map(|dir| dir.join("logs"))
        .unwrap_or_else(|| PathBuf::from("logs"))
}

pub fn active_log_dir() -> Option<PathBuf> {
    ACTIVE_DIR.get().cloned()
}

fn open_log_file(path: &Path) -> Option<File> {
    if let Some(parent) = path.parent() {
        let _ = fs::create_dir_all(parent);
    }
    if let Ok(meta) = fs::metadata(path) {
        if meta.len() >= MAX_LOG_BYTES {
            let backup = path.with_file_name(DESKTOP_LOG_BACKUP);
            let _ = fs::remove_file(&backup);
            let _ = fs::rename(path, &backup);
        }
    }
    OpenOptions::new().create(true).append(true).open(path).ok()
}

pub fn init(app_data_dir: Option<&Path>, isolated: bool) {
    let dir = resolve_log_dir(app_data_dir, isolated);
    let _ = fs::create_dir_all(&dir);
    let _ = ACTIVE_DIR.set(dir.clone());
    let path = dir.join(DESKTOP_LOG_NAME);
    let _ = LOGGER.get_or_init(|| {
        let file = open_log_file(&path);
        Mutex::new(LogSink { path, file })
    });
    install_panic_hook();
    info("Prime Desktop logger initialized");
}

fn install_panic_hook() {
    static HOOK_ONCE: OnceLock<()> = OnceLock::new();
    HOOK_ONCE.get_or_init(|| {
        let previous = std::panic::take_hook();
        std::panic::set_hook(Box::new(move |panic_info| {
            error(&format!("Panic: {panic_info}"));
            previous(panic_info);
        }));
    });
}

fn write_line(level: &str, message: &str) {
    let timestamp = Local::now().format("%Y-%m-%d %H:%M:%S%.3f");
    let line = format!("[{timestamp}] [{level}] {message}\n");
    if cfg!(debug_assertions) {
        eprint!("{line}");
    }
    if let Some(sink) = LOGGER.get() {
        if let Ok(mut guard) = sink.lock() {
            if let Ok(meta) = fs::metadata(&guard.path) {
                if meta.len() >= MAX_LOG_BYTES {
                    guard.file = None;
                    guard.file = open_log_file(&guard.path);
                }
            }
            if guard.file.is_none() {
                guard.file = open_log_file(&guard.path);
            }
            if let Some(file) = guard.file.as_mut() {
                let _ = file.write_all(line.as_bytes());
                let _ = file.flush();
            }
        }
    }
}

pub fn info(message: &str) {
    write_line("INFO", message);
}

pub fn error(message: &str) {
    write_line("ERROR", message);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resolves_non_isolated_logs_under_home_dot_prime_logs() {
        if std::env::var_os("PRIME_DESKTOP_LOG_DIR").is_some() {
            return;
        }
        let app_dir = PathBuf::from("C:/tmp/app-data");
        let resolved = resolve_log_dir(Some(&app_dir), false);
        assert!(resolved.ends_with(Path::new(".prime").join("logs")));
    }

    #[test]
    fn resolves_isolated_logs_inside_app_data_dir() {
        if std::env::var_os("PRIME_DESKTOP_LOG_DIR").is_some() {
            return;
        }
        let app_dir = PathBuf::from("C:/tmp/isolated-data");
        let resolved = resolve_log_dir(Some(&app_dir), true);
        assert_eq!(resolved, app_dir.join("logs"));
    }
}
