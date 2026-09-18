use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize, Default)]
pub enum SkillScope {
    #[default]
    Global,
    Project,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Skill {
    pub id: Uuid,
    pub name: String,
    pub description: String,
    pub scope: SkillScope,
    pub path: PathBuf,
    pub enabled: bool,
    pub instructions: Option<String>,
    pub tools: Vec<String>,
    pub hooks: Vec<String>,
    pub resources: Vec<String>,
}

type ParsedSkill = (Option<String>, Vec<String>, Vec<String>, Vec<String>);

pub struct SkillRegistry {
    pub skills: Vec<Skill>,
    pub global_dir: Option<PathBuf>,
    pub project_dir: Option<PathBuf>,
}

impl SkillRegistry {
    pub fn new() -> Self {
        Self {
            skills: vec![],
            global_dir: None,
            project_dir: None,
        }
    }

    pub fn with_global(mut self, path: PathBuf) -> Self {
        self.global_dir = Some(path);
        self
    }

    pub fn with_project(mut self, path: PathBuf) -> Self {
        self.project_dir = Some(path);
        self
    }

    pub async fn discover(&mut self) -> Result<usize> {
        let global = self.global_dir.clone();
        let project = self.project_dir.clone();
        let mut found = 0;
        if let Some(g) = global {
            found += self.scan_dir(&g, SkillScope::Global).await?;
        }
        if let Some(p) = project {
            found += self.scan_dir(&p, SkillScope::Project).await?;
        }
        Ok(found)
    }

    pub fn discover_sync(&mut self) -> Result<usize> {
        let global = self.global_dir.clone();
        let project = self.project_dir.clone();
        let mut found = 0;
        if let Some(g) = global {
            found += self.scan_dir_sync(&g, SkillScope::Global)?;
        }
        if let Some(p) = project {
            found += self.scan_dir_sync(&p, SkillScope::Project)?;
        }
        Ok(found)
    }

    async fn scan_dir(&mut self, root: &Path, scope: SkillScope) -> Result<usize> {
        self.scan_dir_sync(root, scope)
    }

    fn scan_dir_sync(&mut self, root: &Path, scope: SkillScope) -> Result<usize> {
        let mut found = 0;
        for entry in walkdir::WalkDir::new(root).into_iter().flatten() {
            let p = entry.path();
            if p.is_dir() {
                let skill_md = p.join("SKILL.md");
                if skill_md.exists() && self.find_by_name_and_scope(p, scope).is_none() {
                    let name = p
                        .file_name()
                        .and_then(|n| n.to_str())
                        .unwrap_or("unknown")
                        .to_string();
                    let (instructions, tools, hooks, resources) = Self::parse_skill_md(&skill_md)?;
                    self.skills.push(Skill {
                        id: Uuid::new_v4(),
                        name,
                        description: instructions.clone().unwrap_or_default(),
                        scope,
                        path: p.to_path_buf(),
                        enabled: true,
                        instructions,
                        tools,
                        hooks,
                        resources,
                    });
                    found += 1;
                }
            }
        }
        Ok(found)
    }

    fn find_by_name_and_scope(&self, dir: &Path, scope: SkillScope) -> Option<usize> {
        self.skills
            .iter()
            .position(|s| s.path.starts_with(dir) && s.scope == scope)
    }

    fn parse_skill_md(path: &Path) -> Result<ParsedSkill> {
        let content =
            std::fs::read_to_string(path).with_context(|| format!("reading {}", path.display()))?;
        let mut instructions = Some(content.clone());
        let mut tools = Vec::new();
        let mut hooks = Vec::new();
        let mut resources = Vec::new();

        if let Some(frontmatter) = content
            .strip_prefix("---")
            .and_then(|s| s.split_once("---").map(|(a, _)| a.to_string()))
        {
            for line in frontmatter.lines() {
                if let Some(v) = line.strip_prefix("tools:") {
                    tools = v.split(',').map(|s| s.trim().to_string()).collect();
                }
                if let Some(v) = line.strip_prefix("hooks:") {
                    hooks = v.split(',').map(|s| s.trim().to_string()).collect();
                }
                if let Some(v) = line.strip_prefix("resources:") {
                    resources = v.split(',').map(|s| s.trim().to_string()).collect();
                }
            }
            if let Some((_, rest)) = content.rsplit_once("---\n") {
                instructions = Some(rest.to_string());
            }
        }

        Ok((instructions, tools, hooks, resources))
    }

    pub fn find(&self, name: &str) -> Option<&Skill> {
        self.skills
            .iter()
            .find(|s| s.name.eq_ignore_ascii_case(name))
    }

    pub fn find_mut(&mut self, name: &str) -> Option<&mut Skill> {
        self.skills
            .iter_mut()
            .find(|s| s.name.eq_ignore_ascii_case(name))
    }

    pub fn enabled_skills(&self) -> impl Iterator<Item = &Skill> {
        self.skills.iter().filter(|s| s.enabled)
    }

    pub fn enable(&mut self, name: &str) -> bool {
        if let Some(s) = self.find_mut(name) {
            s.enabled = true;
            true
        } else {
            false
        }
    }

    pub fn disable(&mut self, name: &str) -> bool {
        if let Some(s) = self.find_mut(name) {
            s.enabled = false;
            true
        } else {
            false
        }
    }

    pub fn all(&self) -> &[Skill] {
        &self.skills
    }

    pub fn context_block(&self) -> String {
        self.enabled_skills()
            .map(|s| {
                format!(
                    "## Skill: {}\n{}\n",
                    s.name,
                    s.instructions.as_deref().unwrap_or_default()
                )
            })
            .collect::<Vec<_>>()
            .join("\n")
    }
}

impl Default for SkillRegistry {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn parse_frontmatter() {
        let tmp = tempfile::tempdir().unwrap();
        let skill_dir = tmp.path().join("my-skill");
        std::fs::create_dir_all(&skill_dir).unwrap();
        std::fs::write(
            skill_dir.join("SKILL.md"),
            "---\ntools: read_file\nhooks: on_save\n---\n# Instructions\nDo things.",
        )
        .unwrap();
        let (instructions, tools, hooks, resources) =
            SkillRegistry::parse_skill_md(&skill_dir.join("SKILL.md")).unwrap();
        assert!(tools.contains(&"read_file".to_string()));
        assert!(hooks.contains(&"on_save".to_string()));
        assert!(instructions.unwrap().contains("Do things"));
        let _ = resources;
    }
}
