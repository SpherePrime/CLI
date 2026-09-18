use std::path::PathBuf;

use aes_gcm::aead::{Aead, KeyInit};
use anyhow::{bail, Context, Result};
use base64::engine::general_purpose::STANDARD;
use base64::Engine;

const KEY_FILE_NAME: &str = "credentials.key";
#[cfg(windows)]
const MAGIC_KEY: &[u8; 4] = b"DPK1";
const NONCE_LEN: usize = 12;

#[cfg(windows)]
fn os_keystore_path() -> PathBuf {
    crate::server::credentials::store_path().with_file_name(KEY_FILE_NAME)
}

#[cfg(not(windows))]
fn raw_key_path() -> PathBuf {
    crate::server::credentials::store_path().with_file_name(KEY_FILE_NAME)
}

pub fn encrypt(plaintext: &[u8]) -> Result<String> {
    let key = load_or_create_key()?;
    let cipher = aes_gcm::Aes256Gcm::new_from_slice(&key)
        .map_err(|error| anyhow::anyhow!("credential cipher init failed: {error}"))?;
    let mut nonce_bytes = [0u8; NONCE_LEN];
    aes_gcm::aead::rand_core::RngCore::fill_bytes(&mut aes_gcm::aead::OsRng, &mut nonce_bytes);
    let nonce = aes_gcm::Nonce::from_slice(&nonce_bytes);
    let ciphertext = cipher
        .encrypt(nonce, plaintext)
        .map_err(|error| anyhow::anyhow!("credential encryption failed: {error}"))?;
    let mut blob = Vec::with_capacity(NONCE_LEN + ciphertext.len());
    blob.extend_from_slice(&nonce_bytes);
    blob.extend_from_slice(&ciphertext);
    Ok(STANDARD.encode(blob))
}

pub fn decrypt(encoded: &str) -> Result<Vec<u8>> {
    let key = load_or_create_key()?;
    let blob = STANDARD
        .decode(encoded)
        .context("credential blob is not valid base64")?;
    if blob.len() <= NONCE_LEN {
        bail!("credential blob is truncated");
    }
    let (nonce_bytes, ciphertext) = blob.split_at(NONCE_LEN);
    let cipher = aes_gcm::Aes256Gcm::new_from_slice(&key)
        .map_err(|error| anyhow::anyhow!("credential cipher init failed: {error}"))?;
    cipher
        .decrypt(aes_gcm::Nonce::from_slice(nonce_bytes), ciphertext)
        .map_err(|_| anyhow::anyhow!("credential decryption failed (wrong key or corrupt data)"))
}

fn load_or_create_key() -> Result<[u8; 32]> {
    let path = key_store_path();
    if let Ok(raw) = std::fs::read(&path) {
        if let Ok(key) = decode_key(&raw) {
            return Ok(key);
        }
    }
    let mut key = [0u8; 32];
    aes_gcm::aead::rand_core::RngCore::fill_bytes(&mut aes_gcm::aead::OsRng, &mut key);
    let encoded = encode_key(&key)?;
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)
            .with_context(|| format!("creating {}", parent.display()))?;
    }
    std::fs::write(&path, encoded).with_context(|| format!("writing {}", path.display()))?;
    restrict_permissions(&path)?;
    Ok(key)
}

#[cfg(windows)]
fn key_store_path() -> PathBuf {
    os_keystore_path()
}

#[cfg(not(windows))]
fn key_store_path() -> PathBuf {
    raw_key_path()
}

#[cfg(windows)]
fn encode_key(key: &[u8]) -> Result<Vec<u8>> {
    let mut protected = dpapi::protect(key)?;
    let mut out = MAGIC_KEY.to_vec();
    out.append(&mut protected);
    Ok(STANDARD.encode(out).into_bytes())
}

