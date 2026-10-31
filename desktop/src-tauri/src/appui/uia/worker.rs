use std::sync::mpsc::{self, Receiver, RecvTimeoutError, Sender};
use std::thread;
use std::time::Duration;

use ::windows::Win32::Foundation::HWND;
use ::windows::Win32::System::Com::{
    CoCreateInstance, CoInitializeEx, CLSCTX_INPROC_SERVER, COINIT_MULTITHREADED,
};
use ::windows::Win32::UI::Accessibility::{CUIAutomation, IUIAutomation};
use base64::Engine;
use serde_json::{json, Value};

use super::action::UiAction;
use super::{action, capture, tree};
use crate::appui::render::snapshot_text;
use crate::appui::types::Locator;

pub enum UiaRequest {
    Snapshot {
        hwnd: String,
        max_nodes: usize,
        max_depth: usize,
    },
    Act {
        hwnd: String,
        locator: Locator,
        action: UiAction,
    },
    ElementPng {
        hwnd: String,
        locator: Locator,
    },
}

struct Job {
    request: UiaRequest,
    reply: Sender<Result<Value, String>>,
}

pub struct UiaWorker {
    jobs: Sender<Job>,
}

impl UiaWorker {
    pub fn start() -> Result<Self, String> {
        let (jobs, receiver) = mpsc::channel::<Job>();
        let (ready, started) = mpsc::channel::<Result<(), String>>();
        thread::spawn(move || match unsafe { automation() } {
            Ok(automation) => {
                let _ = ready.send(Ok(()));
                serve(&automation, receiver);
            }
            Err(error) => {
                let _ = ready.send(Err(error));
            }
        });
        started
            .recv()
            .map_err(|_| "The UI Automation worker stopped".to_owned())??;
        Ok(Self { jobs })
    }

    pub fn call(&self, request: UiaRequest, timeout: Duration) -> Result<Value, String> {
        let (reply, answers) = mpsc::channel();
        self.jobs
            .send(Job { request, reply })
            .map_err(|_| "The UI Automation worker stopped".to_owned())?;
        match answers.recv_timeout(timeout) {
            Ok(result) => result,
            Err(RecvTimeoutError::Timeout) => Err(format!(
                "UI Automation did not respond within {} seconds",
                timeout.as_secs_f32()
            )),
            Err(RecvTimeoutError::Disconnected) => {
                Err("The UI Automation worker stopped".to_owned())
            }
        }
    }
}

pub fn window_handle(value: &str) -> Result<HWND, String> {
    let raw = value
        .parse::<usize>()
        .map_err(|_| "Invalid window handle".to_owned())?;
    if raw == 0 {
        return Err("Invalid window handle".to_owned());
    }
    Ok(HWND(raw as *mut core::ffi::c_void))
}

unsafe fn automation() -> Result<IUIAutomation, String> {
    CoInitializeEx(None, COINIT_MULTITHREADED)
        .ok()
        .map_err(|error| error.to_string())?;
    CoCreateInstance(
        &CUIAutomation,
        None::<&::windows::core::IUnknown>,
        CLSCTX_INPROC_SERVER,
    )
    .map_err(|error| error.to_string())
}

fn serve(automation: &IUIAutomation, jobs: Receiver<Job>) {
    while let Ok(job) = jobs.recv() {
        let result = handle(automation, job.request);
        let _ = job.reply.send(result);
    }
}

