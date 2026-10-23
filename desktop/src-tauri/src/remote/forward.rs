use super::connection::Connection;
use crate::error::{HostError, HostResult};
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    io::{Read, Write},
    net::{Shutdown, TcpListener, TcpStream},
    sync::{
        atomic::{AtomicBool, AtomicUsize, Ordering},
        Arc, Mutex,
    },
    time::{Duration, Instant},
};

pub type Connector = Arc<dyn Fn() -> HostResult<Connection> + Send + Sync>;
const BUFFER_LIMIT: usize = 64 * 1024;

struct Forward {
    snapshot: Value,
    stop: Arc<AtomicBool>,
    active: Arc<AtomicUsize>,
    listening: Arc<AtomicBool>,
    error: Arc<Mutex<Option<String>>>,
}

impl Forward {
    fn snapshot(&self) -> Value {
        let mut value = self.snapshot.clone();
        value["activeConnections"] = json!(self.active.load(Ordering::Acquire));
        value["status"] = json!(if self.stop.load(Ordering::Acquire) {
            "closed"
        } else {
            "listening"
        });
        if let Some(error) = self.error.lock().unwrap().as_ref() {
            value["error"] = json!(error);
        }
        value
    }

    fn close(&self) -> HostResult<()> {
        self.stop.store(true, Ordering::Release);
        let deadline = Instant::now() + Duration::from_secs(20);
        while self.active.load(Ordering::Acquire) > 0 || self.listening.load(Ordering::Acquire) {
            if Instant::now() > deadline {
                return Err(HostError::new(
                    "SSH_FORWARD_TIMEOUT",
                    "SSH forwarding connections did not close",
                ));
            }
            std::thread::sleep(Duration::from_millis(10));
        }
        Ok(())
    }
}

#[derive(Default)]
pub struct ForwardManager {
    items: Mutex<HashMap<String, Arc<Forward>>>,
}

