use serde::Serialize;

#[derive(Clone, Serialize)]
pub struct InstallStatus {
    pub phase: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub directory: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub percent: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub warning: Option<String>,
}

impl InstallStatus {
    pub fn ready() -> Self {
        Self {
            phase: "ready".into(),
            error: None,
            directory: None,
            percent: None,
            warning: None,
        }
    }

    pub fn phase(phase: &str, directory: &str) -> Self {
        Self {
            phase: phase.into(),
            directory: Some(directory.into()),
            ..Self::ready()
        }
    }

    pub fn active(&self) -> bool {
        matches!(
            self.phase.as_str(),
            "preparing" | "installing" | "verifying" | "voice"
        )
    }
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct InstallMethod {
    pub label: String,
    pub directory_locked: bool,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SetupInfo {
    pub version: String,
    pub directory: String,
    pub installed_size: u64,
    pub method: InstallMethod,
}

impl SetupInfo {
    pub fn new(version: &str, directory: String, installed_size: u64, update: bool) -> Self {
        Self {
            version: version.into(),
            directory,
            installed_size,
            method: InstallMethod {
                label: if update {
                    "Обновление рабочего пространства".into()
                } else {
                    "Ваше рабочее пространство".into()
                },
                directory_locked: update,
            },
        }
    }
}
