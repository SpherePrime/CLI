#![cfg_attr(not(windows), allow(dead_code))]

use crate::computer::types::WindowInfo;

pub struct Capture {
    pub info: WindowInfo,
    pub png: Vec<u8>,
    pub width: u32,
    pub height: u32,
}

pub fn capture(_hwnd: &str) -> Result<Capture, String> {
    Err("Desktop window screenshots require Windows".to_owned())
}
