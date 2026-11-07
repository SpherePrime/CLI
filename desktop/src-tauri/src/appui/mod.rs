pub(crate) mod audit;
pub(crate) mod changes;
pub(crate) mod content;
pub(crate) mod dom;
pub(crate) mod find;
pub(crate) mod launch;
pub(crate) mod locator;
pub(crate) mod manager;
#[cfg(windows)]
pub(crate) mod explore;
#[cfg(not(windows))]
#[path = "unsupported/explore.rs"]
pub(crate) mod explore;
#[cfg(windows)]
pub(crate) mod picker;
#[cfg(not(windows))]
#[path = "unsupported/picker.rs"]
pub(crate) mod picker;
pub(crate) mod render;
pub(crate) mod report;
pub(crate) mod scenario;
pub(crate) mod tools;
pub(crate) mod types;
pub(crate) mod ui_action;

#[cfg(windows)]
pub(crate) mod uia;
#[cfg(not(windows))]
#[path = "unsupported/uia.rs"]
pub(crate) mod uia;

#[cfg(all(test, windows))]
mod testkit;
