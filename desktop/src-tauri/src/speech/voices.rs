use crate::error::{HostError, HostResult};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    fs,
    io::Write,
    path::{Path, PathBuf},
};

#[derive(Clone, Serialize)]
pub struct Voice {
    pub id: String,
    pub name: String,
    pub custom: bool,
}

#[derive(Clone)]
pub struct StyleTensor {
    pub dims: Vec<i64>,
    pub data: Vec<f32>,
}
#[derive(Clone)]
pub struct Style {
    pub ttl: StyleTensor,
    pub dp: StyleTensor,
}

fn directory(model: &Path) -> PathBuf {
    model.parent().unwrap_or(model).join("voices")
}

pub fn list(model: &Path) -> HostResult<Vec<Voice>> {
    let mut voices = vec![
        Voice {
            id: "F1".into(),
            name: "Женский 1".into(),
            custom: false,
        },
        Voice {
            id: "F2".into(),
            name: "Женский 2".into(),
            custom: false,
        },
        Voice {
            id: "M1".into(),
            name: "Мужской 1".into(),
            custom: false,
        },
        Voice {
            id: "M2".into(),
            name: "Мужской 2".into(),
            custom: false,
        },
    ];
    let entries = match fs::read_dir(directory(model)) {
        Ok(entries) => entries,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(voices),
        Err(error) => return Err(error.into()),
    };
    let mut paths = entries.collect::<Result<Vec<_>, _>>()?;
    paths.sort_by_key(|entry| entry.file_name());
    for entry in paths {
        let path = entry.path();
        if path.extension().and_then(|extension| extension.to_str()) != Some("json") {
            continue;
        }
        let Some(id) = path
            .file_stem()
            .and_then(|id| id.to_str())
            .filter(|id| valid_id(id))
        else {
            continue;
        };
        let meta = fs::read(path.with_extension("meta"))
            .ok()
            .and_then(|bytes| serde_json::from_slice::<Value>(&bytes).ok());
        let name = meta
            .as_ref()
            .and_then(|value| value["name"].as_str())
            .filter(|name| !name.is_empty())
            .unwrap_or(id)
            .to_string();
        voices.push(Voice {
            id: format!("custom:{id}"),
            name,
            custom: true,
        });
    }
    Ok(voices)
}

pub fn read(model: &Path, voice: &str) -> HostResult<Style> {
    let path = if let Some(id) = voice.strip_prefix("custom:") {
        if !valid_id(id) {
            return Err(invalid());
        }
        directory(model).join(format!("{id}.json"))
    } else {
        if !matches!(voice, "F1" | "F2" | "M1" | "M2") {
            return Err(invalid());
        }
        model.join("voice_styles").join(format!("{voice}.json"))
    };
    let value = read_value(&path)?;
    parse_style(&value)
}

pub fn import(model: &Path, path: &Path) -> HostResult<Voice> {
    let value = read_value(path)?;
    parse_style(&value)?;
    let name: String = path
        .file_stem()
        .unwrap_or_default()
        .to_string_lossy()
        .chars()
        .take(60)
        .collect();
    let id = slugify(&name);
    let root = directory(model);
    fs::create_dir_all(&root)?;
    atomic_json(
        &root.join(format!("{id}.json")),
        &json!({"style_ttl":value["style_ttl"],"style_dp":value["style_dp"]}),
    )?;
    atomic_json(&root.join(format!("{id}.meta")), &json!({"name":name}))?;
    Ok(Voice {
        id: format!("custom:{id}"),
        name,
        custom: true,
    })
}

pub fn remove(model: &Path, voice: &str) -> HostResult<()> {
    let id = voice
        .strip_prefix("custom:")
        .filter(|id| valid_id(id))
        .ok_or_else(|| HostError::new("SPEECH_VOICE", "Встроенный голос удалить нельзя"))?;
    for extension in ["json", "meta"] {
        match fs::remove_file(directory(model).join(format!("{id}.{extension}"))) {
            Ok(()) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
            Err(error) => return Err(error.into()),
        }
    }
    Ok(())
}

