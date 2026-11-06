use super::panel::{Panel, Tab};

pub const TABS_PER_OWNER: usize = 12;

impl Tab {
    pub fn visible_to(&self, view: Option<&str>) -> bool {
        self.owner.is_none() || self.owner.as_deref() == view
    }
}

impl Panel {
    fn view_key(&self) -> String {
        self.view.clone().unwrap_or_default()
    }

    pub fn visible_tabs(&self) -> Vec<&Tab> {
        let view = self.view.as_deref();
        self.tabs.iter().filter(|tab| tab.visible_to(view)).collect()
    }

    fn is_visible(&self, id: &str) -> bool {
        let view = self.view.as_deref();
        self.tabs
            .iter()
            .any(|tab| tab.id == id && tab.visible_to(view))
    }

    pub fn set_view(&mut self, view: Option<String>) -> bool {
        if self.view == view {
            return false;
        }
        let key = self.view_key();
        match self.active.take() {
            Some(id) => self.selected.insert(key, id),
            None => self.selected.remove(&key),
        };
        self.view = view;
        let remembered = self
            .selected
            .get(&self.view_key())
            .filter(|id| self.is_visible(id))
            .cloned();
        self.active = remembered.or_else(|| self.visible_tabs().first().map(|tab| tab.id.clone()));
        true
    }

    pub fn owned_count(&self, owner: Option<&str>) -> usize {
        self.tabs
            .iter()
            .filter(|tab| tab.owner.as_deref() == owner)
            .count()
    }

    pub fn show(&mut self, session: Option<&str>, id: &str) -> bool {
        match session {
            Some(session) if Some(session) != self.view.as_deref() => {
                self.selected.insert(session.to_owned(), id.to_owned());
                false
            }
            _ => {
                self.active = Some(id.to_owned());
                true
            }
        }
    }

    pub fn agent_tab(&self, session: &str) -> Option<String> {
        self.agent
            .get(session)
            .filter(|id| self.tabs.iter().any(|tab| &tab.id == *id))
            .cloned()
    }

    pub fn accessible(&self, session: Option<&str>, id: &str) -> Result<(), String> {
        let tab = self
            .tabs
            .iter()
            .find(|tab| tab.id == id)
            .ok_or("Вкладка не найдена")?;
        match (session, tab.owner.as_deref()) {
            (Some(session), Some(owner)) if session != owner => {
                Err("Вкладка принадлежит другому чату".to_owned())
            }
            _ => Ok(()),
        }
    }

    pub fn session_tabs(&self, session: Option<&str>) -> Vec<&Tab> {
        match session {
            Some(session) => self
                .tabs
                .iter()
                .filter(|tab| tab.visible_to(Some(session)))
                .collect(),
            None => self.tabs.iter().collect(),
        }
    }

    pub fn forget(&mut self, id: &str, index: usize) {
        self.selected.retain(|_, tab| tab != id);
        self.agent.retain(|_, tab| tab != id);
        if self.active.as_deref() != Some(id) {
            return;
        }
        let view = self.view.as_deref();
        self.active = self.tabs[index.min(self.tabs.len())..]
            .iter()
            .find(|tab| tab.visible_to(view))
            .or_else(|| {
                self.tabs[..index.min(self.tabs.len())]
                    .iter()
                    .rev()
                    .find(|tab| tab.visible_to(view))
            })
            .map(|tab| tab.id.clone());
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn tab(id: &str, owner: Option<&str>) -> Tab {
        Tab {
            id: id.to_owned(),
            url: "about:blank".to_owned(),
            title: String::new(),
            loading: false,
            owner: owner.map(str::to_owned),
            viewport: None,
            history: vec!["about:blank".to_owned()],
            index: 0,
            traversing: false,
            load_error: None,
        }
    }

    fn panel() -> Panel {
        Panel {
            tabs: vec![tab("shared", None), tab("a", Some("chat-a")), tab("b", Some("chat-b"))],
            view: Some("chat-a".to_owned()),
            active: Some("a".to_owned()),
            ..Panel::default()
        }
    }

    #[test]
    fn each_chat_sees_only_its_own_and_shared_tabs() {
        let mut panel = panel();
        let ids = |panel: &Panel| {
            panel
                .visible_tabs()
                .iter()
                .map(|tab| tab.id.clone())
                .collect::<Vec<_>>()
        };
        assert_eq!(ids(&panel), ["shared", "a"]);
        assert!(panel.set_view(Some("chat-b".to_owned())));
        assert_eq!(ids(&panel), ["shared", "b"]);
        assert_eq!(panel.active.as_deref(), Some("shared"));
        panel.active = Some("b".to_owned());
        panel.set_view(Some("chat-a".to_owned()));
        assert_eq!(panel.active.as_deref(), Some("a"));
        panel.set_view(Some("chat-b".to_owned()));
        assert_eq!(panel.active.as_deref(), Some("b"));
    }

    #[test]
    fn agent_in_background_chat_does_not_change_visible_tab() {
        let mut panel = panel();
        assert!(!panel.show(Some("chat-b"), "b"));
        assert_eq!(panel.active.as_deref(), Some("a"));
        assert_eq!(panel.selected.get("chat-b").map(String::as_str), Some("b"));
        assert!(panel.show(Some("chat-a"), "shared"));
        assert_eq!(panel.active.as_deref(), Some("shared"));
    }

    #[test]
    fn agents_cannot_touch_tabs_of_other_chats() {
        let panel = panel();
        assert!(panel.accessible(Some("chat-a"), "a").is_ok());
        assert!(panel.accessible(Some("chat-a"), "shared").is_ok());
        assert!(panel.accessible(Some("chat-a"), "b").is_err());
        assert!(panel.accessible(None, "b").is_ok());
    }

    #[test]
    fn closing_a_tab_selects_nearest_visible_and_clears_agent_binding() {
        let mut panel = panel();
        panel.agent.insert("chat-a".to_owned(), "a".to_owned());
        panel.tabs.remove(1);
        panel.forget("a", 1);
        assert_eq!(panel.active.as_deref(), Some("shared"));
        assert!(panel.agent_tab("chat-a").is_none());
    }
}
