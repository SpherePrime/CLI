use std::time::Duration;

use anyhow::{bail, Context, Result};
use async_trait::async_trait;
use serde_json::{json, Value};

use crate::builtin::paths::truncate;
use crate::executor::{ToolExecutionContext, ToolExecutor, ToolOutput};

const MAX_BYTES: usize = 262_144;
const DEFAULT_TIMEOUT_SECS: u64 = 30;

fn schema(properties: Value, required: &[&str]) -> Value {
    json!({
        "type": "object",
        "properties": properties,
        "required": required,
        "additionalProperties": false,
    })
}

pub fn validate_url(url: &str) -> Result<()> {
    let parsed = reqwest::Url::parse(url).with_context(|| format!("invalid url: {url}"))?;
    match parsed.scheme() {
        "http" | "https" => Ok(()),
        other => bail!("unsupported url scheme: {other}"),
    }
}

fn is_private_ip(ip: std::net::IpAddr) -> bool {
    match ip {
        std::net::IpAddr::V4(v4) => {
            v4.is_loopback() || v4.is_private() || v4.is_link_local() || v4.is_unspecified()
        }
        std::net::IpAddr::V6(v6) => {
            v6.is_loopback()
                || v6.is_unspecified()
                || v6.segments()[0] & 0xfe00 == 0xfc00
                || v6.segments()[0] & 0xffc0 == 0xfe80
        }
    }
}

pub fn reject_private_target(url: &str) -> Result<()> {
    let parsed = reqwest::Url::parse(url).with_context(|| format!("invalid url: {url}"))?;
    let host = parsed.host_str().context("url has no host")?;
    let port = parsed.port_or_known_default().unwrap_or(80);
    if let Ok(ip) = host.parse::<std::net::IpAddr>() {
        if is_private_ip(ip) {
            bail!("refusing to fetch private address: {ip}");
        }
        return Ok(());
    }
    let addrs: Vec<std::net::SocketAddr> = std::net::ToSocketAddrs::to_socket_addrs(&(host, port))
        .with_context(|| format!("could not resolve host: {host}"))?
        .collect();
    for addr in addrs {
        if is_private_ip(addr.ip()) {
            bail!("refusing to fetch private address: {} ({host})", addr.ip());
        }
    }
    Ok(())
}

pub struct HttpFetchTool;

#[async_trait]
impl ToolExecutor for HttpFetchTool {
    async fn execute(&self, args: Value, _ctx: &ToolExecutionContext) -> Result<ToolOutput> {
        let url = args
            .get("url")
            .and_then(|v| v.as_str())
            .context("url is required")?
            .trim();
        validate_url(url)?;
        reject_private_target(url)?;
        let method = args
            .get("method")
            .and_then(|v| v.as_str())
            .unwrap_or("GET")
            .to_uppercase();
        let body = args
            .get("body")
            .and_then(|v| v.as_str())
            .map(str::to_string);

        let client = reqwest::Client::builder()
            .timeout(Duration::from_secs(DEFAULT_TIMEOUT_SECS))
            .build()
            .context("building http client")?;
        let mut request = client.request(
            reqwest::Method::from_bytes(method.as_bytes()).context("invalid method")?,
            url,
        );
        if let Some(headers) = args.get("headers").and_then(|v| v.as_object()) {
            for (name, value) in headers {
                if let Some(value) = value.as_str() {
                    request = request.header(name, value);
                }
            }
        }
        if let Some(body) = body {
            request = request.body(body);
        }

        let response = request.send().await.context("http request failed")?;
        let status = response.status();
        let content_type = response
            .headers()
            .get(reqwest::header::CONTENT_TYPE)
            .and_then(|value| value.to_str().ok())
            .unwrap_or("")
            .to_string();
        let bytes = response.bytes().await.context("reading response body")?;
        let truncated = bytes.len() > MAX_BYTES;
        let slice = &bytes[..bytes.len().min(MAX_BYTES)];
        let text = if content_type.contains("text")
            || content_type.contains("json")
            || content_type.contains("xml")
            || content_type.contains("javascript")
            || content_type.is_empty()
        {
            String::from_utf8_lossy(slice).to_string()
        } else {
            format!("<{} bytes of {content_type}>", bytes.len())
        };

        let mut out = format!("HTTP {status} {url}\nContent-Type: {content_type}\n\n{text}");
        if truncated {
            out.push_str(&format!("\n...[truncated at {MAX_BYTES} bytes]"));
        }
        let output = ToolOutput::success(truncate(&out, MAX_BYTES + 4096))
            .summary(format!("HTTP {status}"))
            .truncated(truncated);
        if !status.is_success() {
            return Ok(output);
        }
        Ok(output)
    }
}

pub fn http_fetch_tool() -> crate::ToolDefinition {
    crate::ToolDefinition {
        name: "http_fetch".into(),
        description: "Fetch a URL over HTTP(S) and return the response body (truncated).".into(),
        input_schema: schema(
            json!({
                "url": { "type": "string" },
                "method": { "type": "string", "description": "HTTP method (default GET)" },
                "headers": { "type": "object", "description": "Optional request headers" },
                "body": { "type": "string" }
            }),
            &["url"],
        ),
        executor: std::sync::Arc::new(HttpFetchTool),
        permissions: agent_permissions::PermissionScope::Network,
        timeout_secs: DEFAULT_TIMEOUT_SECS + 5,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_http_and_https() {
        assert!(validate_url("https://example.com").is_ok());
        assert!(validate_url("http://example.com/health").is_ok());
    }

    #[test]
    fn rejects_other_schemes() {
        assert!(validate_url("ftp://example.com").is_err());
        assert!(validate_url("file:///etc/passwd").is_err());
        assert!(validate_url("not a url").is_err());
    }

    #[test]
    fn rejects_loopback_and_private_targets() {
        assert!(reject_private_target("http://127.0.0.1:8080/").is_err());
        assert!(reject_private_target("http://localhost:8080/health").is_err());
        assert!(reject_private_target("http://[::1]:8080/").is_err());
        assert!(reject_private_target("http://192.168.1.10/").is_err());
        assert!(reject_private_target("http://10.0.0.5/").is_err());
        assert!(reject_private_target("http://169.254.169.254/latest/meta/").is_err());
        assert!(reject_private_target("https://example.com").is_ok());
    }
}
