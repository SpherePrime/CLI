use std::collections::HashMap;
use std::ffi::{c_char, c_int, c_void, CStr, CString};
use std::path::{Path, PathBuf};
use std::sync::Arc;

use anyhow::{anyhow, Context, Result};
use libloading::Library;
use serde::{Deserialize, Serialize};
use tokio::sync::Mutex;

use crate::manifest::PluginDescriptor;

pub const ABI_VERSION: c_int = 1;

type ManifestFn = unsafe extern "C" fn() -> *const c_char;
type ToolNamesFn = unsafe extern "C" fn() -> *const c_char;
type RunToolFn = unsafe extern "C" fn(*const c_char, *const c_char) -> *const c_char;
type InitFn = unsafe extern "C" fn(*mut c_void) -> c_int;
type ShutdownFn = unsafe extern "C" fn() -> c_int;

#[derive(Clone, Serialize, Deserialize)]
struct ToolResult {
    content: Option<String>,
    error: Option<String>,
}

pub struct NativePlugin {
    library: Library,
    pub descriptor: PluginDescriptor,
    pub tool_names: Vec<String>,
    lock: Arc<Mutex<()>>,
}

unsafe fn lookup<'lib, T>(
    library: &'lib Library,
    name: &[u8],
) -> Result<libloading::Symbol<'lib, T>> {
    library
        .get(name)
        .with_context(|| format!("missing symbol '{}'", String::from_utf8_lossy(name)))
}

unsafe fn cstr_copy(ptr: *const c_char) -> Result<String> {
    CStr::from_ptr(ptr)
        .to_str()
        .map(str::to_string)
        .map_err(|error| anyhow!("invalid plugin string: {error}"))
}

impl NativePlugin {
    pub fn load(file: &Path) -> Result<Self> {
        unsafe {
            let library = Library::new(file)
                .with_context(|| format!("opening plugin library {}", file.display()))?;

            let abi_version: libloading::Symbol<'_, unsafe extern "C" fn() -> c_int> =
                lookup(&library, b"agent_plugin_abi_version\0")?;
            if abi_version() != ABI_VERSION {
                return Err(anyhow!("plugin ABI version mismatch in {}", file.display()));
            }

            let manifest_fn: libloading::Symbol<'_, ManifestFn> =
                lookup(&library, b"agent_plugin_manifest\0")?;
            let manifest = cstr_copy(manifest_fn())?;
            let descriptor: PluginDescriptor = toml::from_str(&manifest)
                .with_context(|| format!("parsing manifest of {}", file.display()))?;
            if descriptor.name.is_empty() {
                return Err(anyhow!("plugin manifest has empty name"));
            }

            let tool_names_fn: libloading::Symbol<'_, ToolNamesFn> =
                lookup(&library, b"agent_plugin_tool_names\0")?;
            let tool_names: Vec<String> = serde_json::from_str(&cstr_copy(tool_names_fn())?)
                .context("parsing plugin tool names")?;

            Ok(Self {
                library,
                descriptor,
                tool_names,
                lock: Arc::new(Mutex::new(())),
            })
        }
    }

    pub fn name(&self) -> &str {
        &self.descriptor.name
    }

    pub fn enabled(&self, config_plugins: &HashMap<String, bool>) -> bool {
        config_plugins
            .get(&self.descriptor.name)
            .copied()
            .unwrap_or(true)
    }

    pub async fn initialize(&self) -> Result<()> {
        let _guard = self.lock.lock().await;
        unsafe {
            if let Ok(init) = lookup::<InitFn>(&self.library, b"agent_plugin_init\0") {
                let code = init(std::ptr::null_mut());
                if code != 0 {
                    return Err(anyhow!("plugin init failed with code {code}"));
                }
            }
        }
        Ok(())
    }

    pub async fn shutdown(&self) {
        let _guard = self.lock.lock().await;
        unsafe {
            if let Ok(shutdown) = lookup::<ShutdownFn>(&self.library, b"agent_plugin_shutdown\0") {
                shutdown();
            }
        }
    }

    pub async fn run_tool(&self, name: &str, args_json: &str) -> Result<String> {
        let args = CString::new(args_json).map_err(|_| anyhow!("tool args contain a NUL byte"))?;
        let name_c = CString::new(name).map_err(|_| anyhow!("tool name contains a NUL byte"))?;

        let raw = {
            let _guard = self.lock.lock().await;
            unsafe {
                let run_tool: libloading::Symbol<'_, RunToolFn> =
                    lookup(&self.library, b"agent_plugin_run_tool\0")?;
                let ptr = run_tool(name_c.as_ptr(), args.as_ptr());
                if ptr.is_null() {
                    return Err(anyhow!("plugin returned a null result for {name}"));
                }
                cstr_copy(ptr)?
            }
        };

        let parsed: ToolResult = serde_json::from_str(&raw)
            .with_context(|| format!("invalid plugin result for {name}"))?;
        if let Some(error) = parsed.error {
            return Err(anyhow!(error));
        }
        Ok(parsed.content.unwrap_or_default())
    }
}

pub const LIBRARY_EXTENSIONS: [&str; 3] = ["dll", "so", "dylib"];

pub fn find_library(dir: &Path, name: &str) -> Option<PathBuf> {
    for extension in LIBRARY_EXTENSIONS {
        for candidate in [
            dir.join(format!("agent_plugin_{name}.{extension}")),
            dir.join(format!("libagent_plugin_{name}.{extension}")),
            dir.join(format!("lib{name}.{extension}")),
            dir.join(format!("{name}.{extension}")),
            dir.join(format!("plugin.{extension}")),
        ] {
            if candidate.exists() {
                return Some(candidate);
            }
        }
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn missing_library_reports_error() {
        let result = NativePlugin::load(Path::new("does-not-exist.dll"));
        assert!(result.is_err());
    }

    #[test]
    fn tool_result_parsing_handles_error() {
        let parsed: ToolResult = serde_json::from_str(r#"{"error":"boom"}"#).unwrap();
        assert!(parsed.error.is_some());
        assert!(parsed.content.is_none());
    }

    #[test]
    fn find_library_checks_plugin_variants() {
        let dir = std::env::temp_dir();
        let probe = dir.join("agent_plugin_demo.dll");
        if probe.exists() {
            assert_eq!(find_library(&dir, "demo"), Some(probe));
        }
    }
}
