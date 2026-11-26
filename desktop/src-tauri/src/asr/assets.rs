use reqwest::blocking::Client;
use sha2::{Digest, Sha256};
use std::fs::{self, File};
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::time::Duration;

pub const RELEASE_REPO: &str = "SpherePrime/prime-asr";
pub const RELEASE_TAG: &str = "v0.1.0";
pub const ASR_MODEL_FILE: &str = "en-ru-uk.tzf.gz";
pub const VAD_MODEL_FILE: &str = "silero_vad.onnx.gz";
pub const ASR_MODEL_SHA256: &str =
    "5215d4e6c90b9c119f1267862d83833ba5b6ea9c28c5df4ebefe4265e26f2f53";
pub const VAD_MODEL_SHA256: &str =
    "cf8770c9376f65ffb8c6667bbca0edb993d9768902b3a0200b1041bcb16adaa4";

#[derive(Clone, Debug)]
pub struct AsrAssets {
    pub binary_path: PathBuf,
    pub asr_model_path: PathBuf,
    pub vad_model_path: PathBuf,
}

pub fn binary_file_name() -> &'static str {
    if cfg!(windows) {
        "prime-asr.exe"
    } else {
        "prime-asr"
    }
}

pub fn release_binary_asset_name() -> &'static str {
    match (std::env::consts::OS, std::env::consts::ARCH) {
        ("windows", "x86_64") => "prime-asr-windows-amd64.exe",
        ("windows", "aarch64") => "prime-asr-windows-arm64.exe",
        ("linux", "x86_64") => "prime-asr-linux-amd64",
        ("linux", "aarch64") => "prime-asr-linux-arm64",
        _ => "prime-asr-linux-amd64",
    }
}

pub fn release_download_url(file_name: &str) -> String {
    format!("https://github.com/{RELEASE_REPO}/releases/download/{RELEASE_TAG}/{file_name}")
}

pub fn fallback_model_url(file_name: &str) -> String {
    format!(
        "https://raw.githubusercontent.com/dkaraush/tinyasr/e034289132667fa8e87dd8d474a4dc59d447b2dc/models/{file_name}"
    )
}

pub fn shared_asr_dir(data_dir: &Path) -> PathBuf {
    data_dir.join("voice").join("asr")
}

fn non_empty_file(path: &Path) -> bool {
    fs::metadata(path)
        .map(|meta| meta.is_file() && meta.len() > 0)
        .unwrap_or(false)
}

fn candidate_dirs(resource_dir: Option<&Path>, data_dir: &Path) -> Vec<PathBuf> {
    let mut dirs = Vec::new();
    if let Ok(explicit) = std::env::var("PRIME_ASR_DIR") {
        if !explicit.trim().is_empty() {
            dirs.push(PathBuf::from(explicit));
        }
    }
    if let Some(res) = resource_dir {
        dirs.push(res.join("asr"));
    }
    dirs.push(shared_asr_dir(data_dir));
    if cfg!(debug_assertions) {
        let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        dirs.push(manifest_dir.join("asr"));
        if let Some(repo_root) = manifest_dir.parent().and_then(Path::parent) {
            if let Some(workspace_parent) = repo_root.parent() {
                dirs.push(workspace_parent.join("prime-asr").join("target").join("release"));
                dirs.push(workspace_parent.join("prime-asr").join("models"));
            }
        }
    }
    dirs
}

pub fn resolve_existing(resource_dir: Option<&Path>, data_dir: &Path) -> Option<AsrAssets> {
    let dirs = candidate_dirs(resource_dir, data_dir);
    let binary_name = binary_file_name();
    let binary_path = dirs
        .iter()
        .map(|dir| dir.join(binary_name))
        .find(|path| non_empty_file(path))?;
    let asr_model_path = dirs
        .iter()
        .map(|dir| dir.join(ASR_MODEL_FILE))
        .find(|path| non_empty_file(path))?;
    let vad_model_path = dirs
        .iter()
        .map(|dir| dir.join(VAD_MODEL_FILE))
        .find(|path| non_empty_file(path))?;
    Some(AsrAssets {
        binary_path,
        asr_model_path,
        vad_model_path,
    })
}

