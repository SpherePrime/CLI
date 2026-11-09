use super::{evaluate, platform};
use serde_json::{json, Value};
use std::sync::atomic::AtomicBool;
use tauri::Webview;

const MAX_OUTPUT_SIDE: f64 = 8000.0;
const MAX_DOCUMENT_HEIGHT: f64 = 16000.0;

const LOAD_LAZY_CONTENT: &str = "(async () => { const start = scrollY; const step = Math.max(200, innerHeight * 0.8); for (let index = 0, top = 0; index < 80 && top < document.documentElement.scrollHeight; index += 1, top += step) { scrollTo(0, top); await new Promise(resolve => setTimeout(resolve, 120)); } scrollTo(0, start); await new Promise(resolve => setTimeout(resolve, 250)); return true })()";

const DOCUMENT_SIZE: &str = "(() => { const root = document.documentElement; const body = document.body; return { width: Math.max(root.scrollWidth, body ? body.scrollWidth : 0, innerWidth), height: Math.max(root.scrollHeight, body ? body.scrollHeight : 0, innerHeight) } })()";

const HIDE_OVERLAYS: &str = "for (const layer of document.querySelectorAll('body *')) { if (layer.contains(element) || element.contains(layer)) continue; const position = getComputedStyle(layer).position; if (position !== 'fixed' && position !== 'sticky') continue; layer.setAttribute('data-prime-shot-hidden', layer.style.visibility); layer.style.visibility = 'hidden'; }";

const RESTORE_OVERLAYS: &str = "(() => { for (const layer of document.querySelectorAll('[data-prime-shot-hidden]')) { layer.style.visibility = layer.getAttribute('data-prime-shot-hidden'); layer.removeAttribute('data-prime-shot-hidden'); } return true })()";

pub enum Target {
    Visible,
    FullPage,
    Element(String),
}

pub fn target(args: &Value, element_selector: impl Fn(&Value) -> Result<String, String>) -> Result<Target, String> {
    let full_page = args["fullPage"] == true;
    let has_ref = !args["ref"].is_null();
    match (full_page, has_ref) {
        (true, true) => Err("Use either fullPage or ref, not both".to_owned()),
        (true, false) => Ok(Target::FullPage),
        (false, true) => Ok(Target::Element(element_selector(args)?)),
        (false, false) => Ok(Target::Visible),
    }
}

pub fn capture(view: &Webview, tab: &str, target: Target, cancel: &AtomicBool) -> Result<Value, String> {
    match target {
        Target::Visible => Ok(json!([platform::screenshot(view, None)?])),
        Target::FullPage => {
            evaluate::evaluate_until_done(view, tab, LOAD_LAZY_CONTENT, cancel)?;
            let size = evaluate::immediate(view, DOCUMENT_SIZE)?;
            let width = dimension(&size, "width")?;
            let full_height = dimension(&size, "height")?;
            let height = full_height.min(MAX_DOCUMENT_HEIGHT);
            let image = platform::screenshot_document(view, region(0.0, 0.0, width, height))?;
            let mut note = format!("Full page {width}x{full_height} CSS px in one image.");
            if height < full_height {
                note.push_str(&format!(" Only the top {height} px fit; capture lower parts by ref."));
            }
            Ok(json!([image, {"type":"text", "text":note}]))
        }
        Target::Element(selector) => {
            let selector = serde_json::to_string(&selector).map_err(|error| error.to_string())?;
            let rect = evaluate::immediate(view, &format!("(() => {{ const element = document.querySelector({selector}); if (!element) return null; element.scrollIntoView({{block:'nearest'}}); {HIDE_OVERLAYS} const box = element.getBoundingClientRect(); return {{ x: box.left + scrollX, y: box.top + scrollY, width: box.width, height: box.height }} }})()"))?;
            if rect.is_null() {
                return Err("Element not found. Call browser_snapshot again.".to_owned());
            }
            let image = element_image(view, &rect);
            evaluate::immediate(view, RESTORE_OVERLAYS)?;
            Ok(json!([image?]))
        }
    }
}

fn element_image(view: &Webview, rect: &Value) -> Result<Value, String> {
    let width = dimension(rect, "width")?;
    let height = dimension(rect, "height")?.min(MAX_DOCUMENT_HEIGHT);
    let x = rect["x"].as_f64().unwrap_or(0.0).max(0.0);
    let y = rect["y"].as_f64().unwrap_or(0.0).max(0.0);
    platform::screenshot_document(view, region(x, y, width, height))
}

fn dimension(value: &Value, name: &str) -> Result<f64, String> {
    value[name]
        .as_f64()
        .filter(|number| number.is_finite() && *number >= 1.0)
        .map(f64::ceil)
        .ok_or_else(|| format!("Page has no visible {name} to capture"))
}

fn region(x: f64, y: f64, width: f64, height: f64) -> Value {
    json!({"x":x, "y":y, "width":width, "height":height, "scale":output_scale(width, height)})
}

fn output_scale(width: f64, height: f64) -> f64 {
    (MAX_OUTPUT_SIDE / width.max(height)).min(1.0)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn by_ref(args: &Value) -> Result<String, String> {
        Ok(format!("[data-prime-ref=\"{}\"]", args["ref"]))
    }

    #[test]
    fn target_follows_arguments() {
        assert!(matches!(target(&json!({}), by_ref), Ok(Target::Visible)));
        assert!(matches!(target(&json!({"fullPage":true}), by_ref), Ok(Target::FullPage)));
        assert!(matches!(target(&json!({"ref":3}), by_ref), Ok(Target::Element(selector)) if selector == "[data-prime-ref=\"3\"]"));
        assert!(target(&json!({"fullPage":true, "ref":3}), by_ref).is_err());
    }

    #[test]
    fn long_pages_are_scaled_to_the_output_limit() {
        assert_eq!(output_scale(1280.0, 4000.0), 1.0);
        assert_eq!(output_scale(1000.0, 16000.0), 0.5);
        assert_eq!(region(0.0, 0.0, 1280.0, 16000.0)["scale"], 0.5);
    }
}
