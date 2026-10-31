use ::windows::Win32::Foundation::HWND;
use ::windows::Win32::UI::Accessibility::IUIAutomation;

use super::action;
use crate::appui::types::{Locator, UiBounds};

pub fn element_png(
    automation: &IUIAutomation,
    window: HWND,
    locator: &Locator,
) -> Result<Vec<u8>, String> {
    let (node, _) = action::bounds_of(automation, window, locator)?;
    let hwnd = (window.0 as usize).to_string();
    let capture = crate::computer::capture::capture(&hwnd)?;
    let window_bounds = UiBounds {
        x: capture.info.bounds.x,
        y: capture.info.bounds.y,
        width: capture.info.bounds.width,
        height: capture.info.bounds.height,
    };
    crop_png(&capture.png, &window_bounds, &node.bounds)
}

pub fn crop_png(png: &[u8], window: &UiBounds, element: &UiBounds) -> Result<Vec<u8>, String> {
    let mut decoder = png::Decoder::new(std::io::Cursor::new(png));
    decoder.set_transformations(png::Transformations::normalize_to_color8());
    let mut reader = decoder.read_info().map_err(|error| error.to_string())?;
    let mut buffer = vec![
        0;
        reader
            .output_buffer_size()
            .ok_or("Unsupported capture size")?
    ];
    let info = reader
        .next_frame(&mut buffer)
        .map_err(|error| error.to_string())?;
    let channels = match info.color_type {
        png::ColorType::Rgba => 4usize,
        png::ColorType::Rgb => 3,
        png::ColorType::Grayscale => 1,
        png::ColorType::GrayscaleAlpha => 2,
        png::ColorType::Indexed => return Err("Unsupported capture format".to_owned()),
    };
    let width = info.width as i32;
    let height = info.height as i32;
    let scale_x = width as f64 / window.width.max(1) as f64;
    let scale_y = height as f64 / window.height.max(1) as f64;
    let left = (((element.x - window.x) as f64) * scale_x)
        .floor()
        .clamp(0.0, (width - 1) as f64) as i32;
    let top = (((element.y - window.y) as f64) * scale_y)
        .floor()
        .clamp(0.0, (height - 1) as f64) as i32;
    let right = (((element.x - window.x + element.width) as f64) * scale_x)
        .ceil()
        .clamp((left + 1) as f64, width as f64) as i32;
    let bottom = (((element.y - window.y + element.height) as f64) * scale_y)
        .ceil()
        .clamp((top + 1) as f64, height as f64) as i32;
    let crop_width = (right - left) as u32;
    let crop_height = (bottom - top) as u32;
    let mut cropped = Vec::with_capacity(crop_width as usize * crop_height as usize * channels);
    for row in top..bottom {
        let start = (row as usize * width as usize + left as usize) * channels;
        let end = start + crop_width as usize * channels;
        cropped.extend_from_slice(&buffer[start..end]);
    }
    let mut encoded = Vec::new();
    let mut encoder = png::Encoder::new(&mut encoded, crop_width, crop_height);
    encoder.set_color(info.color_type);
    encoder.set_depth(png::BitDepth::Eight);
    let mut writer = encoder.write_header().map_err(|error| error.to_string())?;
    writer
        .write_image_data(&cropped)
        .map_err(|error| error.to_string())?;
    drop(writer);
    Ok(encoded)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn png(width: u32, height: u32) -> Vec<u8> {
        let mut bytes = Vec::new();
        let mut encoder = png::Encoder::new(&mut bytes, width, height);
        encoder.set_color(png::ColorType::Rgb);
        encoder.set_depth(png::BitDepth::Eight);
        let mut writer = encoder.write_header().unwrap();
        writer
            .write_image_data(&vec![0u8; width as usize * height as usize * 3])
            .unwrap();
        drop(writer);
        bytes
    }

    fn size(png: &[u8]) -> (u32, u32) {
        let decoder = png::Decoder::new(std::io::Cursor::new(png));
        let reader = decoder.read_info().unwrap();
        (reader.info().width, reader.info().height)
    }

    #[test]
    fn crops_the_element_region_from_the_window_capture() {
        let window = UiBounds { x: 100, y: 100, width: 200, height: 100 };
        let element = UiBounds { x: 140, y: 120, width: 40, height: 20 };
        let cropped = crop_png(&png(200, 100), &window, &element).unwrap();
        assert_eq!(size(&cropped), (40, 20));
    }

    #[test]
    fn scales_the_crop_when_the_capture_is_downscaled() {
        let window = UiBounds { x: 0, y: 0, width: 400, height: 200 };
        let element = UiBounds { x: 200, y: 100, width: 100, height: 50 };
        let cropped = crop_png(&png(200, 100), &window, &element).unwrap();
        assert_eq!(size(&cropped), (50, 25));
    }

    #[test]
    fn elements_outside_the_window_are_clamped_inside_the_capture() {
        let window = UiBounds { x: 0, y: 0, width: 100, height: 100 };
        let element = UiBounds { x: -50, y: -50, width: 40, height: 40 };
        let cropped = crop_png(&png(100, 100), &window, &element).unwrap();
        let (width, height) = size(&cropped);
        assert!(width >= 1 && width <= 100, "{width}");
        assert!(height >= 1 && height <= 100, "{height}");
    }
}
