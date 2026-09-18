use std::path::{Path, PathBuf};
use std::process::ExitCode;

use agent_skills::{Skill, SkillRegistry};

use crate::cli::{load_config, save_config, skills_dir, SkillAction};

pub fn run(action: &SkillAction) -> ExitCode {
    let registry = load_registry();
    match action {
        SkillAction::List => run_list(&registry),
        SkillAction::Install { source } => run_install(source),
        SkillAction::Remove { name } => run_remove(&registry, name),
        SkillAction::Enable { name } => run_toggle(name, true),
        SkillAction::Disable { name } => run_toggle(name, false),
        SkillAction::Inspect { name } => run_inspect(&registry, name),
    }
}

fn load_registry() -> SkillRegistry {
    let global = skills_dir();
    let project = std::env::current_dir()
        .unwrap_or_default()
        .join(".agent")
        .join("skills");
    let mut registry = SkillRegistry::new()
        .with_global(global)
        .with_project(project);
    let _ = registry.discover_sync();
    let config = load_config();
    for name in &config.skills_disabled {
        let _ = registry.disable(name);
    }
    registry
}

fn run_list(registry: &SkillRegistry) -> ExitCode {
    if registry.all().is_empty() {
        println!("(no skills installed)");
        return ExitCode::SUCCESS;
    }
    for skill in registry.all() {
        let description = skill
            .description
            .lines()
            .next()
            .unwrap_or("")
            .trim()
            .to_string();
        println!(
            "[{}] {} ({:?}) {}",
            if skill.enabled { "on" } else { "off" },
            skill.name,
            skill.scope,
            description
        );
    }
    ExitCode::SUCCESS
}

fn run_install(source: &str) -> ExitCode {
    let target_root = skills_dir();
    if let Err(error) = std::fs::create_dir_all(&target_root) {
        eprintln!("failed to create {}: {error}", target_root.display());
        return ExitCode::FAILURE;
    }

    let source_dir = if is_git_url(source) {
        match clone_git(source) {
            Some(dir) => dir,
            None => {
                eprintln!("failed to clone skill source: {source}");
                return ExitCode::FAILURE;
            }
        }
    } else {
        PathBuf::from(source)
    };

    if !source_dir.join("SKILL.md").exists() {
        eprintln!("source has no SKILL.md: {}", source_dir.display());
        return ExitCode::FAILURE;
    }

    let name = source_dir
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("skill");
    let target = target_root.join(name);
    if target.exists() {
        eprintln!("skill '{name}' already exists at {}", target.display());
        return ExitCode::FAILURE;
    }
    if let Err(error) = copy_dir(&source_dir, &target) {
        eprintln!("failed to copy skill: {error}");
        return ExitCode::FAILURE;
    }
    println!("installed skill '{name}' at {}", target.display());
    ExitCode::SUCCESS
}

fn run_remove(registry: &SkillRegistry, name: &str) -> ExitCode {
    let path = registry
        .find(name)
        .map(|skill| skill.path.clone())
        .or_else(|| Some(skills_dir().join(name)))
        .filter(|path| path.is_dir());
    let Some(path) = path else {
        eprintln!("skill '{name}' not found");
        return ExitCode::FAILURE;
    };
    match std::fs::remove_dir_all(&path) {
        Ok(_) => {
            println!("removed skill at {}", path.display());
            ExitCode::SUCCESS
        }
        Err(error) => {
            eprintln!("failed to remove skill: {error}");
            ExitCode::FAILURE
        }
    }
}

fn run_toggle(name: &str, enabled: bool) -> ExitCode {
    let mut config = load_config();
    let mut changed = false;
    match enabled {
        true => {
            if let Some(index) = config
                .skills_disabled
                .iter()
                .position(|entry| entry == name)
            {
                config.skills_disabled.remove(index);
                changed = true;
            }
        }
        false => {
            if !config.skills_disabled.iter().any(|entry| entry == name) {
                config.skills_disabled.push(name.to_string());
                changed = true;
            }
        }
    }
    if !changed {
        eprintln!("skill '{name}' not found or already in the requested state");
        return ExitCode::FAILURE;
    }
    match save_config(&config) {
        Ok(path) => {
            println!(
                "{} skill '{name}' (saved {})",
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

fn run_inspect(registry: &SkillRegistry, name: &str) -> ExitCode {
    let Some(skill) = registry.find(name) else {
        eprintln!("skill '{name}' not found");
        return ExitCode::FAILURE;
    };
    print_skill(skill);
    ExitCode::SUCCESS
}

fn print_skill(skill: &Skill) {
    println!(
        "name: {}\nscope: {:?}\nenabled: {}\npath: {}",
        skill.name,
        skill.scope,
        skill.enabled,
        skill.path.display()
    );
    if !skill.tools.is_empty() {
        println!("tools: {}", skill.tools.join(", "));
    }
    if let Ok(content) = std::fs::read_to_string(skill.path.join("SKILL.md")) {
        println!("\n{content}");
    }
}

fn is_git_url(source: &str) -> bool {
    source.starts_with("https://")
        || source.starts_with("http://")
        || source.starts_with("git@")
        || source.starts_with("ssh://")
}

fn clone_git(source: &str) -> Option<PathBuf> {
    let temp = std::env::temp_dir().join(format!("agent-skill-{}", uuid::Uuid::new_v4()));
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
