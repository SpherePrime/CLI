use sha2::{Digest, Sha256};
use std::{
    env, fs,
    path::{Path, PathBuf},
};

fn stage(output: &Path, source: Option<&Path>, name: &str) -> String {
    let bytes = match source {
        Some(path) => {
            println!("cargo:rerun-if-changed={}", path.display());
            fs::read(path).expect("failed to read setup payload")
        }
        None => Vec::new(),
    };
    let digest = format!("{:x}", Sha256::digest(&bytes));
    fs::write(output.join(name), &bytes).unwrap();
    digest
}

fn main() {
    println!("cargo:rerun-if-env-changed=PRIME_SETUP_PAYLOAD");
    println!("cargo:rerun-if-env-changed=PRIME_SETUP_INSTALLED_SIZE");
    let output = PathBuf::from(env::var("OUT_DIR").unwrap());
    let source = env::var_os("PRIME_SETUP_PAYLOAD").map(PathBuf::from);
    let linux = env::var("CARGO_CFG_TARGET_OS").as_deref() == Ok("linux");
    let appimage = source.as_ref().map(|path| path.join("app.AppImage"));
    let deb = source.as_ref().map(|path| path.join("app.deb"));
    let rpm = source.as_ref().map(|path| path.join("app.rpm"));
    let primary = if linux {
        appimage.as_deref()
    } else {
        source.as_deref()
    };
    let digest = stage(&output, primary, "payload.bin");
    let deb_digest = stage(
        &output,
        if linux { deb.as_deref() } else { None },
        "payload-deb.bin",
    );
    let rpm_digest = stage(
        &output,
        if linux { rpm.as_deref() } else { None },
        "payload-rpm.bin",
    );
    let installed_size: u64 = env::var("PRIME_SETUP_INSTALLED_SIZE")
        .ok()
        .and_then(|value| value.parse().ok())
        .unwrap_or(0);
    fs::write(output.join("payload.rs"), format!(
        "pub const PAYLOAD_SHA256: &str = \"{digest}\";\npub const DEB_SHA256: &str = \"{deb_digest}\";\npub const RPM_SHA256: &str = \"{rpm_digest}\";\npub const INSTALLED_SIZE: u64 = {installed_size};\n"
    )).unwrap();
    tauri_build::build();
}
