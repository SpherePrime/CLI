pub const PRODUCT_KEY: &str = "Prime Desktop";
const LEGACY_PREVIEW_KEY: &str = "Prime Native Preview";

pub struct LegacyInstall {
    pub name: String,
    pub uninstaller: String,
    pub arguments: Vec<String>,
}

#[cfg(windows)]
mod platform {
    use super::*;
    use winreg::{enums::HKEY_CURRENT_USER, RegKey};

    const UNINSTALL: &str = r"Software\Microsoft\Windows\CurrentVersion\Uninstall";

    fn uninstall_root() -> Option<RegKey> {
        RegKey::predef(HKEY_CURRENT_USER).open_subkey(UNINSTALL).ok()
    }

    fn unquote(value: &str) -> String {
        value.trim().trim_matches('"').to_string()
    }

    pub fn current_install() -> Option<String> {
        let key = uninstall_root()?.open_subkey(PRODUCT_KEY).ok()?;
        let location: String = key.get_value("InstallLocation").ok()?;
        let location = unquote(&location);
        std::path::Path::new(&location)
            .is_dir()
            .then_some(location)
    }

    fn split_command(command: &str) -> Option<(String, Vec<String>)> {
        let command = command.trim();
        let (binary, rest) = if let Some(stripped) = command.strip_prefix('"') {
            let end = stripped.find('"')?;
            (stripped[..end].to_string(), stripped[end + 1..].to_string())
        } else {
            let end = command.find(' ').unwrap_or(command.len());
            (command[..end].to_string(), command[end..].to_string())
        };
        Some((
            binary,
            rest.split_whitespace().map(str::to_string).collect(),
        ))
    }

    pub fn legacy_installs() -> Vec<LegacyInstall> {
        let Some(root) = uninstall_root() else {
            return Vec::new();
        };
        let mut found = Vec::new();
        for name in root.enum_keys().flatten() {
            let Ok(key) = root.open_subkey(&name) else {
                continue;
            };
            let display: String = key.get_value("DisplayName").unwrap_or_default();
            let quiet: String = key.get_value("QuietUninstallString").unwrap_or_default();
            let plain: String = key.get_value("UninstallString").unwrap_or_default();
            let command = if name == LEGACY_PREVIEW_KEY {
                format!("{plain} /S")
            } else if display.starts_with("Prime Desktop")
                && quiet.contains("Uninstall PrimeDesktop.exe")
            {
                quiet
            } else {
                continue;
            };
            if let Some((uninstaller, arguments)) = split_command(&command) {
                if std::path::Path::new(&uninstaller).is_file() {
                    found.push(LegacyInstall {
                        name: display,
                        uninstaller,
                        arguments,
                    });
                }
            }
        }
        found
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn quoted_uninstall_commands_keep_their_arguments() {
            let (binary, arguments) = split_command(
                r#""C:\Program Files\Prime\Uninstall PrimeDesktop.exe" /currentuser /S"#,
            )
            .unwrap();
            assert_eq!(binary, r"C:\Program Files\Prime\Uninstall PrimeDesktop.exe");
            assert_eq!(arguments, vec!["/currentuser", "/S"]);
        }
    }
}

#[cfg(not(windows))]
mod platform {
    use super::*;

    pub fn current_install() -> Option<String> {
        None
    }

    pub fn legacy_installs() -> Vec<LegacyInstall> {
        Vec::new()
    }
}

pub use platform::{current_install, legacy_installs};
