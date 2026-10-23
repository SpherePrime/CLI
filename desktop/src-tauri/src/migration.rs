use crate::error::{HostError, HostResult};
use serde_json::{json, Value};

/// Migrates user data from the installed Electron application into the
/// native data directory. The native application never writes legacy
/// secrets in plaintext: encrypted blobs are decrypted in memory and
/// immediately re-encrypted into the native credential vault.
///
/// Platform support:
/// - Windows: Electron `safeStorage` uses DPAPI, so stored secrets can
///   be decrypted and re-protected by the native host.
/// - macOS/Linux: `safeStorage` keys live in the OS keyring under the
///   Electron application's own storage and cannot be read from Rust.
///   Migration reports an explicit unsupported result instead of a
///   silently skipped feature.
pub fn run(state: &crate::state::AppState) -> HostResult<Value> {
    let Some(legacy) = legacy_directory() else {
        return Ok(json!({"status":"absent"}));
    };
    if !legacy.join("remote-credentials.json").exists() && !legacy.join("remote-profiles.json").exists() {
        return Ok(json!({"status":"absent"}));
    }
    let mut report = json!({
        "status":"migrated",
        "profilesImported":0,
        "profilesSkipped":0,
        "credentialsImported":0,
        "credentialsFailed":0,
        "errors":[]
    });
    migrate_profiles(state, &legacy, &mut report)?;
    migrate_credentials(state, &legacy, &mut report);
    finish(state, &report)?;
    Ok(report)
}

pub fn legacy_directory() -> Option<std::path::PathBuf> {
    let base = legacy_base()?;
    ["prime-desktop", "Prime Desktop"]
        .iter()
        .map(|name| base.join(name))
        .find(|directory| directory.is_dir())
}

fn legacy_base() -> Option<std::path::PathBuf> {
    #[cfg(windows)]
    {
        std::env::var_os("APPDATA").map(std::path::PathBuf::from)
    }
    #[cfg(not(windows))]
    {
        #[cfg(target_os = "macos")]
        {
            std::env::var_os("HOME").map(|base| {
                std::path::PathBuf::from(base)
                    .join("Library")
                    .join("Application Support")
            })
        }
        #[cfg(all(unix, not(target_os = "macos")))]
        {
            std::env::var_os("XDG_CONFIG_HOME")
                .map(std::path::PathBuf::from)
                .or_else(|| {
                    std::env::var_os("HOME")
                        .map(|base| std::path::PathBuf::from(base).join(".config"))
                })
        }
    }
}

fn migrate_profiles(
    state: &crate::state::AppState,
    legacy: &std::path::Path,
    report: &mut Value,
) -> HostResult<()> {
    let path = legacy.join("remote-profiles.json");
    let Ok(bytes) = std::fs::read(&path) else {
        return Ok(());
    };
    let saved: Value = match serde_json::from_slice(&bytes) {
        Ok(saved) => saved,
        Err(_) => {
            push_error(report, "remote-profiles.json unreadable");
            return Ok(());
        }
    };
    let imported = state.remotes.import_legacy(&saved)?;
        report["profilesImported"] = json!(imported.0);
    report["profilesSkipped"] = json!(imported.1);
    Ok(())
}

fn migrate_credentials(state: &crate::state::AppState, legacy: &std::path::Path, report: &mut Value) {
    let path = legacy.join("remote-credentials.json");
    let Ok(bytes) = std::fs::read(&path) else {
        return;
    };
    let saved: Value = match serde_json::from_slice(&bytes) {
        Ok(saved) => saved,
        Err(_) => {
            push_error(report, "remote-credentials.json unreadable");
            return;
        }
    };
    let Some(entries) = saved["entries"].as_object() else {
        return;
    };
    for (id, entry) in entries {
        let Some(encrypted) = entry["encrypted"].as_str() else {
            continue;
        };
        let outcome = decrypt_entry(legacy, encrypted)
            .and_then(|plaintext| serde_json::from_slice::<Value>(&plaintext).map_err(|_| unreadable()))
            .and_then(|payload| verify_binding(&payload, entry["binding"].as_str().unwrap_or("")))
            .and_then(|credentials| {
                state
                    .remotes
                    .store_legacy_credentials(id, entry["binding"].as_str().unwrap_or(""), &credentials)
            });
        match outcome {
            Ok(()) => report["credentialsImported"] = json!(report["credentialsImported"].as_u64().unwrap_or(0) + 1),
            Err(error) => {
                report["credentialsFailed"] = json!(report["credentialsFailed"].as_u64().unwrap_or(0) + 1);
                push_error(report, &format!("{id}: {}", error.message));
            }
        }
    }
}

