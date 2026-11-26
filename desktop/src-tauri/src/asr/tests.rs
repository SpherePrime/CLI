use crate::asr::assets::{
    ASR_MODEL_FILE, VAD_MODEL_FILE, binary_file_name, resolve_existing, shared_asr_dir,
};
use crate::asr::process::{AsrProcess, parse_handshake};
use crate::asr::protocol::pb::asr_service_client::AsrServiceClient;
use crate::asr::protocol::pb::{ShutdownRequest, StatusRequest};
use crate::asr::protocol::with_token;
use std::fs;
use std::path::PathBuf;
use tonic::transport::Channel;

#[test]
fn parses_valid_and_rejects_invalid_handshakes() {
    let parsed = parse_handshake(
        r#"{"addr":"127.0.0.1:42424","token":"abcdef","langs":["en","ru","uk"],"chunk_ms":320}"#,
    )
    .expect("valid handshake");
    assert_eq!(parsed.addr, "127.0.0.1:42424");
    assert_eq!(parsed.token, "abcdef");
    assert_eq!(parsed.langs, vec!["en", "ru", "uk"]);
    assert_eq!(parsed.chunk_ms, 320);

    assert!(parse_handshake(r#"{"addr":"","token":"abc","langs":[],"chunk_ms":320}"#).is_err());
    assert!(parse_handshake("not json").is_err());
}

#[test]
fn resolves_bundled_resources_before_data_dir() {
    let temp = tempfile::tempdir().expect("tempdir");
    let resource_dir = temp.path().join("resources");
    let data_dir = temp.path().join("data");
    let res_asr = resource_dir.join("asr");
    let data_asr = shared_asr_dir(&data_dir);
    fs::create_dir_all(&res_asr).unwrap();
    fs::create_dir_all(&data_asr).unwrap();

    for dir in [&res_asr, &data_asr] {
        fs::write(dir.join(binary_file_name()), b"bin").unwrap();
        fs::write(dir.join(ASR_MODEL_FILE), b"asr").unwrap();
        fs::write(dir.join(VAD_MODEL_FILE), b"vad").unwrap();
    }

    let found = resolve_existing(Some(&resource_dir), &data_dir).expect("resolved");
    assert_eq!(found.binary_path, res_asr.join(binary_file_name()));
    assert_eq!(found.asr_model_path, res_asr.join(ASR_MODEL_FILE));
    assert_eq!(found.vad_model_path, res_asr.join(VAD_MODEL_FILE));
}

#[test]
fn spawns_real_prime_asr_when_built_locally() {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let Some(workspace_parent) = manifest_dir
        .parent()
        .and_then(|p| p.parent())
        .and_then(|p| p.parent())
    else {
        return;
    };
    let bin = workspace_parent
        .join("prime-asr")
        .join("target")
        .join("release")
        .join(binary_file_name());
    let asr = workspace_parent
        .join("prime-asr")
        .join("models")
        .join(ASR_MODEL_FILE);
    let vad = workspace_parent
        .join("prime-asr")
        .join("models")
        .join(VAD_MODEL_FILE);
    if !bin.exists() || !asr.exists() || !vad.exists() {
        return;
    }

    let assets = crate::asr::assets::AsrAssets {
        binary_path: bin,
        asr_model_path: asr,
        vad_model_path: vad,
    };
    let mut process = AsrProcess::spawn(&assets).expect("spawn prime-asr");
    let endpoint = format!("http://{}", process.handshake.addr);
    let token = process.handshake.token.clone();

    let runtime = tokio::runtime::Runtime::new().expect("tokio runtime");
    runtime.block_on(async move {
        let channel = Channel::from_shared(endpoint)
            .unwrap()
            .connect()
            .await
            .expect("connect");
        let mut client = AsrServiceClient::new(channel);
        let status = client
            .status(with_token(StatusRequest {}, &token).unwrap())
            .await
            .expect("status")
            .into_inner();
        assert!(status.ready);
        assert!(status.languages.contains(&"ru".to_string()));
        client
            .shutdown(with_token(ShutdownRequest {}, &token).unwrap())
            .await
            .expect("shutdown");
    });

    process.terminate();
    assert!(!process.is_running());
}
