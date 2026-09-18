use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use async_trait::async_trait;
use serde_json::{json, Value};

use crate::builtin::exec::run_shell_command;
use crate::builtin::paths::{display, truncate};
use crate::executor::{ToolExecutionContext, ToolExecutor, ToolOutput};

const MAX_OUTPUT: usize = 262_144;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Ecosystem {
    Node,
    Rust,
    Python,
    Go,
}

impl Ecosystem {
    pub fn label(self) -> &'static str {
        match self {
            Ecosystem::Node => "npm",
            Ecosystem::Rust => "cargo",
            Ecosystem::Python => "pip",
            Ecosystem::Go => "go",
        }
    }
}

struct Manifest {
    path: PathBuf,
    ecosystem: Ecosystem,
}

fn detect_manifest(cwd: &Path) -> Option<Manifest> {
    for (name, ecosystem) in [
        ("package.json", Ecosystem::Node),
        ("Cargo.toml", Ecosystem::Rust),
        ("pyproject.toml", Ecosystem::Python),
        ("requirements.txt", Ecosystem::Python),
        ("go.mod", Ecosystem::Go),
    ] {
        let candidate = cwd.join(name);
        if candidate.is_file() {
            return Some(Manifest {
                path: candidate,
                ecosystem,
            });
        }
    }
    None
}

fn parse_dependencies(manifest: &Manifest) -> Vec<(String, String)> {
    let raw = match std::fs::read_to_string(&manifest.path) {
        Ok(raw) => raw,
        Err(_) => return Vec::new(),
    };
    match manifest.ecosystem {
        Ecosystem::Node => parse_package_json(&raw),
        Ecosystem::Rust => parse_cargo_toml(&raw),
        Ecosystem::Python => parse_python_manifests(&raw, &manifest.path),
        Ecosystem::Go => parse_go_mod(&raw),
    }
}

fn parse_package_json(raw: &str) -> Vec<(String, String)> {
    let Ok(value) = serde_json::from_str::<Value>(raw) else {
        return Vec::new();
    };
    let mut out = Vec::new();
    for section in ["dependencies", "devDependencies", "optionalDependencies"] {
        if let Some(table) = value.get(section).and_then(|v| v.as_object()) {
            for (name, version) in table {
                if let Some(version) = version.as_str() {
                    out.push((name.clone(), version.to_string()));
                }
            }
        }
    }
    out
}

fn parse_cargo_toml(raw: &str) -> Vec<(String, String)> {
    let mut out = Vec::new();
    let mut in_deps = false;
    for line in raw.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with('[') {
            in_deps =
                trimmed.starts_with("[dependencies") || trimmed.starts_with("[dev-dependencies");
            continue;
        }
        if !in_deps || trimmed.is_empty() || trimmed.starts_with('#') {
            continue;
        }
        if let Some((name, spec)) = trimmed.split_once('=') {
            let name = name.trim().trim_matches('"').trim();
            let spec = spec.trim();
            let version = if let Some((_, inner)) = spec.split_once('{') {
                let tokens: Vec<&str> = inner.split_whitespace().collect();
                let mut version = String::new();
                for window in tokens.windows(3) {
                    if window[0].starts_with("version") && window[1] == "=" {
                        version = window[2].trim().trim_matches(['"', ',']).to_string();
                        break;
                    }
                }
                version
            } else {
                spec.trim_matches('"').trim_matches(',').to_string()
            };
            out.push((name.to_string(), version));
        }
    }
    out
}

fn parse_python_manifests(raw: &str, path: &Path) -> Vec<(String, String)> {
    if path
        .file_name()
        .is_some_and(|name| name == "requirements.txt")
    {
        return raw
            .lines()
            .filter_map(|line| {
                let line = line.trim();
                if line.is_empty() || line.starts_with('#') {
                    return None;
                }
                let (name, spec) = line
                    .split_once("==")
                    .or_else(|| line.split_once('<'))
                    .or_else(|| line.split_once('>'))
                    .or_else(|| {
                        line.split_once('=')
                            .filter(|(_, rest)| !rest.starts_with('='))
                    })
                    .map(|(name, rest)| (name.trim(), rest.trim().to_string()))
                    .unwrap_or((line, "*".to_string()));
                let name = name
                    .split(['[', ';'])
                    .next()
                    .unwrap_or(name)
                    .trim()
                    .to_string();
                Some((name, spec))
            })
            .collect();
    }
    let mut out = Vec::new();
    let mut in_deps = false;
    for line in raw.lines() {
        let trimmed = line.trim();
        if let Some(rest) = trimmed.strip_prefix("dependencies") {
            in_deps = rest.trim_start().starts_with('=') && rest.contains('[');
        }
        if trimmed.starts_with('[') && !trimmed.starts_with("[project]") {
            in_deps = false;
        }
        if !in_deps {
            continue;
        }
        if let Some(inner) = trimmed
            .trim_end_matches(',')
            .strip_prefix('"')
            .and_then(|rest| rest.strip_suffix('"'))
        {
            let (name, spec) = inner
                .split_once(">=")
                .or_else(|| inner.split_once("=="))
                .or_else(|| inner.split_once('^'))
                .unwrap_or((inner, "*"));
            out.push((name.trim().to_string(), spec.to_string()));
        }
    }
    out
}

