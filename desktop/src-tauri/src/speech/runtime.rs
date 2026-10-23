use super::assets;
use crate::error::{HostError, HostResult};
use std::{
    fs,
    io::{Read, Write},
    path::{Path, PathBuf},
    sync::{atomic::AtomicBool, OnceLock},
};

const RUNTIME_SHA256: &str = "579b636403983254346a5c1d80bd28f1519cd1e284cd204f8d4ff41f8d711559";
const ARCHIVE_SHA256: &str = "174c616efc0271194488642a72f1a514e01487da4dfe84c49296d66e40ebe0da";
const RUNTIME_SIZE: u64 = 12418080;
const ARCHIVE_SIZE: u64 = 72368545;

pub fn initialize(stop: &AtomicBool) -> HostResult<()> {
    let library = ensure(stop)?;
    static INITIALIZED: OnceLock<Result<(), String>> = OnceLock::new();
    INITIALIZED
        .get_or_init(|| {
            ort::init_from(library.to_string_lossy())
                .with_name("Prime Speech")
                .commit()
                .map(|_| ())
                .map_err(|error| error.to_string())
        })
        .clone()
        .map_err(|error| HostError::new("SPEECH_RUNTIME", error))
}

pub fn ensure(stop: &AtomicBool) -> HostResult<PathBuf> {
    if !cfg!(all(windows, target_arch = "x86_64")) {
        return Err(HostError::new(
            "SPEECH_RUNTIME_PLATFORM",
            "A verified ONNX Runtime 1.22.0 package is not configured for this platform yet",
        ));
    }
    let executable = std::env::current_exe()?;
    let bundled = executable
        .parent()
        .unwrap_or(Path::new("."))
        .join("speech-runtime/onnxruntime.dll");
    if valid(&bundled)? {
        return Ok(bundled);
    }
    let root = assets::model_directory()
        .parent()
        .unwrap()
        .join("runtime/1.22.0/win-x64");
    fs::create_dir_all(&root)?;
    let library = root.join("onnxruntime.dll");
    if valid(&library)? {
        return Ok(library);
    }
    if let Some(source) = std::env::var_os("PRIME_NATIVE_SPEECH_RUNTIME") {
        let source = PathBuf::from(source);
        if !valid(&source)? {
            return Err(HostError::new(
                "SPEECH_RUNTIME_CHECKSUM",
                "Provided ONNX Runtime checksum does not match the pinned CPU runtime",
            ));
        }
        atomic_copy(&source, &library)?;
        return Ok(library);
    }
    let mut archive = tempfile::NamedTempFile::new_in(&root)?;
    let url = "https://github.com/microsoft/onnxruntime/releases/download/v1.22.0/onnxruntime-win-x64-1.22.0.zip";
    assets::download(
        &assets::client()?,
        url,
        &mut archive,
        ARCHIVE_SIZE,
        stop,
        |_| {},
    )?;
    if assets::sha256(archive.path())? != ARCHIVE_SHA256 {
        return Err(HostError::new(
            "SPEECH_RUNTIME_CHECKSUM",
            "ONNX Runtime archive checksum mismatch",
        ));
    }
    let mut zip = zip::ZipArchive::new(archive.reopen()?).map_err(runtime_error)?;
    let mut source = zip
        .by_name("onnxruntime-win-x64-1.22.0/lib/onnxruntime.dll")
        .map_err(runtime_error)?;
    if source.size() != RUNTIME_SIZE {
        return Err(HostError::new(
            "SPEECH_RUNTIME_CHECKSUM",
            "Unexpected ONNX Runtime library size",
        ));
    }
    let mut target = tempfile::NamedTempFile::new_in(&root)?;
    let copied = std::io::copy(&mut source.by_ref().take(RUNTIME_SIZE + 1), &mut target)?;
    if copied != RUNTIME_SIZE || !valid(target.path())? {
        return Err(HostError::new(
            "SPEECH_RUNTIME_CHECKSUM",
            "ONNX Runtime library checksum mismatch",
        ));
    }
    target.as_file().sync_all()?;
    target.persist(&library).map_err(|error| error.error)?;
    Ok(library)
}

fn valid(path: &Path) -> HostResult<bool> {
    if !fs::metadata(path)
        .is_ok_and(|metadata| metadata.is_file() && metadata.len() == RUNTIME_SIZE)
    {
        return Ok(false);
    }
    Ok(assets::sha256(path)? == RUNTIME_SHA256)
}

fn atomic_copy(source: &Path, target: &Path) -> HostResult<()> {
    let mut file = tempfile::NamedTempFile::new_in(target.parent().unwrap())?;
    let bytes = fs::read(source)?;
    file.write_all(&bytes)?;
    file.as_file().sync_all()?;
    file.persist(target).map_err(|error| error.error)?;
    Ok(())
}

fn runtime_error(error: impl std::fmt::Display) -> HostError {
    HostError::new("SPEECH_RUNTIME", error.to_string())
}
