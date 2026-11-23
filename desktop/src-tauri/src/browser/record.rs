use super::{panel::BrowserState, platform, rendering};
use base64::Engine;
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    fs,
    path::{Path, PathBuf},
    process::Command,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex, OnceLock,
    },
    thread::{self, JoinHandle},
    time::{Duration, Instant, SystemTime, UNIX_EPOCH},
};
use tauri::{AppHandle, Webview};

const DEFAULT_FPS: u64 = 10;
const MIN_FPS: u64 = 1;
const MAX_FPS: u64 = 30;
const MAX_FRAMES: usize = 6000;
const MAX_CONSECUTIVE_ERRORS: usize = 10;

struct RecordingSession {
    recording_id: String,
    out_dir: PathBuf,
    frames_dir: PathBuf,
    fps: u64,
    started_at: Instant,
    stop_flag: Arc<AtomicBool>,
    join_handle: JoinHandle<RecordingOutcome>,
}

struct RecordingOutcome {
    frame_count: usize,
    last_error: Option<String>,
}

fn active_recordings() -> &'static Mutex<HashMap<String, RecordingSession>> {
    static SESSIONS: OnceLock<Mutex<HashMap<String, RecordingSession>>> = OnceLock::new();
    SESSIONS.get_or_init(Default::default)
}

pub fn normalize_fps(raw: Option<u64>) -> Result<u64, String> {
    let fps = raw.unwrap_or(DEFAULT_FPS);
    if !(MIN_FPS..=MAX_FPS).contains(&fps) {
        return Err(format!("fps must be between {MIN_FPS} and {MAX_FPS}"));
    }
    Ok(fps)
}

pub fn start(
    app: &AppHandle,
    state: &BrowserState,
    tab_id: &str,
    view: Webview,
    raw_fps: Option<u64>,
) -> Result<Value, String> {
    let fps = normalize_fps(raw_fps)?;
    {
        let sessions = active_recordings()
            .lock()
            .map_err(|error| error.to_string())?;
        if let Some(existing) = sessions.get(tab_id) {
            return Err(format!(
                "Video recording {} is already running for this tab. Call browser_record_stop before starting another recording.",
                existing.recording_id
            ));
        }
    }

    let now_ms = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis();
    let short_id: String = uuid::Uuid::new_v4()
        .to_string()
        .chars()
        .take(8)
        .collect();
    let recording_id = format!("rec-{short_id}");
    let out_dir = std::env::temp_dir().join(format!("prime-gamedev-browser-{now_ms}-{short_id}"));
    let frames_dir = out_dir.join("frames");
    fs::create_dir_all(&frames_dir).map_err(|error| error.to_string())?;

    let rendering_guard = rendering::keep_rendering(app, state, tab_id)?;
    let stop_flag = Arc::new(AtomicBool::new(false));
    let thread_stop = stop_flag.clone();
    let thread_frames_dir = frames_dir.clone();
    let frame_interval = Duration::from_millis((1000 / fps).max(16));

    let join_handle = thread::spawn(move || {
        let _guard = rendering_guard;
        let mut frame_count = 0usize;
        let mut consecutive_errors = 0usize;
        let mut last_error = None;

        while !thread_stop.load(Ordering::Acquire) && frame_count < MAX_FRAMES {
            let tick_start = Instant::now();
            match platform::screenshot(&view, None) {
                Ok(shot) => {
                    if let Some(encoded) = shot["data"].as_str().filter(|data| !data.is_empty()) {
                        match base64::engine::general_purpose::STANDARD.decode(encoded) {
                            Ok(bytes) => {
                                frame_count += 1;
                                let frame_path =
                                    thread_frames_dir.join(format!("frame_{frame_count:06}.png"));
                                if let Err(error) = fs::write(&frame_path, bytes) {
                                    last_error = Some(error.to_string());
                                    break;
                                }
                                consecutive_errors = 0;
                            }
                            Err(error) => {
                                consecutive_errors += 1;
                                last_error = Some(error.to_string());
                            }
                        }
                    } else {
                        consecutive_errors += 1;
                        last_error = Some("Browser returned an empty frame".to_owned());
                    }
                }
                Err(error) => {
                    consecutive_errors += 1;
                    last_error = Some(error);
                }
            }

            if consecutive_errors >= MAX_CONSECUTIVE_ERRORS {
                break;
            }

            while !thread_stop.load(Ordering::Acquire) {
                let elapsed = tick_start.elapsed();
                if elapsed >= frame_interval {
                    break;
                }
                let remaining = frame_interval - elapsed;
                thread::sleep(remaining.min(Duration::from_millis(25)));
            }
        }

        RecordingOutcome {
            frame_count,
            last_error,
        }
    });

    let mut sessions = active_recordings()
        .lock()
        .map_err(|error| error.to_string())?;
    if sessions.contains_key(tab_id) {
        stop_flag.store(true, Ordering::Release);
        let _ = join_handle.join();
        return Err("Video recording is already running for this tab".to_owned());
    }
    sessions.insert(
        tab_id.to_owned(),
        RecordingSession {
            recording_id: recording_id.clone(),
            out_dir: out_dir.clone(),
            frames_dir: frames_dir.clone(),
            fps,
            started_at: Instant::now(),
            stop_flag,
            join_handle,
        },
    );

    Ok(json!({
        "recordingId": recording_id,
        "tabId": tab_id,
        "status": "recording",
        "fps": fps,
        "outDir": out_dir.to_string_lossy(),
        "framesDir": frames_dir.to_string_lossy(),
    }))
}

