mod attachments;
mod automations;
mod browser;
mod commands;
mod console;
mod computer;
mod engine;
mod engine_resource;
mod error;
mod event_batch;
mod events;
mod files;
mod git;
mod jobs;
mod lifecycle;
mod migration;
mod permissions;
mod projects;
mod redact;
mod remote;
mod settings;
mod sudo;
mod speech;
mod state;
mod teams;
mod terminal;
mod test_data;
mod updater;
mod window_chrome;

pub fn run() {
    console::attach_hidden();
    let mut context = tauri::generate_context!();
    if cfg!(debug_assertions) {
        test_data::configure(
            context.config_mut(),
            std::env::var_os("PRIME_NATIVE_TEST_DATA"),
        )
        .expect("Invalid native preview test data directory");
    }
    tauri::Builder::default()
        .plugin(tauri_plugin_updater::Builder::new().build())
        .setup(lifecycle::setup)
        .invoke_handler(tauri::generate_handler![
            commands::desktop_call,
            window_chrome::desktop_window
        ])
        .on_window_event(lifecycle::window_event)
        .build(context)
        .expect("failed to build Prime Native Preview")
        .run(lifecycle::run_event);
}
