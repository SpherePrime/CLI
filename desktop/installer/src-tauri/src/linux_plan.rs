#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Package {
    Deb,
    RpmDnf,
    RpmZypper,
    Portable,
}

impl Package {
    pub fn detect(release: &str) -> Self {
        let mut identities = Vec::new();
        for line in release.lines() {
            let Some((key, value)) = line.split_once('=') else {
                continue;
            };
            if matches!(key.trim(), "ID" | "ID_LIKE") {
                identities.extend(value.trim().trim_matches(['\'', '"']).split_whitespace());
            }
        }
        if identities
            .iter()
            .any(|id| matches!(*id, "debian" | "ubuntu"))
        {
            Self::Deb
        } else if identities.iter().any(|id| {
            matches!(
                *id,
                "suse" | "opensuse" | "opensuse-leap" | "opensuse-tumbleweed"
            )
        }) {
            Self::RpmZypper
        } else if identities
            .iter()
            .any(|id| matches!(*id, "fedora" | "rhel" | "centos" | "rocky" | "almalinux"))
        {
            Self::RpmDnf
        } else {
            Self::Portable
        }
    }

    pub fn manager(self) -> Option<&'static str> {
        match self {
            Self::Deb => Some("/usr/bin/apt-get"),
            Self::RpmDnf => Some("/usr/bin/dnf"),
            Self::RpmZypper => Some("/usr/bin/zypper"),
            Self::Portable => None,
        }
    }

    pub fn arguments(self, path: &str) -> Vec<String> {
        let mut arguments = match self {
            Self::Deb | Self::RpmDnf => vec!["install", "-y"],
            Self::RpmZypper => vec!["--non-interactive", "install", "--allow-unsigned-rpm"],
            Self::Portable => Vec::new(),
        }
        .into_iter()
        .map(str::to_string)
        .collect::<Vec<_>>();
        arguments.push(path.into());
        arguments
    }
}
pub fn desktop_entry(launcher: &std::path::Path) -> Result<String, String> {
    let command = launcher.to_string_lossy();
    if command.contains(['\r', '\n', '\0']) {
        return Err("Путь установки содержит недопустимые символы".into());
    }
    let command = command
        .replace('\\', "\\\\")
        .replace('"', "\\\"")
        .replace('`', "\\`")
        .replace('$', "\\$")
        .replace('\\', "\\\\")
        .replace('%', "%%");
    let directory = launcher.to_string_lossy();
    let directory = directory
        .rsplit_once('/')
        .map(|(parent, _)| parent)
        .unwrap_or("");
    let icon = format!("{directory}/icon.png").replace('\\', "\\\\");
    Ok(format!("[Desktop Entry]\nType=Application\nName=Prime Desktop\nExec=\"{command}\"\nTerminal=false\nIcon={icon}\nCategories=Development;\nStartupWMClass=PrimeDesktop\n"))
}
#[cfg(test)]
mod launcher_tests {
    use super::*;

    #[test]
    fn desktop_launcher_escapes_field_codes_and_shell_characters() {
        let entry = desktop_entry(std::path::Path::new(
            "/home/Prime 100%/$test\"/prime-desktop",
        ))
        .unwrap();
        assert!(entry.contains("Exec=\"/home/Prime 100%%/\\\\$test\\\\\"/prime-desktop\"\n"));
        assert!(entry.contains("Icon=/home/Prime 100%/$test\"/icon.png\n"));
        assert!(desktop_entry(std::path::Path::new(
            "/home/Prime\nInjected=true/prime-desktop"
        ))
        .is_err());
    }
}
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn selects_native_packages_for_derivatives() {
        assert_eq!(Package::detect("ID=ubuntu\nID_LIKE=debian\n"), Package::Deb);
        assert_eq!(
            Package::detect("ID=linuxmint\nID_LIKE=\"ubuntu debian\"\n"),
            Package::Deb
        );
        assert_eq!(Package::detect("ID=fedora\n"), Package::RpmDnf);
        assert_eq!(
            Package::detect("ID=rocky\nID_LIKE=\"rhel centos fedora\"\n"),
            Package::RpmDnf
        );
        assert_eq!(
            Package::detect("ID=opensuse-tumbleweed\nID_LIKE=\"opensuse suse\"\n"),
            Package::RpmZypper
        );
        assert_eq!(Package::detect("ID=arch\n"), Package::Portable);
        assert_eq!(Package::detect("ID=notdebian\n"), Package::Portable);
    }

    #[test]
    fn dependency_install_keeps_package_path_as_one_argument() {
        let arguments = Package::Deb.arguments("/tmp/Prime's package/app.deb");
        assert_eq!(arguments, ["install", "-y", "/tmp/Prime's package/app.deb"]);
        assert_eq!(
            Package::RpmDnf.arguments("/tmp/app.rpm"),
            ["install", "-y", "/tmp/app.rpm"]
        );
        assert_eq!(
            Package::RpmZypper.arguments("/tmp/app.rpm"),
            [
                "--non-interactive",
                "install",
                "--allow-unsigned-rpm",
                "/tmp/app.rpm"
            ]
        );
    }
}

