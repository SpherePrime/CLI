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
        if ru {
            "Открыть Prime"
        } else {
            "Open Prime"
        },
        true,
        None::<&str>,
    )?;
    let quit = MenuItem::with_id(
        app,
        "quit",
        if ru { "Выйти" } else { "Quit" },
        true,
        None::<&str>,
    )?;
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
    let projects = state
        .projects
        .lock()
        .unwrap()
        .values()
        .cloned()
        .collect::<Vec<_>>();
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

pub fn should_hide_to_tray(quitting: bool) -> bool {
    !quitting
}

pub fn background_work(state: &AppState) -> bool {
    !busy_reasons(state).is_empty()
}

fn scheduler_enabled(state: &AppState) -> bool {
    state.automations.counts()["enabledCount"]
        .as_u64()
        .unwrap_or(0)
        > 0
}

fn busy_reasons(state: &AppState) -> Vec<&'static str> {
    let mut reasons = Vec::new();
    if busy_chats(state) {
        reasons.push("chats");
    }
    if state.terminals.count() > 0 || state.remotes.terminals.count() > 0 {
        reasons.push("terminals");
    }
    if state.running_shells.load(Ordering::Acquire) > 0 {
        reasons.push("shells");
    }
    if state.automations.counts()["activeCount"]
        .as_u64()
        .unwrap_or(0)
        > 0
    {
        reasons.push("automations");
    }
    if state.teams.active_count() > 0 {
        reasons.push("teams");
    }
    reasons
}

fn busy_reason_text(reasons: &[&str], ru: bool) -> String {
    let labels = reasons
        .iter()
        .map(|reason| match (*reason, ru) {
            ("chats", true) => "активные чаты агента",
            ("terminals", true) => "открытые терминалы",
            ("shells", true) => "выполняемые команды оболочки",
            ("automations", true) => "активные автоматизации",
            ("teams", true) => "активные команды агентов",
            ("chats", false) => "running agent chats",
            ("terminals", false) => "open terminals",
            ("shells", false) => "running shell commands",
            ("automations", false) => "active automations",
            ("teams", false) => "active agent teams",
            _ => "active work",
        })
        .collect::<Vec<_>>();
    labels.join(if ru { ", " } else { ", " })
}

pub fn hide_to_tray(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.hide();
    }
    let ru = russian(app);
    let state = app.state::<AppState>();
    let body = match (scheduler_enabled(&state), background_work(&state), ru) {
        (_, true, true) => "Работа продолжается. Приложение доступно в трее.",
        (true, false, true) => "Планировщик включён. Приложение доступно в трее.",
        (false, false, true) => "Приложение доступно в трее.",
        (_, true, false) => "Work is still running. Prime is available in the tray.",
        (true, false, false) => "The scheduler is enabled. Prime is available in the tray.",
        (false, false, false) => "Prime is available in the tray.",
    };
    let _ = app
        .notification()
        .builder()
        .title("Prime Desktop")
        .body(body)
        .show();
}

#[cfg(test)]
mod tests {
    use super::{busy_reason_text, should_hide_to_tray};

    #[test]
    fn close_hides_to_tray_until_quitting() {
        assert!(should_hide_to_tray(false));
        assert!(!should_hide_to_tray(true));
    }

    #[test]
    fn busy_reason_text_names_each_kind_of_work() {
        let reasons = ["chats", "terminals", "shells", "automations", "teams"];
        let russian = busy_reason_text(&reasons, true);
        let english = busy_reason_text(&reasons, false);
        assert!(russian.contains("активные чаты агента"));
        assert!(russian.contains("активные команды агентов"));
        assert!(english.contains("running agent chats"));
        assert!(english.contains("active agent teams"));
    }
}

pub fn request_quit(app: AppHandle) {
    std::thread::spawn(move || {
        let state = app.state::<AppState>();
        if background_work(&state) {
            let ru = russian(&app);
            let stop = if ru {
                "Остановить и выйти"
            } else {
                "Stop and quit"
            };
            let keep = if ru {
                "Продолжить работу"
            } else {
                "Keep working"
            };
            let reasons = busy_reason_text(&busy_reasons(&state), ru);
            let answer = rfd::MessageDialog::new()
                .set_title("Prime Desktop")
                .set_description(if ru {
                    format!("Остановить и выйти? Будут остановлены: {reasons}.")
                } else {
                    format!("Stop and quit? This will stop: {reasons}.")
                })
                .set_buttons(rfd::MessageButtons::OkCancelCustom(
                    stop.into(),
                    keep.into(),
                ))
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
