use serde::Serialize;

use super::audit::Audit;

const MAX_CHECKS: usize = 200;

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Check {
    pub time: u64,
    pub name: String,
    pub passed: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<String>,
}

#[derive(Clone, Debug, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Report {
    pub checks: Vec<Check>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audit: Option<Audit>,
}

impl Report {
    pub fn add(&mut self, check: Check) {
        self.checks.retain(|existing| existing.name != check.name);
        self.checks.push(check);
        if self.checks.len() > MAX_CHECKS {
            self.checks.remove(0);
        }
    }

    pub fn summary(&self) -> String {
        let passed = self.checks.iter().filter(|check| check.passed).count();
        let failed = self.checks.len() - passed;
        let mut lines = vec![format!("Checks: {passed} passed, {failed} failed")];
        for check in self.checks.iter().filter(|check| !check.passed) {
            lines.push(format!(
                "FAIL {}{}",
                check.name,
                check
                    .details
                    .as_deref()
                    .map(|details| format!(": {details}"))
                    .unwrap_or_default()
            ));
        }
        lines.join("\n")
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn check(name: &str, passed: bool) -> Check {
        Check {
            time: 0,
            name: name.to_owned(),
            passed,
            details: Some("why".to_owned()),
        }
    }

    #[test]
    fn a_repeated_check_replaces_the_previous_result() {
        let mut report = Report::default();
        report.add(check("login", false));
        report.add(check("login", true));
        assert_eq!(report.checks.len(), 1);
        assert!(report.checks[0].passed);
    }

    #[test]
    fn the_summary_lists_failures() {
        let mut report = Report::default();
        report.add(check("login", true));
        report.add(check("save", false));
        let summary = report.summary();
        assert!(summary.contains("1 passed, 1 failed"));
        assert!(summary.contains("FAIL save: why"));
    }
}
