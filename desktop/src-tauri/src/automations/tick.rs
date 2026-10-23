use super::*;

impl AutomationManager {
    pub(super) fn tick_with(&self, adapter: &dyn AgentAdapter, emit: &Emitter) -> HostResult<()> {
        let _operation = self.operations.lock().unwrap();
        self.writable()?;
        for run in self
            .saved()
            .runs
            .into_iter()
            .filter(|run| run.active() && run.session_id.is_some())
        {
            if run.root.starts_with("prime-ssh://") {
                continue;
            }
            let status = match adapter.status(&run) {
                Ok(status) => status,
                Err(_) => continue,
            };
            if status.status != run.status || status.error != run.error {
                if !matches!(
                    status.status.as_str(),
                    "running" | "completed" | "failed" | "cancelled" | "interrupted"
                ) {
                    return Err(HostError::new(
                        "AUTOMATION_STATUS",
                        "Unknown automation status",
                    ));
                }
                let mut next = self.saved();
                let current = next
                    .runs
                    .iter_mut()
                    .find(|candidate| candidate.id == run.id)
                    .unwrap();
                current.status = status.status;
                current.error = status.error;
                if !current.active() {
                    current.finished_at = Some((self.now)());
                    self.runtime
                        .lock()
                        .unwrap()
                        .rebaseline
                        .insert(run.root.clone());
                }
                self.persist(next, emit)?;
            }
        }
        let monitored: HashSet<_> = self
            .saved()
            .rules
            .into_iter()
            .filter(|rule| rule.enabled && matches!(rule.trigger, Trigger::FilesChanged { .. }))
            .map(|rule| rule.root)
            .collect();
        self.monitors
            .lock()
            .unwrap()
            .retain(|root, _| monitored.contains(root));
        for root in monitored {
            let fingerprint = match adapter.fingerprint(&root) {
                Ok(fingerprint) => fingerprint,
                Err(error) => {
                    self.runtime
                        .lock()
                        .unwrap()
                        .monitor_errors
                        .insert(root.clone(), error.message.clone());
                    emit(
                        json!({"root":root,"error":error.message,"activeCount":self.counts()["activeCount"],"enabledCount":self.counts()["enabledCount"]}),
                    );
                    continue;
                }
            };
            let (previous, reset, cleared) = {
                let mut runtime = self.runtime.lock().unwrap();
                (
                    runtime
                        .fingerprints
                        .insert(root.clone(), fingerprint.clone()),
                    runtime.rebaseline.remove(&root),
                    runtime.monitor_errors.remove(&root).is_some(),
                )
            };
            if cleared {
                emit(
                    json!({"root":root,"activeCount":self.counts()["activeCount"],"enabledCount":self.counts()["enabledCount"]}),
                );
            }
            if previous.is_some_and(|previous| previous != fingerprint) && !reset {
                self.accept_event(
                    &AutomationEvent {
                        root,
                        kind: "filesChanged".into(),
                        source_run_id: None,
                        fingerprint: Some(fingerprint),
                    },
                    adapter,
                    emit,
                )?;
            }
        }
        for rule in
            self.saved().rules.into_iter().filter(|rule| {
                rule.enabled && rule.next_at.is_some_and(|next| next <= (self.now)())
            })
        {
            if rule.root.starts_with("prime-ssh://") {
                continue;
            }
            self.dispatch_run(&rule, rule.trigger.name(), None, adapter, emit)?;
        }
        Ok(())
    }
}
