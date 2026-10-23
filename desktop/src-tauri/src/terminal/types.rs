use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TerminalSnapshot {
    pub id: String,
    pub workspace_id: String,
    pub title: String,
    pub shell: String,
    pub cwd: String,
    pub pid: u32,
    pub cols: u16,
    pub rows: u16,
    pub owner: String,
    pub status: String,
    pub created_at: u64,
    pub sequence: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub exit_code: Option<u32>,
}

#[derive(Default, Deserialize)]
pub struct TerminalOptions {
    pub cols: Option<f64>,
    pub rows: Option<f64>,
    pub shell: Option<String>,
    pub args: Option<Vec<String>>,
    pub title: Option<String>,
}
