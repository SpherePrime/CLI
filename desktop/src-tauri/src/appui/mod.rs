pub(crate) mod dom;
pub(crate) mod locator;
pub(crate) mod manager;
#[cfg(windows)]
pub(crate) mod explore;
#[cfg(windows)]
pub(crate) mod picker;
#[cfg(windows)]
pub(crate) mod recording;
pub(crate) mod render;
pub(crate) mod scenario;
pub(crate) mod tools;
pub(crate) mod types;

#[cfg(windows)]
pub(crate) mod uia;

#[cfg(all(test, windows))]
mod testkit;
