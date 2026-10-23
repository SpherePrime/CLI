use super::*;
use std::io::{Read, Write};

#[test]
fn text_normalization_language_and_utf16_indexing_match_pipeline() {
    assert_eq!(text::detect_language("Привет"), "ru");
    assert_eq!(text::detect_language("Hello world"), "en");
    assert_eq!(text::detect_language("English і"), "uk");
    assert_eq!(
        text::prepare("«Привет» 🚀 @ user", "ru"),
        "<ru>«Привет» at user.</ru>"
    );
    assert_eq!(
        text::prepare("Hello ,  world -- \n\"\"hi\"\"", "en"),
        "<en>Hello, world -- \"hi\"</en>"
    );
    assert_eq!(text::prepare("café", "en"), "<en>cafe\u{301}.</en>");
    let mut indexer = vec![0; 0xd802];
    indexer[0xd801] = 777;
    assert_eq!(text::text_codes("𐐀", &indexer), vec![777]);
}

#[test]
fn wav_headers_rounding_clipping_and_nonfinite_rejection() {
    let wav = audio::encode_wav(&[-2.0, -0.5, 0.0, 0.5, 2.0], 44100).unwrap();
    assert_eq!(&wav[0..4], b"RIFF");
    assert_eq!(&wav[8..12], b"WAVE");
    assert_eq!(u32::from_le_bytes(wav[24..28].try_into().unwrap()), 44100);
    assert_eq!(u32::from_le_bytes(wav[40..44].try_into().unwrap()), 10);
    let samples = wav[44..]
        .chunks_exact(2)
        .map(|bytes| i16::from_le_bytes(bytes.try_into().unwrap()))
        .collect::<Vec<_>>();
    assert_eq!(samples, [-32767, -16383, 0, 16384, 32767]);
    assert!(audio::encode_wav(&[f32::NAN], 44100).is_err());
}

#[test]
fn voice_import_validation_list_remove_and_paths() {
    let directory = tempfile::tempdir().unwrap();
    let model = directory.path().join("supertonic-3");
    let source = directory.path().join("Мой голос.json");
    let style = json!({"style_ttl":{"dims":[1,2,2],"data":[[[1,2],[3,4]]]},"style_dp":{"dims":[1,1,2],"data":[[[0.5,0.25]]]}});
    std::fs::write(&source, serde_json::to_vec(&style).unwrap()).unwrap();
    let voice = voices::import(&model, &source).unwrap();
    assert!(voice.id.starts_with("custom:"));
    assert!(voice.custom);
    assert!(voices::list(&model)
        .unwrap()
        .iter()
        .any(|candidate| candidate.id == voice.id));
    let loaded = voices::read(&model, &voice.id).unwrap();
    assert_eq!(loaded.ttl.data, vec![1.0, 2.0, 3.0, 4.0]);
    assert!(voices::read(&model, "custom:../private").is_err());
    assert!(voices::remove(&model, "F1").is_err());
    assert!(voices::parse_style(
        &json!({"style_ttl":{"dims":[1,2,2],"data":[1]},"style_dp":style["style_dp"]})
    )
    .is_err());
    voices::remove(&model, &voice.id).unwrap();
    assert_eq!(voices::list(&model).unwrap().len(), 4);
    assert_eq!(voices::slugify("Hello world!"), "hello-world");
}

#[test]
fn pinned_asset_integrity_checks_contents_not_only_size() {
    let directory = tempfile::tempdir().unwrap();
    let file = directory.path().join("asset");
    std::fs::write(&file, b"hello").unwrap();
    let asset = assets::Asset {
        path: "asset",
        size: 5,
        digest: "b6fc4c620b67d95f953a5c1c1230aaab5db5a1b0",
        blob: true,
    };
    assert!(assets::verify(&file, &asset).unwrap());
    std::fs::write(&file, b"jello").unwrap();
    assert!(!assets::verify(&file, &asset).unwrap());
}

#[test]
fn stalled_http_download_cancels_without_helper_processes() {
    let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    let server = std::thread::spawn(move || {
        let (mut stream, _) = listener.accept().unwrap();
        let mut buffer = [0; 1024];
        let _ = stream.read(&mut buffer);
        stream
            .write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 1\r\nConnection: close\r\n\r\n")
            .unwrap();
        std::thread::sleep(Duration::from_secs(1));
    });
    let stop = Arc::new(AtomicBool::new(false));
    let cancel = stop.clone();
    let cancellation = std::thread::spawn(move || {
        std::thread::sleep(Duration::from_millis(100));
        cancel.store(true, Ordering::Release);
    });
    let start = Instant::now();
    let mut target = Vec::new();
    let result = assets::download(
        &assets::client().unwrap(),
        &format!("http://{address}"),
        &mut target,
        1,
        &stop,
        |_| {},
    );
    assert!(result.is_err());
    assert!(start.elapsed() < Duration::from_millis(900));
    cancellation.join().unwrap();
    server.join().unwrap();
}

#[test]
fn cached_supertonic_inference_outputs_wav_speed_and_idle_release() {
    if !cfg!(all(windows, target_arch = "x86_64")) {
        eprintln!("Cached inference requires configured Windows x64 native runtime");
        return;
    }
    let directory = assets::model_directory();
    if !assets::ready(&directory) {
        eprintln!("Cached Supertonic models unavailable; actual inference fixture not run");
        return;
    }
    for asset in assets::ASSETS {
        assert!(
            assets::verify(&directory.join(asset.path), asset).unwrap(),
            "Cached asset mismatch: {}",
            asset.path
        );
    }
    let service = SpeechService::with_directory(directory, Duration::from_millis(200));
    let fast = service
        .request(
            json!({"action":"speak","text":"Привет.","voice":"F1","speed":1.6}),
            Arc::new(|_| {}),
        )
        .unwrap();
    let fast = base64::engine::general_purpose::STANDARD
        .decode(fast["audio"].as_str().unwrap())
        .unwrap();
    assert!(service.loaded());
    assert_eq!(&fast[8..12], b"WAVE");
    assert!(fast.len() > 4410);
    let energy: f64 = fast[44..]
        .chunks_exact(2)
        .map(|bytes| {
            let value = i16::from_le_bytes(bytes.try_into().unwrap()) as f64;
            value * value
        })
        .sum();
    assert!(energy > 1000.0, "Inference waveform is silent");
    let slow = service
        .request(
            json!({"action":"speak","text":"Привет.","voice":"F1","speed":0.7}),
            Arc::new(|_| {}),
        )
        .unwrap();
    let slow = base64::engine::general_purpose::STANDARD
        .decode(slow["audio"].as_str().unwrap())
        .unwrap();
    assert!(
        slow.len() > fast.len() * 18 / 10,
        "Speed did not change audio duration"
    );
    let deadline = Instant::now() + Duration::from_secs(5);
    while service.loaded() {
        assert!(
            Instant::now() < deadline,
            "Idle model sessions were not unloaded"
        );
        std::thread::sleep(Duration::from_millis(20));
    }
    service.shutdown();
    assert!(service
        .request(json!({"action":"prepare"}), Arc::new(|_| {}))
        .is_err());
}