#[cfg(test)]
mod environment_tests {
    use super::*;
    use std::{collections::BTreeMap, ffi::OsString, path::PathBuf};

    #[test]
    fn native_launch_drops_setup_libraries_and_preserves_host_search_paths() {
        let bundle = std::env::temp_dir().join(".mount_prime");
        let environment = BTreeMap::from([
            (OsString::from("APPDIR"), bundle.clone().into_os_string()),
            (
                OsString::from("PATH"),
                std::env::join_paths([
                    bundle.join("usr/bin"),
                    PathBuf::from("/usr/bin"),
                    PathBuf::from("/home/user/bin"),
                ])
                .unwrap(),
            ),
            (
                OsString::from("XDG_DATA_DIRS"),
                std::env::join_paths([bundle.join("usr/share"), PathBuf::from("/usr/share")])
                    .unwrap(),
            ),
            (
                OsString::from("GST_PLUGIN_SCANNER_1_0"),
                OsString::from("/tmp/.mount_prime/scanner"),
            ),
            (
                OsString::from("GSETTINGS_SCHEMA_DIR"),
                OsString::from("/tmp/.mount_prime/schemas"),
            ),
            (OsString::from("DISPLAY"), OsString::from(":0")),
        ]);
        let cleaned = sanitized_environment(environment);
        assert!(!cleaned.contains_key(&OsString::from("APPDIR")));
        assert!(!cleaned.contains_key(&OsString::from("GST_PLUGIN_SCANNER_1_0")));
        assert!(!cleaned.contains_key(&OsString::from("GSETTINGS_SCHEMA_DIR")));
        assert_eq!(
            cleaned.get(&OsString::from("DISPLAY")),
            Some(&OsString::from(":0"))
        );
        assert_eq!(
            std::env::split_paths(&cleaned[&OsString::from("PATH")]).collect::<Vec<_>>(),
            [PathBuf::from("/usr/bin"), PathBuf::from("/home/user/bin")]
        );
        assert_eq!(
            std::env::split_paths(&cleaned[&OsString::from("XDG_DATA_DIRS")]).collect::<Vec<_>>(),
            [PathBuf::from("/usr/share")]
        );
    }
}

pub fn sanitized_environment(
    mut environment: std::collections::BTreeMap<std::ffi::OsString, std::ffi::OsString>,
) -> std::collections::BTreeMap<std::ffi::OsString, std::ffi::OsString> {
    let bundle = environment
        .get(std::ffi::OsStr::new("APPDIR"))
        .map(std::path::PathBuf::from);
    if let Some(bundle) = bundle.filter(|path| path.is_absolute()) {
        for name in ["PATH", "XDG_DATA_DIRS"] {
            let key = std::ffi::OsStr::new(name);
            if let Some(value) = environment.get(key) {
                let paths = std::env::split_paths(value)
                    .filter(|path| !path.starts_with(&bundle))
                    .collect::<Vec<_>>();
                if let Ok(value) = std::env::join_paths(paths) {
                    environment.insert(name.into(), value);
                }
            }
        }
    }
    for name in [
        "APPIMAGE",
        "APPDIR",
        "OWD",
        "ARGV0",
        "LD_LIBRARY_PATH",
        "GIO_MODULE_DIR",
        "GIO_EXTRA_MODULES",
        "GDK_PIXBUF_MODULE_FILE",
        "GDK_PIXBUF_MODULEDIR",
        "GTK_PATH",
        "GTK_DATA_PREFIX",
        "GTK_EXE_PREFIX",
        "GTK_IM_MODULE_FILE",
        "GST_PLUGIN_SYSTEM_PATH",
        "GST_PLUGIN_PATH",
        "GST_PLUGIN_SCANNER",
        "GST_PLUGIN_SYSTEM_PATH_1_0",
        "GST_PLUGIN_PATH_1_0",
        "GST_PLUGIN_SCANNER_1_0",
        "GST_PTP_HELPER_1_0",
        "GSETTINGS_SCHEMA_DIR",
        "GI_TYPELIB_PATH",
        "WEBKIT_EXEC_PATH",
        "WEBKIT_INJECTED_BUNDLE_PATH",
        "APPIMAGE_EXTRACT_AND_RUN",
    ] {
        environment.remove(std::ffi::OsStr::new(name));
    }
    environment
}
