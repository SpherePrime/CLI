use serde::Serialize;

#[derive(Debug, Serialize)]
pub struct HostError {
    pub code: String,
    pub message: String,
}

impl HostError {
    pub fn new(code: &str, message: impl Into<String>) -> Self {
        Self {
            code: code.into(),
            message: message.into(),
        }
    }

    pub fn unsupported(method: &str) -> Self {
        Self::new(
            "NOT_IMPLEMENTED",
            format!("Native capability '{method}' is not implemented yet"),
        )
    }
}

impl From<std::io::Error> for HostError {
    fn from(error: std::io::Error) -> Self {
        Self::new("IO_ERROR", error.to_string())
    }
}

impl From<serde_json::Error> for HostError {
    fn from(error: serde_json::Error) -> Self {
        Self::new("INVALID_JSON", error.to_string())
    }
}

pub type HostResult<T> = Result<T, HostError>;
