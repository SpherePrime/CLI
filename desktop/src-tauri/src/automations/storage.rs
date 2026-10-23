use super::{schedule, types::*};
use crate::error::{HostError, HostResult};
use std::{collections::HashSet, io::Write, path::Path};

pub fn validate_state(state: &SavedState) -> HostResult<()> {
    if state.version != 1 {
        return Err(corrupt());
    }
    let mut rules = HashSet::new();
    for rule in &state.rules {
        schedule::validate(&AutomationInput {
            id: Some(rule.id.clone()),
            title: rule.title.clone(),
            prompt: rule.prompt.clone(),
            enabled: rule.enabled,
            trigger: rule.trigger.clone(),
        })
        .map_err(|_| corrupt())?;
        if rule.id.is_empty() || rule.root.is_empty() || !rules.insert(rule.id.as_str()) {
            return Err(corrupt());
        }
    }
    let mut runs = HashSet::new();
    let mut active = HashSet::new();
    for run in &state.runs {
        if run.id.is_empty()
            || !runs.insert(run.id.as_str())
            || !matches!(
                run.status.as_str(),
                "claimed" | "running" | "completed" | "failed" | "cancelled" | "interrupted"
            )
            || !matches!(
                run.trigger.as_str(),
                "manual" | "interval" | "daily" | "weekly" | "filesChanged" | "runFailed"
            )
        {
            return Err(corrupt());
        }
        if run.active()
            && (!active.insert(run.rule_id.as_str())
                || !state
                    .rules
                    .iter()
                    .any(|rule| rule.id == run.rule_id && rule.root == run.root))
        {
            return Err(corrupt());
        }
    }
    Ok(())
}

pub fn load(path: &Path, now: i64) -> HostResult<SavedState> {
    let mut state: SavedState = match std::fs::read(path) {
        Ok(bytes) => serde_json::from_slice(&bytes).map_err(|_| corrupt())?,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            return Ok(SavedState::default())
        }
        Err(error) => return Err(error.into()),
    };
    validate_state(&state)?;
    for rule in &mut state.rules {
        if !rule.enabled {
            rule.next_at = None;
        } else if rule.next_at.is_none_or(|next| next <= now) {
            rule.next_at = schedule::next_occurrence(&rule.trigger, now)?;
        }
    }
    for run in state.runs.iter_mut().filter(|run| run.active()) {
        if run.root.starts_with("prime-ssh://") {
            run.status = "interrupted".into();
            run.error = Some("Native remote automation recovery is not implemented yet".into());
            run.finished_at = Some(now);
        } else if run.workspace_id.is_none() || run.session_id.is_none() {
            run.status = "interrupted".into();
            run.error = Some("automation.interrupted".into());
            run.finished_at = Some(now);
        }
    }
    Ok(state)
}

pub fn persist(path: &Path, state: &mut SavedState) -> HostResult<()> {
    let inactive: HashSet<_> = state
        .runs
        .iter()
        .rev()
        .filter(|run| !run.active())
        .take(200)
        .map(|run| run.id.clone())
        .collect();
    state
        .runs
        .retain(|run| run.active() || inactive.contains(&run.id));
    if state.owned_run_ids.len() > 5000 {
        let retained: HashSet<_> = state
            .owned_run_ids
            .iter()
            .rev()
            .take(5000)
            .cloned()
            .chain(
                state
                    .runs
                    .iter()
                    .filter(|run| run.active())
                    .map(|run| run.id.clone()),
            )
            .collect();
        state.owned_run_ids.retain(|id| retained.contains(id));
    }
    let mut temporary = tempfile::NamedTempFile::new_in(path.parent().unwrap())?;
    temporary.write_all(&serde_json::to_vec_pretty(state)?)?;
    temporary.as_file().sync_all()?;
    temporary.persist(path).map_err(|error| error.error)?;
    Ok(())
}

fn corrupt() -> HostError {
    HostError::new("AUTOMATION_CORRUPT", "automation.corrupt")
}