#[cfg(windows)]
fn decode_key(raw: &[u8]) -> Result<[u8; 32]> {
    let text = std::str::from_utf8(raw).context("key store is not utf-8")?;
    let decoded = STANDARD
        .decode(text.trim())
        .context("key store is not base64")?;
    if !decoded.starts_with(MAGIC_KEY) {
        bail!("unrecognised key store format");
    }
    let plain = dpapi::unprotect(&decoded[MAGIC_KEY.len()..])?;
    to_key(&plain)
}

#[cfg(not(windows))]
fn encode_key(key: &[u8]) -> Result<Vec<u8>> {
    Ok(key.to_vec())
}

#[cfg(not(windows))]
fn decode_key(raw: &[u8]) -> Result<[u8; 32]> {
    to_key(raw)
}

fn to_key(bytes: &[u8]) -> Result<[u8; 32]> {
    if bytes.len() != 32 {
        bail!("credential key has invalid length");
    }
    let mut key = [0u8; 32];
    key.copy_from_slice(bytes);
    Ok(key)
}

#[cfg(unix)]
fn restrict_permissions(path: &std::path::Path) -> Result<()> {
    use std::os::unix::fs::PermissionsExt;
    let permissions = std::fs::Permissions::from_mode(0o600);
    std::fs::set_permissions(path, permissions)
        .with_context(|| format!("securing {}", path.display()))?;
    Ok(())
}

#[cfg(not(unix))]
fn restrict_permissions(_path: &std::path::Path) -> Result<()> {
    Ok(())
}

#[cfg(windows)]
mod dpapi {
    use anyhow::{bail, Result};
    use windows_sys::Win32::Foundation::LocalFree;
    use windows_sys::Win32::Security::Cryptography::{
        CryptProtectData, CryptUnprotectData, CRYPT_INTEGER_BLOB,
    };

    const UI_FORBIDDEN: u32 = 0x1;

    fn blob(data: &[u8]) -> CRYPT_INTEGER_BLOB {
        CRYPT_INTEGER_BLOB {
            cbData: data.len() as u32,
            pbData: data.as_ptr() as *mut u8,
        }
    }

    pub fn protect(data: &[u8]) -> Result<Vec<u8>> {
        unsafe {
            let input = blob(data);
            let mut output = CRYPT_INTEGER_BLOB {
                cbData: 0,
                pbData: std::ptr::null_mut(),
            };
            let ok = CryptProtectData(
                &input,
                std::ptr::null(),
                std::ptr::null(),
                std::ptr::null(),
                std::ptr::null(),
                UI_FORBIDDEN,
                &mut output,
            );
            if ok == 0 {
                bail!("dpapi protect failed");
            }
            let protected =
                std::slice::from_raw_parts(output.pbData, output.cbData as usize).to_vec();
            LocalFree(output.pbData as *mut core::ffi::c_void);
            Ok(protected)
        }
    }

    pub fn unprotect(data: &[u8]) -> Result<Vec<u8>> {
        unsafe {
            let input = blob(data);
            let mut output = CRYPT_INTEGER_BLOB {
                cbData: 0,
                pbData: std::ptr::null_mut(),
            };
            let ok = CryptUnprotectData(
                &input,
                std::ptr::null_mut(),
                std::ptr::null(),
                std::ptr::null(),
                std::ptr::null(),
                UI_FORBIDDEN,
                &mut output,
            );
            if ok == 0 {
                bail!("dpapi unprotect failed");
            }
            let plain = std::slice::from_raw_parts(output.pbData, output.cbData as usize).to_vec();
            LocalFree(output.pbData as *mut core::ffi::c_void);
            Ok(plain)
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn encrypt_decrypt_round_trip() {
        let _guard = crate::server::credentials::ENV_LOCK
            .lock()
            .unwrap_or_else(|error| error.into_inner());
        let tmp = tempfile::tempdir().unwrap();
        std::env::set_var(
            "AGENT_CREDENTIALS_FILE",
            tmp.path().join("credentials.json"),
        );
        let encoded = encrypt(b"top-secret").unwrap();
        assert!(!encoded.contains("top-secret"));
        let decoded = decrypt(&encoded).unwrap();
        assert_eq!(decoded, b"top-secret");
    }
}
