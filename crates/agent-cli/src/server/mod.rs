pub mod api;
pub mod fs;
pub mod message;
pub mod session;
pub mod state;

use std::process::ExitCode;
use std::sync::Arc;

use hyper::body::Incoming;
use hyper::service::service_fn;
use hyper::Request;
use hyper_util::rt::TokioIo;
use tokio::net::TcpListener;
use tokio::runtime::Runtime;

use state::AppState;

pub fn serve(host: &str, port: u16, storage: agent_storage::Storage) -> ExitCode {
    let runtime = Runtime::new().expect("tokio runtime creation failed");

    runtime.block_on(async move {
        let state: Arc<AppState> = match AppState::new(storage) {
            Ok(state) => Arc::new(state),
            Err(error) => {
                eprintln!("server init error: {error}");
                return ExitCode::FAILURE;
            }
        };

        let cfg = agent_config::ConfigLoader::new().load().unwrap_or_default();
        if let Ok(mut guard) = state.config.write() {
            *guard = Some(cfg);
        }

        let addr = format!("{host}:{port}");
        let listener = match TcpListener::bind(&addr).await {
            Ok(listener) => listener,
            Err(error) => {
                eprintln!("cannot bind {addr}: {error}");
                return ExitCode::FAILURE;
            }
        };

        let bound = listener.local_addr().expect("bound address");
        println!("agent server listening on http://{bound}");

        let _ = agent_ui::files::refresh_file_cache();

        loop {
            let (stream, _) = match listener.accept().await {
                Ok(pair) => pair,
                Err(_) => continue,
            };

            let state = Arc::clone(&state);
            tokio::spawn(async move {
                if let Err(error) = hyper::server::conn::http1::Builder::new()
                    .serve_connection(
                        TokioIo::new(stream),
                        service_fn(move |request: Request<Incoming>| {
                            let state = Arc::clone(&state);
                            async move { api::route(request, state).await }
                        }),
                    )
                    .await
                {
                    eprintln!("connection error: {error}");
                }
            });
        }
    })
}