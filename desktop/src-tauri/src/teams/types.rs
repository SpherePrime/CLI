use serde::{Deserialize, Serialize};

pub const LIMITS: Limits = Limits {
    teams: 100,
    tasks: 20,
    concurrency: 6,
};

pub struct Limits {
    pub teams: usize,
    pub tasks: usize,
    pub concurrency: usize,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum TeamTaskStatus {
    Queued,
    Starting,
    Running,
    Completed,
    Failed,
    Cancelled,
    Interrupted,
}

impl TeamTaskStatus {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Queued => "queued",
            Self::Starting => "starting",
            Self::Running => "running",
            Self::Completed => "completed",
            Self::Failed => "failed",
            Self::Cancelled => "cancelled",
            Self::Interrupted => "interrupted",
        }
    }

    pub fn finished(self) -> bool {
        matches!(
            self,
            Self::Completed | Self::Failed | Self::Cancelled | Self::Interrupted
        )
    }
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TeamTask {
    pub id: String,
    pub title: String,
    pub instruction: String,
    pub status: TeamTaskStatus,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub run_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub workspace_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub started_at: Option<i64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub finished_at: Option<i64>,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Team {
    pub id: String,
    pub root: String,
    pub title: String,
    pub goal: String,
    pub concurrency: u32,
    pub created_at: i64,
    pub tasks: Vec<TeamTask>,
}

#[derive(Clone, Debug, Deserialize)]
pub struct TeamTaskInput {
    pub title: String,
    pub instruction: String,
}

#[derive(Clone, Debug, Deserialize)]
pub struct TeamInput {
    pub title: String,
    pub goal: String,
    pub concurrency: u32,
    pub tasks: Vec<TeamTaskInput>,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SavedTeams {
    pub version: u8,
    pub teams: Vec<Team>,
}

impl Default for SavedTeams {
    fn default() -> Self {
        Self {
            version: 1,
            teams: Vec::new(),
        }
    }
}

pub fn validate(input: &TeamInput) -> Result<TeamInput, &'static str> {
    let text = |value: &str, max: usize, required: bool| -> Result<String, &'static str> {
        let trimmed = value.trim();
        if value.len() > max || (required && trimmed.is_empty()) {
            return Err("team.invalid");
        }
        Ok(trimmed.to_owned())
    };
    if input.tasks.is_empty() || input.tasks.len() > LIMITS.tasks {
        return Err("team.invalid");
    }
    if !(1..=LIMITS.concurrency as u32).contains(&input.concurrency) {
        return Err("team.invalid");
    }
    Ok(TeamInput {
        title: text(&input.title, 120, true)?,
        goal: text(&input.goal, 8000, false)?,
        concurrency: input.concurrency,
        tasks: input
            .tasks
            .iter()
            .map(|task| -> Result<TeamTaskInput, &'static str> {
                Ok(TeamTaskInput {
                    title: text(&task.title, 120, true)?,
                    instruction: text(&task.instruction, 20000, true)?,
                })
            })
            .collect::<Result<Vec<_>, _>>()?,
    })
}

pub fn team_prompt(team: &Team, task: &TeamTask) -> String {
    let others = team
        .tasks
        .iter()
        .filter(|item| item.id != task.id)
        .map(|item| format!("- {}", item.title))
        .collect::<Vec<_>>()
        .join("\n");
    [
        format!("You are one member of the agent team \"{}\".", team.title),
        (!team.goal.is_empty())
            .then(|| format!("Team goal:\n{}", team.goal))
            .unwrap_or_default(),
        format!("Your task: {}\n{}", task.title, task.instruction),
        (!others.is_empty())
            .then(|| {
                format!(
                    "Other members work in parallel on:\n{others}\nDo not do their work and avoid editing the same files."
                )
            })
            .unwrap_or_default(),
        "Finish with a short summary of the result and changed files.".to_owned(),
    ]
    .into_iter()
    .filter(|part| !part.is_empty())
    .collect::<Vec<_>>()
    .join("\n\n")
}
