use sha2::{Digest, Sha256};

include!(concat!(env!("OUT_DIR"), "/payload.rs"));

pub static BYTES: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/payload.bin"));

pub fn verify() -> Result<(), String> {
    if BYTES.is_empty() {
        return Err("Установщик собран без файлов приложения".into());
    }
    if format!("{:x}", Sha256::digest(BYTES)) != PAYLOAD_SHA256 {
        return Err("Файл установки повреждён. Скачайте установщик заново.".into());
    }
    Ok(())
}
