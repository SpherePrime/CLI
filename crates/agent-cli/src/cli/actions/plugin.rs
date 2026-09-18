use std::path::Path;
use std::process::ExitCode;

use agent_plugins::manifest::PluginDescriptor;
use agent_plugins::PluginManager;

use crate::cli::{load_config, plugins_dir, save_config, PluginAction};

pub fn run(action: &PluginAction) -> ExitCode {
    match action {
        PluginAction::List => run_list(),
        PluginAction::Install { source } => run_install(source),
        PluginAction::Remove { name } => run_remove(name),
        PluginAction::Enable { name } => run_toggle(name, true),
        PluginAction::Disable { name } => run_toggle(name, false),
        PluginAction::Inspect { name } => run_inspect(name),
        PluginAction::Generate { name } => run_generate(name),
    }
}

fn enabled_map() -> std::collections::HashMap<String, bool> {
    load_config().plugins
}

fn run_list() -> ExitCode {
    let enabled = enabled_map();
    let dirs = PluginManager::discover_from(&plugins_dir()).unwrap_or_default();
    if dirs.is_empty() {
        println!("(no plugins installed at {})", plugins_dir().display());
        return ExitCode::SUCCESS;
    }
    for dir in dirs {
        let name = dir
            .file_name()
            .and_then(|name| name.to_str())
            .unwrap_or("plugin");
        let enabled_state = enabled.get(name).copied().unwrap_or(true);
        match PluginDescriptor::load(&dir) {
            Ok(descriptor) => println!(
                "[{}] {} v{} {}",
                if enabled_state { "on" } else { "off" },
                descriptor.name,
                descriptor.version,
                descriptor.description
            ),
            Err(_) => println!(
                "[{}] {} (no manifest.toml)",
                if enabled_state { "on" } else { "off" },
                name
            ),
        }
    }
    ExitCode::SUCCESS
}

fn run_install(source: &str) -> ExitCode {
    let target_root = plugins_dir();
    if let Err(error) = std::fs::create_dir_all(&target_root) {
        eprintln!("failed to create {}: {error}", target_root.display());
        return ExitCode::FAILURE;
    }

    let source_dir = if is_git_url(source) {
        match clone_git(source) {
            Some(dir) => dir,
            None => {
                eprintln!("failed to clone plugin source: {source}");
                return ExitCode::FAILURE;
            }
        }
    } else {
        std::path::PathBuf::from(source)
    };

    let name = source_dir
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("plugin");
    let target = target_root.join(name);
    if target.exists() {
        eprintln!("plugin '{name}' already exists at {}", target.display());
        return ExitCode::FAILURE;
    }
    if let Err(error) = copy_dir(&source_dir, &target) {
        eprintln!("failed to copy plugin: {error}");
        return ExitCode::FAILURE;
    }
    if !target.join("manifest.toml").exists() {
        let manifest = format!("name = \"{name}\"\nversion = \"0.1.0\"\ndescription = \"\"\n");
        if let Err(error) = std::fs::write(target.join("manifest.toml"), manifest) {
            eprintln!("failed to write manifest: {error}");
            return ExitCode::FAILURE;
        }
    }
    println!("installed plugin '{name}' at {}", target.display());
    ExitCode::SUCCESS
}

fn run_remove(name: &str) -> ExitCode {
    let path = plugins_dir().join(name);
    if !path.is_dir() {
        eprintln!("plugin '{name}' not found");
        return ExitCode::FAILURE;
    }
    if let Err(error) = std::fs::remove_dir_all(&path) {
        eprintln!("failed to remove plugin: {error}");
        return ExitCode::FAILURE;
    }
    let mut config = load_config();
    config.plugins.remove(name);
    let _ = save_config(&config);
    println!("removed plugin at {}", path.display());
    ExitCode::SUCCESS
}

fn run_toggle(name: &str, enabled: bool) -> ExitCode {
    let dir = plugins_dir().join(name);
    if !dir.is_dir() {
        eprintln!("plugin '{name}' not found at {}", dir.display());
        return ExitCode::FAILURE;
    }
    let mut config = load_config();
    config.plugins.insert(name.to_string(), enabled);
    match save_config(&config) {
        Ok(path) => {
            println!(
                "{} plugin '{name}' (saved {})",
                if enabled { "enabled" } else { "disabled" },
                path.display()
            );
            ExitCode::SUCCESS
        }
        Err(error) => {
            eprintln!("failed to save config: {error}");
            ExitCode::FAILURE
        }
    }
}

fn run_inspect(name: &str) -> ExitCode {
    let dir = plugins_dir().join(name);
    if !dir.is_dir() {
        eprintln!("plugin '{name}' not found at {}", dir.display());
        return ExitCode::FAILURE;
    }
    println!("path: {}", dir.display());
    match PluginDescriptor::load(&dir) {
        Ok(descriptor) => {
            println!(
                "name: {}\nversion: {}\ndescription: {}",
                descriptor.name, descriptor.version, descriptor.description
            );
            if let Some(entry) = &descriptor.entry {
                println!("entry: {entry}");
            }
            if !descriptor.tools.is_empty() {
                println!("tools: {}", descriptor.tools.join(", "));
            }
        }
        Err(error) => {
            println!("no manifest.toml ({error})");
        }
    }
    if let Ok(entries) = std::fs::read_dir(&dir) {
        let files: Vec<String> = entries
            .flatten()
            .map(|entry| entry.file_name().to_string_lossy().to_string())
            .collect();
        println!("files: {}", files.join(", "));
    }
    ExitCode::SUCCESS
}

