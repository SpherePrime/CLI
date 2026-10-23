use super::types::WindowInfo;
use ::windows::Win32::{
    Foundation::{HWND, RECT},
    Graphics::Gdi::*,
    Storage::Xps::{PrintWindow, PRINT_WINDOW_FLAGS},
    UI::WindowsAndMessaging::{GetWindowRect, IsHungAppWindow, PW_RENDERFULLCONTENT},
};
use std::{
    sync::{
        atomic::{AtomicUsize, Ordering},
        mpsc,
    },
    time::Duration,
};

static CAPTURES: AtomicUsize = AtomicUsize::new(0);

pub struct Capture {
    pub info: WindowInfo,
    pub png: Vec<u8>,
    pub width: u32,
    pub height: u32,
}

struct Resources {
    window: HWND,
    source: HDC,
    memory: HDC,
    bitmap: HBITMAP,
    previous: HGDIOBJ,
}
impl Drop for Resources {
    fn drop(&mut self) {
        unsafe {
            if !self.memory.is_invalid() && !self.previous.is_invalid() {
                let _ = SelectObject(self.memory, self.previous);
            }
            if !self.bitmap.is_invalid() {
                let _ = DeleteObject(HGDIOBJ(self.bitmap.0));
            }
            if !self.memory.is_invalid() {
                let _ = DeleteDC(self.memory);
            }
            if !self.source.is_invalid() {
                let _ = ReleaseDC(Some(self.window), self.source);
            }
        }
    }
}

fn image(hwnd: &str) -> Result<Capture, String> {
    let info = super::windows::claim(hwnd)?;
    let window = super::windows::handle(hwnd)?;
    unsafe {
        if IsHungAppWindow(window).as_bool() {
            return Err("The selected window is not responding".to_owned());
        }
        let mut rect = RECT::default();
        GetWindowRect(window, &mut rect).map_err(|error| error.to_string())?;
        let width = rect
            .right
            .checked_sub(rect.left)
            .filter(|width| *width > 0)
            .ok_or("Invalid capture width")?;
        let height = rect
            .bottom
            .checked_sub(rect.top)
            .filter(|height| *height > 0)
            .ok_or("Invalid capture height")?;
        if width > 16384 || height > 16384 || width as u64 * height as u64 > 64 * 1024 * 1024 {
            return Err("Window capture exceeds image limits".to_owned());
        }
        let mut resources = Resources {
            window,
            source: GetWindowDC(Some(window)),
            memory: HDC::default(),
            bitmap: HBITMAP::default(),
            previous: HGDIOBJ::default(),
        };
        if resources.source.is_invalid() {
            return Err("Windows could not access this window's drawing context".to_owned());
        }
        resources.memory = CreateCompatibleDC(Some(resources.source));
        if resources.memory.is_invalid() {
            return Err("Windows could not create a capture context".to_owned());
        }
        resources.bitmap = CreateCompatibleBitmap(resources.source, width, height);
        if resources.bitmap.is_invalid() {
            return Err("Windows could not allocate a capture bitmap".to_owned());
        }
        resources.previous = SelectObject(resources.memory, HGDIOBJ(resources.bitmap.0));
        if resources.previous.is_invalid() {
            return Err("Windows could not select the capture bitmap".to_owned());
        }
        if !PrintWindow(
            window,
            resources.memory,
            PRINT_WINDOW_FLAGS(PW_RENDERFULLCONTENT),
        )
        .as_bool()
        {
            return Err(
                "Windows could not capture the selected window. Restore it and retry.".to_owned(),
            );
        }
        let mut data = vec![0u8; width as usize * height as usize * 4];
        let mut bitmap_info = BITMAPINFO {
            bmiHeader: BITMAPINFOHEADER {
                biSize: std::mem::size_of::<BITMAPINFOHEADER>() as u32,
                biWidth: width,
                biHeight: -height,
                biPlanes: 1,
                biBitCount: 32,
                biCompression: BI_RGB.0,
                ..Default::default()
            },
            ..Default::default()
        };
        let _ = SelectObject(resources.memory, resources.previous);
        if GetDIBits(
            resources.memory,
            resources.bitmap,
            0,
            height as u32,
            Some(data.as_mut_ptr().cast()),
            &mut bitmap_info,
            DIB_RGB_COLORS,
        ) != height
        {
            return Err("Windows could not read the captured window image".to_owned());
        }
        let left = info.bounds.x.saturating_sub(rect.left).clamp(0, width - 1) as usize;
        let top = info.bounds.y.saturating_sub(rect.top).clamp(0, height - 1) as usize;
        let crop_width = (info.bounds.width.max(1) as usize).min(width as usize - left);
        let crop_height = (info.bounds.height.max(1) as usize).min(height as usize - top);
        let output_width = crop_width.min(1920);
        let output_height = ((crop_height as f64 * output_width as f64 / crop_width as f64).round()
            as usize)
            .max(1);
        let mut rgba = vec![0u8; output_width * output_height * 4];
        for y in 0..output_height {
            for x in 0..output_width {
                let source = (((top + y * crop_height / output_height) * width as usize)
                    + (left + x * crop_width / output_width))
                    * 4;
                let target = (y * output_width + x) * 4;
                rgba[target..target + 4].copy_from_slice(&[
                    data[source + 2],
                    data[source + 1],
                    data[source],
                    255,
                ]);
            }
        }
        let mut png = Vec::new();
        {
            let mut encoder =
                png::Encoder::new(&mut png, output_width as u32, output_height as u32);
            encoder.set_color(png::ColorType::Rgba);
            encoder.set_depth(png::BitDepth::Eight);
            encoder
                .write_header()
                .map_err(|error| error.to_string())?
                .write_image_data(&rgba)
                .map_err(|error| error.to_string())?;
        }
        Ok(Capture {
            info,
            png,
            width: output_width as u32,
            height: output_height as u32,
        })
    }
}

pub fn capture(hwnd: &str) -> Result<Capture, String> {
    if CAPTURES
        .fetch_update(Ordering::AcqRel, Ordering::Acquire, |count| {
            (count < 2).then_some(count + 1)
        })
        .is_err()
    {
        return Err("Previous window captures have not finished".to_owned());
    }
    let hwnd = hwnd.to_owned();
    let (sender, receiver) = mpsc::channel();
    std::thread::spawn(move || {
        struct Active;
        impl Drop for Active {
            fn drop(&mut self) {
                CAPTURES.fetch_sub(1, Ordering::AcqRel);
            }
        }
        let _active = Active;
        let _ = sender.send(image(&hwnd));
    });
    receiver
        .recv_timeout(Duration::from_secs(5))
        .map_err(|_| "Window capture timed out".to_owned())?
}