pub fn slugify(name: &str) -> String {
    let mut slug = String::new();
    let mut separator = false;
    for character in name.to_lowercase().chars() {
        if character.is_ascii_alphanumeric() || matches!(character, 'а'..='я' | 'ё' | '_' | '-')
        {
            if separator && !slug.is_empty() {
                slug.push('-');
            }
            separator = false;
            slug.push(character);
        } else {
            separator = true;
        }
    }
    let slug: String = slug
        .trim_matches('-')
        .chars()
        .take(40)
        .map(|character| {
            if character.is_ascii() {
                character.to_string()
            } else {
                let mut value = character as u32;
                let mut digits = Vec::new();
                while value > 0 {
                    digits.push(char::from_digit(value % 36, 36).unwrap());
                    value /= 36;
                }
                digits.into_iter().rev().collect()
            }
        })
        .collect();
    if slug.is_empty() {
        "voice".into()
    } else {
        slug
    }
}

pub fn parse_style(value: &Value) -> HostResult<Style> {
    Ok(Style {
        ttl: parse_tensor(&value["style_ttl"])?,
        dp: parse_tensor(&value["style_dp"])?,
    })
}

fn parse_tensor(value: &Value) -> HostResult<StyleTensor> {
    #[derive(Deserialize)]
    struct Dims {
        dims: Vec<i64>,
    }
    let dims = serde_json::from_value::<Dims>(value.clone())
        .map_err(|_| invalid())?
        .dims;
    if dims.len() != 3
        || dims[0] != 1
        || dims
            .iter()
            .any(|dimension| *dimension < 1 || *dimension > 4096)
    {
        return Err(invalid());
    }
    let expected = dims
        .iter()
        .try_fold(1usize, |product, dimension| {
            product.checked_mul(*dimension as usize)
        })
        .ok_or_else(invalid)?;
    if expected > 1024 * 1024 {
        return Err(invalid());
    }
    let mut data = Vec::with_capacity(expected);
    flatten(&value["data"], &mut data, 0, expected)?;
    if data.len() != expected {
        return Err(invalid());
    }
    Ok(StyleTensor { dims, data })
}

fn flatten(value: &Value, data: &mut Vec<f32>, depth: usize, limit: usize) -> HostResult<()> {
    if depth > 8 || data.len() > limit {
        return Err(invalid());
    }
    if let Some(values) = value.as_array() {
        for value in values {
            flatten(value, data, depth + 1, limit)?;
        }
    } else if let Some(value) = value
        .as_f64()
        .filter(|value| value.is_finite() && (*value as f32).is_finite())
    {
        data.push(value as f32);
    } else {
        return Err(invalid());
    }
    Ok(())
}

fn read_value(path: &Path) -> HostResult<Value> {
    if fs::metadata(path)?.len() > 5 * 1024 * 1024 {
        return Err(HostError::new("SPEECH_VOICE", "Файл голоса больше 5 МБ"));
    }
    Ok(serde_json::from_slice(&fs::read(path)?)?)
}
fn atomic_json(path: &Path, value: &Value) -> HostResult<()> {
    let mut file = tempfile::NamedTempFile::new_in(path.parent().unwrap())?;
    file.write_all(&serde_json::to_vec(value)?)?;
    file.as_file().sync_all()?;
    file.persist(path).map_err(|error| error.error)?;
    Ok(())
}
fn valid_id(id: &str) -> bool {
    !id.is_empty()
        && id.len() <= 80
        && id.chars().all(|character| {
            character.is_ascii_lowercase()
                || character.is_ascii_digit()
                || matches!(character, '_' | '-')
        })
}
fn invalid() -> HostError {
    HostError::new(
        "SPEECH_VOICE",
        "Это не стиль голоса (нужны style_ttl и style_dp)",
    )
}
