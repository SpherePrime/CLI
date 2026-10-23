use super::BrowserState;
use base64::Engine;
use glib::translate::{from_glib_full, IntoGlib, ToGlibPtr};
use gtk::prelude::*;
use serde_json::{json, Value};
use std::{sync::mpsc, time::Duration};
use tauri::{AppHandle, Webview};
use webkit2gtk::{
    BackForwardListExt, BackForwardListItemExt, NetworkError, SnapshotOptions, SnapshotRegion,
    WebViewExt,
};

fn native<T: Send + 'static>(
    view: &Webview,
    operation: impl FnOnce(webkit2gtk::WebView) -> Result<T, String> + Send + 'static,
) -> Result<T, String> {
    let (sender, receiver) = mpsc::channel();
    view.with_webview(move |platform| {
        let _ = sender.send(operation(platform.inner()));
    })
    .map_err(|error| error.to_string())?;
    receiver
        .recv_timeout(Duration::from_secs(5))
        .map_err(|_| "Native WebKit operation timed out".to_owned())?
}

fn record_failure(state: &BrowserState, app: &AppHandle, id: &str, message: String, url: String) {
    if let Ok(mut panel) = state.inner.lock() {
        if let Some(tab) = panel.tabs.iter_mut().find(|tab| tab.id == id) {
            tab.load_error = Some(message.clone());
            tab.loading = false;
        }
    }
    state.log(app, json!({"level":"error", "message":message, "url":url}));
    state.publish(app);
}

pub fn install_events(
    view: &Webview,
    state: &BrowserState,
    app: &AppHandle,
    id: &str,
) -> Result<(), String> {
    let state = state.clone();
    let app = app.clone();
    let id = id.to_owned();
    native(view, move |browser| {
        let load_state = state.clone();
        let load_app = app.clone();
        let load_id = id.clone();
        browser.connect_load_failed(move |_, _, url, error| {
            if !error.matches(NetworkError::Cancelled) {
                record_failure(
                    &load_state,
                    &load_app,
                    &load_id,
                    format!("Browser navigation failed: {error}"),
                    url.to_owned(),
                );
            }
            false
        });
        let tls_state = state.clone();
        let tls_app = app.clone();
        let tls_id = id.clone();
        browser.connect_load_failed_with_tls_errors(move |_, url, _, errors| {
            record_failure(
                &tls_state,
                &tls_app,
                &tls_id,
                format!("Browser TLS certificate validation failed: {errors:?}"),
                url.to_owned(),
            );
            false
        });
        browser.connect_web_process_terminated(move |browser, reason| {
            record_failure(
                &state,
                &app,
                &id,
                format!("Browser renderer process failed: {reason:?}"),
                browser.uri().map(|uri| uri.to_string()).unwrap_or_default(),
            );
        });
        Ok(())
    })
}

pub fn history(view: &Webview) -> Result<Value, String> {
    native(view, |browser| {
        let list = browser
            .back_forward_list()
            .ok_or("Browser history is unavailable")?;
        let back = list.back_list();
        let current = list.current_item();
        let forward = list.forward_list();
        let mut entries = Vec::new();
        for offset in -(back.len() as i32)..=forward.len() as i32 {
            if let Some(item) = list.nth_item(offset) {
                entries
                    .push(json!({"url":item.uri().map(|uri| uri.to_string()).unwrap_or_default()}));
            }
        }
        if current.is_none() {
            return Ok(json!({"currentIndex":0,"entries":[]}));
        }
        Ok(json!({"currentIndex":back.len(),"entries":entries}))
    })
}

pub fn navigate_history(view: &Webview, command: &str) -> Result<(), String> {
    let command = command.to_owned();
    native(view, move |browser| {
        match command.as_str() {
            "back" if browser.can_go_back() => browser.go_back(),
            "forward" if browser.can_go_forward() => browser.go_forward(),
            "back" | "forward" => {}
            _ => return Err("Invalid history command".to_owned()),
        }
        Ok(())
    })
}