impl ForwardManager {
    pub fn open(
        &self,
        profile: &str,
        remote_port: u16,
        local_port: u16,
        host: &str,
        connect: Connector,
    ) -> HostResult<Value> {
        if remote_port == 0 || !["127.0.0.1", "localhost", "::1"].contains(&host) {
            return Err(HostError::new(
                "SSH_FORWARD_ARGUMENT",
                "Remote forwarding target must be a loopback host and nonzero port",
            ));
        }
        let mut items = self.items.lock().unwrap();
        if items.len() >= 8 {
            return Err(HostError::new(
                "SSH_FORWARD_LIMIT",
                "Maximum eight SSH forwards",
            ));
        }
        let probe = connect()?;
        probe
            .session
            .channel_direct_tcpip(host, remote_port, None)
            .map_err(|error| HostError::new("SSH_FORWARD_CONNECT", error.to_string()))?
            .close()
            .map_err(|error| HostError::new("SSH_FORWARD_CONNECT", error.to_string()))?;
        probe.close();
        let listener = TcpListener::bind(("127.0.0.1", local_port))?;
        listener.set_nonblocking(true)?;
        let local_port = listener.local_addr()?.port();
        let id = uuid::Uuid::new_v4().to_string();
        let forward = Arc::new(Forward {
            snapshot: json!({"id":id,"profileId":profile,"localPort":local_port,"remotePort":remote_port,"remoteHost":host,"url":format!("http://127.0.0.1:{local_port}")}),
            stop: Arc::new(AtomicBool::new(false)),
            active: Arc::new(AtomicUsize::new(0)),
            listening: Arc::new(AtomicBool::new(true)),
            error: Arc::new(Mutex::new(None)),
        });
        items.insert(id, forward.clone());
        drop(items);
        let worker = forward.clone();
        let host = host.to_owned();
        std::thread::spawn(move || {
            while !worker.stop.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((stream, _)) => {
                        if worker.active.load(Ordering::Acquire) >= 16 {
                            let _ = stream.shutdown(Shutdown::Both);
                            *worker.error.lock().unwrap() =
                                Some("SSH forwarding connection limit reached".into());
                            continue;
                        }
                        worker.active.fetch_add(1, Ordering::AcqRel);
                        let worker = worker.clone();
                        let connect = connect.clone();
                        let host = host.clone();
                        std::thread::spawn(move || {
                            if let Err(error) =
                                bridge(stream, &host, remote_port, connect, &worker.stop)
                            {
                                if !worker.stop.load(Ordering::Acquire) {
                                    *worker.error.lock().unwrap() = Some(error.message);
                                }
                            }
                            worker.active.fetch_sub(1, Ordering::AcqRel);
                        });
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(20))
                    }
                    Err(error) => {
                        *worker.error.lock().unwrap() = Some(error.to_string());
                        worker.stop.store(true, Ordering::Release);
                        break;
                    }
                }
            }
            drop(listener);
            worker.listening.store(false, Ordering::Release);
        });
        Ok(forward.snapshot())
    }

    pub fn list(&self) -> Value {
        json!(self
            .items
            .lock()
            .unwrap()
            .values()
            .map(|forward| forward.snapshot())
            .collect::<Vec<_>>())
    }

    pub fn close(&self, id: &str) -> HostResult<Value> {
        let forward = self
            .items
            .lock()
            .unwrap()
            .get(id)
            .cloned()
            .ok_or_else(|| HostError::new("SSH_FORWARD_NOT_FOUND", "SSH forward not found"))?;
        forward.close()?;
        self.items.lock().unwrap().remove(id);
        Ok(Value::Null)
    }

    pub fn close_owned(&self, profile: &str, id: &str) -> HostResult<Value> {
        let forward = self
            .items
            .lock()
            .unwrap()
            .get(id)
            .cloned()
            .ok_or_else(|| HostError::new("SSH_FORWARD_NOT_FOUND", "SSH forward not found"))?;
        if forward.snapshot["profileId"] != profile {
            return Err(HostError::new(
                "SSH_FORWARD_SCOPE",
                "SSH forward belongs to another profile",
            ));
        }
        self.close(id)
    }

    pub fn close_profile(&self, profile: &str) {
        let ids = self
            .items
            .lock()
            .unwrap()
            .values()
            .filter(|forward| forward.snapshot["profileId"] == profile)
            .filter_map(|forward| forward.snapshot["id"].as_str().map(str::to_owned))
            .collect::<Vec<_>>();
        for id in ids {
            let _ = self.close(&id);
        }
    }

    pub fn shutdown(&self) {
        let ids = self
            .items
            .lock()
            .unwrap()
            .keys()
            .cloned()
            .collect::<Vec<_>>();
        for id in ids {
            let _ = self.close(&id);
        }
    }
}

