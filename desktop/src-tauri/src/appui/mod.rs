pub(crate) mod dom;
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
#[cfg(windows)]
pub(crate) mod recording;
pub(crate) mod render;
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
