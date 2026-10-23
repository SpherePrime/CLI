fn main() {
    tauri_build::try_build(
        tauri_build::Attributes::new()
            .app_manifest(tauri_build::AppManifest::new().commands(&["desktop_call","desktop_window"])),
    )
    .expect("failed to build native application metadata");
}