fn bridge(
    mut local: TcpStream,
    host: &str,
    port: u16,
    connect: Connector,
    stop: &AtomicBool,
) -> HostResult<()> {
    local.set_nonblocking(true)?;
    local.set_nodelay(true)?;
    let connection = connect()?;
    let mut remote = connection
        .session
        .channel_direct_tcpip(host, port, None)
        .map_err(|error| HostError::new("SSH_FORWARD_CONNECT", error.to_string()))?;
    connection.session.set_blocking(false);
    let mut to_remote = Vec::new();
    let mut to_local = Vec::new();
    let mut buffer = [0u8; 16384];
    let mut local_eof = false;
    let mut sent_eof = false;
    let result = (|| {
        while !stop.load(Ordering::Acquire) {
            let mut progressed = false;
            if !local_eof && to_remote.len() < BUFFER_LIMIT {
                match local.read(&mut buffer) {
                    Ok(0) => local_eof = true,
                    Ok(count) => {
                        to_remote.extend_from_slice(&buffer[..count]);
                        progressed = true;
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
                    Err(error) => return Err(error.into()),
                }
            }
            if !to_remote.is_empty() {
                match remote.write(&to_remote) {
                    Ok(0) => {
                        return Err(HostError::new(
                            "SSH_FORWARD_CLOSED",
                            "SSH channel closed during write",
                        ))
                    }
                    Ok(count) => {
                        to_remote.drain(..count);
                        progressed = true;
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
                    Err(error) => return Err(error.into()),
                }
            }
            if local_eof && to_remote.is_empty() && !sent_eof {
                match remote.send_eof() {
                    Ok(()) => sent_eof = true,
                    Err(error) if error.code() == ssh2::ErrorCode::Session(-37) => {}
                    Err(error) => {
                        return Err(HostError::new("SSH_FORWARD_CLOSED", error.to_string()))
                    }
                }
            }
            if to_local.len() < BUFFER_LIMIT {
                match remote.read(&mut buffer) {
                    Ok(count) => {
                        to_local.extend_from_slice(&buffer[..count]);
                        progressed |= count > 0;
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
                    Err(error) => return Err(error.into()),
                }
            }
            if !to_local.is_empty() {
                match local.write(&to_local) {
                    Ok(0) => return Ok(()),
                    Ok(count) => {
                        to_local.drain(..count);
                        progressed = true;
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
                    Err(error) => return Err(error.into()),
                }
            }
            if remote.eof() && to_local.is_empty() {
                break;
            }
            if !progressed {
                std::thread::sleep(Duration::from_millis(5));
            }
        }
        Ok(())
    })();
    let _ = local.shutdown(Shutdown::Both);
    connection.session.set_timeout(1000);
    connection.session.set_blocking(true);
    let _ = remote.close();
    connection.close();
    result
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    #[ignore]
    fn native_loopback_forward_round_trip() {
        use crate::remote::{
            connection::{handshake, Handshake},
            types::{Credentials, Profile},
        };
        let key = std::env::var("PRIME_NATIVE_SSH_TEST_KEY")
            .expect("Set PRIME_NATIVE_SSH_TEST_KEY to an isolated SSH fixture identity");
        let root = std::env::var("PRIME_NATIVE_SSH_TEST_ROOT")
            .unwrap_or_else(|_| "/tmp/prime-native-ssh-fixture/project".into());
        let port = std::env::var("PRIME_NATIVE_SSH_TEST_PORT")
            .unwrap_or_else(|_| "22349".into())
            .parse::<u16>()
            .unwrap();
        let profile=serde_json::from_value::<Profile>(json!({"name":"Fixture","host":"127.0.0.1","port":port,"user":"root","root":root,"auth":"key","keyPath":key,"runtime":"bundled"})).unwrap().validate().unwrap();
        let profile_id = profile.id.clone();
        let fingerprint = match handshake(&profile, None).unwrap() {
            Handshake::Challenge(challenge) => challenge.fingerprint,
            _ => panic!("Unknown host was trusted"),
        };
        let connect: Connector = Arc::new(move || {
            let session = match handshake(&profile, Some(&fingerprint))? {
                Handshake::Trusted(session) => session,
                _ => return Err(HostError::new("SSH_HOST_KEY", "Fixture host key changed")),
            };
            Connection::authenticate(profile.clone(), session, &Credentials::default())
        });
        let manager = ForwardManager::default();
        let forward = manager
            .open(&profile_id, port, 0, "127.0.0.1", connect)
            .unwrap();
        let local_port = forward["localPort"].as_u64().unwrap() as u16;
        let mut stream = TcpStream::connect(("127.0.0.1", local_port)).unwrap();
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap();
        let mut buffer = [0u8; 256];
        let count = stream.read(&mut buffer).unwrap();
        assert!(std::str::from_utf8(&buffer[..count])
            .unwrap()
            .starts_with("SSH-2.0-"));
        drop(stream);
        manager.close(forward["id"].as_str().unwrap()).unwrap();
        assert!(TcpStream::connect_timeout(
            &format!("127.0.0.1:{local_port}").parse().unwrap(),
            Duration::from_millis(200)
        )
        .is_err());
        assert!(manager.list().as_array().unwrap().is_empty());
    }
}
