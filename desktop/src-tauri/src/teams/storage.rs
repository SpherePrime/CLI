use super::types::{SavedTeams, Team, TeamTaskStatus};
use crate::error::{HostError, HostResult};
use std::{io::Write, path::Path};

pub fn load(path: &Path) -> HostResult<SavedTeams> {
    let state: SavedTeams = match std::fs::read(path) {
        Ok(bytes) => serde_json::from_slice(&bytes).map_err(|_| corrupt())?,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            return Ok(SavedTeams::default())
        }
        Err(error) => return Err(error.into()),
    };
    validate_state(&state)?;
    Ok(state)
}

fn validate_state(state: &SavedTeams) -> HostResult<()> {
    if state.version != 1 {
        return Err(corrupt());
    }
    let mut identifiers = std::collections::HashSet::new();
    for team in &state.teams {
        if team.id.is_empty()
            || team.root.is_empty()
            || team.title.trim().is_empty()
            || team.title.len() > 120
            || team.goal.len() > 8000
            || !(1..=super::types::LIMITS.concurrency as u32).contains(&team.concurrency)
            || team.tasks.is_empty()
            || team.tasks.len() > super::types::LIMITS.tasks
            || !identifiers.insert(team.id.as_str())
        {
            return Err(corrupt());
        }
        let mut tasks = std::collections::HashSet::new();
        for task in &team.tasks {
            if task.id.is_empty()
                || task.title.trim().is_empty()
                || task.title.len() > 120
                || task.instruction.trim().is_empty()
                || task.instruction.len() > 20000
                || !tasks.insert(task.id.as_str())
            {
                return Err(corrupt());
            }
        }
    }
    Ok(())
}

pub fn persist(path: &Path, state: &SavedTeams) -> HostResult<()> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)?;
    }
    let mut temporary = tempfile::NamedTempFile::new_in(path.parent().unwrap())?;
    temporary.write_all(&serde_json::to_vec_pretty(state)?)?;
    temporary.as_file().sync_all()?;
    temporary.persist(path).map_err(|error| error.error)?;
    Ok(())
}

fn recover(team: &mut Team) {
    for task in &mut team.tasks {
        if task.status == TeamTaskStatus::Starting {
            task.status = if task.session_id.is_some() {
                TeamTaskStatus::Running
            } else {
                TeamTaskStatus::Queued
            };
        }
    }
}

pub fn recover_all(state: &mut SavedTeams) {
    for team in &mut state.teams {
        recover(team);
    }
}

pub fn corrupt() -> HostError {
    HostError::new("TEAM_CORRUPT", "team.corrupt")
}