pub fn stop(tab_id: &str) -> Result<Value, String> {
    let session = {
        let mut sessions = active_recordings()
            .lock()
            .map_err(|error| error.to_string())?;
        sessions.remove(tab_id).ok_or_else(|| {
            "No active video recording for this browser tab. Call browser_record_start first."
                .to_owned()
        })?
    };

    session.stop_flag.store(true, Ordering::Release);
    let outcome = session
        .join_handle
        .join()
        .map_err(|_| "Video recording thread panicked".to_owned())?;
    let duration_sec = session.started_at.elapsed().as_secs_f64().max(0.1);

    if outcome.frame_count == 0 {
        return Err(outcome
            .last_error
            .unwrap_or_else(|| "Recorded zero frames from browser tab".to_owned()));
    }

    let actual_fps = ((outcome.frame_count as f64) / duration_sec).clamp(1.0, MAX_FPS as f64);
    let video_path = session.out_dir.join("playthrough.mp4");
    let encode_result = encode_frames_to_mp4(&session.frames_dir, &video_path, actual_fps);

    let (video_path_value, ffmpeg_error) = match encode_result {
        Ok(()) => (json!(video_path.to_string_lossy()), Value::Null),
        Err(error) => (Value::Null, json!(error)),
    };

    Ok(json!({
        "recordingId": session.recording_id,
        "tabId": tab_id,
        "status": "stopped",
        "outDir": session.out_dir.to_string_lossy(),
        "framesDir": session.frames_dir.to_string_lossy(),
        "videoPath": video_path_value,
        "frameCount": outcome.frame_count,
        "requestedFps": session.fps,
        "actualFps": (actual_fps * 100.0).round() / 100.0,
        "durationSec": (duration_sec * 100.0).round() / 100.0,
        "ffmpegError": ffmpeg_error,
        "captureWarning": outcome.last_error,
    }))
}

pub fn stop_if_active(tab_id: &str) {
    let session = active_recordings()
        .lock()
        .ok()
        .and_then(|mut sessions| sessions.remove(tab_id));
    if let Some(session) = session {
        session.stop_flag.store(true, Ordering::Release);
        let _ = session.join_handle.join();
    }
}

fn build_encode_args(frames_dir: &Path, video_path: &Path, fps: f64) -> Vec<String> {
    let pattern = frames_dir.join("frame_%06d.png");
    vec![
        "-y".to_owned(),
        "-hide_banner".to_owned(),
        "-loglevel".to_owned(),
        "error".to_owned(),
        "-framerate".to_owned(),
        format!("{fps:.2}"),
        "-i".to_owned(),
        pattern.to_string_lossy().into_owned(),
        "-vf".to_owned(),
        "pad=ceil(iw/2)*2:ceil(ih/2)*2".to_owned(),
        "-c:v".to_owned(),
        "libx264".to_owned(),
        "-pix_fmt".to_owned(),
        "yuv420p".to_owned(),
        "-movflags".to_owned(),
        "+faststart".to_owned(),
        video_path.to_string_lossy().into_owned(),
    ]
}