fn png(surface: cairo::Surface, clip: Option<Value>, zoom: f64) -> Result<Value, String> {
    let surface = if let Some(clip) = clip {
        let number = |name: &str| {
            clip[name]
                .as_f64()
                .filter(|number| number.is_finite())
                .ok_or_else(|| format!("Invalid screenshot {name}"))
        };
        let x = number("x")?.max(0.0) * zoom;
        let y = number("y")?.max(0.0) * zoom;
        let width = number("width")? * zoom;
        let height = number("height")? * zoom;
        let (scale_x, scale_y) = surface.device_scale();
        let pixel_width = (width * scale_x).ceil();
        let pixel_height = (height * scale_y).ceil();
        if pixel_width < 1.0
            || pixel_height < 1.0
            || pixel_width > 16384.0
            || pixel_height > 16384.0
            || pixel_width * pixel_height > 64.0 * 1024.0 * 1024.0
        {
            return Err("Screenshot region exceeds image limits".to_owned());
        }
        let cropped = cairo::ImageSurface::create(
            cairo::Format::ARgb32,
            pixel_width as i32,
            pixel_height as i32,
        )
        .map_err(|error| error.to_string())?;
        cropped.set_device_scale(scale_x, scale_y);
        let context = cairo::Context::new(&cropped).map_err(|error| error.to_string())?;
        context
            .set_source_surface(&surface, -x, -y)
            .map_err(|error| error.to_string())?;
        context.paint().map_err(|error| error.to_string())?;
        cairo::Surface::clone(cropped.as_ref())
    } else {
        surface
    };
    let mut bytes = Vec::new();
    surface
        .write_to_png(&mut bytes)
        .map_err(|error| error.to_string())?;
    if bytes.is_empty() {
        return Err("Browser returned an empty screenshot".to_owned());
    }
    Ok(
        json!({"type":"image", "data":base64::engine::general_purpose::STANDARD.encode(bytes), "mimeType":"image/png"}),
    )
}

pub fn screenshot(view: &Webview, clip: Option<Value>) -> Result<Value, String> {
    let (sender, receiver) = mpsc::channel();
    view.with_webview(move |platform| {
        let browser = platform.inner();
        let zoom = browser.zoom_level();
        browser.snapshot(
            SnapshotRegion::Visible,
            SnapshotOptions::NONE,
            None::<&gio::Cancellable>,
            move |result| {
                let result = result
                    .map_err(|error| error.to_string())
                    .and_then(|surface| png(surface, clip, zoom));
                let _ = sender.send(result);
            },
        );
    })
    .map_err(|error| error.to_string())?;
    receiver
        .recv_timeout(Duration::from_secs(10))
        .map_err(|_| "Native WebKit screenshot timed out".to_owned())?
}

pub fn insert_text(view: &Webview, text: &str) -> Result<(), String> {
    if text.contains('\0') {
        return Err("Browser text cannot contain NUL characters".to_owned());
    }
    let text = text.to_owned();
    native(view, move |browser| {
        browser.execute_editing_command_with_argument("InsertText", &text);
        Ok(())
    })
}

fn key_value(key: &str) -> Result<(u32, String), String> {
    let named = match key {
        "Enter" => Some((gdk::keys::constants::Return, "\r")),
        "Tab" => Some((gdk::keys::constants::Tab, "\t")),
        "Escape" => Some((gdk::keys::constants::Escape, "")),
        "Backspace" => Some((gdk::keys::constants::BackSpace, "")),
        "Delete" => Some((gdk::keys::constants::Delete, "")),
        "ArrowLeft" => Some((gdk::keys::constants::Left, "")),
        "ArrowRight" => Some((gdk::keys::constants::Right, "")),
        "ArrowUp" => Some((gdk::keys::constants::Up, "")),
        "ArrowDown" => Some((gdk::keys::constants::Down, "")),
        "Home" => Some((gdk::keys::constants::Home, "")),
        "End" => Some((gdk::keys::constants::End, "")),
        "PageUp" => Some((gdk::keys::constants::Page_Up, "")),
        "PageDown" => Some((gdk::keys::constants::Page_Down, "")),
        "Space" => Some((gdk::keys::constants::space, " ")),
        _ => None,
    };
    if let Some((key, text)) = named {
        return Ok((key.into_glib(), text.to_owned()));
    }
    let mut characters = key.chars();
    let character = characters.next().ok_or("Empty browser key")?;
    if characters.next().is_some() {
        return Err(format!("Unsupported browser key: {key}"));
    }
    let key_value = unsafe { gdk::ffi::gdk_unicode_to_keyval(character as u32) };
    Ok((key_value, character.to_string()))
}

