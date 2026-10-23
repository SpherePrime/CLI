use super::*;

impl AutomationManager {
    pub(super) fn dispatch_run(
        &self,
        rule: &AutomationRule,
        trigger: &str,
        fingerprint: Option<String>,
        adapter: &dyn AgentAdapter,
        emit: &Emitter,
    ) -> HostResult<bool> {
        let mut claimed = self.saved();
        if self.stopped.load(Ordering::Acquire)
            || claimed
                .runs
                .iter()
                .any(|run| run.rule_id == rule.id && run.active())
            || claimed.runs.iter().filter(|run| run.active()).count() >= MAX_CONCURRENT
        {
            return Ok(false);
        }
        let mut run = AutomationRun {
            id: uuid::Uuid::new_v4().to_string(),
            rule_id: rule.id.clone(),
            root: rule.root.clone(),
            title: rule.title.clone(),
            trigger: trigger.into(),
            status: "claimed".into(),
            created_at: (self.now)(),
            finished_at: None,
            workspace_id: None,
            session_id: None,
            error: None,
        };
        claimed.runs.push(run.clone());
        claimed.owned_run_ids.push(run.id.clone());
        let claimed_rule = claimed
            .rules
            .iter_mut()
            .find(|candidate| candidate.id == rule.id)
            .unwrap();
        claimed_rule.last_triggered_at = Some((self.now)());
        if fingerprint.is_some() {
            claimed_rule.last_fingerprint = fingerprint;
        }
        if trigger != "manual" {
            claimed_rule.next_at = schedule::next_occurrence(&rule.trigger, (self.now)())?;
        }
        self.persist(claimed, emit)?;
        let mut attempted = false;
        let dispatched = (|| {
            let prepared = adapter.prepare(rule, &run)?;
            run.workspace_id = Some(prepared.workspace_id);
            run.session_id = Some(prepared.session_id);
            let mut prepared_state = self.saved();
            *prepared_state
                .runs
                .iter_mut()
                .find(|candidate| candidate.id == run.id)
                .unwrap() = run.clone();
            self.persist(prepared_state, emit)?;
            if self.stopped.load(Ordering::Acquire) {
                return Err(HostError::new(
                    "AUTOMATION_INTERRUPTED",
                    "automation.interrupted",
                ));
            }
            attempted = true;
            adapter.dispatch(rule, &run)
        })();
        let mut next = self.saved();
        let current = next
            .runs
            .iter_mut()
            .find(|candidate| candidate.id == run.id)
            .unwrap();
        match dispatched {
            Ok(()) => {
                current.status = "running".into();
                current.error = None;
            }
            Err(error) => {
                current.status = if attempted { "running" } else { "interrupted" }.into();
                current.error = Some(error.message);
                if !attempted {
                    current.finished_at = Some((self.now)());
                }
            }
        }
        self.persist(next, emit)?;
        Ok(true)
    }
}
