use crate::error::{HostError, HostResult};
use base64::Engine;
use serde_json::{json, Value};
use std::{collections::HashMap, path::Path};

#[derive(Default)]
pub struct Attachments {
    items: HashMap<String, Value>,
    bytes: usize,
}

impl Attachments {
    fn prepare(
        name: &str,
        path: &str,
        bytes: &[u8],
        image_only: bool,
    ) -> HostResult<(String, Value, Value)> {
        if bytes.is_empty() || bytes.len() > 20 * 1024 * 1024 {
            return Err(HostError::new(
                "ATTACHMENT_LIMIT",
                "Attachment must be between 1 byte and 20 MiB",
            ));
        }
        let image = if bytes.starts_with(&[137, 80, 78, 71, 13, 10, 26, 10]) {
            Some("image/png")
        } else if bytes.starts_with(&[255, 216, 255]) {
            Some("image/jpeg")
        } else if bytes.starts_with(b"GIF87a") || bytes.starts_with(b"GIF89a") {
            Some("image/gif")
        } else if bytes.starts_with(b"RIFF") && bytes.get(8..12) == Some(b"WEBP") {
            Some("image/webp")
        } else {
            None
        };
        if image_only && image.is_none() {
            return Err(HostError::new("INVALID_IMAGE", "Unsupported image format"));
        }
        let mime = image.unwrap_or(if bytes.starts_with(b"%PDF-") {
            "application/pdf"
        } else {
            "text/plain"
        });
        let id = uuid::Uuid::new_v4().to_string();
        let name: String = name
            .chars()
            .take(200)
            .map(|character| {
                if character.is_control() || matches!(character, '/' | '\\') {
                    '_'
                } else {
                    character
                }
            })
            .collect();
        let encoded = base64::engine::general_purpose::STANDARD.encode(bytes);
        let stored = json!({"file_path":path,"file_name":name,"mime_type":mime,"content":encoded});
        let mut descriptor = json!({"id":id,"name":name,"size":bytes.len(),"mimeType":mime});
        if image.is_some() {
            descriptor["preview"] = json!(format!("data:{mime};base64,{encoded}"));
        }
        Ok((id, stored, descriptor))
    }

    pub fn import(&mut self, input: &Value) -> HostResult<Value> {
        let input = input
            .as_array()
            .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Expected image array"))?;
        if input.is_empty() || input.len() > 10 {
            return Err(HostError::new(
                "ATTACHMENT_LIMIT",
                "Import between 1 and 10 images",
            ));
        }
        let mut prepared = Vec::new();
        for image in input {
            let name = image["name"]
                .as_str()
                .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Missing attachment name"))?;
            let bytes = if let Some(encoded) = image["data"].as_str() {
                if encoded.len() > 28 * 1024 * 1024 {
                    return Err(HostError::new("ATTACHMENT_LIMIT", "Image exceeds 20 MiB"));
                }
                base64::engine::general_purpose::STANDARD
                    .decode(encoded)
                    .map_err(|_| HostError::new("INVALID_ARGUMENT", "Invalid base64 image"))?
            } else {
                let data = image["data"].as_array().ok_or_else(|| {
                    HostError::new(
                        "INVALID_ARGUMENT",
                        "Image data must be base64 or byte array",
                    )
                })?;
                if data.len() > 20 * 1024 * 1024 {
                    return Err(HostError::new("ATTACHMENT_LIMIT", "Image exceeds 20 MiB"));
                }
                data.iter()
                    .map(|byte| {
                        byte.as_u64()
                            .filter(|byte| *byte <= 255)
                            .map(|byte| byte as u8)
                            .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Invalid image byte"))
                    })
                    .collect::<HostResult<Vec<_>>>()?
            };
            prepared.push(Self::prepare(name, name, &bytes, true)?);
        }
        self.store(prepared)
    }

    pub fn files(&mut self, paths: &[std::path::PathBuf]) -> HostResult<Value> {
        let mut prepared = Vec::new();
        if paths.len() > 100 {
            return Err(HostError::new(
                "ATTACHMENT_LIMIT",
                "Maximum 100 attachments",
            ));
        }
        for path in paths {
            if path.metadata()?.len() > 20 * 1024 * 1024 {
                return Err(HostError::new(
                    "ATTACHMENT_LIMIT",
                    "Attachment exceeds 20 MiB",
                ));
            }
            let name = path
                .file_name()
                .unwrap_or(Path::new("attachment").as_os_str())
                .to_string_lossy();
            prepared.push(Self::prepare(
                &name,
                &path.to_string_lossy(),
                &std::fs::read(path)?,
                false,
            )?);
        }
        self.store(prepared)
    }

    pub fn image(&mut self, name: &str, bytes: &[u8]) -> HostResult<Value> {
        let image = Self::prepare(name, name, bytes, true)?;
        self.store(vec![image])
    }

    fn store(&mut self, prepared: Vec<(String, Value, Value)>) -> HostResult<Value> {
        let bytes = prepared
            .iter()
            .map(|(_, _, descriptor)| descriptor["size"].as_u64().unwrap() as usize)
            .sum::<usize>();
        if self.bytes + bytes > 200 * 1024 * 1024 {
            return Err(HostError::new(
                "ATTACHMENT_LIMIT",
                "Attachment store exceeds 200 MiB",
            ));
        }
        let mut output = Vec::new();
        for (id, stored, descriptor) in prepared {
            self.items.insert(id, stored);
            output.push(descriptor);
        }
        self.bytes += bytes;
        Ok(json!(output))
    }

    pub fn resolve(&self, ids: &Value) -> HostResult<Value> {
        let ids = ids
            .as_array()
            .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Expected attachment identifiers"))?;
        ids.iter()
            .map(|id| {
                id.as_str()
                    .and_then(|id| self.items.get(id))
                    .cloned()
                    .ok_or_else(|| {
                        HostError::new(
                            "ATTACHMENT_NOT_FOUND",
                            "Attachment expired; select it again",
                        )
                    })
            })
            .collect::<HostResult<Vec<_>>>()
            .map(Value::Array)
    }

    pub fn release(&mut self, ids: &Value) -> HostResult<Value> {
        let ids = ids
            .as_array()
            .ok_or_else(|| HostError::new("INVALID_ARGUMENT", "Expected attachment identifiers"))?;
        for id in ids {
            if let Some(stored) = id.as_str().and_then(|id| self.items.remove(id)) {
                self.bytes = self.bytes.saturating_sub(
                    stored["content"]
                        .as_str()
                        .and_then(|content| {
                            base64::engine::general_purpose::STANDARD
                                .decode(content)
                                .ok()
                        })
                        .map(|bytes| bytes.len())
                        .unwrap_or(0),
                );
            }
        }
        Ok(Value::Null)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn attachments_validate_resolve_and_expire() {
        let mut store = Attachments::default();
        assert!(store
            .import(&json!([{"name":"bad.png","data":"aGVsbG8="}]))
            .is_err());
        let imported = store
            .image("preview.png", &[137, 80, 78, 71, 13, 10, 26, 10])
            .unwrap();
        let ids = json!([imported[0]["id"]]);
        let resolved = store.resolve(&ids).unwrap();
        assert_eq!(resolved[0]["mime_type"], "image/png");
        store.release(&ids).unwrap();
        assert!(store.resolve(&ids).is_err());
        assert_eq!(store.bytes, 0);
    }
}
