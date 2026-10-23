use std::sync::OnceLock;
use unicode_normalization::UnicodeNormalization;

pub fn detect_language(text: &str) -> &'static str {
    let text = text.to_lowercase();
    if text
        .chars()
        .any(|character| matches!(character, 'і' | 'ї' | 'є' | 'ґ'))
    {
        return "uk";
    }
    let cyrillic = text
        .chars()
        .filter(|character| matches!(character, 'а'..='я' | 'ё'))
        .count();
    let latin = text.chars().filter(char::is_ascii_alphabetic).count();
    if cyrillic >= latin {
        "ru"
    } else {
        "en"
    }
}

pub fn prepare(text: &str, language: &str) -> String {
    static PICTURES: OnceLock<regex::Regex> = OnceLock::new();
    let normalized = text.nfkd().collect::<String>();
    let mut value = PICTURES
        .get_or_init(|| regex::Regex::new(r"\p{Extended_Pictographic}").unwrap())
        .replace_all(&normalized, "")
        .into_owned();
    for (from, to) in [
        ('–', "-"),
        ('‑', "-"),
        ('—', "-"),
        ('_', " "),
        ('“', "\""),
        ('”', "\""),
        ('‘', "'"),
        ('’', "'"),
        ('´', "'"),
        ('`', "'"),
        ('[', " "),
        (']', " "),
        ('|', " "),
        ('/', " "),
        ('#', " "),
        ('→', " "),
        ('←', " "),
        ('@', " at "),
    ] {
        value = value.replace(from, to);
    }
    value.retain(|character| !matches!(character, '♥' | '☆' | '♡' | '©' | '\\'));
    static PUNCTUATION: OnceLock<regex::Regex> = OnceLock::new();
    value = PUNCTUATION
        .get_or_init(|| regex::Regex::new(r" ([,.!?;:'])").unwrap())
        .replace_all(&value, "$1")
        .into_owned();
    static QUOTES: OnceLock<regex::Regex> = OnceLock::new();
    value = QUOTES
        .get_or_init(|| regex::Regex::new(r#""{2,}|'{2,}"#).unwrap())
        .replace_all(&value, |captures: &regex::Captures| {
            captures[0].chars().next().unwrap().to_string()
        })
        .into_owned();
    value = value.split_whitespace().collect::<Vec<_>>().join(" ");
    if !value.ends_with([
        '.', '!', '?', ';', ':', ',', '\'', '"', ')', ']', '}', '…', '»',
    ]) {
        value.push('.');
    }
    format!("<{language}>{value}</{language}>")
}

pub fn text_codes(text: &str, indexer: &[i64]) -> Vec<i64> {
    text.chars()
        .map(|character| {
            let scalar = character as u32;
            let unit = if scalar > 0xffff {
                ((scalar - 0x10000) >> 10) + 0xd800
            } else {
                scalar
            };
            indexer.get(unit as usize).copied().unwrap_or(0)
        })
        .collect()
}
