use std::path::Path;

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct PluginDescriptor {
    pub name: String,
    #[serde(default)]
    pub version: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub entry: Option<String>,
    #[serde(default)]
    pub tools: Vec<String>,
}

impl PluginDescriptor {
    pub fn load(dir: &Path) -> Result<Self> {
        let path = dir.join("manifest.toml");
        let content = std::fs::read_to_string(&path)
            .with_context(|| format!("reading {}", path.display()))?;
        toml::from_str(&content).with_context(|| format!("parsing {}", path.display()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn manifest_loads_and_defaults() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(
            tmp.path().join("manifest.toml"),
            "name = \"demo\"\ndescription = \"a demo plugin\"\n",
        )
        .unwrap();
        let descriptor = PluginDescriptor::load(tmp.path()).unwrap();
        assert_eq!(descriptor.name, "demo");
        assert_eq!(descriptor.version, "");
        assert_eq!(descriptor.tools, Vec::<String>::new());
    }

    #[test]
    fn missing_manifest_fails() {
        let tmp = tempfile::tempdir().unwrap();
        assert!(PluginDescriptor::load(tmp.path()).is_err());
    }
}
