use crate::error::{HostError, HostResult};
use ignore::{
    gitignore::{Gitignore, GitignoreBuilder},
    WalkBuilder, WalkState,
};
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    fs,
    path::{Path, PathBuf},
    sync::{
        atomic::{AtomicUsize, Ordering},
        Arc, Mutex,
    },
    time::{Duration, Instant, SystemTime},
};

pub const ALWAYS_SKIP_ENTRIES: [&str; 5] =
    [".git", "node_modules", ".superpowers", ".env", ".env.local"];

pub const FAST_SKIP_DIRS: [&str; 11] = [
    "dist",
    "build",
    "target",
    ".prime",
    "__pycache__",
    ".next",
    ".nuxt",
    ".turbo",
    ".venv",
    ".cache",
    "vendor",
];

const IGNORE_CACHE_MAX_ROOTS: usize = 32;
const INDEX_CACHE_MAX_ROOTS: usize = 16;
const INDEX_CACHE_TTL: Duration = Duration::from_secs(15);
const MAX_INDEXED_ENTRIES: usize = 5000;

pub fn should_fast_skip(name: &str, is_dir: bool) -> bool {
    ALWAYS_SKIP_ENTRIES.contains(&name) || (is_dir && FAST_SKIP_DIRS.contains(&name))
}

#[derive(Clone, Copy, PartialEq, Eq)]
struct IgnoreStamp {
    gitignore: Option<(SystemTime, u64)>,
    primeignore: Option<(SystemTime, u64)>,
}

impl IgnoreStamp {
    fn read(root: &Path) -> Self {
        Self {
            gitignore: file_stamp(&root.join(".gitignore")),
            primeignore: file_stamp(&root.join(".primeignore")),
        }
    }
}

fn file_stamp(path: &Path) -> Option<(SystemTime, u64)> {
    let metadata = fs::metadata(path).ok()?;
    let modified = metadata.modified().ok()?;
    Some((modified, metadata.len()))
}

struct CachedIgnore {
    stamp: IgnoreStamp,
    rules: Arc<Gitignore>,
    last_used: Instant,
}

#[derive(Clone)]
pub struct IndexedEntry {
    pub name: String,
    pub path: String,
    pub directory: bool,
    pub name_lower: String,
    pub path_lower: String,
    pub depth: usize,
}

impl IndexedEntry {
    pub fn from_relative(name: String, path: String, directory: bool) -> Self {
        let name_lower = name.to_lowercase();
        let path_lower = path.to_lowercase();
        let depth = path.bytes().filter(|byte| *byte == b'/').count();
        Self {
            name,
            path,
            directory,
            name_lower,
            path_lower,
            depth,
        }
    }
}

struct CachedIndex {
    stamp: IgnoreStamp,
    entries: Arc<Vec<IndexedEntry>>,
    built_at: Instant,
}

pub struct FileCache {
    ignores: Mutex<HashMap<PathBuf, CachedIgnore>>,
    indexes: Mutex<HashMap<PathBuf, CachedIndex>>,
}

impl Default for FileCache {
    fn default() -> Self {
        Self::new()
    }
}

impl FileCache {
    pub fn new() -> Self {
        Self {
            ignores: Mutex::new(HashMap::new()),
            indexes: Mutex::new(HashMap::new()),
        }
    }

    pub fn gitignore(&self, canonical_root: &Path) -> HostResult<Arc<Gitignore>> {
        let stamp = IgnoreStamp::read(canonical_root);
        {
            let mut cache = self.ignores.lock().unwrap();
            if let Some(cached) = cache.get_mut(canonical_root) {
                if cached.stamp == stamp {
                    cached.last_used = Instant::now();
                    return Ok(Arc::clone(&cached.rules));
                }
            }
        }
        let rules = build_gitignore(canonical_root)?;
        let mut cache = self.ignores.lock().unwrap();
        if cache.len() >= IGNORE_CACHE_MAX_ROOTS && !cache.contains_key(canonical_root) {
            if let Some(oldest_key) = cache
                .iter()
                .min_by_key(|(_, entry)| entry.last_used)
                .map(|(key, _)| key.clone())
            {
                cache.remove(&oldest_key);
            }
        }
        cache.insert(
            canonical_root.to_path_buf(),
            CachedIgnore {
                stamp,
                rules: Arc::clone(&rules),
                last_used: Instant::now(),
            },
        );
        Ok(rules)
    }

