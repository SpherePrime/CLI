pub(crate) mod locator;
pub(crate) mod render;
pub(crate) mod types;

#[cfg(windows)]
pub(crate) mod uia;

#[cfg(all(test, windows))]
mod testkit;
