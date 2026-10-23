use crate::error::{HostError, HostResult};
use sha2::{Digest, Sha256};
use std::{
    fs::{self, File},
    io::{Read, Write},
    path::{Path, PathBuf},
    sync::atomic::{AtomicBool, Ordering},
    time::{Duration, Instant},
};

const REPOSITORY: &str = "https://huggingface.co/supertone-oss-archive/supertonic-3/resolve/aafc6e32416a594460b32413efc49d7fe4ce6d46";

pub struct Asset {
    pub path: &'static str,
    pub size: u64,
    pub digest: &'static str,
    pub blob: bool,
}

pub const ASSETS: &[Asset] = &[
    Asset {
        path: "onnx/tts.json",
        size: 8253,
        digest: "28575b0793c92607a0f0a292550df9bd17709a79",
        blob: true,
    },
    Asset {
        path: "onnx/unicode_indexer.json",
        size: 277676,
        digest: "3aaf4d33024aff0be455633c24b3636b3a810150",
        blob: true,
    },
    Asset {
        path: "onnx/duration_predictor.onnx",
        size: 3700147,
        digest: "c3eb91414d5ff8a7a239b7fe9e34e7e2bf8a8140d8375ffb14718b1c639325db",
        blob: false,
    },
    Asset {
        path: "onnx/text_encoder.onnx",
        size: 36416150,
        digest: "c7befd5ea8c3119769e8a6c1486c4edc6a3bc8365c67621c881bbb774b9902ff",
        blob: false,
    },
    Asset {
        path: "onnx/vector_estimator.onnx",
        size: 256534781,
        digest: "883ac868ea0275ef0e991524dc64f16b3c0376efd7c320af6b53f5b780d7c61c",
        blob: false,
    },
    Asset {
        path: "onnx/vocoder.onnx",
        size: 101424195,
        digest: "085de76dd8e8d5836d6ca66826601f615939218f90e519f70ee8a36ed2a4c4ba",
        blob: false,
    },
    Asset {
        path: "voice_styles/F1.json",
        size: 292046,
        digest: "421365f307ed1535d8da16845031d0e9fa3e60c5",
        blob: true,
    },
    Asset {
        path: "voice_styles/F2.json",
        size: 292423,
        digest: "b09a3a433df8a4856d1d7d64736a56ddf37a4ea3",
        blob: true,
    },
    Asset {
        path: "voice_styles/M1.json",
        size: 291748,
        digest: "bddfff85a7c4c140c11620bd7005d86681dccdac",
        blob: true,
    },
    Asset {
        path: "voice_styles/M2.json",
        size: 292055,
        digest: "602ff6c4d6d48cdb71c37461f7c175d71fe8b34b",
        blob: true,
    },
];

pub fn model_directory() -> PathBuf {
    if let Some(directory) = std::env::var_os("PRIME_NATIVE_SPEECH_MODEL_DIR") {
        return PathBuf::from(directory);
    }
    let base = if cfg!(windows) {
        std::env::var_os("LOCALAPPDATA")
            .map(PathBuf::from)
            .unwrap_or_else(|| home().join("AppData/Local"))
    } else {
        std::env::var_os("XDG_DATA_HOME")
            .map(PathBuf::from)
            .unwrap_or_else(|| home().join(".local/share"))
    };
    base.join("Prime Desktop/speech/supertonic-3")
}

fn home() -> PathBuf {
    std::env::var_os(if cfg!(windows) { "USERPROFILE" } else { "HOME" })
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("."))
}

pub fn ready(directory: &Path) -> bool {
    ASSETS.iter().all(|asset| {
        fs::metadata(directory.join(asset.path))
            .is_ok_and(|metadata| metadata.is_file() && metadata.len() == asset.size)
    })
}

pub fn verify(path: &Path, asset: &Asset) -> HostResult<bool> {
    if !fs::metadata(path).is_ok_and(|metadata| metadata.is_file() && metadata.len() == asset.size)
    {
        return Ok(false);
    }
    if asset.blob {
        let mut digest = sha1::Sha1::new();
        digest.update(format!("blob {}\0", asset.size).as_bytes());
        hash_file(path, |bytes| digest.update(bytes))?;
        Ok(format!("{:x}", digest.finalize()) == asset.digest)
    } else {
        Ok(sha256(path)? == asset.digest)
    }
}