pub fn ensure_assets<F>(
    resource_dir: Option<&Path>,
    data_dir: &Path,
    mut on_progress: F,
) -> Result<AsrAssets, String>
where
    F: FnMut(f32, &str),
{
    if let Some(found) = resolve_existing(resource_dir, data_dir) {
        return Ok(found);
    }

    let target_dir = shared_asr_dir(data_dir);
    fs::create_dir_all(&target_dir)
        .map_err(|err| format!("failed to create ASR directory: {err}"))?;

    let client = Client::builder()
        .timeout(Duration::from_secs(300))
        .build()
        .map_err(|err| format!("failed to create HTTP client: {err}"))?;

    let dirs = candidate_dirs(resource_dir, data_dir);
    let binary_name = binary_file_name();
    let binary_path = match dirs
        .iter()
        .map(|dir| dir.join(binary_name))
        .find(|path| non_empty_file(path))
    {
        Some(existing) => existing,
        None => {
            let dest = target_dir.join(binary_name);
            let url = release_download_url(release_binary_asset_name());
            on_progress(0.05, "Загрузка сервиса распознавания речи…");
            download_file(&client, &url, &dest, None, |fraction| {
                on_progress(0.05 + fraction * 0.2, "Загрузка сервиса распознавания речи…");
            })?;
            set_executable(&dest)?;
            dest
        }
    };

    let asr_model_path = match dirs
        .iter()
        .map(|dir| dir.join(ASR_MODEL_FILE))
        .find(|path| non_empty_file(path))
    {
        Some(existing) => existing,
        None => {
            let dest = target_dir.join(ASR_MODEL_FILE);
            on_progress(0.25, "Загрузка языковой модели (en-ru-uk)…");
            let primary = release_download_url(ASR_MODEL_FILE);
            let fallback = fallback_model_url(ASR_MODEL_FILE);
            if download_file(
                &client,
                &primary,
                &dest,
                Some(ASR_MODEL_SHA256),
                |fraction| {
                    on_progress(0.25 + fraction * 0.65, "Загрузка языковой модели (en-ru-uk)…");
                },
            )
            .is_err()
            {
                download_file(
                    &client,
                    &fallback,
                    &dest,
                    Some(ASR_MODEL_SHA256),
                    |fraction| {
                        on_progress(0.25 + fraction * 0.65, "Загрузка языковой модели (en-ru-uk)…");
                    },
                )?;
            }
            dest
        }
    };

    let vad_model_path = match dirs
        .iter()
        .map(|dir| dir.join(VAD_MODEL_FILE))
        .find(|path| non_empty_file(path))
    {
        Some(existing) => existing,
        None => {
            let dest = target_dir.join(VAD_MODEL_FILE);
            on_progress(0.9, "Загрузка VAD-модели…");
            let primary = release_download_url(VAD_MODEL_FILE);
            let fallback = fallback_model_url(VAD_MODEL_FILE);
            if download_file(
                &client,
                &primary,
                &dest,
                Some(VAD_MODEL_SHA256),
                |fraction| {
                    on_progress(0.9 + fraction * 0.08, "Загрузка VAD-модели…");
                },
            )
            .is_err()
            {
                download_file(
                    &client,
                    &fallback,
                    &dest,
                    Some(VAD_MODEL_SHA256),
                    |fraction| {
                        on_progress(0.9 + fraction * 0.08, "Загрузка VAD-модели…");
                    },
                )?;
            }
            dest
        }
    };

    on_progress(1.0, "Модель загружена");
    Ok(AsrAssets {
        binary_path,
        asr_model_path,
        vad_model_path,
    })
}

fn download_file<F>(
    client: &Client,
    url: &str,
    dest: &Path,
    expected_sha256: Option<&str>,
    mut on_fraction: F,
) -> Result<(), String>
where
    F: FnMut(f32),
{
    let mut response = client
        .get(url)
        .send()
        .map_err(|err| format!("download failed ({url}): {err}"))?;
    if !response.status().is_success() {
        return Err(format!("download returned HTTP {} for {url}", response.status()));
    }
    let total = response.content_length().unwrap_or(0);
    let tmp = dest.with_extension("part");
    let mut file =
        File::create(&tmp).map_err(|err| format!("failed to create {}: {err}", tmp.display()))?;
    let mut hasher = Sha256::new();
    let mut buf = [0u8; 64 * 1024];
    let mut downloaded = 0u64;

    loop {
        let count = response
            .read(&mut buf)
            .map_err(|err| format!("failed reading stream ({url}): {err}"))?;
        if count == 0 {
            break;
        }
        file.write_all(&buf[..count])
            .map_err(|err| format!("failed writing {}: {err}", tmp.display()))?;
        hasher.update(&buf[..count]);
        downloaded += count as u64;
        if total > 0 {
            on_fraction((downloaded as f32 / total as f32).clamp(0.0, 1.0));
        }
    }
    file.flush()
        .map_err(|err| format!("failed flushing {}: {err}", tmp.display()))?;
    drop(file);

    if let Some(expected) = expected_sha256 {
        let actual = format!("{:x}", hasher.finalize());
        if !actual.eq_ignore_ascii_case(expected) {
            let _ = fs::remove_file(&tmp);
            return Err(format!(
                "checksum mismatch for {}: expected {expected}, got {actual}",
                dest.display()
            ));
        }
    }

    if dest.exists() {
        let _ = fs::remove_file(dest);
    }
    fs::rename(&tmp, dest)
        .map_err(|err| format!("failed moving {} to {}: {err}", tmp.display(), dest.display()))?;
    Ok(())
}

#[cfg(unix)]
fn set_executable(path: &Path) -> Result<(), String> {
    use std::os::unix::fs::PermissionsExt;
    let mut perms = fs::metadata(path)
        .map_err(|err| format!("failed reading permissions: {err}"))?
        .permissions();
    perms.set_mode(0o755);
    fs::set_permissions(path, perms)
        .map_err(|err| format!("failed setting executable permission: {err}"))
}

#[cfg(not(unix))]
fn set_executable(_path: &Path) -> Result<(), String> {
    Ok(())
}