fn decrypt_entry(legacy: &std::path::Path, encrypted: &str) -> HostResult<Vec<u8>> {
    use base64::Engine as _;
    let blob = base64::engine::general_purpose::STANDARD
        .decode(encrypted)
        .map_err(|_| unreadable())?;
    if blob.is_empty() || blob.len() > 48 * 1024 {
        return Err(unreadable());
    }
    match blob.strip_prefix(b"v10") {
        Some(sealed) => decrypt_chromium(&chromium_key(legacy)?, sealed),
        None => protect::unprotect(&blob),
    }
}

/// Electron `safeStorage` on Windows uses Chromium `os_crypt`: values are
/// `v10` + 12-byte nonce + AES-256-GCM ciphertext, and the AES key is kept
/// DPAPI-protected in the application's `Local State` file.
fn chromium_key(legacy: &std::path::Path) -> HostResult<Vec<u8>> {
    use base64::Engine as _;
    let state: Value = serde_json::from_slice(&std::fs::read(legacy.join("Local State"))?)?;
    let wrapped = base64::engine::general_purpose::STANDARD
        .decode(state["os_crypt"]["encrypted_key"].as_str().ok_or_else(unreadable)?)
        .map_err(|_| unreadable())?;
    let key = protect::unprotect(wrapped.strip_prefix(b"DPAPI").ok_or_else(unreadable)?)?;
    if key.len() != 32 {
        return Err(unreadable());
    }
    Ok(key)
}

fn decrypt_chromium(key: &[u8], sealed: &[u8]) -> HostResult<Vec<u8>> {
    use aes_gcm::{aead::Aead, Aes256Gcm, KeyInit, Nonce};
    if sealed.len() < 12 + 16 {
        return Err(unreadable());
    }
    let (nonce, ciphertext) = sealed.split_at(12);
    Aes256Gcm::new_from_slice(key)
        .map_err(|_| unreadable())?
        .decrypt(Nonce::from_slice(nonce), ciphertext)
        .map_err(|_| unreadable())
}

fn unreadable() -> HostError {
    HostError::new("MIGRATION_READ", "Legacy credentials cannot be decrypted")
}

fn verify_binding(payload: &Value, expected: &str) -> HostResult<Value> {
    let binding = payload["binding"].as_str().unwrap_or("");
    if binding.is_empty() || binding != expected {
        return Err(unreadable());
    }
    let credentials = payload["credentials"].as_object().ok_or_else(unreadable)?;
    for value in credentials.values() {
        if !value.is_string() || value.as_str().unwrap_or("").len() > 4096 {
            return Err(unreadable());
        }
    }
    Ok(payload["credentials"].clone())
}

fn push_error(report: &mut Value, message: &str) {
    if let Some(errors) = report.get_mut("errors").and_then(Value::as_array_mut) {
        errors.push(json!(message));
    }
}

fn finish(state: &crate::state::AppState, report: &Value) -> HostResult<()> {
    std::fs::write(
        state.directory.join("migration-complete.json"),
        serde_json::to_vec_pretty(report)?,
    )?;
    Ok(())
}

pub fn import_legacy_state(directory: &std::path::Path) {
    let marker = directory.join("legacy-state-imported.json");
    if marker.exists() {
        return;
    }
    let Some(legacy) = legacy_directory() else {
        return;
    };
    if let Err(error) = import_state_files(directory, &legacy) {
        eprintln!("Legacy state import failed: {}", error.message);
        return;
    }
    let _ = std::fs::write(marker, json!({"source":legacy}).to_string());
}

