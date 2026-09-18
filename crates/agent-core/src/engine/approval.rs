use std::collections::{HashMap, HashSet};
use std::sync::{Arc, Mutex};

use agent_permissions::{PermissionDecision, PermissionScope};
use agent_tools::{PermissionApprover, PermissionRequest, UserPrompter, UserQuestion};
use async_trait::async_trait;
use tokio::sync::{mpsc, oneshot};
use uuid::Uuid;

use crate::engine::event::{EngineEvent, EventClock};

struct Pending {
    tool: String,
    scope: PermissionScope,
    sender: oneshot::Sender<PermissionDecision>,
}

pub struct EngineApprover {
    tx: mpsc::Sender<EngineEvent>,
    clock: EventClock,
    pending: Mutex<HashMap<Uuid, Pending>>,
    questions: Mutex<HashMap<Uuid, oneshot::Sender<Option<String>>>>,
    session_allowed: Mutex<HashSet<String>>,
    auto_allow: bool,
}

impl EngineApprover {
    pub fn new(tx: mpsc::Sender<EngineEvent>, clock: EventClock, auto_allow: bool) -> Arc<Self> {
        Arc::new(Self {
            tx,
            clock,
            pending: Mutex::new(HashMap::new()),
            questions: Mutex::new(HashMap::new()),
            session_allowed: Mutex::new(HashSet::new()),
            auto_allow,
        })
    }

    pub fn clock(&self) -> &EventClock {
        &self.clock
    }

    pub fn auto_allow(&self) -> bool {
        self.auto_allow
    }

    pub fn allow_all_pending(&self, decision: PermissionDecision) {
        let pending = std::mem::take(&mut *self.pending.lock().unwrap());
        for (id, item) in pending {
            let _ = self.tx.try_send(EngineEvent::PermissionResolved {
                meta: self.clock.meta(format!("permission_{id}")),
                id,
                decision: decision_name(decision).to_string(),
            });
            let _ = item.sender.send(decision);
        }
    }

    pub fn resolve_pending_file_scopes(&self) {
        let pending = std::mem::take(&mut *self.pending.lock().unwrap());
        let (auto, keep): (Vec<_>, Vec<_>) = pending.into_iter().partition(|(_, item)| {
            matches!(
                item.scope,
                PermissionScope::Read | PermissionScope::Write | PermissionScope::Delete
            )
        });
        let mut map = self.pending.lock().unwrap();
        for (id, item) in keep {
            map.insert(id, item);
        }
        drop(map);
        for (id, item) in auto {
            let _ = self.tx.try_send(EngineEvent::PermissionResolved {
                meta: self.clock.meta(format!("permission_{id}")),
                id,
                decision: "allow".to_string(),
            });
            let _ = item.sender.send(PermissionDecision::Allow);
        }
    }

    pub async fn resolve(&self, id: Uuid, decision: PermissionDecision, remember: bool) -> bool {
        let Some(pending) = self.pending.lock().unwrap().remove(&id) else {
            return false;
        };
        if remember && decision == PermissionDecision::Allow {
            self.session_allowed
                .lock()
                .unwrap()
                .insert(pending.tool.clone());
        }
        let _ = self
            .tx
            .send(EngineEvent::PermissionResolved {
                meta: self.clock.meta(format!("permission_{id}")),
                id,
                decision: decision_name(decision).to_string(),
            })
            .await;
        pending.sender.send(decision).is_ok()
    }

    pub fn pending(&self) -> Vec<Uuid> {
        self.pending.lock().unwrap().keys().copied().collect()
    }

    pub async fn answer(&self, id: Uuid, answer: Option<String>) -> bool {
        let Some(sender) = self.questions.lock().unwrap().remove(&id) else {
            return false;
        };
        let _ = self
            .tx
            .send(EngineEvent::QuestionAnswered {
                meta: self.clock.meta(format!("question_{id}")),
                id,
                answer: answer.clone(),
            })
            .await;
        sender.send(answer).is_ok()
    }

    pub fn pending_questions(&self) -> Vec<Uuid> {
        self.questions.lock().unwrap().keys().copied().collect()
    }
}

#[async_trait]
impl UserPrompter for EngineApprover {
    async fn ask(&self, question: UserQuestion) -> Option<String> {
        let id = Uuid::new_v4();
        let (sender, receiver) = oneshot::channel();
        self.questions.lock().unwrap().insert(id, sender);
        let sent = self
            .tx
            .send(EngineEvent::QuestionAsked {
                meta: self.clock.meta(format!("question_{id}")),
                id,
                question: question.question.clone(),
                options: question.options.clone(),
            })
            .await;
        if sent.is_err() {
            self.questions.lock().unwrap().remove(&id);
            return None;
        }
        receiver.await.ok().flatten()
    }
}

#[async_trait]
impl PermissionApprover for EngineApprover {
    async fn approve(&self, request: PermissionRequest) -> PermissionDecision {
        if self.auto_allow {
            return PermissionDecision::Allow;
        }
        if self.session_allowed.lock().unwrap().contains(&request.tool) {
            return PermissionDecision::Allow;
        }
        let id = Uuid::new_v4();
        let (sender, receiver) = oneshot::channel();
        self.pending.lock().unwrap().insert(
            id,
            Pending {
                tool: request.tool.clone(),
                scope: request.scope,
                sender,
            },
        );
        let sent = self
            .tx
            .send(EngineEvent::PermissionRequested {
                meta: self.clock.meta(format!("permission_{id}")),
                id,
                tool: request.tool.clone(),
                scope: format!("{:?}", request.scope).to_lowercase(),
                target: request.target.clone(),
                reason: request.reason.clone(),
            })
            .await;
        if sent.is_err() {
            self.pending.lock().unwrap().remove(&id);
            return PermissionDecision::Deny;
        }
        receiver.await.unwrap_or(PermissionDecision::Deny)
    }
}

pub fn decision_name(decision: PermissionDecision) -> &'static str {
    match decision {
        PermissionDecision::Allow => "allow",
        PermissionDecision::Deny => "deny",
        PermissionDecision::Ask => "ask",
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::sync::mpsc;

    #[tokio::test]
    async fn ask_emits_question_and_answer_resolves() {
        let (tx, mut rx) = mpsc::channel(8);
        let approver = EngineApprover::new(tx, EventClock::new(Uuid::new_v4()), true);
        let asking = tokio::spawn({
            let approver = Arc::clone(&approver);
            async move {
                approver
                    .ask(UserQuestion {
                        question: "Which?".into(),
                        options: vec!["a".into(), "b".into()],
                    })
                    .await
            }
        });
        let event = rx.recv().await.expect("question event");
        let EngineEvent::QuestionAsked { id, options, .. } = event else {
            panic!("expected question_asked event");
        };
        assert_eq!(options, vec!["a".to_string(), "b".to_string()]);
        assert!(approver.answer(id, Some("b".into())).await);
        assert_eq!(asking.await.unwrap(), Some("b".to_string()));
    }

    #[tokio::test]
    async fn answering_unknown_question_returns_false() {
        let (tx, _rx) = mpsc::channel(8);
        let approver = EngineApprover::new(tx, EventClock::new(Uuid::new_v4()), true);
        assert!(!approver.answer(Uuid::new_v4(), Some("x".into())).await);
    }
}