fn handle(automation: &IUIAutomation, request: UiaRequest) -> Result<Value, String> {
    match request {
        UiaRequest::Snapshot {
            hwnd,
            max_nodes,
            max_depth,
        } => {
            let window = window_handle(&hwnd)?;
            let snapshot = tree::snapshot(automation, window, max_nodes, max_depth)?;
            Ok(json!({"text": snapshot_text(&snapshot), "snapshot": snapshot}))
        }
        UiaRequest::Act {
            hwnd,
            locator,
            action,
        } => {
            let window = window_handle(&hwnd)?;
            action::perform(automation, window, &locator, action)
        }
        UiaRequest::ElementPng { hwnd, locator } => {
            let window = window_handle(&hwnd)?;
            let png = capture::element_png(automation, window, &locator)?;
            Ok(json!({
                "png": base64::engine::general_purpose::STANDARD.encode(&png),
                "size": png.len(),
            }))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::appui::testkit;
    use crate::appui::types::UiSnapshot;

    fn snapshot_request(hwnd: String) -> UiaRequest {
        UiaRequest::Snapshot {
            hwnd,
            max_nodes: 4000,
            max_depth: 40,
        }
    }

    fn by_name(name: &str) -> Locator {
        Locator {
            name: Some(name.to_owned()),
            ..Default::default()
        }
    }

    fn by_role(role: &str) -> Locator {
        Locator {
            role: Some(role.to_owned()),
            ..Default::default()
        }
    }

    fn by_role_name(role: &str, name: &str) -> Locator {
        Locator {
            role: Some(role.to_owned()),
            name: Some(name.to_owned()),
            ..Default::default()
        }
    }

    fn wait_for_label(fixture: &testkit::FixtureWindow, expected: &str) -> bool {
        wait_until(|| fixture.label() == expected)
    }

    fn wait_until(check: impl Fn() -> bool) -> bool {
        for _ in 0..40 {
            if check() {
                return true;
            }
            thread::sleep(Duration::from_millis(50));
        }
        false
    }

    #[test]
    fn snapshot_contains_fixture_controls_with_roles_and_actions() {
        let fixture = testkit::window();
        let worker = UiaWorker::start().unwrap();
        let value = worker
            .call(snapshot_request(fixture.hwnd()), Duration::from_secs(20))
            .unwrap();
        let text = value["text"].as_str().unwrap();
        assert!(text.contains("button \"Save\""), "{text}");
        assert!(text.contains("checkbox \"Enabled\""), "{text}");
        assert!(text.contains("actions=invoke"), "{text}");
    }

    #[test]
    fn a_window_without_controls_still_returns_a_usable_snapshot() {
        let fixture = testkit::bare_window();
        let worker = UiaWorker::start().unwrap();
        let value = worker
            .call(snapshot_request(fixture.hwnd()), Duration::from_secs(20))
            .unwrap();
        let snapshot: UiSnapshot = serde_json::from_value(value["snapshot"].clone()).unwrap();
        assert!(!snapshot.title.is_empty());
        assert!(snapshot.count >= 1);
        assert!(value["text"]
            .as_str()
            .unwrap()
            .contains("Prime app UI fixture"));
    }

    #[test]
    fn invoke_changes_the_fixture_label() {
        let fixture = testkit::window();
        let worker = UiaWorker::start().unwrap();
        let value = worker
            .call(
                UiaRequest::Act {
                    hwnd: fixture.hwnd(),
                    locator: by_role_name("button", "Save"),
                    action: UiAction::Invoke,
                },
                Duration::from_secs(15),
            )
            .unwrap();
        assert_eq!(value["confidence"], "high");
        assert!(wait_for_label(&fixture, "Saved"), "{}", fixture.label());
    }

    #[test]
    fn set_value_writes_into_the_edit_control() {
        let fixture = testkit::window();
        let worker = UiaWorker::start().unwrap();
        worker
            .call(
                UiaRequest::Act {
                    hwnd: fixture.hwnd(),
                    locator: by_role("edit"),
                    action: UiAction::SetValue("Готово".to_owned()),
                },
                Duration::from_secs(15),
            )
            .unwrap();
        let value = worker
            .call(snapshot_request(fixture.hwnd()), Duration::from_secs(20))
            .unwrap();
        assert!(
            value["text"].as_str().unwrap().contains("value=\"Готово\""),
            "{}",
            value["text"]
        );
    }

    #[test]
    fn toggle_switches_the_checkbox_state() {
        let fixture = testkit::window();
        let worker = UiaWorker::start().unwrap();
        worker
            .call(
                UiaRequest::Act {
                    hwnd: fixture.hwnd(),
                    locator: by_name("Enabled"),
                    action: UiAction::Toggle,
                },
                Duration::from_secs(15),
            )
            .unwrap();
        assert!(wait_until(|| fixture.checkbox_state()));
    }

    #[test]
    fn a_missing_element_is_rejected_without_guessing() {
        let fixture = testkit::window();
        let worker = UiaWorker::start().unwrap();
        let error = worker
            .call(
                UiaRequest::Act {
                    hwnd: fixture.hwnd(),
                    locator: by_name("Нет такой кнопки"),
                    action: UiAction::Invoke,
                },
                Duration::from_secs(15),
            )
            .unwrap_err();
        assert!(error.contains("No element matches"), "{error}");
    }

    #[test]
    fn an_element_without_the_required_pattern_is_rejected() {
        let fixture = testkit::window();
        let worker = UiaWorker::start().unwrap();
        let error = worker
            .call(
                UiaRequest::Act {
                    hwnd: fixture.hwnd(),
                    locator: by_role_name("text", "Ready"),
                    action: UiAction::Toggle,
                },
                Duration::from_secs(15),
            )
            .unwrap_err();
        assert!(error.contains("cannot be toggled"), "{error}");
    }

    #[test]
    fn a_call_that_exceeds_its_timeout_reports_it() {
        let fixture = testkit::window();
        let worker = UiaWorker::start().unwrap();
        let error = worker
            .call(snapshot_request(fixture.hwnd()), Duration::from_millis(0))
            .unwrap_err();
        assert!(
            error.contains("UI Automation did not respond within"),
            "{error}"
        );
    }

    #[test]
    fn invalid_window_handles_are_rejected() {
        let worker = UiaWorker::start().unwrap();
        for value in ["", "0", "-1", "1e10", "0x123", "999999999999999999999"] {
            assert!(
                worker
                    .call(snapshot_request(value.to_owned()), Duration::from_secs(5))
                    .is_err(),
                "{value}"
            );
        }
    }
}
