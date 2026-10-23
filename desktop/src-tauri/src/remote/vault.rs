use super::types::{Credentials, Profile};
use crate::error::{HostError, HostResult};
use keyring::v1::{Entry, Error};

const SERVICE: &str = "io.sphereprime.native-preview.ssh";

fn entry(profile: &Profile) -> HostResult<Entry> {
    Entry::new(SERVICE, &profile.credential_id())
        .map_err(|_| HostError::new("CREDENTIAL_STORAGE", "remote.credentialsUnavailable"))
}

pub fn available() -> bool {
    Entry::store_status().is_ok()
}

pub fn load(profile: &Profile) -> HostResult<Credentials> {
    match entry(profile)?.get_password() {
        Ok(secret) => serde_json::from_str(&secret)
            .map_err(|_| HostError::new("CREDENTIAL_STORAGE", "remote.credentialsUnreadable")),
        Err(Error::NoEntry) => Ok(Credentials::default()),
        Err(_) => Err(HostError::new(
            "CREDENTIAL_STORAGE",
            "remote.credentialsUnreadable",
        )),
    }
}

pub fn has(profile: &Profile) -> bool {
    load(profile).is_ok_and(|credentials| credentials.supplied())
}

pub fn save(profile: &Profile, credentials: &Credentials) -> HostResult<()> {
    let secret = serde_json::to_string(credentials)?;
    entry(profile)?
        .set_password(&secret)
        .map_err(|_| HostError::new("CREDENTIAL_STORAGE", "remote.credentialsUnavailable"))
}

pub fn remove(profile: &Profile) -> HostResult<()> {
    match entry(profile)?.delete_credential() {
        Ok(()) | Err(Error::NoEntry) => Ok(()),
        Err(_) => Err(HostError::new(
            "CREDENTIAL_STORAGE",
            "remote.credentialsUnavailable",
        )),
    }
}
