use crate::error::{HostError, HostResult};

pub fn require_trusted(webview: &tauri::Webview) -> HostResult<()> {
    if webview.label() != "main" {
        return Err(HostError::new(
            "ACCESS_DENIED",
            "Native commands are restricted to the application renderer",
        ));
    }
    let url = webview
        .url()
        .map_err(|error| HostError::new("ACCESS_DENIED", error.to_string()))?;
    let bundled = (url.scheme() == "tauri" && url.host_str() == Some("localhost"))
        || (url.scheme() == "http" && url.host_str() == Some("tauri.localhost"));
    let development = cfg!(debug_assertions)
        && url.scheme() == "http"
        && url.host_str() == Some("127.0.0.1")
        && url.port() == Some(5173);
    let trusted = bundled || development;
    if !trusted {
        return Err(HostError::new(
            "ACCESS_DENIED",
            "Remote pages cannot use native commands",
        ));
    }
    Ok(())
}
