use super::scenario_parse::Clip;
use base64::Engine;

const MODEL_MAX_EDGE: f64 = 1568.0;
const MODEL_MAX_PIXELS: f64 = 1_150_000.0;
const LEGIBLE_FACTOR: f64 = 0.8;
const MIN_TILE_HEIGHT: f64 = 240.0;

pub fn bounded(clip: &Clip, page_width: f64, page_height: f64) -> Option<Clip> {
    let left = clip.x.max(0.0);
    let top = clip.y.max(0.0);
    let right = (clip.x + clip.width).min(page_width);
    let bottom = (clip.y + clip.height).min(page_height);
    (right - left >= 1.0 && bottom - top >= 1.0).then(|| Clip {
        x: left.floor(),
        y: top.floor(),
        width: (right - left).ceil(),
        height: (bottom - top).ceil(),
    })
}

pub fn output_scale(width: f64, height: f64, zoom: f64, max_side: f64) -> f64 {
    (max_side / width.max(height)).min(zoom)
}

pub fn model_factor(width: f64, height: f64) -> f64 {
    (MODEL_MAX_EDGE / width.max(height))
        .min((MODEL_MAX_PIXELS / (width * height)).sqrt())
        .min(1.0)
}

pub fn tiles(width: f64, height: f64, pixel_ratio: f64, max_tiles: usize) -> Vec<Clip> {
    let output_width = width * pixel_ratio;
    let output_height = MODEL_MAX_EDGE.min(MODEL_MAX_PIXELS / output_width.max(1.0));
    let tile_height = (output_height / pixel_ratio).floor().max(MIN_TILE_HEIGHT);
    let mut sections = Vec::new();
    let mut top = 0.0;
    while top < height && sections.len() < max_tiles {
        let section = tile_height.min(height - top);
        sections.push(Clip { x: 0.0, y: top, width, height: section });
        top += section;
    }
    sections
}

pub fn png_size(encoded: &str) -> Option<(f64, f64)> {
    let head: String = encoded.chars().take(32).collect();
    let bytes = base64::engine::general_purpose::STANDARD.decode(head).ok()?;
    if bytes.len() < 24 || &bytes[1..4] != b"PNG" {
        return None;
    }
    let width = u32::from_be_bytes(bytes[16..20].try_into().ok()?);
    let height = u32::from_be_bytes(bytes[20..24].try_into().ok()?);
    (width > 0 && height > 0).then_some((width as f64, height as f64))
}

pub fn model_note(encoded: &str) -> Option<String> {
    let (width, height) = png_size(encoded)?;
    let factor = model_factor(width, height);
    (factor < LEGIBLE_FACTOR).then(|| {
        format!(
            "This {width}x{height} px image reaches the model at about {}% of its size, so small text, icons and thin lines are not legible in it. Judge only composition from it; inspect details with close-ups (selector or ref with padding and zoom, clip) or fullPage with tiles.",
            (factor * 100.0).round()
        )
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn clips_stay_inside_the_page() {
        let inside = bounded(&Clip { x: -10.0, y: 20.0, width: 100.0, height: 50.0 }, 1440.0, 3000.0).unwrap();
        assert_eq!(inside, Clip { x: 0.0, y: 20.0, width: 90.0, height: 50.0 });
        assert!(bounded(&Clip { x: 2000.0, y: 0.0, width: 100.0, height: 50.0 }, 1440.0, 3000.0).is_none());
    }

    #[test]
    fn scale_respects_zoom_and_output_limit() {
        assert_eq!(output_scale(1280.0, 4000.0, 1.0, 8000.0), 1.0);
        assert_eq!(output_scale(1000.0, 16000.0, 1.0, 8000.0), 0.5);
        assert_eq!(output_scale(300.0, 200.0, 2.0, 8000.0), 2.0);
        assert_eq!(output_scale(5000.0, 200.0, 3.0, 8000.0), 1.6);
    }

    #[test]
    fn tiles_stay_legible_for_the_model() {
        let sections = tiles(1440.0, 5000.0, 1.0, 10);
        assert!(sections.len() > 1);
        assert_eq!(sections[0].y, 0.0);
        assert_eq!(sections.iter().map(|tile| tile.height).sum::<f64>(), 5000.0);
        for tile in &sections {
            assert!(model_factor(tile.width, tile.height) >= 0.95);
        }
        assert_eq!(tiles(1440.0, 50000.0, 1.0, 4).len(), 4);
    }

    #[test]
    fn notes_flag_downscaled_images_only() {
        let header = |width: u32, height: u32| {
            let mut bytes = vec![0x89, b'P', b'N', b'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 13, b'I', b'H', b'D', b'R'];
            bytes.extend(width.to_be_bytes());
            bytes.extend(height.to_be_bytes());
            base64::engine::general_purpose::STANDARD.encode(bytes)
        };
        assert_eq!(png_size(&header(1440, 900)), Some((1440.0, 900.0)));
        assert!(model_note(&header(1200, 800)).is_none());
        assert!(model_note(&header(1440, 6000)).unwrap().contains("26%"));
    }
}
