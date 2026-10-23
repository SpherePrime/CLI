use super::*;

impl AutomationManager {
    pub(super) fn handle_with(
        &self,
        request: &Value,
        adapter: &dyn AgentAdapter,
        emit: &Emitter,
    ) -> HostResult<Value> {
        let _operation = self.operations.lock().unwrap();
        let action = string(request, "action")?;
        let root = self.canonical(string(request, "root")?)?;
        if action == "list" {
            return Ok(self.snapshot(&root));
        }
        self.writable()?;
        if root.starts_with("prime-ssh://") {
            return Err(HostError::unsupported("remote automation"));
        }
        match action {
            "save" => self.save_rule(
                &root,
                serde_json::from_value(
                    request
                        .get("input")
                        .cloned()
                        .ok_or_else(schedule::invalid)?,
                )?,
                emit,
            )?,
            "remove" => {
                let id = string(request, "id")?;
                let mut next = self.saved();
                if next
                    .runs
                    .iter()
                    .any(|run| run.root == root && run.rule_id == id && run.active())
                {
                    return Err(HostError::new("AUTOMATION_ACTIVE", "automation.active"));
                }
                if !next
                    .rules
                    .iter()
                    .any(|rule| rule.id == id && rule.root == root)
                {
                    return Err(not_found());
                }
                next.rules.retain(|rule| rule.id != id || rule.root != root);
                self.persist(next, emit)?;
                emit(
                    json!({"root":root,"activeCount":self.counts()["activeCount"],"enabledCount":self.counts()["enabledCount"]}),
                );
            }
            "run" => {
                let id = string(request, "id")?;
                let rule = self
                    .saved()
                    .rules
                    .into_iter()
                    .find(|rule| rule.id == id && rule.root == root)
                    .ok_or_else(not_found)?;
                if !self.dispatch_run(&rule, "manual", None, adapter, emit)? {
                    return Err(HostError::new(
                        "AUTOMATION_BUSY",
                        "Automation is already running or the concurrency limit has been reached",
                    ));
                }
            }
            "cancel" => {
                let id = string(request, "runId")?;
                let run = self
                    .saved()
                    .runs
                    .into_iter()
                    .find(|run| run.id == id && run.root == root)
                    .ok_or_else(not_found)?;
                if run.active() {
                    adapter.cancel(&run)?;
                    let mut next = self.saved();
                    let run = next.runs.iter_mut().find(|run| run.id == id).unwrap();
                    run.status = "cancelled".into();
                    run.finished_at = Some((self.now)());
                    run.error = None;
                    self.runtime.lock().unwrap().rebaseline.insert(root.clone());
                    self.persist(next, emit)?;
                }
            }
            _ => return Err(schedule::invalid()),
        }
        Ok(self.snapshot(&root))
    }

    fn save_rule(&self, root: &str, input: AutomationInput, emit: &Emitter) -> HostResult<()> {
        schedule::validate(&input)?;
        let mut next = self.saved();
        let previous = input
            .id
            .as_ref()
            .and_then(|id| {
                next.rules
                    .iter()
                    .find(|rule| &rule.id == id && rule.root == root)
            })
            .cloned();
        if input.id.is_some() && previous.is_none() {
            return Err(not_found());
        }
        let changed = previous
            .as_ref()
            .is_none_or(|rule| rule.trigger != input.trigger || rule.enabled != input.enabled);
        let now = (self.now)();
        let rule = AutomationRule {
            id: previous
                .as_ref()
                .map(|rule| rule.id.clone())
                .unwrap_or_else(|| uuid::Uuid::new_v4().to_string()),
            title: input.title.trim().into(),
            prompt: input.prompt.trim().into(),
            enabled: input.enabled,
            trigger: input.trigger.clone(),
            root: root.into(),
            created_at: previous.as_ref().map(|rule| rule.created_at).unwrap_or(now),
            updated_at: now,
            next_at: if !input.enabled {
                None
            } else if changed {
                schedule::next_occurrence(&input.trigger, now)?
            } else {
                previous.as_ref().and_then(|rule| rule.next_at)
            },
            last_triggered_at: if changed {
                Some(now)
            } else {
                previous.as_ref().and_then(|rule| rule.last_triggered_at)
            },
            last_fingerprint: if changed {
                None
            } else {
                previous
                    .as_ref()
                    .and_then(|rule| rule.last_fingerprint.clone())
            },
        };
        if changed {
            self.runtime.lock().unwrap().fingerprints.remove(root);
        }
        next.rules.retain(|candidate| candidate.id != rule.id);
        next.rules.push(rule);
        self.persist(next, emit)
    }

    pub(super) fn event_with(
        &self,
        mut event: AutomationEvent,
        adapter: &dyn AgentAdapter,
        emit: &Emitter,
    ) -> HostResult<()> {
        let _operation = self.operations.lock().unwrap();
        self.writable()?;
        event.root = self.canonical(&event.root)?;
        self.accept_event(&event, adapter, emit)
    }

    pub(super) fn accept_event(
        &self,
        event: &AutomationEvent,
        adapter: &dyn AgentAdapter,
        emit: &Emitter,
    ) -> HostResult<()> {
        if !matches!(event.kind.as_str(), "filesChanged" | "runFailed") {
            return Err(schedule::invalid());
        }
        if event.root.starts_with("prime-ssh://") {
            return Err(HostError::unsupported("remote automation trigger"));
        }
        let saved = self.saved();
        if event
            .source_run_id
            .as_ref()
            .is_some_and(|id| saved.owned_run_ids.contains(id))
        {
            return Ok(());
        }
        if event.kind == "filesChanged"
            && saved
                .runs
                .iter()
                .any(|run| run.root == event.root && run.active())
        {
            return Ok(());
        }
        for rule in saved.rules.iter().filter(|rule| {
            rule.root == event.root && rule.enabled && rule.trigger.name() == event.kind
        }) {
            let Some(cooldown) = rule.trigger.cooldown() else {
                continue;
            };
            let fingerprint = event.fingerprint.as_ref().or(event.source_run_id.as_ref());
            if fingerprint.is_some() && fingerprint == rule.last_fingerprint.as_ref() {
                continue;
            }
            if rule
                .last_triggered_at
                .is_some_and(|last| (((self.now)() - last) as f64) < cooldown * 60000.0)
            {
                continue;
            }
            self.dispatch_run(rule, &event.kind, fingerprint.cloned(), adapter, emit)?;
        }
        Ok(())
    }
}

fn not_found() -> HostError {
    HostError::new("AUTOMATION_NOT_FOUND", "automation.notFound")
}
