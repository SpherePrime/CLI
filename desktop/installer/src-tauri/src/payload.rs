use sha2::{Digest, Sha256};

include!(concat!(env!("OUT_DIR"), "/payload.rs"));

pub static BYTES: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/payload.bin"));
#[cfg(target_os = "linux")]
pub static DEB_BYTES: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/payload-deb.bin"));
#[cfg(target_os = "linux")]
pub static RPM_BYTES: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/payload-rpm.bin"));

pub fn verify_bytes(bytes: &[u8], digest: &str) -> Result<(), String> {
    if bytes.is_empty() {
        return Err("Установщик собран без файлов приложения".into());
    }
    if format!("{:x}", Sha256::digest(bytes)) != digest {
        return Err("Файл установки повреждён. Скачайте установщик заново.".into());
    }
    Ok(())
}

pub fn verify() -> Result<(), String> {
    verify_bytes(BYTES, PAYLOAD_SHA256)
}
