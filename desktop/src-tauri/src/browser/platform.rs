#[cfg(target_os = "windows")]
use serde_json::json;
use serde_json::Value;
use tauri::Webview;

pub fn install_events(
    view: &Webview,
    state: &super::BrowserState,
    app: &tauri::AppHandle,
    id: &str,
) -> Result<(), String> {
    #[cfg(target_os = "windows")]
    {
        use std::{sync::mpsc, time::Duration};
        use webview2_com::{
            Microsoft::Web::WebView2::Win32::{
                COREWEBVIEW2_WEB_ERROR_STATUS_OPERATION_CANCELED,
                COREWEBVIEW2_WEB_ERROR_STATUS_UNKNOWN,
            },
            NavigationCompletedEventHandler, ProcessFailedEventHandler,
        };
        use windows::core::BOOL;
        let state = state.clone();
        let app = app.clone();
        let id = id.to_owned();
        let (sender, receiver) = mpsc::channel();
        view.with_webview(move |native| {
            let load_state = state.clone();
            let load_app = app.clone();
            let load_id = id.clone();
            let completed = NavigationCompletedEventHandler::create(Box::new(move |_, args| {
                if let Some(args) = args {
                    let mut success = BOOL::default();
                    let mut status = COREWEBVIEW2_WEB_ERROR_STATUS_UNKNOWN;
                    unsafe { args.IsSuccess(&mut success)?; args.WebErrorStatus(&mut status)?; }
                    if !success.as_bool() && status != COREWEBVIEW2_WEB_ERROR_STATUS_OPERATION_CANCELED {
                        let message = format!("Browser navigation failed: {}", status.0);
                        let url = load_state.inner.lock().ok().and_then(|mut panel| panel.tabs.iter_mut().find(|tab| tab.id == load_id).map(|tab| { tab.load_error = Some(message.clone()); tab.loading = false; tab.url.clone() })).unwrap_or_default();
                        load_state.log(&load_app, json!({"level":"error", "message":message, "url":url}));
                        load_state.publish(&load_app);
                    }
                }
                Ok(())
            }));
            let failed = ProcessFailedEventHandler::create(Box::new(move |_, _| {
                let url = state.inner.lock().ok().and_then(|mut panel| panel.tabs.iter_mut().find(|tab| tab.id == id).map(|tab| { tab.load_error = Some("Browser renderer process failed".to_owned()); tab.loading = false; tab.url.clone() })).unwrap_or_default();
                state.log(&app, json!({"level":"error", "message":"Browser renderer process failed", "url":url}));
                state.publish(&app);
                Ok(())
            }));
            let result = unsafe {
                native.controller().CoreWebView2().and_then(|browser| {
                    let mut load_token = 0;
                    let mut failed_token = 0;
                    browser.add_NavigationCompleted(&completed, &mut load_token)?;
                    browser.add_ProcessFailed(&failed, &mut failed_token)
                })
            };
            let _ = sender.send(result.map_err(|error| error.to_string()));
        }).map_err(|error| error.to_string())?;
        receiver
            .recv_timeout(Duration::from_secs(5))
            .map_err(|_| "Browser event registration timed out".to_owned())?
    }
    #[cfg(target_os = "linux")]
    {
        super::platform_linux::install_events(view, state, app, id)
    }
    #[cfg(not(any(target_os = "windows", target_os = "linux")))]
    {
        let _ = (view, state, app, id);
        Ok(())
    }
}

#[cfg(target_os = "windows")]
pub fn protocol(view: &Webview, method: &str, parameters: Value) -> Result<Value, String> {
    use std::{sync::mpsc, time::Duration};
    use webview2_com::CallDevToolsProtocolMethodCompletedHandler;
    use windows::core::HSTRING;
    let method = method.to_owned();
    let parameters = parameters.to_string();
    let (sender, receiver) = mpsc::channel();
    view.with_webview(move |native| {
        let callback_sender = sender.clone();
        let handler = CallDevToolsProtocolMethodCompletedHandler::create(Box::new(
            move |status, response| {
                let result = status
                    .map_err(|error| error.to_string())
                    .and_then(|_| {
                        serde_json::from_str::<Value>(&response).map_err(|error| error.to_string())
                    })
                    .and_then(|value| {
                        if value.get("error").is_some() {
                            Err(format!("Browser protocol error: {}", value["error"]))
                        } else {
                            Ok(value)
                        }
                    });
                let _ = callback_sender.send(result);
                Ok(())
            },
        ));
        let result = unsafe {
            native.controller().CoreWebView2().and_then(|browser| {
                browser.CallDevToolsProtocolMethod(
                    &HSTRING::from(method),
                    &HSTRING::from(parameters),
                    &handler,
                )
            })
        };
        if let Err(error) = result {
            let _ = sender.send(Err(error.to_string()));
        }
    })
    .map_err(|error| error.to_string())?;
    receiver
        .recv_timeout(Duration::from_secs(10))
        .map_err(|_| "Browser protocol operation timed out".to_owned())?
}

