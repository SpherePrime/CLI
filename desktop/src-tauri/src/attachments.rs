use crate::error::{HostError, HostResult};
use base64::Engine;
use serde_json::{json, Value};
use std::{collections::HashMap, path::Path, path::PathBuf};

pub struct Attachments {
    root: PathBuf,
    items: HashMap<String, Stored>,
    bytes: usize,
}

struct Stored {
    path: PathBuf,
    file_name: String,
    mime_type: String,
    size: usize,
}

impl Attachments {
    pub fn new(root: PathBuf) -> HostResult<Self> {
        std::fs::create_dir_all(&root)?;
        let root = dunce::canonicalize(&root)?;
        for entry in std::fs::read_dir(&root)?.flatten() {
            let _ = std::fs::remove_file(entry.path());
        }
        Ok(Self {
            root,
            items: HashMap::new(),
            bytes: 0,
        })
    }

    fn prepare(
        name: &str,
        path: &str,
        bytes: &[u8],
        image_only: bool,
    ) -> HostResult<(String, Vec<u8>, String, String, Value)> {
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
        let mut descriptor = json!({"id":id,"name":name,"size":bytes.len(),"mimeType":mime});
        if image.is_some() && bytes.len() <= 8 * 1024 * 1024 {
            let encoded = base64::engine::general_purpose::STANDARD.encode(bytes);
            descriptor["preview"] = json!(format!("data:{mime};base64,{encoded}"));
        }
        Ok((id, bytes.to_vec(), name, mime.to_owned(), descriptor))
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

    fn store(
        &mut self,
        prepared: Vec<(String, Vec<u8>, String, String, Value)>,
    ) -> HostResult<Value> {
        let bytes = prepared
            .iter()
            .map(|(_, bytes, _, _, _)| bytes.len())
            .sum::<usize>();
        if self.bytes + bytes > 200 * 1024 * 1024 {
            return Err(HostError::new(
                "ATTACHMENT_LIMIT",
                "Attachment store exceeds 200 MiB",
            ));
        }
        let mut output = Vec::new();
        for (id, content, file_name, mime_type, descriptor) in prepared {
            let path = self.root.join(format!("{id}.bin"));
            std::fs::write(&path, &content)?;
            self.items.insert(
                id.clone(),
                Stored {
                    path,
                    file_name,
                    mime_type,
                    size: content.len(),
                },
            );
            output.push(descriptor);
            drop(content);
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
                let stored = id
                    .as_str()
                    .and_then(|id| self.items.get(id))
                    .ok_or_else(|| {
                        HostError::new(
                            "ATTACHMENT_NOT_FOUND",
                            "Attachment expired; select it again",
                        )
                    })?;
                Ok(json!({
                    "file_path": stored.path.to_string_lossy(),
                    "file_name": stored.file_name,
                    "mime_type": stored.mime_type,
                }))
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
                self.bytes = self.bytes.saturating_sub(stored.size);
                let _ = std::fs::remove_file(&stored.path);
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
        let directory = tempfile::tempdir().unwrap();
        let mut store = Attachments::new(directory.path().join("attachments").to_path_buf()).unwrap();
        assert!(store
            .import(&json!([{"name":"bad.png","data":"aGVsbG8="}]))
            .is_err());
        let imported = store
            .image("preview.png", &[137, 80, 78, 71, 13, 10, 26, 10])
            .unwrap();
        let ids = json!([imported[0]["id"]]);
        let resolved = store.resolve(&ids).unwrap();
        assert_eq!(resolved[0]["mime_type"], "image/png");
        assert!(resolved[0]["content"].is_null());
        let path = resolved[0]["file_path"].as_str().unwrap();
        assert!(Path::new(path).exists());
        assert!(path.contains("attachments") || Path::new(path).is_absolute());
        store.release(&ids).unwrap();
        assert!(store.resolve(&ids).is_err());
        assert_eq!(store.bytes, 0);
        assert!(!Path::new(path).exists());
    }

    #[test]
    fn stale_files_are_cleared_on_startup() {
        let directory = tempfile::tempdir().unwrap();
        let root = directory.path().join("attachments");
        std::fs::create_dir_all(&root).unwrap();
        std::fs::write(root.join("stale.bin"), b"leftover").unwrap();
        Attachments::new(root.clone()).unwrap();
        assert!(std::fs::read_dir(&root).unwrap().next().is_none());
    }

    #[test]
    fn resolved_references_stay_inside_the_private_root() {
        let directory = tempfile::tempdir().unwrap();
        let mut store =
            Attachments::new(directory.path().join("attachments").to_path_buf()).unwrap();
        let imported = store.image("pic.png", &[137, 80, 78, 71, 13, 10, 26, 10]).unwrap();
        let resolved = store.resolve(&json!([imported[0]["id"]])).unwrap();
        let path = resolved[0]["file_path"].as_str().unwrap();
        assert!(path.starts_with(
            dunce::canonicalize(directory.path().join("attachments"))
                .unwrap()
                .to_string_lossy()
                .as_ref()
        ));
    }
}
