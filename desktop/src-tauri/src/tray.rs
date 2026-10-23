use crate::state::AppState;
use std::sync::atomic::Ordering;
use tauri::{
    menu::{Menu, MenuItem, PredefinedMenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    AppHandle, Manager,
};
use tauri_plugin_notification::NotificationExt;

fn russian(app: &AppHandle) -> bool {
    app.state::<AppState>().settings.lock().unwrap().get()["language"] != "en"
}

pub fn show_main(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.unminimize();
        let _ = window.show();
        let _ = window.set_focus();
    }
}

pub fn create(app: &AppHandle) -> tauri::Result<()> {
    let ru = russian(app);
    let open = MenuItem::with_id(
        app,
        "open",
        if ru { "Открыть Prime" } else { "Open Prime" },
        true,
        None::<&str>,
    )?;
    let quit = MenuItem::with_id(app, "quit", if ru { "Выйти" } else { "Quit" }, true, None::<&str>)?;
    let separator = PredefinedMenuItem::separator(app)?;
    let menu = Menu::with_items(app, &[&open, &separator, &quit])?;
    let mut builder = TrayIconBuilder::with_id("main")
        .tooltip("Prime Desktop")
        .menu(&menu)
        .show_menu_on_left_click(false)
        .on_menu_event(|app, event| match event.id().as_ref() {
            "open" => show_main(app),
            "quit" => request_quit(app.clone()),
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                show_main(tray.app_handle());
            }
        });
    if let Some(icon) = app.default_window_icon() {
        builder = builder.icon(icon.clone());
    }
    builder.build(app)?;
    Ok(())
}

fn busy_chats(state: &AppState) -> bool {
    let Ok(engine) = state.require_engine() else {
        return false;
    };
    let projects = state.projects.lock().unwrap().values().cloned().collect::<Vec<_>>();
    projects.iter().any(|project| {
        project["id"].as_str().is_some_and(|id| {
            crate::remote::workspace_profile(state, id).is_none()
                && engine
                    .request("GET", &format!("/v1/workspaces/{id}/sessions"), None)
                    .ok()
                    .and_then(|sessions| sessions.as_array().cloned())
                    .is_some_and(|sessions| {
                        sessions.iter().any(|session| session["is_busy"] == true)
                    })
        })
    })
}

pub fn background_work(state: &AppState) -> bool {
    state.running_shells.load(Ordering::Acquire) > 0
        || state.terminals.count() > 0
        || state.remotes.terminals.count() > 0
        || state.teams.active_count() > 0
        || state.automations.counts()["activeCount"].as_u64().unwrap_or(0) > 0
        || busy_chats(state)
}

fn scheduler_enabled(state: &AppState) -> bool {
    state.automations.counts()["enabledCount"].as_u64().unwrap_or(0) > 0
}

pub fn keep_running(state: &AppState) -> bool {
    scheduler_enabled(state) || background_work(state)
}

pub fn hide_to_tray(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.hide();
    }
    let ru = russian(app);
    let body = match (scheduler_enabled(&app.state::<AppState>()), ru) {
        (true, true) => "Планировщик работает в трее. Для выхода используйте меню значка.",
        (false, true) => "Задачи продолжают работать. Приложение доступно в трее.",
        (true, false) => "The scheduler keeps running in the tray. Use the tray menu to quit.",
        (false, false) => "Tasks keep running. Prime is available in the tray.",
    };
    let _ = app
        .notification()
        .builder()
        .title("Prime Desktop")
        .body(body)
        .show();
}

pub fn request_quit(app: AppHandle) {
    std::thread::spawn(move || {
        let state = app.state::<AppState>();
        if background_work(&state) {
            let ru = russian(&app);
            let stop = if ru { "Остановить и выйти" } else { "Stop and quit" };
            let keep = if ru { "Продолжить работу" } else { "Keep working" };
            let answer = rfd::MessageDialog::new()
                .set_title("Prime Desktop")
                .set_description(if ru {
                    "Остановить задачи и выйти?"
                } else {
                    "Stop running tasks and quit?"
                })
                .set_buttons(rfd::MessageButtons::OkCancelCustom(stop.into(), keep.into()))
                .show();
            if answer != rfd::MessageDialogResult::Custom(stop.into()) {
                return;
            }
        }
        state.quitting.store(true, Ordering::Release);
        match app.get_webview_window("main") {
            Some(window) => {
                let _ = window.close();
            }
            None => app.exit(0),
        }
    });
}