pub fn screenshot(view: &Webview, clip: Option<Value>) -> Result<Value, String> {
    #[cfg(target_os = "windows")]
    {
        let mut parameters =
            json!({"format":"png", "captureBeyondViewport":false, "fromSurface":true});
        if let Some(clip) = clip {
            parameters["clip"] = clip;
        }
        let result = protocol(view, "Page.captureScreenshot", parameters)?;
        let data = result["data"]
            .as_str()
            .filter(|data| !data.is_empty())
            .ok_or("Browser returned an empty screenshot")?;
        Ok(json!({"type":"image", "data":data, "mimeType":"image/png"}))
    }
    #[cfg(target_os = "linux")]
    {
        super::platform_linux::screenshot(view, clip)
    }
    #[cfg(not(any(target_os = "windows", target_os = "linux")))]
    {
        let _ = (view, clip);
        Err("Native browser screenshots require a platform capture adapter on this operating system".to_owned())
    }
}

pub fn insert_text(view: &Webview, text: &str) -> Result<(), String> {
    #[cfg(target_os = "windows")]
    {
        protocol(view, "Input.insertText", json!({"text":text}))?;
        Ok(())
    }
    #[cfg(target_os = "linux")]
    {
        super::platform_linux::insert_text(view, text)
    }
    #[cfg(not(any(target_os = "windows", target_os = "linux")))]
    {
        let value = serde_json::to_string(text).map_err(|error| error.to_string())?;
        super::evaluate::immediate(view, &format!("(() => {{ const element = document.activeElement; if (!element || !(element.matches('input,textarea') || element.isContentEditable)) throw Error('No editable element is focused'); if (!document.execCommand('insertText', false, {value})) {{ if (element.isContentEditable) element.textContent = {value}; else {{ const prototype = element.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype; Object.getOwnPropertyDescriptor(prototype, 'value').set.call(element, {value}); }} element.dispatchEvent(new InputEvent('input', {{bubbles:true, inputType:'insertText', data:{value}}})); }} return true }})()"))?;
        Ok(())
    }
}

pub fn press_key(view: &Webview, key: &str) -> Result<(), String> {
    #[cfg(target_os = "windows")]
    {
        let (code, virtual_key) = match key {
            "Enter" => ("Enter".to_owned(), 13),
            "Tab" => ("Tab".to_owned(), 9),
            "Escape" => ("Escape".to_owned(), 27),
            "Backspace" => ("Backspace".to_owned(), 8),
            "Delete" => ("Delete".to_owned(), 46),
            "ArrowLeft" => ("ArrowLeft".to_owned(), 37),
            "ArrowUp" => ("ArrowUp".to_owned(), 38),
            "ArrowRight" => ("ArrowRight".to_owned(), 39),
            "ArrowDown" => ("ArrowDown".to_owned(), 40),
            "Home" => ("Home".to_owned(), 36),
            "End" => ("End".to_owned(), 35),
            "PageUp" => ("PageUp".to_owned(), 33),
            "PageDown" => ("PageDown".to_owned(), 34),
            " " | "Space" => ("Space".to_owned(), 32),
            value if value.chars().count() == 1 => {
                let character = value.chars().next().ok_or("Empty key")?;
                (
                    if character.is_ascii_alphabetic() {
                        format!("Key{}", character.to_ascii_uppercase())
                    } else if character.is_ascii_digit() {
                        format!("Digit{character}")
                    } else {
                        String::new()
                    },
                    character.to_ascii_uppercase() as u32,
                )
            }
            _ => return Err(format!("Unsupported browser key: {key}")),
        };
        protocol(
            view,
            "Input.dispatchKeyEvent",
            json!({"type":"rawKeyDown", "key":key, "code":code, "windowsVirtualKeyCode":virtual_key}),
        )?;
        if key.chars().count() == 1 || key == "Enter" {
            protocol(
                view,
                "Input.dispatchKeyEvent",
                json!({"type":"char", "key":key, "text":if key == "Enter" { "\r" } else { key }, "windowsVirtualKeyCode":virtual_key}),
            )?;
        }
        protocol(
            view,
            "Input.dispatchKeyEvent",
            json!({"type":"keyUp", "key":key, "code":code, "windowsVirtualKeyCode":virtual_key}),
        )?;
        Ok(())
    }
    #[cfg(target_os = "linux")]
    {
        super::platform_linux::press_key(view, key)
    }
    #[cfg(not(any(target_os = "windows", target_os = "linux")))]
    {
        let _ = (view, key);
        Err("Trusted browser keyboard input requires a platform input adapter on this operating system".to_owned())
    }
}

pub fn history(view: &Webview) -> Result<Value, String> {
    #[cfg(target_os = "windows")]
    {
        protocol(view, "Page.getNavigationHistory", json!({}))
    }
    #[cfg(target_os = "linux")]
    {
        super::platform_linux::history(view)
    }
    #[cfg(not(any(target_os = "windows", target_os = "linux")))]
    {
        let _ = view;
        Err("Native browser history is unavailable on this platform".to_owned())
    }
}
