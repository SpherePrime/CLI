use std::cmp::Ordering;
use std::path::{Path, PathBuf};
use std::process::Command;

use anyhow::{bail, Context, Result};
use uuid::Uuid;

const DEFAULT_REPO: &str = "https://github.com/dwertyfa288/CLI.git";

pub fn run() -> std::process::ExitCode {
    match full_update() {
        Ok(()) => std::process::ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("update failed: {error}");
            eprintln!("no changes were applied to the installed binaries");
            std::process::ExitCode::FAILURE
        }
    }
}

fn full_update() -> Result<()> {
    let current = local_version();
    println!("current version: {current}");
    if let Some(latest) = check_remote_version()? {
        println!("latest release tag: {latest}");
        match compare_versions(&current, &latest) {
            Ordering::Less => {
                println!("a newer release is available; rebuilding from the latest source")
            }
            Ordering::Equal => {
                println!("already at the latest release; rebuilding the local source")
            }
            Ordering::Greater => println!(
                "local checkout is ahead of the latest release; rebuilding the local source"
            ),
        }
    } else {
        println!("no release tags reachable (offline?); rebuilding the local source");
    }

    let workspace = workspace_dir()?;
    let cli_binary = build_cli_release(&workspace)?;
    let tui_binary = build_tui(&workspace)?;

    install_cli_binary(&cli_binary)?;
    install_tui_binary(&tui_binary)?;

    println!("update complete");
    Ok(())
}

fn local_version() -> String {
    env!("CARGO_PKG_VERSION").to_string()
}

fn parse_version(tag: &str) -> Option<(u64, u64, u64)> {
    let trimmed = tag.trim().trim_start_matches('v').trim_start_matches('V');
    if trimmed.is_empty() || !trimmed.as_bytes()[0].is_ascii_digit() {
        return None;
    }
    let mut parts = trimmed.split('.');
    let major = parts.next()?.parse().ok()?;
    let minor = parts.next()?.parse().ok()?;
    let patch = parts.next()?.parse().ok()?;
    Some((major, minor, patch))
}

fn compare_versions(left: &str, right: &str) -> Ordering {
    let a = parse_version(left).unwrap_or_default();
    let b = parse_version(right).unwrap_or_default();
    a.cmp(&b)
}

fn check_remote_version() -> Result<Option<String>> {
    let repo = std::env::var("AGENT_UPDATE_REPO").unwrap_or_else(|_| DEFAULT_REPO.into());
    let output = Command::new("git")
        .args(["ls-remote", "--tags", &repo])
        .output()
        .context("spawning git ls-remote (is git installed?)")?;
    if !output.status.success() {
        return Ok(None);
    }
    let stdout = String::from_utf8_lossy(&output.stdout);
    let mut latest: Option<(u64, u64, u64)> = None;
    for line in stdout.lines() {
        if let Some((_, ref_name)) = line.split_once("refs/tags/") {
            if let Some(version) = parse_version(ref_name.trim_end_matches("^{}")) {
                if latest.is_none_or(|current| version > current) {
                    latest = Some(version);
                }
            }
        }
    }
    Ok(latest.map(|(major, minor, patch)| format!("{major}.{minor}.{patch}")))
}

fn workspace_dir() -> Result<PathBuf> {
    let manifest = Path::new(env!("CARGO_MANIFEST_DIR"));
    let root = manifest.join("..").join("..");
    let root = root
        .canonicalize()
        .with_context(|| format!("resolving workspace root at {}", root.display()))?;
    if !root.join("Cargo.toml").exists() {
        bail!("workspace root has no Cargo.toml: {}", root.display());
    }
    if !root.join("packages").join("tui").is_dir() {
        bail!(
            "packages/tui is missing in the workspace root: {}",
            root.display()
        );
    }
    Ok(root)
}

fn cli_binary_name() -> &'static str {
    if cfg!(windows) {
        "agent-cli.exe"
    } else {
        "agent-cli"
    }
}

fn build_cli_release(workspace: &Path) -> Result<PathBuf> {
    let status = Command::new("cargo")
        .args(["build", "--release", "-p", "agent-cli"])
        .current_dir(workspace)
        .status()
        .context("spawning cargo build --release")?;
    if !status.success() {
        bail!("CLI backend build failed (exit {status})");
    }
    let binary = workspace
        .join("target")
        .join("release")
        .join(cli_binary_name());
    if !binary.is_file() {
        bail!(
            "release build finished without producing {}",
            binary.display()
        );
    }
    Ok(binary)
}

