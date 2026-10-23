pub const OUTPUT_LIMIT: usize = 1024 * 1024;

pub struct StartupCursor {
    pub active: bool,
    pending: String,
}

impl StartupCursor {
    pub fn new(active: bool) -> Self {
        Self {
            active,
            pending: String::new(),
        }
    }
    pub fn consume(&mut self, text: &str) -> (String, bool) {
        if !self.active {
            return (text.into(), false);
        }
        self.pending.push_str(text);
        let query = "\u{1b}[6n";
        if self.pending.starts_with(query) {
            self.active = false;
            return (self.pending.split_off(query.len()), true);
        }
        if query.starts_with(&self.pending) {
            return (String::new(), false);
        }
        self.active = false;
        (std::mem::take(&mut self.pending), false)
    }
}

#[derive(Default)]
pub struct TerminalOutput {
    text: String,
    pub truncated: bool,
}

impl TerminalOutput {
    pub fn append(&mut self, text: &str) {
        self.text.push_str(text);
        if self.text.len() > OUTPUT_LIMIT {
            let mut start = self.text.len() - OUTPUT_LIMIT;
            while !self.text.is_char_boundary(start) {
                start += 1;
            }
            self.text.drain(..start);
            self.truncated = true;
        }
    }

    pub fn read(&self) -> &str {
        &self.text
    }
}

#[derive(Default)]
pub struct Utf8Decoder {
    pending: Vec<u8>,
}

impl Utf8Decoder {
    pub fn decode(&mut self, bytes: &[u8], final_chunk: bool) -> String {
        self.pending.extend_from_slice(bytes);
        let mut text = String::new();
        loop {
            match std::str::from_utf8(&self.pending) {
                Ok(value) => {
                    text.push_str(value);
                    self.pending.clear();
                    break;
                }
                Err(error) => {
                    let valid = error.valid_up_to();
                    text.push_str(std::str::from_utf8(&self.pending[..valid]).unwrap());
                    match error.error_len() {
                        Some(length) => {
                            text.push('\u{fffd}');
                            self.pending.drain(..valid + length);
                        }
                        None => {
                            self.pending.drain(..valid);
                            if final_chunk {
                                text.push_str(&String::from_utf8_lossy(&self.pending));
                                self.pending.clear();
                            }
                            break;
                        }
                    }
                }
            }
        }
        text
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn tail_is_bounded_at_unicode_boundaries() {
        let mut output = TerminalOutput::default();
        output.append(&"я".repeat(OUTPUT_LIMIT));
        assert!(output.truncated);
        assert!(output.read().len() <= OUTPUT_LIMIT);
        assert!(output.read().chars().all(|character| character == 'я'));
    }

    #[test]
    fn decoder_preserves_split_utf8() {
        let mut decoder = Utf8Decoder::default();
        let bytes = "Привет 🦀".as_bytes();
        let mut output = String::new();
        for byte in bytes {
            output.push_str(&decoder.decode(&[*byte], false));
        }
        output.push_str(&decoder.decode(&[], true));
        assert_eq!(output, "Привет 🦀");
    }
}
