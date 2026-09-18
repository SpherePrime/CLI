use std::collections::{HashMap, HashSet};
use std::sync::{Arc, Mutex};

use agent_permissions::{PermissionDecision, PermissionScope};
use agent_tools::{PermissionApprover, PermissionRequest};
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
    session_allowed: Mutex<HashSet<String>>,
    auto_allow: bool,
}

impl EngineApprover {
    pub fn new(tx: mpsc::Sender<EngineEvent>, clock: EventClock, auto_allow: bool) -> Arc<Self> {
        Arc::new(Self {
            tx,
            clock,
            pending: Mutex::new(HashMap::new()),
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
