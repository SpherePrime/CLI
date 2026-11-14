use super::{evaluate, platform, scenario_parse::Clip, shot_view};
use serde_json::{json, Value};
use std::sync::atomic::AtomicBool;
use tauri::Webview;

const MAX_OUTPUT_SIDE: f64 = 8000.0;
const MAX_DOCUMENT_HEIGHT: f64 = 16000.0;
const MAX_TILES: usize = 10;
const MAX_PADDING: f64 = 400.0;
const MAX_ZOOM: f64 = 3.0;

const LOAD_LAZY_CONTENT: &str = "(async () => { const start = scrollY; const step = Math.max(200, innerHeight * 0.8); for (let index = 0, top = 0; index < 80 && top < document.documentElement.scrollHeight; index += 1, top += step) { scrollTo(0, top); await new Promise(resolve => setTimeout(resolve, 120)); } scrollTo(0, start); await new Promise(resolve => setTimeout(resolve, 250)); return true })()";

const DOCUMENT_SIZE: &str = "(() => { const root = document.documentElement; const body = document.body; return { width: Math.max(root.scrollWidth, body ? body.scrollWidth : 0, innerWidth), height: Math.max(root.scrollHeight, body ? body.scrollHeight : 0, innerHeight), dpr: devicePixelRatio || 1 } })()";

const VIEWPORT: &str = "(() => ({ x: scrollX, y: scrollY, width: innerWidth, height: innerHeight }))()";

const HIDE_OVERLAYS: &str = "for (const layer of document.querySelectorAll('body *')) { if (layer.contains(element) || element.contains(layer)) continue; const position = getComputedStyle(layer).position; if (position !== 'fixed' && position !== 'sticky') continue; layer.setAttribute('data-prime-shot-hidden', layer.style.visibility); layer.style.visibility = 'hidden'; }";

const RESTORE_OVERLAYS: &str = "(() => { for (const layer of document.querySelectorAll('[data-prime-shot-hidden]')) { layer.style.visibility = layer.getAttribute('data-prime-shot-hidden'); layer.removeAttribute('data-prime-shot-hidden'); } return true })()";

#[derive(Debug, PartialEq)]
pub enum Target {
    Visible,
    FullPage { tiles: bool },
    Element(String),
    Region(Clip),
}

#[derive(Debug, PartialEq)]
pub struct Shot {
    pub target: Target,
    pub padding: f64,
    pub zoom: f64,
}

impl Shot {
    pub fn new(target: Target, padding: Option<f64>, zoom: Option<f64>) -> Result<Self, String> {
        let padding = padding.unwrap_or(0.0);
        if !padding.is_finite() || !(0.0..=MAX_PADDING).contains(&padding) {
            return Err(format!("padding must be between 0 and {MAX_PADDING} CSS px"));
        }
        let zoom = zoom.unwrap_or(1.0);
        if !zoom.is_finite() || !(1.0..=MAX_ZOOM).contains(&zoom) {
            return Err(format!("zoom must be between 1 and {MAX_ZOOM}"));
        }
        if let Target::Region(clip) = &target {
            if [clip.x, clip.y, clip.width, clip.height].iter().any(|value| !value.is_finite()) || clip.width < 1.0 || clip.height < 1.0 {
                return Err("clip needs finite x, y and a width and height of at least 1 CSS px".to_owned());
            }
        }
        Ok(Self { target, padding, zoom })
    }
}

pub fn target(args: &Value, element_selector: impl Fn(&Value) -> Result<String, String>) -> Result<Shot, String> {
    let full_page = args["fullPage"] == true;
    let has_ref = !args["ref"].is_null();
    let clip = match args.get("clip").filter(|value| !value.is_null()) {
        Some(value) => Some(serde_json::from_value::<Clip>(value.clone()).map_err(|_| "clip must be {x, y, width, height} in CSS px")?),
        None => None,
    };
    let chosen = [full_page, has_ref, clip.is_some()].iter().filter(|flag| **flag).count();
    if chosen > 1 {
        return Err("Use only one of fullPage, ref or clip".to_owned());
    }
    let target = if full_page {
        Target::FullPage { tiles: args["tiles"] == true }
    } else if has_ref {
        Target::Element(element_selector(args)?)
    } else if let Some(clip) = clip {
        Target::Region(clip)
    } else {
        Target::Visible
    };
    Shot::new(target, args["padding"].as_f64(), args["zoom"].as_f64())
}

