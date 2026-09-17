 use thiserror::Error;
 
#[derive(Debug, Error)]
pub enum ModelError {
    #[error("auth failed for {0}")]
    Auth(String),
     #[error("rate limited")]
     RateLimited,
     #[error("upstream error: {0}")]
     Upstream(String),
     #[error("invalid request: {0}")]
     InvalidRequest(String),
     #[error("network error: {0}")]
     Network(#[source] std::io::Error),
 }
 
 impl From<std::io::Error> for ModelError {
     fn from(e: std::io::Error) -> Self {
         Self::Network(e)
     }
 }
