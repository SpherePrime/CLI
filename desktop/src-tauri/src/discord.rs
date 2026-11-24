use std::io::{Read, Write};
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

const MAX_IPC_RESPONSE: usize = 1024 * 1024;

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

fn config_path() -> Option<PathBuf> {
    #[cfg(target_os = "windows")]
    {
        std::env::var("LOCALAPPDATA")
            .ok()
            .map(|dir| std::path::Path::new(&dir).join("prime").join("discord.json"))
    }
    #[cfg(not(target_os = "windows"))]
    {
        std::env::var("HOME").ok().map(|home| {
            std::path::Path::new(&home)
                .join(".config")
                .join("prime")
                .join("discord.json")
        })
    }
}

fn config_string(key: &str) -> Option<String> {
    let content = std::fs::read_to_string(config_path()?).ok()?;
    let val = serde_json::from_str::<serde_json::Value>(&content).ok()?;
    let trimmed = val.get(key)?.as_str()?.trim().to_string();
    (!trimmed.is_empty()).then_some(trimmed)
}

fn get_client_id() -> Option<String> {
    if let Ok(id) = std::env::var("PRIME_DISCORD_CLIENT_ID") {
        let trimmed = id.trim().to_string();
        if !trimmed.is_empty() {
            return Some(trimmed);
        }
    }
    config_string("client_id").or_else(|| Some("1571300499410386975".to_string()))
}

fn get_large_image() -> String {
    config_string("large_image").unwrap_or_else(|| "primedesctop".to_string())
}

fn get_details() -> String {
    config_string("details").unwrap_or_else(|| "SpherePrime".to_string())
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

fn read_response(stream: &mut dyn ReadWrite) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    let mut header = [0u8; 8];
    stream.read_exact(&mut header)?;
    let res_len = u32::from_le_bytes([header[4], header[5], header[6], header[7]]) as usize;
    if res_len > MAX_IPC_RESPONSE {
        return Err("Discord IPC response exceeds maximum size".into());
    }
    let mut res_buf = vec![0u8; res_len];
    stream.read_exact(&mut res_buf)?;
    Ok(res_buf)
}

fn handshake(stream: &mut dyn ReadWrite, client_id: &str) -> Result<(), Box<dyn std::error::Error>> {
    let payload = serde_json::json!({
        "v": 1,
        "client_id": client_id
    })
    .to_string();
    let opcode: u32 = 0;
    let length = payload.len() as u32;

    let mut buf = Vec::new();
    buf.extend_from_slice(&opcode.to_le_bytes());
    buf.extend_from_slice(&length.to_le_bytes());
    buf.extend_from_slice(payload.as_bytes());

    stream.write_all(&buf)?;
    stream.flush()?;

    let res_buf = read_response(stream)?;

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

    let _ = read_response(stream)?;

    Ok(())
}