pub fn capture(view: &Webview, tab: &str, shot: &Shot, cancel: &AtomicBool) -> Result<Vec<Value>, String> {
    let parts = match &shot.target {
        Target::Visible if shot.zoom == 1.0 => vec![platform::screenshot(view, None)?],
        Target::Visible => {
            let viewport = evaluate::immediate(view, VIEWPORT)?;
            let clip = Clip {
                x: number(&viewport, "x"),
                y: number(&viewport, "y"),
                width: dimension(&viewport, "width")?,
                height: dimension(&viewport, "height")?,
            };
            vec![platform::screenshot_document(view, region(&clip, shot.zoom))?]
        }
        Target::FullPage { tiles } => full_page(view, tab, *tiles, shot.zoom, cancel)?,
        Target::Region(clip) => {
            let document = evaluate::immediate(view, DOCUMENT_SIZE)?;
            let clip = shot_view::bounded(clip, dimension(&document, "width")?, dimension(&document, "height")?)
                .ok_or("clip lies outside the page")?;
            vec![platform::screenshot_document(view, region(&clip, shot.zoom))?]
        }
        Target::Element(selector) => vec![element(view, selector, shot.padding, shot.zoom)?],
    };
    Ok(with_model_notes(parts))
}

fn full_page(view: &Webview, tab: &str, tiles: bool, zoom: f64, cancel: &AtomicBool) -> Result<Vec<Value>, String> {
    evaluate::evaluate_until_done(view, tab, LOAD_LAZY_CONTENT, cancel)?;
    let size = evaluate::immediate(view, DOCUMENT_SIZE)?;
    let width = dimension(&size, "width")?;
    let full_height = dimension(&size, "height")?;
    let height = full_height.min(MAX_DOCUMENT_HEIGHT);
    if !tiles {
        let page = Clip { x: 0.0, y: 0.0, width, height };
        let image = platform::screenshot_document(view, region(&page, zoom))?;
        let mut note = format!("Full page {width}x{full_height} CSS px in one image.");
        if height < full_height {
            note.push_str(&format!(" Only the top {height} px fit; capture lower parts with clip or tiles."));
        }
        return Ok(vec![image, evaluate::text_part(note)]);
    }
    let dpr = size["dpr"].as_f64().filter(|value| value.is_finite() && *value > 0.0).unwrap_or(1.0);
    let sections = shot_view::tiles(width, height, zoom * dpr, MAX_TILES);
    let mut parts = Vec::with_capacity(sections.len() * 2 + 1);
    for (index, tile) in sections.iter().enumerate() {
        parts.push(evaluate::text_part(format!(
            "Tile {}/{}: y {}-{} of {full_height} CSS px",
            index + 1,
            sections.len(),
            tile.y,
            tile.y + tile.height
        )));
        parts.push(platform::screenshot_document(view, region(tile, zoom))?);
    }
    let covered = sections.last().map_or(0.0, |tile| tile.y + tile.height);
    if covered < full_height {
        parts.push(evaluate::text_part(format!("Tiles cover the top {covered} px; capture the rest with clip from y {covered}.")));
    }
    Ok(parts)
}

fn element(view: &Webview, selector: &str, padding: f64, zoom: f64) -> Result<Value, String> {
    let selector = serde_json::to_string(selector).map_err(|error| error.to_string())?;
    let rect = evaluate::immediate(view, &format!("(() => {{ const element = document.querySelector({selector}); if (!element) return null; element.scrollIntoView({{block:'nearest'}}); {HIDE_OVERLAYS} const box = element.getBoundingClientRect(); const root = document.documentElement; return {{ x: box.left + scrollX, y: box.top + scrollY, width: box.width, height: box.height, pageWidth: Math.max(root.scrollWidth, innerWidth), pageHeight: Math.max(root.scrollHeight, innerHeight) }} }})()"))?;
    if rect.is_null() {
        return Err("Element not found. Call browser_snapshot again.".to_owned());
    }
    let image = (|| {
        let clip = Clip {
            x: number(&rect, "x") - padding,
            y: number(&rect, "y") - padding,
            width: dimension(&rect, "width")? + padding * 2.0,
            height: dimension(&rect, "height")?.min(MAX_DOCUMENT_HEIGHT) + padding * 2.0,
        };
        let clip = shot_view::bounded(&clip, dimension(&rect, "pageWidth")?, dimension(&rect, "pageHeight")?)
            .ok_or("Element lies outside the page")?;
        platform::screenshot_document(view, region(&clip, zoom))
    })();
    evaluate::immediate(view, RESTORE_OVERLAYS)?;
    image
}

