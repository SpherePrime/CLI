use crate::error::{HostError, HostResult};
use serde_json::{json, Value};
use std::{fs, io::Write, path::PathBuf};

pub struct Settings {
    path: PathBuf,
    value: Value,
}

impl Settings {
    pub fn load(directory: &std::path::Path) -> HostResult<Self> {
        fs::create_dir_all(directory)?;
        let path = directory.join("preferences.json");
        let mut value = json!({"theme":"dark","language":"ru","recent":[],"pinned":[],"drafts":{},"sidebarWidth":256,"inspectorWidth":350});
        match fs::read(&path) {
            Ok(bytes) => {
                let saved: Value = serde_json::from_slice(&bytes)?;
                if let Some(preferences) = saved.get("preferences").and_then(Value::as_object) {
                    value.as_object_mut().unwrap().extend(preferences.clone());
                }
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
            Err(error) => return Err(error.into()),
        }
        Ok(Self { path, value })
    }

    pub fn get(&self) -> Value {
        self.value.clone()
    }

    pub fn update(&mut self, patch: &Value) -> HostResult<Value> {
        let patch = patch.as_object().ok_or_else(|| {
            HostError::new("INVALID_ARGUMENT", "Preferences patch must be an object")
        })?;
        for (key, value) in patch {
            let valid = match key.as_str() {
                "theme" => value
                    .as_str()
                    .is_some_and(|value| ["dark", "light", "system"].contains(&value)),
                "language" => value
                    .as_str()
                    .is_some_and(|value| ["ru", "en"].contains(&value)),
                "sidebarWidth" => value
                    .as_f64()
                    .is_some_and(|value| (200.0..=500.0).contains(&value)),
                "inspectorWidth" => value
                    .as_f64()
                    .is_some_and(|value| (260.0..=800.0).contains(&value)),
                "recent" | "pinned" => value
                    .as_array()
                    .is_some_and(|items| items.len() <= 1000 && items.iter().all(Value::is_string)),
                _ => true,
            };
            if !valid {
                return Err(HostError::new(
                    "INVALID_ARGUMENT",
                    format!("Invalid preference {key}"),
                ));
            }
        }
        let mut next = self.value.clone();
        for (key, value) in patch {
            if matches!(
                key.as_str(),
                "drafts"
                    | "activeSessions"
                    | "skipBySession"
                    | "reasoningBySession"
                    | "agentContexts"
                    | "sessionOrder"
            ) {
                let current = next
                    .as_object_mut()
                    .unwrap()
                    .entry(key)
                    .or_insert(json!({}));
                if let (Some(current), Some(patch)) = (current.as_object_mut(), value.as_object()) {
                    current.extend(patch.clone());
                } else {
                    return Err(HostError::new(
                        "INVALID_ARGUMENT",
                        format!("{key} must be an object"),
                    ));
                }
            } else {
                next[key] = value.clone();
            }
        }
        let mut temporary = tempfile::NamedTempFile::new_in(self.path.parent().unwrap())?;
        temporary.write_all(&serde_json::to_vec_pretty(
            &json!({"version":1,"preferences":next}),
        )?)?;
        temporary.as_file().sync_all()?;
        temporary.persist(&self.path).map_err(|error| error.error)?;
        self.value = next;
        Ok(self.get())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn settings_round_trip_and_nested_merge() {
        let directory = tempfile::tempdir().unwrap();
        let mut settings = Settings::load(directory.path()).unwrap();
        settings
            .update(&json!({"theme":"light","drafts":{"one":"first"}}))
            .unwrap();
        settings
            .update(&json!({"drafts":{"two":"second"}}))
            .unwrap();
        let persisted = Settings::load(directory.path()).unwrap().get();
        assert_eq!(persisted["theme"], "light");
        assert_eq!(persisted["drafts"], json!({"one":"first","two":"second"}));
        assert!(settings.update(&json!({"sidebarWidth":-1})).is_err());
        assert_eq!(settings.get()["sidebarWidth"], 256);
    }
}