    pub fn invalidate(&self, root: &Path) {
        let canonical = dunce::canonicalize(root).unwrap_or_else(|_| root.to_path_buf());
        self.indexes.lock().unwrap().remove(&canonical);
        self.ignores.lock().unwrap().remove(&canonical);
    }

    pub fn search(&self, root: &Path, query: &str, limit: usize) -> HostResult<Value> {
        let canonical_root = dunce::canonicalize(root)?;
        let stamp = IgnoreStamp::read(&canonical_root);
        let cached_entries = {
            let cache = self.indexes.lock().unwrap();
            cache.get(&canonical_root).and_then(|cached| {
                if cached.stamp == stamp && cached.built_at.elapsed() < INDEX_CACHE_TTL {
                    Some(Arc::clone(&cached.entries))
                } else {
                    None
                }
            })
        };
        let entries = match cached_entries {
            Some(entries) => entries,
            None => {
                let built = Arc::new(build_index(&canonical_root)?);
                let mut cache = self.indexes.lock().unwrap();
                if cache.len() >= INDEX_CACHE_MAX_ROOTS && !cache.contains_key(&canonical_root) {
                    if let Some(oldest_key) = cache
                        .iter()
                        .min_by_key(|(_, entry)| entry.built_at)
                        .map(|(key, _)| key.clone())
                    {
                        cache.remove(&oldest_key);
                    }
                }
                cache.insert(
                    canonical_root,
                    CachedIndex {
                        stamp,
                        entries: Arc::clone(&built),
                        built_at: Instant::now(),
                    },
                );
                built
            }
        };
        Ok(filter_and_rank(&entries, query, limit))
    }
}

pub fn build_gitignore(canonical_root: &Path) -> HostResult<Arc<Gitignore>> {
    let mut rules = GitignoreBuilder::new(canonical_root);
    for name in [".gitignore", ".primeignore"] {
        rules.add(canonical_root.join(name));
    }
    let built = rules
        .build()
        .map_err(|error| HostError::new("INVALID_IGNORE", error.to_string()))?;
    Ok(Arc::new(built))
}

#[cfg(test)]
pub fn search_uncached(root: &Path, query: &str, limit: usize) -> HostResult<Value> {
    let canonical_root = dunce::canonicalize(root)?;
    let entries = build_index(&canonical_root)?;
    Ok(filter_and_rank(&entries, query, limit))
}

struct ThreadCollector {
    local: Vec<IndexedEntry>,
    shared: Arc<Mutex<Vec<IndexedEntry>>>,
}

impl Drop for ThreadCollector {
    fn drop(&mut self) {
        if !self.local.is_empty() {
            let mut guard = self.shared.lock().unwrap();
            guard.extend(self.local.drain(..));
        }
    }
}