fn parse_go_mod(raw: &str) -> Vec<(String, String)> {
    let mut out = Vec::new();
    let mut in_require = false;
    for line in raw.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with("require") {
            in_require = true;
            if !trimmed.contains('(') {
                if let Some(rest) = trimmed.strip_prefix("require") {
                    let mut parts = rest.split_whitespace();
                    if let (Some(name), Some(version)) = (parts.next(), parts.next()) {
                        out.push((name.to_string(), version.to_string()));
                    }
                }
            }
            continue;
        }
        if trimmed == ")" {
            in_require = false;
            continue;
        }
        if in_require {
            let mut parts = trimmed.split_whitespace();
            if let (Some(name), Some(version)) = (parts.next(), parts.next()) {
                out.push((name.to_string(), version.to_string()));
            }
        }
    }
    out
}

fn detect_result(cwd: &Path) -> Value {
    match detect_manifest(cwd) {
        None => {
            json!({ "found": false, "message": "no package manifest found (package.json, Cargo.toml, pyproject.toml, requirements.txt, go.mod)" })
        }
        Some(manifest) => {
            let dependencies = parse_dependencies(&manifest);
            json!({
                "found": true,
                "ecosystem": manifest.ecosystem.label(),
                "manifest": display(&manifest.path),
                "dependencies_count": dependencies.len(),
                "dependencies": dependencies,
            })
        }
    }
}

async fn run_manager_command(
    ctx: &ToolExecutionContext,
    command: &str,
    max_secs: u64,
) -> Result<ToolOutput> {
    let program = command.split_whitespace().next().unwrap_or("tool");
    if !binary_available(program) {
        return Ok(ToolOutput::success(format!(
            "{program} is not installed; cannot run: {command}"
        )));
    }
    let mut output = run_shell_command(ctx, command, ctx.working_dir.clone(), max_secs).await?;
    let body = truncate(&output.content, MAX_OUTPUT);
    output.content = body;
    Ok(output)
}

fn binary_available(program: &str) -> bool {
    std::process::Command::new(program)
        .arg("--version")
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .status()
        .is_ok()
}

fn install_command(ecosystem: Ecosystem, package: &str, version: Option<&str>) -> String {
    match (ecosystem, version) {
        (Ecosystem::Node, Some(version)) => {
            format!("npm install {package}@{version}")
        }
        (Ecosystem::Node, None) => format!("npm install {package}"),
        (Ecosystem::Rust, Some(version)) => format!("cargo add {package}@{version}"),
        (Ecosystem::Rust, None) => format!("cargo add {package}"),
        (Ecosystem::Python, Some(version)) => format!("pip install {package}=={version}"),
        (Ecosystem::Python, None) => format!("pip install {package}"),
        (Ecosystem::Go, Some(version)) => format!("go get {package}@{version}"),
        (Ecosystem::Go, None) => format!("go get {package}"),
    }
}

pub struct DependencyTool;

#[async_trait]
impl ToolExecutor for DependencyTool {
    async fn execute(&self, args: Value, ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let action = args
            .get("action")
            .and_then(|v| v.as_str())
            .context("action is required")?;

        match action {
            "detect" => {
                let result = detect_result(&ctx.working_dir);
                Ok(ToolOutput::success(result.to_string()))
            }
            "outdated" => {
                let Some(manifest) = detect_manifest(&ctx.working_dir) else {
                    return Ok(ToolOutput::success(
                        "no package manifest found; nothing to check".into(),
                    ));
                };
                let command = match manifest.ecosystem {
                    Ecosystem::Node => "npm outdated --json",
                    Ecosystem::Rust => "cargo outdated",
                    Ecosystem::Python => "pip list --outdated",
                    Ecosystem::Go => "go list -u -m all",
                };
                run_manager_command(ctx, command, 120).await
            }
            "audit" => {
                let Some(manifest) = detect_manifest(&ctx.working_dir) else {
                    return Ok(ToolOutput::success(
                        "no package manifest found; nothing to audit".into(),
                    ));
                };
                let command = match manifest.ecosystem {
                    Ecosystem::Node => "npm audit --json",
                    Ecosystem::Rust => "cargo audit",
                    Ecosystem::Python => "pip-audit",
                    Ecosystem::Go => "govulncheck ./...",
                };
                run_manager_command(ctx, command, 180).await
            }
            "install" => {
                let package = args
                    .get("package")
                    .and_then(|v| v.as_str())
                    .context("package is required")?;
                let version = args.get("version").and_then(|v| v.as_str());
                let Some(manifest) = detect_manifest(&ctx.working_dir) else {
                    anyhow::bail!("no package manifest found; install into which ecosystem?");
                };
                let command = install_command(manifest.ecosystem, package, version);
                let mut output = run_manager_command(ctx, &command, 300).await?;
                let note = format!(
                    "installing into {}: {}\n\n",
                    manifest.ecosystem.label(),
                    command
                );
                output.summary = Some(format!("{} {}", manifest.ecosystem.label(), package));
                output.content = format!("{note}{}", output.content);
                Ok(output)
            }
            _ => Err(anyhow::anyhow!(
                "unknown action `{action}` (detect, outdated, audit, install)"
            )),
        }
    }
}

