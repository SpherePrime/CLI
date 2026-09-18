use std::collections::HashMap;
use std::path::PathBuf;

use anyhow::{Context, Result};

const FILE_NAME: &str = "credentials.json";

fn store_path() -> PathBuf {
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
    serde_json::from_str(&raw).unwrap_or_default()
}

pub fn save(keys: &HashMap<String, String>) -> Result<()> {
    let path = store_path();
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)
            .with_context(|| format!("creating {}", parent.display()))?;
    }
    let raw = serde_json::to_string_pretty(keys)?;
    std::fs::write(&path, raw).with_context(|| format!("writing {}", path.display()))?;
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
mod tests {
    use super::*;

    #[test]
    fn round_trips_keys() {
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

        remove_key("my-openai").unwrap();
        let keys = load();
        assert!(!keys.contains_key("my-openai"));
    }

    #[test]
    fn env_name_upper_sanitized() {
        assert_eq!(env_name("My-OpenAI"), "AGENT_PROVIDER_MY_OPENAI");
    }
}
