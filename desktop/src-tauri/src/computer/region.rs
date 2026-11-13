use serde::Deserialize;
use serde_json::Value;

pub const MAX_OUTPUT_WIDTH: usize = 1920;

#[derive(Debug, Clone, Copy, Deserialize, PartialEq)]
pub struct Region {
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub struct Crop {
    pub left: usize,
    pub top: usize,
    pub width: usize,
    pub height: usize,
}

pub fn parse(args: &Value) -> Result<Option<Region>, String> {
    let Some(value) = args.get("region").filter(|value| !value.is_null()) else {
        return Ok(None);
    };
    let region: Region = serde_json::from_value(value.clone())
        .map_err(|_| "region must be {x, y, width, height} in window image pixels".to_owned())?;
    if [region.x, region.y, region.width, region.height].iter().any(|value| !value.is_finite())
        || region.width < 1.0
        || region.height < 1.0
    {
        return Err("region needs finite x, y and a width and height of at least 1 pixel".to_owned());
    }
    Ok(Some(region))
}

pub fn view_size(crop_width: usize, crop_height: usize) -> (usize, usize) {
    let width = crop_width.min(MAX_OUTPUT_WIDTH).max(1);
    let height = ((crop_height as f64 * width as f64 / crop_width.max(1) as f64).round() as usize).max(1);
    (width, height)
}

pub fn source_crop(region: &Region, window: Crop) -> Option<Crop> {
    let (view_width, view_height) = view_size(window.width, window.height);
    let scale_x = window.width as f64 / view_width as f64;
    let scale_y = window.height as f64 / view_height as f64;
    let left = (region.x.max(0.0) * scale_x).floor() as usize;
    let top = (region.y.max(0.0) * scale_y).floor() as usize;
    let right = (((region.x + region.width) * scale_x).ceil() as usize).min(window.width);
    let bottom = (((region.y + region.height) * scale_y).ceil() as usize).min(window.height);
    (right > left && bottom > top).then(|| Crop {
        left: window.left + left,
        top: window.top + top,
        width: right - left,
        height: bottom - top,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn regions_map_to_native_pixels() {
        let window = Crop { left: 8, top: 30, width: 3840, height: 2160 };
        assert_eq!(view_size(3840, 2160), (1920, 1080));
        let crop = source_crop(&Region { x: 100.0, y: 50.0, width: 200.0, height: 100.0 }, window).unwrap();
        assert_eq!(crop, Crop { left: 208, top: 130, width: 400, height: 200 });
        assert!(source_crop(&Region { x: 5000.0, y: 0.0, width: 10.0, height: 10.0 }, window).is_none());
    }

    #[test]
    fn region_arguments_are_validated() {
        assert_eq!(parse(&json!({})).unwrap(), None);
        assert!(parse(&json!({"region":{"x":0,"y":0,"width":0,"height":5}})).is_err());
        assert_eq!(parse(&json!({"region":{"x":1,"y":2,"width":3,"height":4}})).unwrap(), Some(Region { x: 1.0, y: 2.0, width: 3.0, height: 4.0 }));
    }
}
