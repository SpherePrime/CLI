use crate::error::{HostError, HostResult};
use std::{ffi::OsString, path::PathBuf};
use tauri::utils::config::AppDirectoriesOverride;

pub fn configure(config: &mut tauri::Config, directory: Option<OsString>) -> HostResult<()> {
    let Some(directory) = directory else {
        return Ok(());
    };
    let directory = PathBuf::from(directory);
    if !directory.is_absolute() || directory.parent().is_none() {
        return Err(HostError::new(
            "INVALID_TEST_DATA",
            "Native test data must use an absolute dedicated directory",
        ));
    }
    std::fs::create_dir_all(&directory)?;
    let directory = dunce::canonicalize(directory)?;
    config.app.app_directories_override = Some(AppDirectoriesOverride::Root(directory));
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_data_overrides_all_application_directories() {
        let temporary = tempfile::tempdir().unwrap();
        let directory = temporary.path().join("isolated");
        let mut config = tauri::Config::default();
        configure(&mut config, Some(directory.clone().into_os_string())).unwrap();
        assert_eq!(
            config.app.app_directories_override,
            Some(AppDirectoriesOverride::Root(
                dunce::canonicalize(&directory).unwrap()
            ))
        );
        assert!(directory.is_dir());
    }

    #[test]
    fn relative_test_data_is_rejected() {
        let mut config = tauri::Config::default();
        assert!(configure(&mut config, Some(OsString::from("relative-fixture"))).is_err());
        assert!(config.app.app_directories_override.is_none());
    }
}
