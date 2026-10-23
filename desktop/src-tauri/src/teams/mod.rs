pub(crate) mod adapter;
mod manager;
mod storage;
pub(crate) mod types;

pub use manager::TeamManager;

#[cfg(test)]
mod tests;