pub fn build_index(canonical_root: &Path) -> HostResult<Vec<IndexedEntry>> {
    let count = Arc::new(AtomicUsize::new(0));
    let collected = Arc::new(Mutex::new(Vec::with_capacity(256)));
    let root_arc = Arc::new(canonical_root.to_path_buf());
    let mut builder = WalkBuilder::new(canonical_root);
    builder
        .hidden(false)
        .git_ignore(true)
        .git_global(false)
        .git_exclude(false)
        .parents(false)
        .add_custom_ignore_filename(".primeignore")
        .follow_links(false)
        .threads(4)
        .filter_entry(|entry| {
            if entry.depth() == 0 {
                return true;
            }
            let name = entry.file_name().to_string_lossy();
            let is_dir = entry.file_type().is_some_and(|ft| ft.is_dir());
            !should_fast_skip(&name, is_dir)
        });
    builder.build_parallel().run(|| {
        let count = Arc::clone(&count);
        let root = Arc::clone(&root_arc);
        let mut collector = ThreadCollector {
            local: Vec::new(),
            shared: Arc::clone(&collected),
        };
        Box::new(move |result| {
            if count.load(Ordering::Relaxed) >= MAX_INDEXED_ENTRIES {
                return WalkState::Quit;
            }
            let Ok(entry) = result else {
                return WalkState::Continue;
            };
            if entry.depth() == 0 {
                return WalkState::Continue;
            }
            let is_symlink = entry.path_is_symlink();
            let directory = if is_symlink {
                let Ok(resolved) = dunce::canonicalize(entry.path()) else {
                    return WalkState::Continue;
                };
                if !resolved.starts_with(root.as_path()) {
                    return WalkState::Continue;
                }
                resolved.is_dir()
            } else {
                entry.file_type().is_some_and(|ft| ft.is_dir())
            };
            let name = entry.file_name().to_string_lossy().into_owned();
            if should_fast_skip(&name, directory) {
                return WalkState::Continue;
            }
            let Ok(relative) = entry.path().strip_prefix(root.as_path()) else {
                return WalkState::Continue;
            };
            let relative = relative.to_string_lossy().replace('\\', "/");
            if relative.is_empty() {
                return WalkState::Continue;
            }
            if count.fetch_add(1, Ordering::Relaxed) >= MAX_INDEXED_ENTRIES {
                return WalkState::Quit;
            }
            collector
                .local
                .push(IndexedEntry::from_relative(name, relative, directory));
            WalkState::Continue
        })
    });
    let mut guard = collected.lock().unwrap();
    Ok(std::mem::take(&mut *guard))
}

pub fn filter_and_rank(entries: &[IndexedEntry], query: &str, limit: usize) -> Value {
    let capped = limit.clamp(1, 200);
    let normalized_query = query.trim().replace('\\', "/").to_lowercase();
    if normalized_query.is_empty() {
        let mut sorted: Vec<&IndexedEntry> = entries.iter().collect();
        sorted.sort_by(|left, right| {
            left.depth
                .cmp(&right.depth)
                .then_with(|| left.directory.cmp(&right.directory))
                .then_with(|| left.path_lower.cmp(&right.path_lower))
        });
        return Value::Array(
            sorted
                .into_iter()
                .take(capped)
                .map(|entry| {
                    json!({
                        "name": entry.name,
                        "path": entry.path,
                        "directory": entry.directory,
                    })
                })
                .collect(),
        );
    }

    let mut matched: Vec<(u8, &IndexedEntry)> = entries
        .iter()
        .filter_map(|entry| {
            let score = if entry.name_lower == normalized_query {
                0
            } else if entry.name_lower.starts_with(&normalized_query) {
                1
            } else if entry.name_lower.contains(&normalized_query) {
                2
            } else if entry.path_lower.contains(&normalized_query) {
                3
            } else {
                return None;
            };
            Some((score, entry))
        })
        .collect();

    matched.sort_by(|(left_score, left), (right_score, right)| {
        left_score
            .cmp(right_score)
            .then_with(|| left.depth.cmp(&right.depth))
            .then_with(|| left.path.len().cmp(&right.path.len()))
            .then_with(|| left.path_lower.cmp(&right.path_lower))
    });

    Value::Array(
        matched
            .into_iter()
            .take(capped)
            .map(|(_, entry)| {
                json!({
                    "name": entry.name,
                    "path": entry.path,
                    "directory": entry.directory,
                })
            })
            .collect(),
    )
}