pub fn press_key(view: &Webview, key: &str) -> Result<(), String> {
    let (key_value, text) = key_value(key)?;
    native(view, move |browser| {
        dispatch_key(&browser, key_value, &text)
    })
}

fn dispatch_key(browser: &webkit2gtk::WebView, key_value: u32, text: &str) -> Result<(), String> {
    if !browser.is_realized() {
        browser.realize();
    }
    let window = browser
        .window()
        .ok_or("Browser has no native input window")?;
    let display = gtk::prelude::WidgetExt::display(browser);
    let keymap =
        gdk::Keymap::for_display(&display).ok_or("Native keyboard mapping is unavailable")?;
    let mapping = keymap
        .entries_for_keyval(key_value)
        .into_iter()
        .next()
        .ok_or("Requested key is absent from the native keyboard map")?;
    let device = display
        .default_seat()
        .and_then(|seat| seat.keyboard())
        .ok_or("Native keyboard device is unavailable")?;
    for kind in [gdk::EventType::KeyPress, gdk::EventType::KeyRelease] {
        let event: gdk::Event = unsafe {
            let pointer = gdk::ffi::gdk_event_new(kind.into_glib());
            if pointer.is_null() {
                return Err("Native key event allocation failed".to_owned());
            }
            let event = &mut *pointer.cast::<gdk::ffi::GdkEventKey>();
            event.window = window.to_glib_full();
            event.send_event = 0;
            event.time = gdk::ffi::GDK_CURRENT_TIME as u32;
            event.state = if mapping.level() == 1 {
                gdk::ModifierType::SHIFT_MASK.into_glib()
            } else {
                0
            };
            event.keyval = key_value;
            event.hardware_keycode = mapping.keycode() as u16;
            event.group = mapping.group() as u8;
            event.length = text.len() as i32;
            event.string = glib::ffi::g_strndup(text.as_ptr().cast(), text.len());
            gdk::ffi::gdk_event_set_device(pointer, device.to_glib_none().0);
            from_glib_full(pointer)
        };
        if !browser.event(&event) && kind == gdk::EventType::KeyPress {
            return Err("Native browser did not accept the key event".to_owned());
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn wait<T>(receiver: &mpsc::Receiver<T>) -> T {
        let deadline = std::time::Instant::now() + Duration::from_secs(20);
        loop {
            if let Ok(value) = receiver.try_recv() {
                return value;
            }
            assert!(
                std::time::Instant::now() < deadline,
                "Native browser smoke operation timed out"
            );
            while gtk::events_pending() {
                gtk::main_iteration_do(false);
            }
            std::thread::sleep(Duration::from_millis(5));
        }
    }

    fn evaluate(browser: &webkit2gtk::WebView, expression: &str) -> Value {
        use javascriptcore::ValueExt;
        let (sender, receiver) = mpsc::channel();
        browser.evaluate_javascript(
            expression,
            None,
            None,
            None::<&gio::Cancellable>,
            move |result| {
                let result = result
                    .map_err(|error| error.to_string())
                    .and_then(|value| {
                        value
                            .to_json(0)
                            .ok_or("JavaScript result is not JSON".to_owned())
                    })
                    .and_then(|value| {
                        serde_json::from_str::<Value>(&value).map_err(|error| error.to_string())
                    });
                let _ = sender.send(result);
            },
        );
        wait(&receiver).unwrap()
    }

    #[test]
    #[ignore = "requires a GTK display and WebKit renderer"]
    fn native_keyboard_and_text_generate_trusted_dom_events() {
        gtk::init().unwrap();
        let window = gtk::Window::new(gtk::WindowType::Toplevel);
        window.set_default_size(640, 480);
        let browser = webkit2gtk::WebView::new();
        window.add(&browser);
        let (sender, receiver) = mpsc::channel();
        browser.connect_load_changed(move |_, event| {
            if event == webkit2gtk::LoadEvent::Finished {
                let _ = sender.send(());
            }
        });
        browser.load_html("<input id='field'><script>window.received=[];for(const type of ['input','keydown','keyup'])document.addEventListener(type,event=>received.push({type:event.type,key:event.key,trusted:event.isTrusted}));</script>", Some("https://native-fixture.invalid/"));
        window.show_all();
        browser.grab_focus();
        wait(&receiver);
        assert_eq!(
            evaluate(
                &browser,
                "(()=>{document.querySelector('#field').focus();return true})()"
            ),
            true
        );
        browser.execute_editing_command_with_argument("InsertText", "Привет");
        let (value, text) = key_value("Enter").unwrap();
        dispatch_key(&browser, value, &text).unwrap();
        let deadline = std::time::Instant::now() + Duration::from_secs(3);
        let page = loop {
            let page = evaluate(
                &browser,
                "({value:document.querySelector('#field').value,events:received})",
            );
            if page["events"]
                .as_array()
                .is_some_and(|events| events.iter().any(|event| event["type"] == "keyup"))
                || std::time::Instant::now() >= deadline
            {
                break page;
            }
        };
        assert_eq!(page["value"], "Привет");
        let events = page["events"].as_array().unwrap();
        for kind in ["input", "keydown", "keyup"] {
            assert!(
                events
                    .iter()
                    .any(|event| event["type"] == kind && event["trusted"] == true),
                "Missing trusted {kind}: {page}"
            );
        }
        let (sender, receiver) = mpsc::channel();
        browser.snapshot(
            SnapshotRegion::Visible,
            SnapshotOptions::NONE,
            None::<&gio::Cancellable>,
            move |result| {
                let _ = sender.send(
                    result
                        .map_err(|error| error.to_string())
                        .and_then(|surface| png(surface, None, 1.0)),
                );
            },
        );
        assert_eq!(wait(&receiver).unwrap()["mimeType"], "image/png");
        window.close();
    }

    #[test]
    fn maps_named_and_unicode_keys() {
        assert_eq!(
            key_value("Enter").unwrap().0,
            gdk::keys::constants::Return.into_glib()
        );
        assert_eq!(
            key_value("ArrowLeft").unwrap().0,
            gdk::keys::constants::Left.into_glib()
        );
        assert_eq!(key_value("я").unwrap().1, "я");
        assert!(key_value("Ctrl+Enter").is_err());
    }

    #[test]
    fn encodes_and_crops_native_surfaces() {
        let surface = cairo::ImageSurface::create(cairo::Format::ARgb32, 100, 80).unwrap();
        let result = png(
            surface.as_ref().clone(),
            Some(json!({"x":5,"y":5,"width":30,"height":20})),
            1.0,
        )
        .unwrap();
        let bytes = base64::engine::general_purpose::STANDARD
            .decode(result["data"].as_str().unwrap())
            .unwrap();
        assert_eq!(&bytes[..8], &[137, 80, 78, 71, 13, 10, 26, 10]);
        assert_eq!(u32::from_be_bytes(bytes[16..20].try_into().unwrap()), 30);
        assert_eq!(u32::from_be_bytes(bytes[20..24].try_into().unwrap()), 20);
    }
}
