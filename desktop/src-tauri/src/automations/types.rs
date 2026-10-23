use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum Trigger {
    #[serde(rename = "interval")]
    Interval { minutes: f64 },
    #[serde(rename = "daily")]
    Daily { time: String },
    #[serde(rename = "weekly")]
    Weekly { time: String, days: Vec<u32> },
    #[serde(rename = "filesChanged")]
    FilesChanged {
        #[serde(rename = "cooldownMinutes")]
        cooldown_minutes: f64,
    },
    #[serde(rename = "runFailed")]
    RunFailed {
        #[serde(rename = "cooldownMinutes")]
        cooldown_minutes: f64,
    },
}

impl Trigger {
    pub fn name(&self) -> &'static str {
        match self {
            Self::Interval { .. } => "interval",
            Self::Daily { .. } => "daily",
            Self::Weekly { .. } => "weekly",
            Self::FilesChanged { .. } => "filesChanged",
            Self::RunFailed { .. } => "runFailed",
        }
    }
    pub fn cooldown(&self) -> Option<f64> {
        match self {
            Self::FilesChanged { cooldown_minutes } | Self::RunFailed { cooldown_minutes } => {
                Some(*cooldown_minutes)
            }
            _ => None,
        }
    }
}

#[derive(Clone, Serialize, Deserialize)]
pub struct AutomationInput {
    pub id: Option<String>,
    pub title: String,
    pub prompt: String,
    pub enabled: bool,
    pub trigger: Trigger,
}

#[derive(Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AutomationRule {
    pub id: String,
    pub title: String,
    pub prompt: String,
    pub enabled: bool,
    pub trigger: Trigger,
    pub root: String,
    #[serde(deserialize_with = "timestamp")]
    pub created_at: i64,
    #[serde(deserialize_with = "timestamp")]
    pub updated_at: i64,
    #[serde(
        default,
        deserialize_with = "optional_timestamp",
        skip_serializing_if = "Option::is_none"
    )]
    pub next_at: Option<i64>,
    #[serde(
        default,
        deserialize_with = "optional_timestamp",
        skip_serializing_if = "Option::is_none"
    )]
    pub last_triggered_at: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_fingerprint: Option<String>,
}

#[derive(Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AutomationRun {
    pub id: String,
    pub rule_id: String,
    pub root: String,
    pub title: String,
    pub trigger: String,
    pub status: String,
    #[serde(deserialize_with = "timestamp")]
    pub created_at: i64,
    #[serde(
        default,
        deserialize_with = "optional_timestamp",
        skip_serializing_if = "Option::is_none"
    )]
    pub finished_at: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub workspace_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub session_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

impl AutomationRun {
    pub fn active(&self) -> bool {
        matches!(self.status.as_str(), "claimed" | "running")
    }
}

#[derive(Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SavedState {
    pub version: u32,
    pub rules: Vec<AutomationRule>,
    pub runs: Vec<AutomationRun>,
    pub owned_run_ids: Vec<String>,
}

impl Default for SavedState {
    fn default() -> Self {
        Self {
            version: 1,
            rules: vec![],
            runs: vec![],
            owned_run_ids: vec![],
        }
    }
}

#[derive(Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AutomationEvent {
    pub root: String,
    #[serde(rename = "type")]
    pub kind: String,
    pub source_run_id: Option<String>,
    pub fingerprint: Option<String>,
}

fn timestamp<'de, D: serde::Deserializer<'de>>(deserializer: D) -> Result<i64, D::Error> {
    let value = f64::deserialize(deserializer)?;
    if !value.is_finite() || value < i64::MIN as f64 || value >= i64::MAX as f64 {
        return Err(serde::de::Error::custom("invalid timestamp"));
    }
    Ok(value as i64)
}

fn optional_timestamp<'de, D: serde::Deserializer<'de>>(
    deserializer: D,
) -> Result<Option<i64>, D::Error> {
    let value = Option::<f64>::deserialize(deserializer)?;
    value
        .map(|value| {
            if !value.is_finite() || value < i64::MIN as f64 || value >= i64::MAX as f64 {
                return Err(serde::de::Error::custom("invalid timestamp"));
            }
            Ok(value as i64)
        })
        .transpose()
}