fn import_state_files(directory: &std::path::Path, legacy: &std::path::Path) -> HostResult<()> {
    for name in ["automations.json", "launch.json"] {
        let target = directory.join(name);
        if !target.exists() && legacy.join(name).is_file() {
            std::fs::copy(legacy.join(name), target)?;
        }
    }
    let Ok(bytes) = std::fs::read(legacy.join("preferences.json")) else {
        return Ok(());
    };
    let legacy_preferences: Value = serde_json::from_slice(&bytes)?;
    let target = directory.join("preferences.json");
    let mut native: Value = match std::fs::read(&target) {
        Ok(bytes) => serde_json::from_slice(&bytes)?,
        Err(_) => json!({"version":1,"preferences":{}}),
    };
    let fresh = native["preferences"]["recent"]
        .as_array()
        .is_none_or(|recent| recent.is_empty());
    let (Some(source), Some(current)) = (
        legacy_preferences["preferences"].as_object(),
        native["preferences"].as_object_mut(),
    ) else {
        return Ok(());
    };
    for (key, value) in source {
        let empty = current.get(key).is_none_or(|existing| {
            existing.is_null()
                || existing.as_array().is_some_and(Vec::is_empty)
                || existing.as_object().is_some_and(serde_json::Map::is_empty)
                || existing.as_str().is_some_and(str::is_empty)
        });
        if fresh || empty {
            current.insert(key.clone(), value.clone());
        }
    }
    std::fs::write(target, serde_json::to_vec_pretty(&native)?)?;
    Ok(())
}

/// Runs the migration once: a completion marker prevents repeated work,
/// and the marker itself records the report for diagnostics.
pub fn run_once(state: &crate::state::AppState) {
    let complete = std::fs::read(state.directory.join("migration-complete.json"))
        .ok()
        .and_then(|bytes| serde_json::from_slice::<Value>(&bytes).ok())
        .is_some_and(|report| report["credentialsFailed"].as_u64().unwrap_or(0) == 0);
    if complete {
        return;
    }
    match run(state) {
        Ok(report) => eprintln!("Legacy migration: {report}"),
        Err(error) => eprintln!("Legacy migration failed: {}", error.message),
    }
}

#[cfg(windows)]
mod protect {
    use crate::error::{HostError, HostResult};
    use windows::core::PWSTR;
    use windows::Win32::Foundation::LocalFree;
    use windows::Win32::Security::Cryptography::{
        CryptProtectData, CryptUnprotectData, CRYPT_INTEGER_BLOB,
    };

    pub fn unprotect(blob: &[u8]) -> HostResult<Vec<u8>> {
        let input = CRYPT_INTEGER_BLOB {
            cbData: blob.len() as u32,
            pbData: blob.as_ptr() as *mut u8,
        };
        let mut output = CRYPT_INTEGER_BLOB::default();
        let mut description = PWSTR::null();
        let result = unsafe {
            CryptUnprotectData(
                &input,
                Some(&mut description),
                None,
                None,
                None,
                0,
                &mut output,
            )
        };
        if result.is_err() {
            return Err(HostError::new(
                "MIGRATION_DECRYPT",
                "DPAPI could not decrypt the legacy secret",
            ));
        }
        let bytes = unsafe { std::slice::from_raw_parts(output.pbData, output.cbData as usize) }.to_vec();
        unsafe { LocalFree(Some(windows::Win32::Foundation::HLOCAL(output.pbData as *mut _))) };
        if !description.is_null() {
            unsafe { LocalFree(Some(windows::Win32::Foundation::HLOCAL(description.as_ptr() as *mut _))) };
        }
        Ok(bytes)
    }

    #[cfg(test)]
    pub fn protect(blob: &[u8]) -> HostResult<Vec<u8>> {
        let input = CRYPT_INTEGER_BLOB {
            cbData: blob.len() as u32,
            pbData: blob.as_ptr() as *mut u8,
        };
        let mut output = CRYPT_INTEGER_BLOB::default();
        let mut description = PWSTR::null();
        let result = unsafe {
            CryptProtectData(
                &input,
                windows::core::w!("prime-desktop-migration"),
                None,
                None,
                None,
                0,
                &mut output,
            )
        };
        if result.is_err() {
            return Err(HostError::new("MIGRATION_ENCRYPT", "DPAPI protect failed"));
        }
        let bytes = unsafe { std::slice::from_raw_parts(output.pbData, output.cbData as usize) }.to_vec();
        unsafe { LocalFree(Some(windows::Win32::Foundation::HLOCAL(output.pbData as *mut _))) };
        if !description.is_null() {
            unsafe { LocalFree(Some(windows::Win32::Foundation::HLOCAL(description.as_ptr() as *mut _))) };
        }
        Ok(bytes)
    }
}

#[cfg(not(windows))]
mod protect {
    use crate::error::{HostError, HostResult};

    pub fn unprotect(_blob: &[u8]) -> HostResult<Vec<u8>> {
        Err(HostError::unsupported(
            "legacy secret decryption on this platform; save the credentials again once",
        ))
    }
}

