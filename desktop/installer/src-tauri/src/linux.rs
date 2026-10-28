use crate::{linux_plan::Package, payload};
use std::{
    fs,
    io::Write,
    os::unix::fs::PermissionsExt,
    path::{Path, PathBuf},
    process::{Command, Stdio},
};

fn package() -> Package {
    let release = fs::read_to_string("/etc/os-release").unwrap_or_default();
    let selected = Package::detect(&release);
    if selected
        .manager()
        .is_some_and(|path| Path::new(path).is_file())
    {
        selected
    } else {
        Package::Portable
    }
}

fn data_directory() -> PathBuf {
    std::env::var_os("XDG_DATA_HOME")
        .map(PathBuf::from)
        .filter(|path| path.is_absolute())
        .unwrap_or_else(|| {
            PathBuf::from(std::env::var_os("HOME").unwrap_or_default()).join(".local/share")
        })
}

pub fn default_directory() -> String {
    match package() {
        Package::Portable => data_directory().join("prime-desktop"),
        _ => PathBuf::from("/usr/bin"),
    }
    .to_string_lossy()
    .into_owned()
}

pub fn executable(directory: &str) -> PathBuf {
    Path::new(directory).join(if package() == Package::Portable {
        "PrimeDesktop.AppImage"
    } else {
        "PrimeDesktop"
    })
}

pub fn current_install() -> Option<String> {
    let directory = default_directory();
    executable(&directory).is_file().then_some(directory)
}

pub fn method_label() -> String {
    match package() {
        Package::Deb => "Linux · DEB · зависимости устанавливаются автоматически",
        Package::RpmDnf => "Linux · RPM · зависимости устанавливаются автоматически",
        Package::RpmZypper => "Linux · openSUSE · зависимости устанавливаются автоматически",
        Package::Portable => "Linux · AppImage · установка в профиль пользователя",
    }
    .into()
}

pub fn validate_directory(directory: &str) -> Result<(), String> {
    if directory != default_directory()
        || !Path::new(directory).is_absolute()
        || directory.contains(['\r', '\n', '\0'])
    {
        return Err("Папка установки определяется автоматически для вашей системы".into());
    }
    Ok(())
}

pub fn app_running() -> bool {
    fs::read_dir("/proc")
        .into_iter()
        .flatten()
        .flatten()
        .any(|entry| {
            if entry.file_name().to_string_lossy().parse::<u32>().ok() == Some(std::process::id()) {
                return false;
            }
            fs::read_link(entry.path().join("exe"))
                .ok()
                .and_then(|path| {
                    path.file_name()
                        .map(|name| name.to_string_lossy().into_owned())
                })
                .is_some_and(|name| {
                    matches!(
                        name.trim_end_matches(" (deleted)"),
                        "PrimeDesktop" | "prime-native" | "PrimeDesktop.AppImage"
                    )
                })
        })
}

pub fn install(directory: &str) -> Result<(), String> {
    let selected = package();
    match selected {
        Package::Portable => install_portable(directory),
        _ => install_package(selected),
    }
}

fn install_package(selected: Package) -> Result<(), String> {
    let (bytes, digest, extension) = match selected {
        Package::Deb => (payload::DEB_BYTES, payload::DEB_SHA256, ".deb"),
        _ => (payload::RPM_BYTES, payload::RPM_SHA256, ".rpm"),
    };
    payload::verify_bytes(bytes, digest)?;
    let directory = tempfile::Builder::new()
        .prefix("prime-desktop-")
        .tempdir()
        .map_err(|error| error.to_string())?;
    fs::set_permissions(directory.path(), fs::Permissions::from_mode(0o755))
        .map_err(|error| error.to_string())?;
    let path = directory.path().join(format!("prime-desktop{extension}"));
    fs::write(&path, bytes).map_err(|error| error.to_string())?;
    fs::set_permissions(&path, fs::Permissions::from_mode(0o644))
        .map_err(|error| error.to_string())?;
    let manager = selected.manager().unwrap();
    let mut command = Command::new("/usr/bin/pkexec");
    command
        .arg(manager)
        .args(selected.arguments(&path.to_string_lossy()));
    clean_environment(&mut command);
    let result = command.stdin(Stdio::null()).output().map_err(|error| {
        format!("Не удалось запустить установку пакета. Проверьте наличие polkit/pkexec: {error}")
    })?;
    if !result.status.success() {
        let details = String::from_utf8_lossy(&result.stderr);
        return Err(format!(
            "Установка пакета не завершена ({}). {}",
            result.status,
            details.trim()
        ));
    }
    Ok(())
}

fn install_portable(directory: &str) -> Result<(), String> {
    payload::verify()?;
    let root = Path::new(directory);
    fs::create_dir_all(root).map_err(|error| error.to_string())?;
    let mut staged = tempfile::NamedTempFile::new_in(root).map_err(|error| error.to_string())?;
    staged
        .write_all(payload::BYTES)
        .and_then(|_| staged.flush())
        .map_err(|error| error.to_string())?;
    staged
        .as_file()
        .set_permissions(fs::Permissions::from_mode(0o755))
        .map_err(|error| error.to_string())?;
    staged
        .persist(executable(directory))
        .map_err(|error| error.to_string())?;
    let launcher = root.join("prime-desktop");
    let appimage = executable(directory)
        .to_string_lossy()
        .replace('\'', "'\\''");
    fs::write(
        &launcher,
        format!("#!/bin/sh\nexport APPIMAGE_EXTRACT_AND_RUN=1\nexec '{appimage}' \"$@\"\n"),
    )
    .map_err(|error| error.to_string())?;
    fs::set_permissions(&launcher, fs::Permissions::from_mode(0o755))
        .map_err(|error| error.to_string())?;
    fs::write(
        root.join("icon.png"),
        include_bytes!("../../../build/icon.png"),
    )
    .map_err(|error| error.to_string())?;
    let applications = data_directory().join("applications");
    fs::create_dir_all(&applications).map_err(|error| error.to_string())?;
    fs::write(
        applications.join("io.sphereprime.native-preview.desktop"),
        crate::linux_plan::desktop_entry(&launcher)?,
    )
    .map_err(|error| error.to_string())?;
    Ok(())
}

pub fn verify(directory: &str) -> Result<(), String> {
    if !executable(directory).is_file() {
        return Err("Файлы приложения не найдены после установки".into());
    }
    Ok(())
}

pub fn launch(directory: &str) -> Result<(), String> {
    let binary = executable(directory);
    let mut command = Command::new(&binary);
    clean_environment(&mut command);
    if package() == Package::Portable {
        command.env("APPIMAGE_EXTRACT_AND_RUN", "1");
    }
    command
        .current_dir(binary.parent().unwrap_or(Path::new("/")))
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .map(|_| ())
        .map_err(|error| error.to_string())
}

fn clean_environment(command: &mut Command) {
    command
        .env_clear()
        .envs(crate::linux_plan::sanitized_environment(
            std::env::vars_os().collect(),
        ));
}
