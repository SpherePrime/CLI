use super::types::Profile;
use crate::error::{HostError, HostResult};
use serde::{Deserialize, Serialize};
use std::{collections::HashMap, io::Write, path::PathBuf};

#[derive(Clone, Serialize, Deserialize)]
pub struct Saved {
    pub version: u32,
    pub profiles: Vec<Profile>,
    pub hosts: HashMap<String, String>,
}

impl Default for Saved {
    fn default() -> Self {
        Self {
            version: 1,
            profiles: Vec::new(),
            hosts: HashMap::new(),
        }
    }
}

pub struct Store {
    pub path: PathBuf,
    pub value: Saved,
}

impl Store {
    pub fn load(directory: &std::path::Path) -> HostResult<Self> {
        let path = directory.join("remote-profiles.json");
        let value = match std::fs::read(&path) {
            Ok(bytes) => {
                let mut saved: Saved = serde_json::from_slice(&bytes)?;
                if saved.version != 1 {
                    return Err(HostError::new("REMOTE_STORE", "remote.corrupt"));
                }
                saved.profiles = saved
                    .profiles
                    .into_iter()
                    .map(Profile::validate)
                    .collect::<HostResult<Vec<_>>>()?;
                let unique: std::collections::HashSet<_> =
                    saved.profiles.iter().map(|profile| &profile.id).collect();
                if unique.len() != saved.profiles.len()
                    || saved.hosts.values().any(|fingerprint| {
                        !fingerprint.starts_with("SHA256:") || fingerprint.len() != 50
                    })
                {
                    return Err(HostError::new("REMOTE_STORE", "remote.corrupt"));
                }
                saved
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => Saved::default(),
            Err(error) => return Err(error.into()),
        };
        Ok(Self { path, value })
    }

    pub fn persist(&mut self, value: Saved) -> HostResult<()> {
        let directory = self.path.parent().unwrap();
        std::fs::create_dir_all(directory)?;
        let mut temporary = tempfile::NamedTempFile::new_in(directory)?;
        temporary.write_all(&serde_json::to_vec_pretty(&value)?)?;
        temporary.as_file().sync_all()?;
        temporary.persist(&self.path).map_err(|error| error.error)?;
        self.value = value;
        Ok(())
    }
}
