fn main() {
    let protoc_path =
        protoc_bin_vendored::protoc_bin_path().expect("failed to locate vendored protoc");
    std::env::set_var("PROTOC", protoc_path);
    tonic_build::configure()
        .build_server(false)
        .build_client(true)
        .compile_protos(&["proto/asr.proto"], &["proto"])
        .expect("failed to compile proto/asr.proto");
    println!("cargo:rerun-if-changed=proto/asr.proto");
    tauri_build::try_build(
        tauri_build::Attributes::new()
            .app_manifest(tauri_build::AppManifest::new().commands(&["desktop_call", "desktop_window"])),
    )
    .expect("failed to build native application metadata");
}
