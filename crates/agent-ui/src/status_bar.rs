#[derive(Debug, Clone)]
pub struct StatusBar {
    model: String,
    project: String,
    mode: String,
}

impl Default for StatusBar {
    fn default() -> Self {
        Self::new()
    }
}

impl StatusBar {
    pub fn new() -> Self {
        Self {
            model: String::new(),
            project: String::new(),
            mode: String::new(),
        }
    }

    pub fn with_model(mut self, model: &str) -> Self {
        self.model = model.to_string();
        self
    }

    pub fn with_project(mut self, project: &std::path::Path) -> Self {
        self.project = project.display().to_string();
        self
    }

    pub fn with_mode(mut self, mode: &str) -> Self {
        self.mode = mode.to_string();
        self
    }

    pub fn model(&self) -> &str {
        &self.model
    }

    pub fn project(&self) -> &str {
        &self.project
    }

    pub fn mode(&self) -> &str {
        &self.mode
    }
}

impl std::fmt::Display for StatusBar {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(
            f,
            "model={} project={} mode={}",
            self.model, self.project, self.mode
        )
    }
}