#[cfg(all(test, windows))]
mod tests {
    use super::*;

    #[test]
    fn dpapi_round_trip_recovers_plaintext() {
        let plaintext = b"{\"binding\":\"x\",\"credentials\":{\"password\":\"p\"}}";
        let protected = protect::protect(plaintext).unwrap();
        assert_ne!(protected, plaintext);
        assert_eq!(protect::unprotect(&protected).unwrap(), plaintext);
    }

    #[test]
    fn chromium_v10_values_decrypt_with_the_local_state_key() {
        use aes_gcm::{aead::Aead, Aes256Gcm, KeyInit, Nonce};
        use base64::Engine as _;
        let legacy = tempfile::tempdir().unwrap();
        let key = [7u8; 32];
        let mut wrapped = b"DPAPI".to_vec();
        wrapped.extend(protect::protect(&key).unwrap());
        std::fs::write(
            legacy.path().join("Local State"),
            json!({"os_crypt":{"encrypted_key":base64::engine::general_purpose::STANDARD.encode(&wrapped)}}).to_string(),
        )
        .unwrap();
        let nonce = [3u8; 12];
        let mut blob = b"v10".to_vec();
        blob.extend(nonce);
        blob.extend(
            Aes256Gcm::new_from_slice(&key)
                .unwrap()
                .encrypt(Nonce::from_slice(&nonce), b"secret".as_ref())
                .unwrap(),
        );
        let encoded = base64::engine::general_purpose::STANDARD.encode(&blob);
        assert_eq!(decrypt_entry(legacy.path(), &encoded).unwrap(), b"secret");
        blob[20] ^= 1;
        let tampered = base64::engine::general_purpose::STANDARD.encode(&blob);
        assert!(decrypt_entry(legacy.path(), &tampered).is_err());
    }

    #[test]
    fn random_bytes_are_rejected() {
        assert!(protect::unprotect(&[1, 2, 3, 4]).is_err());
    }
}

#[cfg(test)]
mod state_tests {
    use super::*;

    fn write(path: &std::path::Path, value: Value) {
        std::fs::write(path, value.to_string()).unwrap();
    }

    fn read(path: &std::path::Path) -> Value {
        serde_json::from_slice(&std::fs::read(path).unwrap()).unwrap()
    }

    #[test]
    fn fresh_profile_takes_legacy_preferences_and_files() {
        let native = tempfile::tempdir().unwrap();
        let legacy = tempfile::tempdir().unwrap();
        write(
            &legacy.path().join("preferences.json"),
            json!({"version":1,"preferences":{"recent":["/work/project"],"theme":"light","pinned":["/work/project"]}}),
        );
        write(&legacy.path().join("launch.json"), json!({"version":1,"configs":{}}));
        write(
            &native.path().join("preferences.json"),
            json!({"version":1,"preferences":{"recent":[],"theme":"dark"}}),
        );
        import_state_files(native.path(), legacy.path()).unwrap();
        let preferences = read(&native.path().join("preferences.json"));
        assert_eq!(preferences["preferences"]["recent"], json!(["/work/project"]));
        assert_eq!(preferences["preferences"]["theme"], "light");
        assert!(native.path().join("launch.json").is_file());
        assert!(!native.path().join("automations.json").exists());
    }

    #[test]
    fn used_profile_keeps_its_values_and_fills_gaps() {
        let native = tempfile::tempdir().unwrap();
        let legacy = tempfile::tempdir().unwrap();
        write(
            &legacy.path().join("preferences.json"),
            json!({"version":1,"preferences":{"recent":["/work/old"],"theme":"light","pinned":["/work/old"]}}),
        );
        write(
            &native.path().join("preferences.json"),
            json!({"version":1,"preferences":{"recent":["/work/new"],"theme":"dark","pinned":[]}}),
        );
        write(&native.path().join("launch.json"), json!({"version":1,"configs":{"keep":[]}}));
        write(&legacy.path().join("launch.json"), json!({"version":1,"configs":{}}));
        import_state_files(native.path(), legacy.path()).unwrap();
        let preferences = read(&native.path().join("preferences.json"));
        assert_eq!(preferences["preferences"]["recent"], json!(["/work/new"]));
        assert_eq!(preferences["preferences"]["theme"], "dark");
        assert_eq!(preferences["preferences"]["pinned"], json!(["/work/old"]));
        assert_eq!(read(&native.path().join("launch.json"))["configs"]["keep"], json!([]));
    }
}
