use crate::error::{HostError, HostResult};
use sha2::{Digest, Sha256};
use std::{io::Read, path::{Path, PathBuf}};

pub fn resolve(resources: Option<&Path>) -> HostResult<PathBuf> {
    let filename = if cfg!(windows) { "prime_desktop_engine.dll" } else if cfg!(target_os = "macos") { "libprime_desktop_engine.dylib" } else { "libprime_desktop_engine.so" };
    if cfg!(debug_assertions) {
        if let Some(path) = std::env::var_os("PRIME_NATIVE_ENGINE_PATH") {
            return Ok(PathBuf::from(path));
        }
        let path = Path::new(env!("CARGO_MANIFEST_DIR")).join("engine").join(filename);
        if path.is_file() { return Ok(path); }
    }
    if let Some(resources) = resources {
        let path = resources.join("engine").join(filename);
        if path.is_file() {
            verify(&path)?;
            return Ok(path);
        }
    }
    Err(HostError::new("ENGINE_UNAVAILABLE", "Bundled native engine is missing"))
}

fn verify(path: &Path) -> HostResult<()> {
    let manifest: serde_json::Value = serde_json::from_reader(std::fs::File::open(path.with_file_name("manifest.json"))?)?;
    let os = if cfg!(target_os = "macos") { "darwin" } else { std::env::consts::OS };
    let arch = match std::env::consts::ARCH { "x86_64" => "amd64", "aarch64" => "arm64", other => other };
    if manifest["abi"] != 1 || manifest["version"] != env!("CARGO_PKG_VERSION") || manifest["os"] != os || manifest["arch"] != arch || manifest["library"].as_str() != path.file_name().and_then(|name|name.to_str()) {
        return Err(HostError::new("ENGINE_MANIFEST", "Bundled native engine does not match this application"));
    }
    let mut file = std::fs::File::open(path)?;
    let mut hash = Sha256::new();
    let mut buffer = [0u8; 65536];
    loop {
        let count = file.read(&mut buffer)?;
        if count == 0 { break; }
        hash.update(&buffer[..count]);
    }
    if manifest["sha256"].as_str() != Some(format!("{:x}", hash.finalize()).as_str()) {
        return Err(HostError::new("ENGINE_CHECKSUM", "Bundled native engine checksum does not match"));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;
    #[test]
    fn validates_manifest_and_rejects_changed_library() {
        let directory = tempfile::tempdir().unwrap();
        let library = directory.path().join("engine.fixture");
        std::fs::write(&library, b"engine").unwrap();
        let os = if cfg!(target_os="macos") {"darwin"}else{std::env::consts::OS};
        let arch = match std::env::consts::ARCH {"x86_64"=>"amd64","aarch64"=>"arm64",other=>other};
        let mut manifest = json!({"abi":1,"version":env!("CARGO_PKG_VERSION"),"os":os,"arch":arch,"library":"engine.fixture","sha256":format!("{:x}",Sha256::digest(b"engine"))});
        let manifest_path = directory.path().join("manifest.json");
        std::fs::write(&manifest_path,manifest.to_string()).unwrap();
        verify(&library).unwrap();
        std::fs::write(&library,b"changed").unwrap();
        assert_eq!(verify(&library).unwrap_err().code,"ENGINE_CHECKSUM");
        manifest["abi"]=json!(2);
        std::fs::write(&manifest_path,manifest.to_string()).unwrap();
        assert_eq!(verify(&library).unwrap_err().code,"ENGINE_MANIFEST");
    }
}
