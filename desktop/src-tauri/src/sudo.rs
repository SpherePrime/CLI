use crate::error::{HostError, HostResult};
use serde_json::{json, Value};

const SERVICE: &str = "io.sphereprime.native-preview.sudo";

fn entry() -> HostResult<keyring::v1::Entry> {
    keyring::v1::Entry::new(SERVICE, "root")
        .map_err(|_| HostError::new("SUDO_STORAGE", "Sudo settings storage is unavailable"))
}

pub fn available() -> bool {
    cfg!(target_os = "linux") && keyring::v1::Entry::store_status().is_ok()
}

pub fn load() -> HostResult<Option<String>> {
    if !cfg!(target_os = "linux") {
        return Ok(None);
    }
    match entry()?.get_password() {
        Ok(password) if !password.is_empty() => Ok(Some(password)),
        Ok(_) => Ok(None),
        Err(keyring::v1::Error::NoEntry) => Ok(None),
        Err(_) => Err(HostError::new("SUDO_STORAGE", "Sudo settings storage is unavailable")),
    }
}

pub fn save(password: &str) -> HostResult<()> {
    if !cfg!(target_os = "linux") {
        return Err(HostError::unsupported(
            "sudo settings are only available on Linux",
        ));
    }
    entry()?
        .set_password(password)
        .map_err(|_| HostError::new("SUDO_STORAGE", "Sudo settings storage is unavailable"))
}

pub fn remove() -> HostResult<()> {
    match entry()?.delete_credential() {
        Ok(()) | Err(keyring::v1::Error::NoEntry) => Ok(()),
        Err(_) => Err(HostError::new("SUDO_STORAGE", "Sudo settings storage is unavailable")),
    }
}

/// Publishes the stored sudo password as the runtime environment entry
/// the embedded Go sudo tool reads. Called before the engine loads; on
/// other platforms the variable is removed so a stale value never
/// leaks into a new platform build.
pub fn publish_environment() {
    let value = load().ok().flatten();
    match value {
        Some(password) => std::env::set_var("PRIME_DESKTOP_SUDO_PASSWORD", password),
        None => std::env::remove_var("PRIME_DESKTOP_SUDO_PASSWORD"),
    }
}

pub fn handle(request: &Value) -> HostResult<Value> {
    match request["action"].as_str() {
        Some("status") => Ok(json!({
            "hasPassword": load().map(|value| value.is_some()).unwrap_or(false),
            "storageAvailable": available(),
        })),
        Some("set") => {
            let password = request["password"]
                .as_str()
                .filter(|password| !password.is_empty() && password.len() <= 4096)
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "invalid password"))?;
            save(password)?;
            Ok(json!({"hasPassword":true,"storageAvailable":available()}))
        }
        Some("remove") => {
            remove()?;
            Ok(json!({"hasPassword":false,"storageAvailable":available()}))
        }
        _ => Err(HostError::new("INVALID_ARGUMENT", "Invalid sudo request")),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn status_shape_and_set_validation() {
        let response = handle(&serde_json::json!({"action":"status"})).unwrap();
        assert!(response["hasPassword"].is_boolean());
        assert!(response["storageAvailable"].is_boolean());
        assert!(handle(&serde_json::json!({"action":"set","password":""})).is_err());
        assert!(handle(&serde_json::json!({"action":"other"})).is_err());
    }
}