fn encode_frames_to_mp4(frames_dir: &Path, video_path: &Path, fps: f64) -> Result<(), String> {
    let ffmpeg = find_ffmpeg_binary()
        .ok_or_else(|| "ffmpeg was not found; inspect framesDir directly with extract_frames.mjs --frames-dir".to_owned())?;
    let args = build_encode_args(frames_dir, video_path, fps);
    let mut command = Command::new(&ffmpeg);
    command.args(&args);
    #[cfg(target_os = "windows")]
    {
        use std::os::windows::process::CommandExt;
        command.creation_flags(0x08000000);
    }
    let output = command.output().map_err(|error| error.to_string())?;
    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!("ffmpeg failed: {}", stderr.trim()));
    }
    if !video_path.exists() {
        return Err("ffmpeg completed without creating playthrough.mp4".to_owned());
    }
    Ok(())
}

fn probe_ffmpeg(candidate: &Path) -> bool {
    let mut command = Command::new(candidate);
    command.arg("-version");
    #[cfg(target_os = "windows")]
    {
        use std::os::windows::process::CommandExt;
        command.creation_flags(0x08000000);
    }
    command
        .output()
        .map(|output| output.status.success())
        .unwrap_or(false)
}

fn find_ffmpeg_binary() -> Option<PathBuf> {
    let mut candidates = Vec::new();
    if let Ok(explicit) = std::env::var("FFMPEG_PATH") {
        if !explicit.trim().is_empty() {
            candidates.push(PathBuf::from(explicit.trim()));
        }
    }
    candidates.push(PathBuf::from("ffmpeg"));
    #[cfg(target_os = "windows")]
    {
        if let Ok(local_app_data) = std::env::var("LOCALAPPDATA") {
            let packages = PathBuf::from(local_app_data)
                .join("Microsoft")
                .join("WinGet")
                .join("Packages");
            if let Ok(entries) = fs::read_dir(packages) {
                for entry in entries.flatten() {
                    let name = entry.file_name().to_string_lossy().to_lowercase();
                    if !name.contains("ffmpeg") {
                        continue;
                    }
                    if let Ok(sub_entries) = fs::read_dir(entry.path()) {
                        for sub in sub_entries.flatten() {
                            let bin = sub.path().join("bin").join("ffmpeg.exe");
                            if bin.is_file() {
                                candidates.push(bin);
                            }
                        }
                    }
                }
            }
        }
        candidates.push(PathBuf::from(r"C:\ProgramData\chocolatey\bin\ffmpeg.exe"));
        candidates.push(PathBuf::from(r"C:\ffmpeg\bin\ffmpeg.exe"));
    }
    #[cfg(not(target_os = "windows"))]
    {
        candidates.push(PathBuf::from("/opt/homebrew/bin/ffmpeg"));
        candidates.push(PathBuf::from("/usr/local/bin/ffmpeg"));
        candidates.push(PathBuf::from("/usr/bin/ffmpeg"));
    }

    candidates.into_iter().find(|candidate| probe_ffmpeg(candidate))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn normalizes_and_validates_fps_bounds() {
        assert_eq!(normalize_fps(None).unwrap(), DEFAULT_FPS);
        assert_eq!(normalize_fps(Some(1)).unwrap(), 1);
        assert_eq!(normalize_fps(Some(30)).unwrap(), 30);
        assert!(normalize_fps(Some(0)).is_err());
        assert!(normalize_fps(Some(31)).is_err());
    }

    #[test]
    fn stop_without_active_session_returns_clear_error() {
        let err = stop("nonexistent-tab-id").unwrap_err();
        assert!(err.contains("browser_record_start"));
    }

    #[test]
    fn builds_even_padded_mp4_encode_args() {
        let frames = PathBuf::from("/tmp/rec/frames");
        let video = PathBuf::from("/tmp/rec/playthrough.mp4");
        let args = build_encode_args(&frames, &video, 12.5);
        assert!(args.contains(&"12.50".to_owned()));
        assert!(args.contains(&"pad=ceil(iw/2)*2:ceil(ih/2)*2".to_owned()));
        assert!(args.contains(&"yuv420p".to_owned()));
        assert_eq!(args.last().unwrap(), &video.to_string_lossy().into_owned());
    }
}
