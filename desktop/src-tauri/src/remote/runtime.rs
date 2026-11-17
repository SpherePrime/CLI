use std::sync::OnceLock;
use tokio::runtime::{Builder, Runtime};

pub fn runtime() -> &'static Runtime {
    static RUNTIME: OnceLock<Runtime> = OnceLock::new();
    RUNTIME.get_or_init(|| {
        Builder::new_multi_thread()
            .enable_all()
            .build()
            .expect("Failed to initialize SSH Tokio runtime")
    })
}

pub fn block_on<F: std::future::Future>(future: F) -> F::Output {
    runtime().block_on(future)
}
