use std::io::Write;
use std::time::{Duration, Instant};

pub struct Spinner {
    frames: Vec<char>,
    index: usize,
    last_tick: Instant,
    frame_duration: Duration,
    active: bool,
}

impl Spinner {
    pub fn new() -> Self {
        Self {
            frames: vec!['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'],
            index: 0,
            last_tick: Instant::now(),
            frame_duration: Duration::from_millis(80),
            active: false,
        }
    }

    pub fn start(&mut self) {
        self.active = true;
        self.index = 0;
        self.last_tick = Instant::now();
        print!("\r{}\x1b[0m", self.current());
        let _ = std::io::stdout().flush();
    }

    pub fn stop(&mut self) {
        if self.active {
            print!("\r\x1b[2K");
            let _ = std::io::stdout().flush();
        }
        self.active = false;
    }

    pub fn tick(&mut self) {
        if !self.active {
            return;
        }
        if self.last_tick.elapsed() >= self.frame_duration {
            self.index = (self.index + 1) % self.frames.len();
            self.last_tick = Instant::now();
            print!("\r{}", self.current());
            let _ = std::io::stdout().flush();
        }
    }

    pub fn current(&self) -> char {
        self.frames[self.index]
    }
}

impl Default for Spinner {
    fn default() -> Self {
        Self::new()
    }
}
