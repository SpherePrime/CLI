use sha2::{Digest, Sha256};
use std::{env, fs, path::PathBuf};

fn main() {
    println!("cargo:rerun-if-env-changed=PRIME_SETUP_PAYLOAD");
    println!("cargo:rerun-if-env-changed=PRIME_SETUP_INSTALLED_SIZE");
    let output = PathBuf::from(env::var("OUT_DIR").unwrap());
    let payload = env::var_os("PRIME_SETUP_PAYLOAD")
        .map(PathBuf::from)
        .filter(|path| path.is_file());
    let bytes = match &payload {
        Some(path) => {
            println!("cargo:rerun-if-changed={}", path.display());
            fs::read(path).expect("failed to read setup payload")
        }
        None => Vec::new(),
    };
    let digest = format!("{:x}", Sha256::digest(&bytes));
    fs::write(output.join("payload.bin"), &bytes).unwrap();
    let installed_size: u64 = env::var("PRIME_SETUP_INSTALLED_SIZE")
        .ok()
        .and_then(|value| value.parse().ok())
        .unwrap_or(0);
    fs::write(
        output.join("payload.rs"),
        format!("pub const PAYLOAD_SHA256: &str = \"{digest}\";\npub const INSTALLED_SIZE: u64 = {installed_size};\n"),
    )
    .unwrap();
    tauri_build::build();
}