fn with_model_notes(parts: Vec<Value>) -> Vec<Value> {
    let mut output = Vec::with_capacity(parts.len() + 1);
    for part in parts {
        let note = (part["type"] == "image")
            .then(|| part["data"].as_str().and_then(shot_view::model_note))
            .flatten();
        output.push(part);
        if let Some(note) = note {
            output.push(evaluate::text_part(note));
        }
    }
    output
}

fn number(value: &Value, name: &str) -> f64 {
    value[name].as_f64().filter(|number| number.is_finite()).unwrap_or(0.0)
}

fn dimension(value: &Value, name: &str) -> Result<f64, String> {
    value[name]
        .as_f64()
        .filter(|number| number.is_finite() && *number >= 1.0)
        .map(f64::ceil)
        .ok_or_else(|| format!("Page has no visible {name} to capture"))
}

fn region(clip: &Clip, zoom: f64) -> Value {
    json!({"x":clip.x, "y":clip.y, "width":clip.width, "height":clip.height, "scale":shot_view::output_scale(clip.width, clip.height, zoom, MAX_OUTPUT_SIDE)})
}

#[cfg(test)]
mod tests {
    use super::*;

    fn by_ref(args: &Value) -> Result<String, String> {
        Ok(format!("[data-prime-ref=\"{}\"]", args["ref"]))
    }

    #[test]
    fn target_follows_arguments() {
        assert_eq!(target(&json!({}), by_ref).unwrap().target, Target::Visible);
        assert_eq!(target(&json!({"fullPage":true}), by_ref).unwrap().target, Target::FullPage { tiles: false });
        assert_eq!(target(&json!({"fullPage":true, "tiles":true}), by_ref).unwrap().target, Target::FullPage { tiles: true });
        assert_eq!(target(&json!({"ref":3, "padding":16, "zoom":2}), by_ref).unwrap(), Shot { target: Target::Element("[data-prime-ref=\"3\"]".to_owned()), padding: 16.0, zoom: 2.0 });
        assert_eq!(target(&json!({"clip":{"x":0,"y":10,"width":200,"height":100}}), by_ref).unwrap().target, Target::Region(Clip { x: 0.0, y: 10.0, width: 200.0, height: 100.0 }));
        assert!(target(&json!({"fullPage":true, "ref":3}), by_ref).is_err());
        assert!(target(&json!({"zoom":5}), by_ref).is_err());
        assert!(target(&json!({"clip":{"x":0,"y":0,"width":0,"height":10}}), by_ref).is_err());
    }

    #[test]
    fn with_model_notes_returns_flat_objects() {
        use base64::Engine;

        let png_base64 = |width: u32, height: u32| {
            let mut bytes = vec![0x89, b'P', b'N', b'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 13, b'I', b'H', b'D', b'R'];
            bytes.extend(width.to_be_bytes());
            bytes.extend(height.to_be_bytes());
            base64::engine::general_purpose::STANDARD.encode(bytes)
        };

        let small_image = json!({ "type": "image", "data": png_base64(1200, 800), "mimeType": "image/png" });
        let large_image = json!({ "type": "image", "data": png_base64(1440, 6000), "mimeType": "image/png" });
        let tile_label = evaluate::text_part("Tile 1/2: y 0-1000 of 2000 CSS px");

        let parts = with_model_notes(vec![tile_label, small_image, large_image]);
        assert_eq!(parts.len(), 4);
        for part in &parts {
            assert!(part.is_object(), "every screenshot part must be a JSON object, got {part}");
            assert!(!part.is_array());
            assert!(part["type"].as_str().is_some());
        }
        assert_eq!(parts[0]["type"], "text");
        assert_eq!(parts[1]["type"], "image");
        assert_eq!(parts[2]["type"], "image");
        assert_eq!(parts[3]["type"], "text");
    }
}
