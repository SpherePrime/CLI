use std::collections::HashMap;
use std::path::PathBuf;

use anyhow::{Context, Result};

mod crypto;

pub(crate) const FILE_NAME: &str = "credentials.json";
const VERSION: u32 = 2;

pub(crate) fn store_path() -> PathBuf {
    if let Ok(override_path) = std::env::var("AGENT_CREDENTIALS_FILE") {
        return PathBuf::from(override_path);
    }
    dirs::home_dir()
        .map(|home| home.join(".agent").join(FILE_NAME))
        .unwrap_or_else(|| PathBuf::from(FILE_NAME))
}

pub fn env_name(id: &str) -> String {
    let sanitized: String = id
        .chars()
        .map(|character| match character {
            ascii if ascii.is_ascii_alphanumeric() => ascii.to_ascii_uppercase(),
            '-' | ' ' => '_',
            _ => '_',
        })
        .collect();
    format!("AGENT_PROVIDER_{sanitized}")
}

pub fn load() -> HashMap<String, String> {
    let raw = match std::fs::read_to_string(store_path()) {
        Ok(raw) => raw,
        Err(_) => return HashMap::new(),
    };
    let value: serde_json::Value = match serde_json::from_str(&raw) {
        Ok(value) => value,
        Err(_) => return HashMap::new(),
    };
    if value.get("version").and_then(|v| v.as_u64()) == Some(VERSION as u64) {
        let Some(encoded) = value.get("data").and_then(|v| v.as_str()) else {
            return HashMap::new();
        };
        let plain = match crypto::decrypt(encoded) {
            Ok(plain) => plain,
            Err(_) => return HashMap::new(),
        };
        return serde_json::from_slice(&plain).unwrap_or_default();
    }
    let legacy: HashMap<String, String> = serde_json::from_value(value).unwrap_or_default();
    if !legacy.is_empty() {
        let _ = save(&legacy);
    }
    legacy
}

pub fn save(keys: &HashMap<String, String>) -> Result<()> {
    let path = store_path();
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)
            .with_context(|| format!("creating {}", parent.display()))?;
    }
    let plain = serde_json::to_vec(keys)?;
    let data = crypto::encrypt(&plain)?;
    let envelope = serde_json::json!({ "version": VERSION, "data": data });
    std::fs::write(&path, serde_json::to_string_pretty(&envelope)?)
        .with_context(|| format!("writing {}", path.display()))?;
    Ok(())
}

pub fn set_key(id: &str, key: &str) -> Result<()> {
    let mut keys = load();
    keys.insert(id.to_string(), key.to_string());
    save(&keys)
}

pub fn remove_key(id: &str) -> Result<()> {
    let mut keys = load();
    keys.remove(id);
    save(&keys)
}

pub fn apply_to_env() {
    for (id, key) in load() {
        std::env::set_var(env_name(&id), key);
    }
}

#[cfg(test)]
pub(crate) static ENV_LOCK: std::sync::Mutex<()> = std::sync::Mutex::new(());

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn round_trips_keys() {
        let _guard = ENV_LOCK.lock().unwrap_or_else(|error| error.into_inner());
        let tmp = tempfile::tempdir().unwrap();
        std::env::set_var(
            "AGENT_CREDENTIALS_FILE",
            tmp.path().join("credentials.json"),
        );

        set_key("my-openai", "sk-test-123").unwrap();
        let keys = load();
        assert_eq!(
            keys.get("my-openai").map(|s| s.as_str()),
            Some("sk-test-123")
        );

        let raw = std::fs::read_to_string(store_path()).unwrap();
        assert!(
            !raw.contains("sk-test-123"),
            "key must not be stored in plaintext"
        );
        assert!(raw.contains("\"version\": 2"));

        remove_key("my-openai").unwrap();
        let keys = load();
        assert!(!keys.contains_key("my-openai"));
    }

    #[test]
    fn migrates_legacy_plaintext_store() {
        let _guard = ENV_LOCK.lock().unwrap_or_else(|error| error.into_inner());
        let tmp = tempfile::tempdir().unwrap();
        let path = tmp.path().join("credentials.json");
        std::env::set_var("AGENT_CREDENTIALS_FILE", &path);
        std::fs::write(&path, r#"{"legacy-provider":"sk-legacy"}"#).unwrap();

        let keys = load();
        assert_eq!(
            keys.get("legacy-provider").map(|s| s.as_str()),
            Some("sk-legacy")
        );

        let raw = std::fs::read_to_string(&path).unwrap();
        assert!(raw.contains("\"version\": 2"));
        assert!(!raw.contains("sk-legacy"));
    }

    #[test]
    fn env_name_upper_sanitized() {
        assert_eq!(env_name("My-OpenAI"), "AGENT_PROVIDER_MY_OPENAI");
    }
}
