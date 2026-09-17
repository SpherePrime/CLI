use std::time::{Duration, Instant};

pub struct Spinner {
    frames: Vec<char>,
    index: usize,
    last_tick: Instant,
    frame_duration: Duration,
}

impl Spinner {
    pub fn new() -> Self {
        Self {
            frames: vec!['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'],
            index: 0,
            last_tick: Instant::now(),
            frame_duration: Duration::from_millis(80),
        }
    }

    pub fn tick(&mut self) {
        if self.last_tick.elapsed() >= self.frame_duration {
            self.index = (self.index + 1) % self.frames.len();
            self.last_tick = Instant::now();
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