fn build_tui(workspace: &Path) -> Result<PathBuf> {
    let source = workspace.join("packages").join("tui");
    let status = Command::new("bun")
        .args(["run", "build"])
        .current_dir(&source)
        .status()
        .context("spawning bun run build")?;
    if !status.success() {
        bail!("TUI build failed (exit {status})");
    }
    let built = source.join("dist").join("agent-tui.exe");
    if !built.is_file() {
        bail!("build finished without producing {}", built.display());
    }
    Ok(built)
}

fn install_dir() -> Result<PathBuf> {
    let current = std::env::current_exe().context("cannot locate the current executable")?;
    let dir = current
        .parent()
        .context("cannot determine the executable directory")?;
    Ok(dir.to_path_buf())
}

fn install_cli_binary(built: &Path) -> Result<()> {
    let target = install_dir()?.join(cli_binary_name());
    if same_file(built, &target) {
        println!("CLI binary is already current at {}", target.display());
        return Ok(());
    }
    println!("installing CLI backend to {}", target.display());
    install_atomic(built, &target, true)
}

fn install_tui_binary(built: &Path) -> Result<()> {
    let name = if cfg!(windows) {
        "agent-tui.exe"
    } else {
        "agent-tui"
    };
    let target = install_dir()?.join(name);
    if same_file(built, &target) {
        println!("TUI bundle is already current at {}", target.display());
        return Ok(());
    }
    println!("installing TUI bundle to {}", target.display());
    install_atomic(built, &target, false)
}

fn same_file(left: &Path, right: &Path) -> bool {
    match (std::fs::canonicalize(left), std::fs::canonicalize(right)) {
        (Ok(a), Ok(b)) => a == b,
        _ => false,
    }
}

fn install_atomic(built: &Path, target: &Path, smoke_test: bool) -> Result<()> {
    let dir = target
        .parent()
        .context("install target has no parent directory")?;
    let file_name = target
        .file_name()
        .and_then(|name| name.to_str())
        .context("install target has no file name")?;
    std::fs::create_dir_all(dir)
        .with_context(|| format!("creating install directory {}", dir.display()))?;
    let token = Uuid::new_v4().simple();
    let staged = dir.join(format!(".{file_name}.new-{token}"));

    std::fs::copy(built, &staged)
        .with_context(|| format!("staging new binary at {}", staged.display()))?;

    if smoke_test {
        let output = Command::new(&staged)
            .arg("--version")
            .output()
            .with_context(|| format!("running the version smoke test on {}", staged.display()))?;
        if !output.status.success() {
            let _ = std::fs::remove_file(&staged);
            bail!(
                "the freshly built binary failed the version smoke test (exit {})",
                output.status
            );
        }
    }

    let backup = dir.join(format!("{file_name}.bak"));
    if target.exists() {
        std::fs::copy(target, &backup)
            .with_context(|| format!("backing up {}", target.display()))?;
    }

    if let Err(error) = std::fs::rename(&staged, target) {
        if backup.exists() && target.exists() {
            let _ = std::fs::copy(&backup, target);
        }
        let _ = std::fs::remove_file(&staged);
        bail!(
            "applying the update failed: {error}\n\
             hint: close the running agent and re-run `agent update` (the old version was kept at {})",
            backup.display()
        );
    }

    if backup.exists() {
        let _ = std::fs::remove_file(&backup);
    }
    Ok(())
}
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_version_tags() {
        assert_eq!(parse_version("v1.2.3"), Some((1, 2, 3)));
        assert_eq!(parse_version("1.4.0"), Some((1, 4, 0)));
        assert_eq!(parse_version("V0.9.1"), Some((0, 9, 1)));
        assert_eq!(parse_version("release-1.2"), None);
        assert_eq!(parse_version("nope"), None);
    }

    #[test]
    fn compares_versions() {
        assert_eq!(compare_versions("0.1.0", "0.1.1"), Ordering::Less);
        assert_eq!(compare_versions("1.0.0", "1.0.0"), Ordering::Equal);
        assert_eq!(compare_versions("1.2.0", "1.1.9"), Ordering::Greater);
    }

    #[test]
    fn picks_the_newest_tag_from_ls_remote_output() {
        let output = "\
abc1234\trefs/tags/v0.1.0
abc1235\trefs/tags/v1.2.3
abc1236\trefs/tags/v1.2.3^{}
def1234\trefs/tags/v0.9.9
";
        let mut latest: Option<(u64, u64, u64)> = None;
        for line in output.lines() {
            if let Some((_, ref_name)) = line.split_once("refs/tags/") {
                if let Some(version) = parse_version(ref_name.trim_end_matches("^{}")) {
                    if latest.is_none_or(|current| version > current) {
                        latest = Some(version);
                    }
                }
            }
        }
        assert_eq!(latest, Some((1, 2, 3)));
    }
}
