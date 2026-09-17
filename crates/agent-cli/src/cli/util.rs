 use std::path::PathBuf;
 
 use agent_config::{AgentConfig, ConfigLoader};
 
 pub fn load_config() -> AgentConfig {
     ConfigLoader::new().load().unwrap_or_default()
 }
 
 pub fn save_config(cfg: &AgentConfig) -> anyhow::Result<PathBuf> {
     ConfigLoader::save_global(cfg)
 }
 
 fn storage_root() -> PathBuf {
     agent_storage::Storage::global()
         .ok()
         .map(|s| s.root().clone())
         .unwrap_or_else(|| PathBuf::from("agent"))
 }
 
 pub fn plugins_dir() -> PathBuf {
     storage_root().join("plugins")
 }
 
 pub fn skills_dir() -> PathBuf {
     storage_root().join("skills")
 }
 
 pub fn list_session_files() -> Vec<String> {
     let dir = agent_storage::Storage::global()
         .ok()
         .map(|s| s.sessions_dir())
         .unwrap_or_else(|| PathBuf::from("sessions"));
     std::fs::read_dir(&dir)
         .ok()
         .into_iter()
         .flatten()
         .filter_map(|e| e.ok())
         .filter(|e| e.path().extension().and_then(|s| s.to_str()) == Some("jsonl"))
         .map(|e| {
             e.file_name()
                 .to_string_lossy()
                 .trim_end_matches(".jsonl")
                 .to_string()
         })
         .collect()
 }
