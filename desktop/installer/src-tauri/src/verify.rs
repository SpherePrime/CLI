#[cfg(any(not(target_os = "linux"), test))]
use sha2::{Digest, Sha256};
#[cfg(any(not(target_os = "linux"), test))]
use std::{io::Read, path::Path};

#[cfg(any(not(target_os = "linux"), test))]
const FIXED_INFO_SIGNATURE: [u8; 4] = [0xBD, 0x04, 0xEF, 0xFE];
#[cfg(any(not(target_os = "linux"), test))]
const FIXED_INFO_STRUC_VERSION: [u8; 4] = [0x00, 0x00, 0x01, 0x00];

pub fn wait_for_writable(directory: &str, timeout: std::time::Duration) -> Result<(), String> {
    #[cfg(not(target_os = "linux"))]
    {
        let root = std::path::Path::new(directory);
        let targets = [
            root.join("PrimeDesktop.exe"),
            root.join("engine").join("prime_desktop_engine.dll"),
            root.join("asr").join("prime-asr.exe"),
        ];
        let deadline = std::time::Instant::now() + timeout;
        for path in targets {
            if !path.exists() {
                continue;
            }
            loop {
                match std::fs::OpenOptions::new().read(true).write(true).open(&path) {
                    Ok(_) => break,
                    Err(_) => {
                        if std::time::Instant::now() >= deadline {
                            return Err(
                                "Закройте Prime Desktop и подождите освобождения файлов перед обновлением."
                                    .into(),
                            );
                        }
                        std::thread::sleep(std::time::Duration::from_millis(150));
                    }
                }
            }
        }
    }
    #[cfg(target_os = "linux")]
    {
        let _ = (directory, timeout);
    }
    Ok(())
}

#[cfg(any(not(target_os = "linux"), test))]
pub fn pe_file_version(bytes: &[u8]) -> Option<(u16, u16, u16)> {
    for window in bytes.windows(16) {
        if window[0..4] == FIXED_INFO_SIGNATURE && window[4..8] == FIXED_INFO_STRUC_VERSION {
            let ms = u32::from_le_bytes([window[8], window[9], window[10], window[11]]);
            let ls = u32::from_le_bytes([window[12], window[13], window[14], window[15]]);
            let major = (ms >> 16) as u16;
            let minor = (ms & 0xFFFF) as u16;
            let patch = (ls >> 16) as u16;
            return Some((major, minor, patch));
        }
    }
    None
}

#[cfg(any(not(target_os = "linux"), test))]
fn expected_arch() -> &'static str {
    match std::env::consts::ARCH {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        other => other,
    }
}

#[cfg(any(not(target_os = "linux"), test))]
fn parse_semver_triplet(version: &str) -> Result<(u16, u16, u16), String> {
    let core = version.split(['-', '+']).next().unwrap_or(version);
    let mut parts = core.split('.');
    let major = parts
        .next()
        .and_then(|value| value.parse::<u16>().ok())
        .ok_or_else(|| format!("Некорректный формат версии: {version}"))?;
    let minor = parts
        .next()
        .and_then(|value| value.parse::<u16>().ok())
        .ok_or_else(|| format!("Некорректный формат версии: {version}"))?;
    let patch = parts
        .next()
        .and_then(|value| value.parse::<u16>().ok())
        .ok_or_else(|| format!("Некорректный формат версии: {version}"))?;
    if parts.next().is_some() {
        return Err(format!("Некорректный формат версии: {version}"));
    }
    Ok((major, minor, patch))
}

#[cfg(any(not(target_os = "linux"), test))]
fn sha256_hex(path: &Path) -> Result<String, String> {
    let mut file = std::fs::File::open(path)
        .map_err(|error| format!("Не удалось открыть {}: {error}", path.display()))?;
    let mut hash = Sha256::new();
    let mut buffer = [0u8; 65536];
    loop {
        let count = file
            .read(&mut buffer)
            .map_err(|error| format!("Не удалось прочитать {}: {error}", path.display()))?;
        if count == 0 {
            break;
        }
        hash.update(&buffer[..count]);
    }
    Ok(format!("{:x}", hash.finalize()))
}

#[cfg(any(not(target_os = "linux"), test))]
fn verify_manifest(
    manifest_path: &Path,
    engine_path: &Path,
    expected_version: &str,
) -> Result<(), String> {
    let file = std::fs::File::open(manifest_path)
        .map_err(|error| format!("Не удалось открыть manifest.json: {error}"))?;
    let manifest: serde_json::Value = serde_json::from_reader(file)
        .map_err(|error| format!("Не удалось прочитать manifest.json: {error}"))?;
    if manifest["abi"] != 1
        || manifest["version"] != expected_version
        || manifest["os"] != "windows"
        || manifest["arch"] != expected_arch()
        || manifest["library"] != "prime_desktop_engine.dll"
    {
        return Err(
            "Установленный движок Prime Desktop не соответствует версии обновления. Закройте все окна Prime Desktop и повторите установку."
                .into(),
        );
    }
    let actual_sha256 = sha256_hex(engine_path)?;
    if manifest["sha256"].as_str() != Some(actual_sha256.as_str()) {
        return Err(
            "Контрольная сумма движка Prime Desktop не совпадает. Повторите установку.".into(),
        );
    }
    Ok(())
}

