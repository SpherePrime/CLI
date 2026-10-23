use crate::error::{HostError, HostResult};
use serde::{Deserialize, Serialize};

#[derive(Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Profile {
    #[serde(default = "new_id")]
    pub id: String,
    pub name: String,
    pub host: String,
    pub port: u16,
    pub user: String,
    pub root: String,
    pub auth: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub key_path: Option<String>,
    pub runtime: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub runtime_path: Option<String>,
}

fn new_id() -> String {
    uuid::Uuid::new_v4().to_string()
}

impl Profile {
    pub fn validate(mut self) -> HostResult<Self> {
        if uuid::Uuid::parse_str(&self.id).is_err()
            || self.port == 0
            || !["agent", "key", "password"].contains(&self.auth.as_str())
            || !["bundled", "existing"].contains(&self.runtime.as_str())
        {
            return Err(HostError::new("REMOTE_PROFILE", "remote.invalidProfile"));
        }
        for (name, value, limit) in [
            ("name", &mut self.name, 120),
            ("host", &mut self.host, 253),
            ("user", &mut self.user, 120),
            ("root", &mut self.root, 4096),
        ] {
            *value = value.trim().to_owned();
            if value.is_empty()
                || value.chars().count() > limit
                || value
                    .chars()
                    .any(|character| matches!(character, '\0' | '\r' | '\n'))
            {
                return Err(HostError::new(
                    "REMOTE_PROFILE",
                    format!("remote.invalidProfile: {name}"),
                ));
            }
        }
        if !self.root.starts_with('/')
            || self.root.contains('\\')
            || self.root.split('/').any(|part| part == "..")
        {
            return Err(HostError::new("REMOTE_PROFILE", "remote.invalidProfile"));
        }
        if self.auth == "key"
            && self
                .key_path
                .as_ref()
                .is_none_or(|path| path.is_empty() || path.len() > 4096 || path.contains('\0'))
        {
            return Err(HostError::new("REMOTE_PROFILE", "remote.invalidKey"));
        }
        if self.runtime == "existing"
            && self.runtime_path.as_ref().is_none_or(|path| {
                !path.starts_with('/')
                    || path.len() > 4096
                    || path
                        .chars()
                        .any(|character| matches!(character, '\0' | '\r' | '\n'))
            })
        {
            return Err(HostError::new("REMOTE_PROFILE", "remote.invalidProfile"));
        }
        self.root = format!(
            "/{}",
            self.root
                .split('/')
                .filter(|part| !part.is_empty() && *part != ".")
                .collect::<Vec<_>>()
                .join("/")
        );
        Ok(self)
    }

    pub fn host_id(&self) -> String {
        format!("[{}]:{}", self.host.to_lowercase(), self.port)
    }

    pub fn credential_id(&self) -> String {
        use sha2::{Digest, Sha256};
        let binding = serde_json::to_vec(&serde_json::json!([
            self.host.to_lowercase(),
            self.port,
            self.user,
            self.auth,
            self.key_path.as_deref().unwrap_or("")
        ]))
        .unwrap();
        format!("{}:{:x}", self.id, Sha256::digest(binding))
    }
}

#[derive(Clone, Default, Deserialize, Serialize)]
pub struct Credentials {
    pub password: Option<String>,
    pub passphrase: Option<String>,
}

impl Credentials {
    pub fn supplied(&self) -> bool {
        self.password
            .as_ref()
            .is_some_and(|value| !value.is_empty())
            || self
                .passphrase
                .as_ref()
                .is_some_and(|value| !value.is_empty())
    }
}

#[derive(Clone, Default, Serialize)]
pub struct ConnectionStatus {
    pub status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub challenge: Option<Challenge>,
}

#[derive(Clone, Serialize)]
pub struct Challenge {
    pub fingerprint: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub previous: Option<String>,
}