pub fn dependency_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "dependency".into(),
        description: "Manage project dependencies. Actions: detect (list manifests and parsed dependencies), outdated (check for newer versions via the package manager), audit (scan for known vulnerabilities), install (add a package with an exact version, showing the exact command being run). Requires install permission for the install action.".into(),
        input_schema: json!({
            "type": "object",
            "properties": {
                "action": { "type": "string", "enum": ["detect", "outdated", "audit", "install"] },
                "package": { "type": "string" },
                "version": { "type": "string" }
            },
            "required": ["action"],
            "additionalProperties": false,
        }),
        executor: std::sync::Arc::new(DependencyTool),
        permissions: agent_permissions::PermissionScope::Execute,
        timeout_secs: 360,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::tempdir;

    fn write_manifest(dir: &std::path::Path, ecosystem: Ecosystem, content: &str) -> Manifest {
        let name = match ecosystem {
            Ecosystem::Node => "package.json",
            Ecosystem::Rust => "Cargo.toml",
            Ecosystem::Python => "pyproject.toml",
            Ecosystem::Go => "go.mod",
        };
        let path = dir.join(name);
        fs::write(&path, content).unwrap();
        Manifest { path, ecosystem }
    }

    #[test]
    fn parses_package_json() {
        let dir = tempdir().unwrap();
        let manifest = write_manifest(
            dir.path(),
            Ecosystem::Node,
            r#"{"dependencies":{"react":"^18.0.0"},"devDependencies":{"typescript":"^5.0.0"}}"#,
        );
        let deps = parse_dependencies(&manifest);
        assert!(deps.contains(&("react".into(), "^18.0.0".into())));
        assert!(deps.contains(&("typescript".into(), "^5.0.0".into())));
    }

    #[test]
    fn parses_cargo_toml() {
        let dir = tempdir().unwrap();
        let manifest = write_manifest(
            dir.path(),
            Ecosystem::Rust,
            "[dependencies]\nserde = \"1.0\"\nclap = { version = \"4.5\", features = [\"derive\"] }\n\ntokio = { version = \"1\", features = [\"full\"] }\n[dev-dependencies]\npretty = \"0.1\"",
        );
        let deps = parse_dependencies(&manifest);
        assert!(deps.contains(&("serde".into(), "1.0".into())));
        assert!(deps.contains(&("clap".into(), "4.5".into())));
        assert!(deps.contains(&("pretty".into(), "0.1".into())));
    }

    #[test]
    fn parses_pyproject_dependencies() {
        let dir = tempdir().unwrap();
        let manifest = write_manifest(
            dir.path(),
            Ecosystem::Python,
            "[project]\ndependencies = [\n  \"fastapi>=0.100\",\n  \"httpx==0.27\",\n]\n",
        );
        let deps = parse_dependencies(&manifest);
        assert!(deps.contains(&("fastapi".into(), "0.100".into())));
        assert!(deps.contains(&("httpx".into(), "0.27".into())));
    }

    #[test]
    fn parses_go_mod() {
        let dir = tempdir().unwrap();
        let manifest = write_manifest(
            dir.path(),
            Ecosystem::Go,
            "module example.com/x\n\nrequire (\n\tgithub.com/foo/bar v1.2.3\n)\n",
        );
        let deps = parse_dependencies(&manifest);
        assert!(deps.contains(&("github.com/foo/bar".into(), "v1.2.3".into())));
    }

    #[test]
    fn builds_install_commands() {
        assert_eq!(
            install_command(Ecosystem::Node, "lodash", Some("4.17.21")),
            "npm install lodash@4.17.21"
        );
        assert_eq!(
            install_command(Ecosystem::Rust, "serde", Some("1.0")),
            "cargo add serde@1.0"
        );
        assert_eq!(
            install_command(Ecosystem::Python, "httpx", None),
            "pip install httpx"
        );
    }
}
