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
    #[cfg(windows)]
    {
        std::env::var_os("APPDATA").map(|base| std::path::PathBuf::from(base).join("Prime Desktop"))
    }
    #[cfg(not(windows))]
    {
        #[cfg(target_os = "macos")]
        {
            std::env::var_os("HOME").map(|base| {
                std::path::PathBuf::from(base)
                    .join("Library")
                    .join("Application Support")
                    .join("Prime Desktop")
            })
        }
        #[cfg(all(unix, not(target_os = "macos")))]
        {
            std::env::var_os("HOME").map(|base| {
                std::path::PathBuf::from(base)
                    .join(".config")
                    .join("Prime Desktop")
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
        let outcome = decrypt_entry(encrypted)
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

fn decrypt_entry(encrypted: &str) -> HostResult<Vec<u8>> {
    use base64::Engine as _;
    let blob = base64::engine::general_purpose::STANDARD
        .decode(encrypted)
        .map_err(|_| unreadable())?;
    if blob.is_empty() || blob.len() > 48 * 1024 {
        return Err(unreadable());
    }
    protect::unprotect(&blob)
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

/// Runs the migration once: a completion marker prevents repeated work,
/// and the marker itself records the report for diagnostics.
pub fn run_once(state: &crate::state::AppState) {
    if state.directory.join("migration-complete.json").exists() {
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
    fn random_bytes_are_rejected() {
        assert!(protect::unprotect(&[1, 2, 3, 4]).is_err());
    }
}
