use std::path::{Path, PathBuf};

use agent_storage::ProjectRef;

pub fn resolve_workspace(path: &Path) -> PathBuf {
    agent_context::project::canonical_project_root(path)
}

pub fn project_ref_for(path: &Path) -> ProjectRef {
    let root = resolve_workspace(path);
    let name = root
        .file_name()
        .map(|s| s.to_string_lossy().into_owned())
        .unwrap_or_else(|| root.to_string_lossy().into_owned());
    ProjectRef {
        id: agent_context::project::project_id(&root),
        name,
        path: root.clone(),
        remote_url: agent_context::project::git_remote_origin(&root),
    }
}

pub fn canonical_existing_root(path: &Path) -> Option<PathBuf> {
    let canonical = std::fs::canonicalize(path).ok()?;
    agent_context::project::git_root(&canonical).or(Some(canonical))
}
