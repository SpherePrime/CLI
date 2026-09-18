use std::process::Command;

use anyhow::{bail, Context, Result};

pub fn run() -> std::process::ExitCode {
    match update_tui() {
        Ok(()) => std::process::ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("update failed: {error}");
            std::process::ExitCode::FAILURE
        }
    }
}

fn tui_source_dir() -> Result<std::path::PathBuf> {
    let candidates = [
        std::env::current_dir()
            .unwrap_or_default()
            .join("packages/tui"),
        std::env::current_dir()
            .unwrap_or_default()
            .join("../packages/tui"),
    ];
    for candidate in candidates {
        if candidate.is_dir() {
            return Ok(candidate);
        }
    }
    bail!("could not locate the TUI source (packages/tui)")
}

fn install_dir() -> Result<std::path::PathBuf> {
    let current = std::env::current_exe().context("cannot locate current executable")?;
    let dir = current
        .parent()
        .context("cannot determine executable directory")?;
    Ok(dir.to_path_buf())
}

fn update_tui() -> Result<()> {
    let source = tui_source_dir()?;
    let status = Command::new("bun")
        .args(["run", "build"])
        .current_dir(&source)
        .status()
        .context("spawning bun build")?;
    if !status.success() {
        bail!("TUI build failed (exit {status})");
    }
    let built = source.join("dist/agent-tui.exe");
    if !built.exists() {
        bail!("build finished without producing dist/agent-tui.exe");
    }
    let target = install_dir()?.join("agent-tui.exe");
    std::fs::copy(&built, &target).with_context(|| format!("copying to {}", target.display()))?;
    println!("updated the TUI bundle at {}", target.display());
    Ok(())
}
