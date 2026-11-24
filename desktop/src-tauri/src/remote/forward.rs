use super::connection::Connection;
use super::runtime;
use crate::error::{HostError, HostResult};
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    net::TcpListener,
    sync::{
        atomic::{AtomicBool, AtomicUsize, Ordering},
        Arc, Mutex,
    },
    time::{Duration, Instant},
};
use tokio::io::AsyncWriteExt;
use tokio::net::{TcpListener as TokioTcpListener, TcpStream as TokioTcpStream};
use tokio::sync::watch;

pub type Connector = Arc<dyn Fn() -> HostResult<Connection> + Send + Sync>;

struct Forward {
    snapshot: Value,
    stop: Arc<AtomicBool>,
    stop_tx: watch::Sender<bool>,
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
        let _ = self.stop_tx.send(true);
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
        if self.items.lock().unwrap().len() >= 8 {
            return Err(HostError::new(
                "SSH_FORWARD_LIMIT",
                "Maximum eight SSH forwards",
            ));
        }
        let probe = connect()?;
        let probe_channel = probe.open_direct_tcpip(host, remote_port)?;
        runtime::block_on(async {
            let _ = probe_channel.close().await;
        });
        probe.close();
        let listener = TcpListener::bind(("127.0.0.1", local_port))?;
        listener.set_nonblocking(true)?;
        let local_port = listener.local_addr()?.port();
        let id = uuid::Uuid::new_v4().to_string();
        let (stop_tx, stop_rx) = watch::channel(false);
        let forward = Arc::new(Forward {
            snapshot: json!({"id":id,"profileId":profile,"localPort":local_port,"remotePort":remote_port,"remoteHost":host,"url":format!("http://127.0.0.1:{local_port}")}),
            stop: Arc::new(AtomicBool::new(false)),
            stop_tx,
            active: Arc::new(AtomicUsize::new(0)),
            listening: Arc::new(AtomicBool::new(true)),
            error: Arc::new(Mutex::new(None)),
        });
        {
            let mut items = self.items.lock().unwrap();
            if items.len() >= 8 {
                return Err(HostError::new(
                    "SSH_FORWARD_LIMIT",
                    "Maximum eight SSH forwards",
                ));
            }
            items.insert(id, forward.clone());
        }
        let worker = forward.clone();
        let host = host.to_owned();
        runtime::runtime().spawn(async move {
            let mut stop_rx = stop_rx;
            let listener = match TokioTcpListener::from_std(listener) {
                Ok(listener) => listener,
                Err(error) => {
                    *worker.error.lock().unwrap() = Some(error.to_string());
                    worker.stop.store(true, Ordering::Release);
                    let _ = worker.stop_tx.send(true);
                    worker.listening.store(false, Ordering::Release);
                    return;
                }
            };
            while !worker.stop.load(Ordering::Acquire) {
                tokio::select! {
                    _ = stop_rx.wait_for(|&stopped| stopped) => break,
                    accept_res = listener.accept() => {
                        match accept_res {
                            Ok((stream, _)) => {
                                if worker.active.load(Ordering::Acquire) >= 16 {
                                    drop(stream);
                                    *worker.error.lock().unwrap() =
                                        Some("SSH forwarding connection limit reached".into());
                                    continue;
                                }
                                worker.active.fetch_add(1, Ordering::AcqRel);
                                let worker = worker.clone();
                                let connect = connect.clone();
                                let host = host.clone();
                                let mut conn_stop_rx = worker.stop_tx.subscribe();
                                runtime::runtime().spawn(async move {
                                    if let Err(error) = bridge(
                                        stream,
                                        host,
                                        remote_port,
                                        connect,
                                        &mut conn_stop_rx,
                                    )
                                    .await
                                    {
                                        if !worker.stop.load(Ordering::Acquire) {
                                            *worker.error.lock().unwrap() = Some(error.message);
                                        }
                                    }
                                    worker.active.fetch_sub(1, Ordering::AcqRel);
                                });
                            }
                            Err(error) => {
                                *worker.error.lock().unwrap() = Some(error.to_string());
                                worker.stop.store(true, Ordering::Release);
                                let _ = worker.stop_tx.send(true);
                                break;
                            }
                        }
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

async fn bridge(
    mut local: TokioTcpStream,
    host: String,
    port: u16,
    connect: Connector,
    stop_rx: &mut watch::Receiver<bool>,
) -> HostResult<()> {
    let _ = local.set_nodelay(true);
    let connection = tokio::task::spawn_blocking(move || connect())
        .await
        .map_err(|error| HostError::new("SSH_FORWARD_CONNECT", error.to_string()))??;
    let channel = connection.open_direct_tcpip_async(&host, port).await?;
    let mut remote = channel.into_stream();
    let result = tokio::select! {
        _ = stop_rx.wait_for(|&stopped| stopped) => Ok(()),
        copy_res = tokio::io::copy_bidirectional(&mut local, &mut remote) => {
            copy_res.map(|_| ()).map_err(HostError::from)
        }
    };
    let _ = local.shutdown().await;
    let _ = remote.shutdown().await;
    connection.close_async().await;
    result
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::{io::Read, net::TcpStream};

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