fn run_generate(name: &str) -> ExitCode {
    let name = normalize_plugin_name(name);
    if name.is_empty() {
        eprintln!("plugin name must contain at least one letter or digit");
        return ExitCode::FAILURE;
    }
    let target = plugins_dir().join(&name);
    if target.exists() {
        eprintln!("plugin '{name}' already exists at {}", target.display());
        return ExitCode::FAILURE;
    }
    let src = target.join("src");
    if let Err(error) = std::fs::create_dir_all(&src) {
        eprintln!("failed to create {}: {error}", target.display());
        return ExitCode::FAILURE;
    }

    let manifest = format!(
        "name = \"{name}\"\nversion = \"0.1.0\"\ndescription = \"Generated native plugin\"\n"
    );
    let cargo = format!(
        r#"[package]
name = "{name}"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
serde_json = "1"

[profile.release]
opt-level = 2
"#
    );
    let lib_source = format!(
        r#"use std::ffi::CStr;
use std::os::raw::{{c_char, c_int, c_void}};
use std::sync::Mutex;

static RESULT_BUF: Mutex<Vec<u8>> = Mutex::new(Vec::new());

#[no_mangle]
pub extern "C" fn agent_plugin_abi_version() -> c_int {{
    1
}}

#[no_mangle]
pub extern "C" fn agent_plugin_manifest() -> *const c_char {{
    b"name = \"{name}\"\nversion = \"0.1.0\"\ndescription = \"Generated native plugin\"\n\0".as_ptr() as *const c_char
}}

#[no_mangle]
pub extern "C" fn agent_plugin_tool_names() -> *const c_char {{
    b"[\"echo\"]\0".as_ptr() as *const c_char
}}

#[no_mangle]
pub extern "C" fn agent_plugin_run_tool(
    name: *const c_char,
    args_json: *const c_char,
) -> *const c_char {{
    let tool = unsafe {{ CStr::from_ptr(name) }}.to_string_lossy();
    if tool != "echo" {{
        return b"{{\"error\":\"unknown tool\"}}\0".as_ptr() as *const c_char;
    }}
    let args = unsafe {{ CStr::from_ptr(args_json) }}.to_string_lossy();
    let content = serde_json::json!({{"content": format!("echo: {{args}}")}}).to_string();
    let mut buf = RESULT_BUF.lock().unwrap();
    buf.clear();
    buf.extend_from_slice(content.as_bytes());
    buf.push(0);
    buf.as_ptr() as *const c_char
}}

#[no_mangle]
pub extern "C" fn agent_plugin_init(_ctx: *mut c_void) -> c_int {{
    0
}}

#[no_mangle]
pub extern "C" fn agent_plugin_shutdown() -> c_int {{
    0
}}
"#,
        name = name
    );

    for (filename, content) in [
        ("manifest.toml".to_string(), manifest),
        ("Cargo.toml".to_string(), cargo),
        ("src/lib.rs".to_string(), lib_source),
    ] {
        let path = target.join(&filename);
        if let Err(error) = std::fs::write(&path, content) {
            eprintln!("failed to write {}: {error}", path.display());
            return ExitCode::FAILURE;
        }
    }

    println!("generated plugin project at {}", target.display());
    println!(
        "build it with: cargo build --manifest-path {}\\Cargo.toml --release",
        target.display()
    );
    println!(
        "then copy the built library into {} and enable it with: agent plugin enable {name}",
        target.display()
    );
    ExitCode::SUCCESS
}

fn normalize_plugin_name(name: &str) -> String {
    name.chars()
        .filter(|character| {
            character.is_ascii_alphanumeric() || *character == '_' || *character == '-'
        })
        .collect()
}

fn is_git_url(source: &str) -> bool {
    source.starts_with("https://")
        || source.starts_with("http://")
        || source.starts_with("git@")
        || source.starts_with("ssh://")
}

fn clone_git(source: &str) -> Option<std::path::PathBuf> {
    let temp = std::env::temp_dir().join(format!("agent-plugin-{}", uuid::Uuid::new_v4()));
    let output = std::process::Command::new("git")
        .args(["clone", "--depth", "1", source])
        .arg(&temp)
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    Some(temp)
}

fn copy_dir(source: &Path, target: &Path) -> std::io::Result<()> {
    std::fs::create_dir_all(target)?;
    for entry in std::fs::read_dir(source)? {
        let entry = entry?;
        let from = entry.path();
        let to = target.join(entry.file_name());
        if from.is_dir() {
            copy_dir(&from, &to)?;
        } else if !entry
            .path()
            .file_name()
            .and_then(|name| name.to_str())
            .map(|name| name.starts_with(".git"))
            .unwrap_or(false)
        {
            std::fs::copy(&from, &to)?;
        }
    }
    Ok(())
}