pub fn sha256(path: &Path) -> HostResult<String> {
    let mut digest = Sha256::new();
    hash_file(path, |bytes| digest.update(bytes))?;
    Ok(format!("{:x}", digest.finalize()))
}

fn hash_file(path: &Path, mut update: impl FnMut(&[u8])) -> HostResult<()> {
    let mut file = File::open(path)?;
    let mut buffer = [0u8; 65536];
    loop {
        let length = file.read(&mut buffer)?;
        if length == 0 {
            break;
        }
        update(&buffer[..length]);
    }
    Ok(())
}

pub fn ensure(
    directory: &Path,
    stop: &AtomicBool,
    mut progress: impl FnMut(f64),
) -> HostResult<()> {
    let client = client()?;
    let total: u64 = ASSETS.iter().map(|asset| asset.size).sum();
    let mut done = 0;
    for asset in ASSETS {
        if stop.load(Ordering::Acquire) {
            return Err(cancelled());
        }
        let path = directory.join(asset.path);
        if verify(&path, asset)? {
            done += asset.size;
            progress(done as f64 / total as f64 * 100.0);
            continue;
        }
        fs::create_dir_all(path.parent().unwrap())?;
        let mut last_error = None;
        for _ in 0..3 {
            let result = (|| {
                let mut temporary = tempfile::NamedTempFile::new_in(path.parent().unwrap())?;
                download(
                    &client,
                    &format!("{REPOSITORY}/{}", asset.path),
                    &mut temporary,
                    asset.size,
                    stop,
                    |received| progress((done + received) as f64 / total as f64 * 100.0),
                )?;
                temporary.as_file().sync_all()?;
                if !verify(temporary.path(), asset)? {
                    return Err(HostError::new(
                        "SPEECH_CHECKSUM",
                        format!("Speech asset checksum mismatch: {}", asset.path),
                    ));
                }
                temporary.persist(&path).map_err(|error| error.error)?;
                Ok::<_, HostError>(())
            })();
            match result {
                Ok(()) => {
                    last_error = None;
                    break;
                }
                Err(error) => {
                    last_error = Some(error);
                    if stop.load(Ordering::Acquire) {
                        break;
                    }
                }
            }
        }
        if let Some(error) = last_error {
            return Err(error);
        }
        done += asset.size;
        progress(done as f64 / total as f64 * 100.0);
    }
    fs::write(
        directory.join("verified.json"),
        serde_json::to_vec(
            &serde_json::json!({"total":total,"at":chrono::Utc::now().timestamp_millis()}),
        )?,
    )?;
    Ok(())
}

pub fn client() -> HostResult<reqwest::Client> {
    reqwest::Client::builder()
        .connect_timeout(Duration::from_secs(20))
        .timeout(Duration::from_secs(600))
        .user_agent("Prime-Desktop-Native")
        .build()
        .map_err(network_error)
}

pub fn download(
    client: &reqwest::Client,
    url: &str,
    file: &mut impl Write,
    limit: u64,
    stop: &AtomicBool,
    mut progress: impl FnMut(u64),
) -> HostResult<u64> {
    tauri::async_runtime::block_on(async {
        let mut response = tokio::select! { response = client.get(url).send() => response.map_err(network_error)?.error_for_status().map_err(network_error)?, _ = cancelled_when(stop) => return Err(cancelled()) };
        let mut received = 0u64;
        let mut reported = Instant::now();
        loop {
            let chunk = tokio::select! { chunk = response.chunk() => chunk.map_err(network_error)?, _ = cancelled_when(stop) => return Err(cancelled()) };
            let Some(chunk) = chunk else {
                break;
            };
            received += chunk.len() as u64;
            if received > limit {
                return Err(HostError::new(
                    "SPEECH_LIMIT",
                    "Speech download exceeds expected asset size",
                ));
            }
            file.write_all(&chunk)?;
            if reported.elapsed() >= Duration::from_millis(100) {
                progress(received);
                reported = Instant::now();
            }
        }
        progress(received);
        Ok(received)
    })
}

async fn cancelled_when(stop: &AtomicBool) {
    while !stop.load(Ordering::Acquire) {
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
}

fn network_error(error: impl std::fmt::Display) -> HostError {
    HostError::new("SPEECH_DOWNLOAD", error.to_string())
}
pub fn cancelled() -> HostError {
    HostError::new("SPEECH_CANCELLED", "Speech service is shutting down")
}