#[cfg(any(not(target_os = "linux"), test))]
fn verify_executable_version(executable_path: &Path, expected_version: &str) -> Result<(), String> {
    let expected = parse_semver_triplet(expected_version)?;
    let executable_bytes = std::fs::read(executable_path)
        .map_err(|error| format!("Не удалось прочитать PrimeDesktop.exe: {error}"))?;
    match pe_file_version(&executable_bytes) {
        Some(actual) if actual == expected => Ok(()),
        Some(_) => Err(
            "Установленная версия PrimeDesktop.exe не совпадает с версией обновления. Закройте все окна Prime Desktop и повторите установку."
                .into(),
        ),
        None => Err("Не удалось определить версию установленного PrimeDesktop.exe.".into()),
    }
}

#[cfg(any(not(target_os = "linux"), test))]
pub fn verify_installed(directory: &str, expected_version: &str) -> Result<(), String> {
    let root = Path::new(directory);
    let executable_path = root.join("PrimeDesktop.exe");
    let engine_path = root.join("engine").join("prime_desktop_engine.dll");
    let manifest_path = root.join("engine").join("manifest.json");
    if !executable_path.is_file() || !engine_path.is_file() || !manifest_path.is_file() {
        return Err("Файлы приложения не найдены".into());
    }
    verify_manifest(&manifest_path, &engine_path, expected_version)?;
    verify_executable_version(&executable_path, expected_version)?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn fixed_file_info_bytes(major: u16, minor: u16, patch: u16) -> Vec<u8> {
        let mut bytes = Vec::new();
        bytes.extend_from_slice(&FIXED_INFO_SIGNATURE);
        bytes.extend_from_slice(&FIXED_INFO_STRUC_VERSION);
        let ms = ((major as u32) << 16) | (minor as u32);
        let ls = (patch as u32) << 16;
        bytes.extend_from_slice(&ms.to_le_bytes());
        bytes.extend_from_slice(&ls.to_le_bytes());
        bytes.extend_from_slice(&[0u8; 36]);
        bytes
    }

    fn write_bundle(
        root: &Path,
        exe_version: (u16, u16, u16),
        manifest_version: &str,
        dll_bytes: &[u8],
        manifest_sha256: &str,
    ) {
        let mut exe_bytes = vec![0x4D, 0x5A, 0x90, 0x00];
        exe_bytes.extend_from_slice(&fixed_file_info_bytes(
            exe_version.0,
            exe_version.1,
            exe_version.2,
        ));
        std::fs::write(root.join("PrimeDesktop.exe"), exe_bytes).unwrap();
        let engine_dir = root.join("engine");
        std::fs::create_dir_all(&engine_dir).unwrap();
        std::fs::write(engine_dir.join("prime_desktop_engine.dll"), dll_bytes).unwrap();
        let manifest = json!({
            "abi": 1,
            "version": manifest_version,
            "os": "windows",
            "arch": expected_arch(),
            "library": "prime_desktop_engine.dll",
            "sha256": manifest_sha256,
        });
        std::fs::write(engine_dir.join("manifest.json"), manifest.to_string()).unwrap();
    }

    #[test]
    fn pe_file_version_parses_fixed_info_and_ignores_false_signatures() {
        let mut bytes = vec![0x00, 0x11, 0x22];
        bytes.extend_from_slice(&FIXED_INFO_SIGNATURE);
        bytes.extend_from_slice(&[0x01, 0x00, 0x01, 0x00]);
        bytes.extend_from_slice(&((9u32 << 16) | 9u32).to_le_bytes());
        bytes.extend_from_slice(&(9u32 << 16).to_le_bytes());
        bytes.extend_from_slice(&[0xAA; 8]);
        bytes.extend_from_slice(&fixed_file_info_bytes(0, 4, 125));

        assert_eq!(pe_file_version(&bytes), Some((0, 4, 125)));

        let mut false_only = Vec::new();
        false_only.extend_from_slice(&FIXED_INFO_SIGNATURE);
        false_only.extend_from_slice(&[0x02, 0x00, 0x00, 0x00]);
        false_only.extend_from_slice(&[0x00; 16]);
        assert_eq!(pe_file_version(&false_only), None);
    }

    #[test]
    fn verify_installed_accepts_matching_bundle() {
        let directory = tempfile::tempdir().unwrap();
        let dll_bytes = b"prime-desktop-engine-fixture";
        let digest = format!("{:x}", Sha256::digest(dll_bytes));
        write_bundle(directory.path(), (0, 4, 125), "0.4.125", dll_bytes, &digest);

        let result = verify_installed(&directory.path().to_string_lossy(), "0.4.125");
        assert!(result.is_ok(), "expected Ok, got: {result:?}");
    }

    #[test]
    fn verify_installed_rejects_mismatches_and_corrupted_dll() {
        let directory = tempfile::tempdir().unwrap();
        let root_str = directory.path().to_string_lossy().into_owned();
        let dll_bytes = b"prime-desktop-engine-fixture";
        let digest = format!("{:x}", Sha256::digest(dll_bytes));

        write_bundle(directory.path(), (0, 4, 124), "0.4.125", dll_bytes, &digest);
        let exe_mismatch = verify_installed(&root_str, "0.4.125").unwrap_err();
        assert_eq!(
            exe_mismatch,
            "Установленная версия PrimeDesktop.exe не совпадает с версией обновления. Закройте все окна Prime Desktop и повторите установку."
        );

        write_bundle(directory.path(), (0, 4, 125), "0.4.124", dll_bytes, &digest);
        assert!(verify_installed(&root_str, "0.4.125").is_err());

        write_bundle(
            directory.path(),
            (0, 4, 125),
            "0.4.125",
            b"corrupted-engine-bytes",
            &digest,
        );
        assert!(verify_installed(&root_str, "0.4.125").is_err());
    }
}
