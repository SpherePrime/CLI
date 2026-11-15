use std::io::{Read, Write};
use std::time::{SystemTime, UNIX_EPOCH};

pub fn start() {
    std::thread::spawn(move || {
        let launch_time = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs();

        loop {
            if let Some(client_id) = get_client_id() {
                if let Some(mut stream) = connect_ipc() {
                    if let Ok(()) = handshake(&mut stream, &client_id) {
                        let pid = std::process::id();
                        let _ = update_activity(&mut stream, pid, launch_time);

                        loop {
                            std::thread::sleep(std::time::Duration::from_secs(15));
                            if update_activity(&mut stream, pid, launch_time).is_err() {
                                break;
                            }
                        }
                    }
                }
            }
            std::thread::sleep(std::time::Duration::from_secs(10));
        }
    });
}

fn get_client_id() -> Option<String> {
    if let Ok(id) = std::env::var("PRIME_DISCORD_CLIENT_ID") {
        let trimmed = id.trim().to_string();
        if !trimmed.is_empty() {
            return Some(trimmed);
        }
    }

    #[cfg(target_os = "windows")]
    {
        if let Ok(local_app_data) = std::env::var("LOCALAPPDATA") {
            let path = std::path::Path::new(&local_app_data).join("prime").join("discord.json");
            if let Ok(content) = std::fs::read_to_string(&path) {
                if let Ok(val) = serde_json::from_str::<serde_json::Value>(&content) {
                    if let Some(id) = val.get("client_id").and_then(|v| v.as_str()) {
                        let trimmed = id.trim().to_string();
                        if !trimmed.is_empty() {
                            return Some(trimmed);
                        }
                    }
                }
            }
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        if let Ok(home) = std::env::var("HOME") {
            let path = std::path::Path::new(&home).join(".config").join("prime").join("discord.json");
            if let Ok(content) = std::fs::read_to_string(&path) {
                if let Ok(val) = serde_json::from_str::<serde_json::Value>(&content) {
                    if let Some(id) = val.get("client_id").and_then(|v| v.as_str()) {
                        let trimmed = id.trim().to_string();
                        if !trimmed.is_empty() {
                            return Some(trimmed);
                        }
                    }
                }
            }
        }
    }

    Some("1571300499410386975".to_string())
}

fn get_large_image() -> String {
    #[cfg(target_os = "windows")]
    {
        if let Ok(local_app_data) = std::env::var("LOCALAPPDATA") {
            let path = std::path::Path::new(&local_app_data).join("prime").join("discord.json");
            if let Ok(content) = std::fs::read_to_string(&path) {
                if let Ok(val) = serde_json::from_str::<serde_json::Value>(&content) {
                    if let Some(img) = val.get("large_image").and_then(|v| v.as_str()) {
                        let trimmed = img.trim().to_string();
                        if !trimmed.is_empty() {
                            return trimmed;
                        }
                    }
                }
            }
        }
    }

    "primedesctop".to_string()
}

fn get_details() -> String {
    #[cfg(target_os = "windows")]
    {
        if let Ok(local_app_data) = std::env::var("LOCALAPPDATA") {
            let path = std::path::Path::new(&local_app_data).join("prime").join("discord.json");
            if let Ok(content) = std::fs::read_to_string(&path) {
                if let Ok(val) = serde_json::from_str::<serde_json::Value>(&content) {
                    if let Some(d) = val.get("details").and_then(|v| v.as_str()) {
                        let trimmed = d.trim().to_string();
                        if !trimmed.is_empty() {
                            return trimmed;
                        }
                    }
                }
            }
        }
    }

    "SpherePrime".to_string()
}

fn connect_ipc() -> Option<Box<dyn ReadWrite>> {
    #[cfg(target_os = "windows")]
    {
        for i in 0..10 {
            let path = format!(r"\\.\pipe\discord-ipc-{}", i);
            if let Ok(file) = std::fs::OpenOptions::new().read(true).write(true).open(&path) {
                return Some(Box::new(file));
            }
        }
    }
    #[cfg(not(target_os = "windows"))]
    {
        let runtime_dir = std::env::var("XDG_RUNTIME_DIR")
            .unwrap_or_else(|_| std::env::var("TMPDIR").unwrap_or_else(|_| "/tmp".to_string()));
        let base_paths = vec![
            runtime_dir,
            "/var/run".to_string(),
            "/tmp".to_string(),
        ];
        for base in base_paths {
            for i in 0..10 {
                let path = format!("{}/discord-ipc-{}", base, i);
                if let Ok(stream) = std::os::unix::net::UnixStream::connect(&path) {
                    return Some(Box::new(stream));
                }
            }
        }
    }
    None
}

trait ReadWrite: Read + Write {}
impl<T: Read + Write> ReadWrite for T {}

fn handshake(stream: &mut dyn ReadWrite, client_id: &str) -> Result<(), Box<dyn std::error::Error>> {
    let payload = format!(r#"{{"v":1,"client_id":"{}"}}"#, client_id);
    let opcode: u32 = 0;
    let length = payload.len() as u32;

    let mut buf = Vec::new();
    buf.extend_from_slice(&opcode.to_le_bytes());
    buf.extend_from_slice(&length.to_le_bytes());
    buf.extend_from_slice(payload.as_bytes());

    stream.write_all(&buf)?;
    stream.flush()?;

    let mut header = [0u8; 8];
    stream.read_exact(&mut header)?;
    let res_len = u32::from_le_bytes([header[4], header[5], header[6], header[7]]) as usize;
    let mut res_buf = vec![0u8; res_len];
    stream.read_exact(&mut res_buf)?;

    if let Ok(val) = serde_json::from_slice::<serde_json::Value>(&res_buf) {
        if val.get("code").is_some() || val.get("evt").and_then(|e| e.as_str()) == Some("ERROR") {
            return Err(format!("Discord handshake error: {}", String::from_utf8_lossy(&res_buf)).into());
        }
    }

    Ok(())
}

fn update_activity(
    stream: &mut dyn ReadWrite,
    pid: u32,
    launch_time: u64,
) -> Result<(), Box<dyn std::error::Error>> {
    let nonce = uuid::Uuid::new_v4().to_string();
    let large_image = get_large_image();
    let details = get_details();
    let activity_json = serde_json::json!({
        "cmd": "SET_ACTIVITY",
        "args": {
            "pid": pid,
            "activity": {
                "details": details,
                "state": "В приложении",
                "timestamps": {
                    "start": launch_time
                },
                "assets": {
                    "large_image": large_image,
                    "large_text": details
                },
                "buttons": [
                    {
                        "label": "Открыть сайт",
                        "url": "https://sphereprime.online"
                    }
                ]
            }
        },
        "nonce": nonce
    });

    let payload = activity_json.to_string();
    let opcode: u32 = 1;
    let length = payload.len() as u32;

    let mut buf = Vec::new();
    buf.extend_from_slice(&opcode.to_le_bytes());
    buf.extend_from_slice(&length.to_le_bytes());
    buf.extend_from_slice(payload.as_bytes());

    stream.write_all(&buf)?;
    stream.flush()?;

    let mut header = [0u8; 8];
    stream.read_exact(&mut header)?;
    let res_len = u32::from_le_bytes([header[4], header[5], header[6], header[7]]) as usize;
    let mut res_buf = vec![0u8; res_len];
    stream.read_exact(&mut res_buf)?;

    Ok(())
}
