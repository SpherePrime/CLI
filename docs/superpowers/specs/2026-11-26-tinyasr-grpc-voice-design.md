# TinyASR gRPC Voice Input Design

Date: 2026-11-26
Status: Approved

## 1. Goal & Scope

Replace all legacy speech-to-text providers (Google Web Speech, Vosk, Whisper.cpp, OpenAI STT, custom shell commands, CUDA/Vulkan acceleration toggles, and the multi-provider model catalog) with a single built-in local ASR engine powered by [`dkaraush/tinyasr`](https://github.com/dkaraush/tinyasr) (`en-ru-uk` Zipformer2 int4 + `silero_vad.onnx`).

Key requirements:
1. **GPL-3.0 isolation**: `dkaraush/tinyasr` and its models are licensed under `GPL-3.0-or-later`. To keep Prime CLI and Prime Desktop cleanly separated from GPL copyleft obligations, `tinyasr` runs strictly as an **independent open-source executable** (`prime-asr` / `prime-asr.exe`) hosted in its own public GPL-3.0 repository (`SpherePrime/prime-asr`). Prime Desktop and Prime CLI communicate with `prime-asr` exclusively over local **gRPC**.
2. **Works everywhere & updates cleanly**: Supported in both **Prime Desktop** (Tauri Rust host + React renderer) and **Prime CLI** (pure Go `CGO_ENABLED=0` via vendored `google.golang.org/grpc`), bundled out-of-the-box in Desktop releases and automatically kept compatible with `prime update` and Tauri updater.
3. **Instant readiness**: `en-ru-uk.tzf.gz` (~31.9 MiB) and `silero_vad.onnx.gz` (~1.9 MiB) plus the `prime-asr` binary are prepared at build time (ignored in git) and shipped inside Desktop resources. Desktop spawns and warms `prime-asr` in the background on startup (~46 MiB RAM) so microphone dictation begins with zero load delay.
4. **Live real-time transcription**: Microphone audio streams continuously (~80-100 ms chunks, 16 kHz mono s16le) over bidirectional gRPC `StreamRecognize`. In the composer, a clean live transcript panel appears smoothly directly **above** the `VoiceWave` recording bar (`Говорите…` + waveform), showing committed words clearly and active partial hypotheses slightly muted. Voice conversation (`ConversationCall`) uses the same gRPC stream with Silero VAD and end-of-turn (`eou`) events.
5. **Minimal settings**: Remove provider picker, hardware acceleration picker, and third-party model catalog. Keep only recognition language (`auto` default, `ru`, `en`, `uk`) and the local engine status indicator.

---

## 2. Separate Open-Source Repository: `SpherePrime/prime-asr`

Location: sibling directory `../prime-asr` published to `https://github.com/SpherePrime/prime-asr` under `GPL-3.0-or-later`.

### 2.1 Crate & Modules
- Rust edition 2024 crate depending on `tinyasr` (pinned git commit from `https://github.com/dkaraush/tinyasr`), `tonic`, `prost`, `tokio`, `tokio-stream`, `flate2`, `clap` (or minimal arg parser), `rand` / `getrandom`.
- Clean modular structure:
  - `proto/asr.proto` — canonical Protobuf / gRPC contract.
  - `src/main.rs` — CLI flags (`--asr`, `--vad`, `--threads`, `--idle-secs`), model loading, loopback `127.0.0.1:0` server bind, single-line JSON readiness handshake to `stdout`, graceful shutdown.
  - `src/model.rs` — gzip decompression + `tinyasr::Models` initialization and shared `Arc` holder.
  - `src/session.rs` — mapping between `tinyasr::Session` / `tinyasr::Transcription` and gRPC events (`Partial`, `Commit`, `Eou`, `Language`, `Final`), s16le PCM to `f32` conversion, language list resolution (`auto` -> `["en", "ru", "uk"]` with `start_lang`, or single pinned language).
  - `src/service.rs` — `tonic` implementation of `prime.asr.v1.AsrService` with constant-time token interceptor (`x-prime-asr-token`).
  - `src/auth.rs` — random 32-byte hex token generation and metadata verification.
- `.github/workflows/release.yml` — builds release binaries for `windows-x86_64`, `windows-aarch64`, `linux-x86_64`, `linux-aarch64`, packages `en-ru-uk.tzf.gz` and `silero_vad.onnx.gz`, and publishes `SHA256SUMS.txt` on GitHub Releases.

### 2.2 Startup Handshake & Security
1. Parent launches `prime-asr --asr <path> --vad <path> [--threads <N>]`.
2. `prime-asr` decompresses models into RAM, constructs `tinyasr::Models`, binds a TCP listener on `127.0.0.1:0`, generates a 256-bit random token, and prints one flushed JSON line to `stdout`:
   ```json
   {"addr":"127.0.0.1:54321","token":"<64-hex>","langs":["en","ru","uk"],"chunk_ms":320}
   ```
3. Every gRPC RPC requires header `x-prime-asr-token: <token>`; requests without a matching token fail with `UNAUTHENTICATED`.
4. `prime-asr` also monitors `stdin` EOF (or parent death) so if the parent process crashes abnormally, `prime-asr` terminates cleanly without lingering.

### 2.3 gRPC Contract (`prime.asr.v1.AsrService`)

```protobuf
syntax = "proto3";

package prime.asr.v1;

service AsrService {
  rpc Status(StatusRequest) returns (StatusResponse);
  rpc StreamRecognize(stream RecognizeRequest) returns (stream RecognizeEvent);
  rpc Transcribe(TranscribeRequest) returns (TranscribeResponse);
  rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);
}

message StatusRequest {}

message StatusResponse {
  bool ready = 1;
  repeated string languages = 2;
  uint32 chunk_ms = 3;
  uint32 sample_rate = 4;
}

message RecognizeRequest {
  oneof payload {
    StartSession start = 1;
    AudioChunk audio = 2;
    FinishSession finish = 3;
    CancelSession cancel = 4;
  }
}

message StartSession {
  string language = 1;
  string start_language = 2;
}

message AudioChunk {
  bytes pcm_s16le = 1;
}

message FinishSession {}
message CancelSession {}

message RecognizeEvent {
  oneof event {
    PartialTranscript partial = 1;
    CommitTranscript commit = 2;
    EndOfUtterance eou = 3;
    LanguageDetected language = 4;
    FinalTranscript final_transcript = 5;
  }
}

message PartialTranscript {
  string committed = 1;
  string open = 2;
  string tail = 3;
}

message CommitTranscript {
  string text = 1;
}

message EndOfUtterance {
  float at_seconds = 1;
  float probability = 2;
}

message LanguageDetected {
  string language = 1;
}

message FinalTranscript {
  string text = 1;
}

message TranscribeRequest {
  bytes pcm_s16le = 1;
  string language = 2;
  string start_language = 3;
}

message TranscribeResponse {
  string text = 1;
  string language = 2;
}

message ShutdownRequest {}
message ShutdownResponse {}
```

---

## 3. Prime Desktop Integration (`desktop/src-tauri/src/asr/`)

### 3.1 Build-Time Bundling & Asset Resolution
- `desktop/scripts/prepare-asr.mjs` runs as part of `native:prepare` and `native:dev:prepare`:
  - Places `prime-asr(.exe)`, `en-ru-uk.tzf.gz`, and `silero_vad.onnx.gz` into `desktop/src-tauri/asr/`.
  - During local development, uses a sibling `../prime-asr/target/release/prime-asr(.exe)` build if present or builds it automatically; in CI, downloads the pinned release artifact + models and verifies SHA-256.
  - `desktop/src-tauri/asr/` is listed in `.gitignore`.
- `desktop/src-tauri/tauri.conf.json` and `desktop/scripts/build-native-installer.mjs` both include `asr` in `bundle.resources`.
- At runtime, `asr/assets.rs` resolves the binary and model paths in order:
  1. Tauri bundled resource dir (`<resource_dir>/asr/`)
  2. Shared Prime data dir (`<data_dir>/voice/asr/`)
  3. Dev fallback (`desktop/src-tauri/asr/` or `../prime-asr/target/release/`)
  4. Automatic background download from `SpherePrime/prime-asr` GitHub Releases (with progress events) if missing.

### 3.2 Native Tauri Module (`desktop/src-tauri/src/asr/`)
Split cleanly across focused files:
- `mod.rs` — `AsrService` managing child process lifecycle, gRPC client channel, active streaming session handle, warmup on startup, auto-restart on unexpected exit, and clean shutdown (including Windows Job Object `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`).
- `assets.rs` — path resolution, manifest constants, and fallback downloader with SHA-256 verification.
- `process.rs` — spawning `prime-asr` with hidden console (`CREATE_NO_WINDOW` on Windows), reading the JSON handshake line, and process cleanup.
- `protocol.rs` — `tonic` generated client from `proto/asr.proto` + `x-prime-asr-token` interceptor.
- `tests.rs` — unit tests for language normalization, asset path resolution, handshake parsing, and gRPC round-trip against a mock/real server.

### 3.3 Tauri IPC & Events
Single Tauri command `asr(payload)` (exposed on `window.prime.asr` alongside `window.prime.speech`):
- `status` → `{ state: 'idle' | 'downloading' | 'loading' | 'ready' | 'error', ready, language, languages, progress?, message? }`
- `warmup` → ensures assets & spawns `prime-asr` in background (also invoked automatically on app setup)
- `start` (`{ language?, startLanguage? }`) → opens a new bidirectional `StreamRecognize` gRPC stream, sends `StartSession`, spawns reader task forwarding `RecognizeEvent` as Tauri `"asr"` events
- `push` (`{ sessionId, pcmBase64 }`) → forwards `AudioChunk` on the open gRPC stream
- `finish` (`{ sessionId }`) → sends `FinishSession`, awaits `FinalTranscript`, returns `{ text }`
- `cancel` (`{ sessionId }`) → sends `CancelSession`, drops stream
- `transcribe` (`{ pcmBase64, language?, startLanguage? }`) → calls unary `Transcribe` gRPC RPC

Events emitted on `"asr"` channel:
- `{ type: 'status', status }`
- `{ type: 'partial', sessionId, committed, open, tail }`
- `{ type: 'commit', sessionId, text }`
- `{ type: 'eou', sessionId, atSeconds, probability }`
- `{ type: 'language', sessionId, language }`
- `{ type: 'final', sessionId, text }`
- `{ type: 'error', sessionId?, message }`

---

## 4. Prime CLI & Go Backend (`internal/voice/`, `internal/config/`, `internal/backend/`, `internal/server/`)

### 4.1 Complete Removal of Legacy STT Providers
Delete obsolete files:
- `internal/voice/google.go`
- `internal/voice/vosk.go`, `vosk_cache.go`, `vosk_cache_test.go`, `vosk_load_unix.go`, `vosk_load_windows.go`
- `internal/voice/whisper.go`
- `internal/voice/gpu.go`
- `internal/voice/transcribers.go` (replaced by single TinyASR gRPC client)
- `internal/voice/setup.go`, `internal/voice/stt_install.go` (replaced by clean `asr_install.go`)
- `internal/voice/http.go`, `http_test.go`, `upload.go`, `upload_test.go` (if only used for OpenAI/HTTP STT)

### 4.2 Config Simplification (`internal/config/config.go`)
In `VoiceOptions`:
- Keep `Language` (normalized to `""` / `"auto"`, `"ru"`, `"en"`, `"uk"`; default `"auto"`), `InputDevice`, `OutputDevice`, `Tts*`, `Voice*` settings.
- Remove `SttEngine`, `Whisper*`, `Vosk*`, `Google*`, `OpenAI*`, `Command`, `Accelerator` fields (unrecognized legacy keys in existing `options.json` are safely ignored on load so upgrades never break user configs).

### 4.3 Pure Go gRPC Client (`internal/voice/`)
Using vendored `google.golang.org/grpc` and `google.golang.org/protobuf` (keeping `CGO_ENABLED=0` 100% intact):
- `internal/voice/asrpb/` — generated Go protobuf + gRPC stubs for `prime.asr.v1.AsrService`.
- `internal/voice/asr_install.go` — finds `prime-asr` + `en-ru-uk.tzf.gz` + `silero_vad.onnx.gz` in installed Prime Desktop resources or `<dataDir>/voice/asr/`; if absent, downloads from `SpherePrime/prime-asr` GitHub Releases with SHA-256 verification and progress callbacks.
- `internal/voice/asr_process.go` — manages a shared warm `prime-asr` subprocess with idle shutdown timer for CLI/server use, reads JSON handshake, dials `127.0.0.1:<port>` with `x-prime-asr-token` per-RPC metadata.
- `internal/voice/asr_stream.go` — streaming dictation session (`Start`, `PushPCM`, `Finish`, `Cancel`) emitting live partial/commit updates for CLI TUI (`internal/ui/model/voice.go`), plus one-shot `TranscribeWAV` for `POST /voice/transcribe` compatibility.
- `internal/voice/stt.go`, `internal/backend/stt.go`, `internal/server/stt.go`, `internal/proto/stt.go` — slimmed down to status + language selection (`auto`, `ru`, `en`, `uk`), removing provider/model install/remove/select machinery.

---

## 5. Frontend UI & Real-Time Live Transcript

### 5.1 Live Audio Capture (`desktop/src/renderer/src/features/voice/pcmStream.ts`)
- Uses `AudioContext` + `AudioWorkletNode` (loaded via inline Blob URL module so it works cleanly in both Vite dev and Tauri production, with `ScriptProcessorNode` fallback) to downsample microphone audio to 16 kHz mono s16le in real time.
- Emits ~96 ms PCM chunks (1536 samples = 3072 bytes) continuously while recording, simultaneously feeding the existing waveform analyser (`VoiceWave`) with zero extra mic stream.

### 5.2 Composer Live Transcript (`VoiceWave.tsx`, `Composer.tsx`, `useVoice.ts`)
- `useVoice.ts` starts a `window.prime.asr({ action: 'start', language })` session immediately when recording starts, streams PCM chunks via `push`, and tracks live `{ committed, open, tail }` state from `"asr"` events.
- Above the `VoiceWave` recording bar (`[×] [mic] • Говорите… ||||||| 0:05`), a dedicated live transcript strip renders smoothly inside the composer frame once text starts arriving:
  - Appears with a subtle height/opacity transition when the first words are recognized (zero dead space while silent).
  - Compact scrollable container (max ~3.5 lines, auto-pinned to bottom, soft top fade mask when scrolling).
  - Clear visual hierarchy: `committed` text in full primary ink (`--text-primary`), `open` + `tail` hypothesis in softer muted ink (`--text-secondary`) so the user immediately feels real-time recognition stabilizing without jarring jumps.
- On stop (mic button / hotkey): sends `finish`, receives finalized punctuated text, merges into composer input (preserving any pre-existing text). On Enter while recording: finalizes and submits. On cancel (`×` / Escape): sends `cancel` and clears live preview.

### 5.3 Voice Conversation (`useConversation.ts`, `listener.ts`)
- Replaces amplitude-only pause detection + Go WAV upload with `window.prime.asr` live streaming.
- Uses `eou` (end-of-turn) + `commit`/`final` events from `tinyasr` to detect when the user finished speaking, showing live spoken words in the call UI while listening.

### 5.4 Simplified Voice Settings (`VoiceSettings.tsx`)
- Removes `SttProvider` cards (Google / Whisper / Vosk), `SttAccelerator`, and model download/delete tables.
- Keeps a clean **"Распознавание речи"** card matching the **"Озвучка ответов"** card style:
  - Subtitle: local model (`en-ru-uk`), on-device real-time recognition.
  - Status row: ready badge / loading / download progress / error + retry button if error.
  - Single language selector: **Язык распознавания** — `Авто` (default), `Русский`, `Английский`, `Украинский`.

---

## 6. Error Handling & Edge Cases

- **Missing binary or models in dev/CLI**: Automatic download with progress events and clear Russian error message + retry if offline.
- **`prime-asr` process crash mid-session**: `AsrService` detects broken gRPC stream, reports an error cleanly to the active session, resets state, and auto-restarts `prime-asr` on the next request.
- **Orphan process prevention**: Closing Desktop or CLI sends gRPC `Shutdown`, closes `stdin`, and uses OS-level child termination (Windows Job Object / Unix process kill) so `prime-asr` never leaks.
- **Legacy `options.json`**: Old `stt_engine: "google" | "vosk" | "whisper"` fields are ignored on load; language values outside `auto | ru | en | uk` cleanly fall back to `auto`.
- **Microphone permission / no audio device**: Clear Russian error toast, session never leaked (`cancel` called on error).

---

## 7. Verification Plan

1. **`SpherePrime/prime-asr`**: `cargo test` covering model loading, s16le conversion, language mapping, auth interceptor, `Status`, `StreamRecognize` (streaming partial/commit/final), `Transcribe`, and `Shutdown`.
2. **Prime Desktop Rust (`desktop/src-tauri`)**: `cargo test` for `asr` module + full release/dev compile check.
3. **Prime CLI Go (`internal/voice`, `internal/config`, `internal/backend`, `internal/server`)**: `go test ./...` with `CGO_ENABLED=0` including live gRPC round-trip tests against `prime-asr`.
4. **Desktop Frontend**: `npm run typecheck`, unit tests (`vitest`), and live UI verification in built-in browser (`browser_run`) checking `VoiceSettings` (only Auto/RU/EN/UK, zero Google/Vosk/Whisper traces) and `VoiceWave` live transcript rendering above the waveform in overview + targeted close-up screenshots.
