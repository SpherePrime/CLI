# TinyASR gRPC Voice Input Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Google, Vosk, Whisper, and OpenAI STT with a single local TinyASR (`en-ru-uk` + Silero VAD) engine running in a separate open-source GPL-3.0 gRPC executable (`SpherePrime/prime-asr`), bundled out-of-the-box in Prime Desktop and supported in pure-Go Prime CLI, with live real-time transcription rendered above the composer waveform and simplified Auto/RU/EN/UK settings.

**Architecture:** `dkaraush/tinyasr` lives exclusively inside an independent public GPL-3.0 Rust repository (`SpherePrime/prime-asr`) that exposes `prime.asr.v1.AsrService` over loopback gRPC (`127.0.0.1:0` with a per-process random `x-prime-asr-token` header printed once on stdout). Prime Desktop (`desktop/src-tauri/src/asr/`) bundles `prime-asr` + `en-ru-uk.tzf.gz` + `silero_vad.onnx.gz`, pre-warms the server on startup (~46 MiB RAM), and bridges bidirectional gRPC streaming to the React renderer via `window.prime.asr`. Prime CLI (`internal/voice/`) uses vendored `google.golang.org/grpc` with `CGO_ENABLED=0` to talk to the exact same `prime-asr` binary (reusing Desktop's copy or downloading from GitHub Releases with SHA-256 verification).

**Tech Stack:** Rust (edition 2024, `tinyasr`, `tonic`, `prost`, `tokio`, `flate2`), Go 1.27 (`CGO_ENABLED=0`, vendored `google.golang.org/grpc` + `google.golang.org/protobuf`), Tauri 2, React + TypeScript + Web Audio `AudioWorklet`.

**Spec:** `docs/superpowers/specs/2026-11-26-tinyasr-grpc-voice-design.md`

## Global Constraints
- `dkaraush/tinyasr` MUST NEVER be linked directly into Prime Desktop or Prime CLI; it runs strictly inside the standalone GPL-3.0-or-later executable `prime-asr` in `SpherePrime/prime-asr`.
- Prime CLI (`prime`) MUST build with `CGO_ENABLED=0`.
- Model files (`*.tzf.gz`, `*.onnx.gz`) and compiled binaries (`prime-asr*`) MUST stay in `.gitignore` and never be committed to `SpherePrime/CLI`.
- Default language is `auto` (`["en", "ru", "uk"]`); allowed settings options are strictly `auto`, `ru`, `en`, `uk`.
- Zero comments in newly written source files unless non-obvious invariants require explanation (per project `AGENTS.md`).
- Work directly on `main`, commit after each completed milestone, verify `git log -1 --format=%B` has zero AI attribution trailers, and push to `origin/main`.

## Review Focus
- Unauthenticated local loopback calls to `prime-asr` without valid `x-prime-asr-token` MUST be rejected with `UNAUTHENTICATED` (pinned in Task 1 tests).
- Abrupt parent termination (closed stdin / Desktop exit) MUST terminate `prime-asr` without leaving orphan processes on Windows or Linux (pinned in Task 1 & Task 2).
- Legacy `options.json` containing `"engine": "google" | "vosk" | "whisper"` or unsupported language codes MUST load without error and normalize to `"auto"` (pinned in Task 3 tests).
- Rapid start/cancel/start dictation sequences or switching chats mid-recording MUST never leak AudioContext/mic streams or drop/misroute finalized transcripts (pinned in Task 5 tests).
- Long continuous speech in the live transcript strip above `VoiceWave` MUST stay bounded in height, auto-scroll to the newest words, and never clip or push composer buttons off-screen (pinned in Task 5 & Task 7 UI verification).

---

### Task 1: Separate Open-Source GPL-3.0 Repository `SpherePrime/prime-asr`

**Files:**
- Create: `../prime-asr/LICENSE` (GNU GPL v3.0)
- Create: `../prime-asr/README.md`
- Create: `../prime-asr/.gitignore`
- Create: `../prime-asr/Cargo.toml`
- Create: `../prime-asr/build.rs`
- Create: `../prime-asr/proto/asr.proto`
- Create: `../prime-asr/src/main.rs`
- Create: `../prime-asr/src/auth.rs`
- Create: `../prime-asr/src/model.rs`
- Create: `../prime-asr/src/session.rs`
- Create: `../prime-asr/src/service.rs`
- Create: `../prime-asr/.github/workflows/release.yml`

**Interfaces:**
- Produces: `prime-asr` CLI binary (`--asr <path> --vad <path> [--threads <N>]`), stdout single-line JSON `{"addr":"127.0.0.1:<port>","token":"<64-hex>","langs":["en","ru","uk"],"chunk_ms":320}`, gRPC `prime.asr.v1.AsrService` (`Status`, `StreamRecognize`, `Transcribe`, `Shutdown`) authenticated via `x-prime-asr-token`.

- [ ] **Step 1:** Scaffold `../prime-asr` repo with GPL-3.0 license, `proto/asr.proto`, `Cargo.toml` (pinning `tinyasr` from `https://github.com/dkaraush/tinyasr`, `tonic`, `prost`, `tokio`, `tokio-stream`, `flate2`, `getrandom`, `serde`, `serde_json`), and `build.rs`.
- [ ] **Step 2:** Implement `src/auth.rs`, `src/model.rs`, `src/session.rs`, `src/service.rs`, `src/main.rs` cleanly separated by responsibility, including `stdin` EOF watcher for parent-crash shutdown.
- [ ] **Step 3:** Add unit & integration tests (PCM s16le conversion, language mapping, unauthenticated gRPC rejection, `Status`, `StreamRecognize`, `Transcribe`, `Shutdown`), download `en-ru-uk.tzf.gz` + `silero_vad.onnx.gz` to temp/local ignored folder, and run `cargo test` and `cargo build --release`.
- [ ] **Step 4:** Add `.github/workflows/release.yml` building `windows-x86_64`, `windows-aarch64`, `linux-x86_64`, `linux-aarch64` + attaching `en-ru-uk.tzf.gz`, `silero_vad.onnx.gz`, and `SHA256SUMS.txt`.
- [ ] **Step 5:** Initialize git, create public repo `SpherePrime/prime-asr` via `gh repo create`, push `main` and tag `v0.1.0`.

---

### Task 2: Prime Desktop Rust gRPC Host (`desktop/src-tauri/src/asr/`) & Out-of-the-Box Bundling

**Files:**
- Create: `desktop/src-tauri/proto/asr.proto`
- Create: `desktop/src-tauri/src/asr/mod.rs`
- Create: `desktop/src-tauri/src/asr/assets.rs`
- Create: `desktop/src-tauri/src/asr/process.rs`
- Create: `desktop/src-tauri/src/asr/protocol.rs`
- Create: `desktop/src-tauri/src/asr/tests.rs`
- Create: `desktop/scripts/prepare-asr.mjs`
- Modify: `desktop/src-tauri/Cargo.toml`, `desktop/src-tauri/build.rs`, `desktop/src-tauri/src/lib.rs`, `desktop/src-tauri/src/state.rs`, `desktop/src-tauri/src/commands.rs`
- Modify: `desktop/src-tauri/tauri.conf.json`, `desktop/scripts/build-native-installer.mjs`, `desktop/package.json`, `.gitignore`

**Interfaces:**
- Consumes: `prime-asr` gRPC protocol from Task 1.
- Produces: Tauri command `asr(payload)` supporting `status`, `warmup`, `start`, `push`, `finish`, `cancel`, `transcribe`, emitting `"asr"` events (`status`, `partial`, `commit`, `eou`, `language`, `final`, `error`).

- [ ] **Step 1:** Copy `proto/asr.proto` into `desktop/src-tauri/proto/asr.proto`, add `tonic`/`prost`/`tonic-build`/`tokio-stream` to `desktop/src-tauri/Cargo.toml`, compile client stubs in `build.rs`.
- [ ] **Step 2:** Implement `asr/{assets.rs, process.rs, protocol.rs, mod.rs, tests.rs}` with background warmup on app launch, hidden console + Windows Job Object cleanup, automatic restart on crash, and fallback download with SHA-256 verification.
- [ ] **Step 3:** Wire `AsrService` into `AppState`, `commands.rs` (`asr`), and `lib.rs` (startup warmup + graceful exit shutdown).
- [ ] **Step 4:** Create `desktop/scripts/prepare-asr.mjs`, add `desktop/src-tauri/asr/` to `.gitignore`, wire `prepare-asr.mjs` into `desktop/package.json`, and include `asr` in `tauri.conf.json` and `build-native-installer.mjs`.
- [ ] **Step 5:** Run `node desktop/scripts/prepare-asr.mjs` and `cargo test` inside `desktop/src-tauri/`. Commit and push.

---

### Task 3: Prime CLI & Go Backend Migration to TinyASR gRPC (`CGO_ENABLED=0`)

**Files:**
- Delete: `internal/voice/google.go`, `internal/voice/vosk.go`, `internal/voice/vosk_cache.go`, `internal/voice/vosk_cache_test.go`, `internal/voice/vosk_load_unix.go`, `internal/voice/vosk_load_windows.go`, `internal/voice/whisper.go`, `internal/voice/gpu.go`, `internal/voice/setup.go`, `internal/voice/stt_install.go`, `internal/voice/http.go`, `internal/voice/http_test.go`, `internal/voice/upload.go`, `internal/voice/upload_test.go`
- Create: `internal/voice/asrpb/asr.pb.go`, `internal/voice/asrpb/asr_grpc.pb.go` (generated from `proto/asr.proto` using `protoc` or pure-Go compatible protobuf codegen matching vendored `google.golang.org/grpc`)
- Create: `internal/voice/asr_install.go`, `internal/voice/asr_process.go`, `internal/voice/asr_stream.go`, `internal/voice/asr_test.go`
- Modify: `internal/voice/stt.go`, `internal/voice/voice.go`, `internal/voice/dictate.go`, `internal/voice/settings.go`, `internal/voice/transcribers.go`, `internal/voice/language.go`, `internal/voice/stt_test.go`
- Modify: `internal/config/config.go`, `internal/backend/stt.go`, `internal/server/stt.go`, `internal/server/stt_test.go`, `internal/proto/stt.go`, `internal/ui/model/voice.go`

**Interfaces:**
- Consumes: `prime-asr` gRPC protocol from Task 1.
- Produces: Pure-Go `CGO_ENABLED=0` `voice.AsrClient` (`Status`, `StartStream`, `TranscribeWAV`, `Close`), simplified `GET /stt` + `POST /stt/select` (`language` only) + `POST /voice/transcribe`.

- [ ] **Step 1:** Generate `internal/voice/asrpb/` Go gRPC bindings from `proto/asr.proto`.
- [ ] **Step 2:** Remove Google, Vosk, Whisper, OpenAI, GPU, and legacy installer files from `internal/voice/`.
- [ ] **Step 3:** Implement `asr_install.go`, `asr_process.go`, `asr_stream.go` and simplify `settings.go`, `language.go`, `stt.go`, `voice.go`, `dictate.go`, `transcribers.go`.
- [ ] **Step 4:** Simplify `internal/config/config.go`, `internal/proto/stt.go`, `internal/backend/stt.go`, `internal/server/stt.go`, `internal/ui/model/voice.go`.
- [ ] **Step 5:** Write/update unit & integration tests (legacy config tolerance, language normalization, gRPC streaming & transcribe tests) and run `CGO_ENABLED=0 go test ./internal/voice/... ./internal/config/... ./internal/backend/... ./internal/server/...`. Commit and push.

---

### Task 4: Desktop Shared Types, Native Bridge & Real-Time Audio Capture

**Files:**
- Modify: `desktop/src/shared/stt.ts`, `desktop/src/shared/contracts.ts`, `desktop/src/shared/operations.ts`
- Modify: `desktop/src/renderer/src/host/nativeBridge.ts`, `desktop/src/renderer/src/host/installPreviewHost.ts`
- Create: `desktop/src/renderer/src/features/voice/pcmStream.ts`
- Create: `desktop/src/renderer/src/features/voice/pcmStream.test.ts`

**Interfaces:**
- Consumes: Tauri `asr` command and `"asr"` event channel from Task 2.
- Produces: `window.prime.asr(payload)` typed API, `AsrStatus` / `AsrEvent` / `AsrLanguage` types, `openPcmStream(options)` delivering 16 kHz mono s16le chunks + live `AnalyserNode`.

- [ ] **Step 1:** Simplify `desktop/src/shared/stt.ts` to `AsrLanguage`, `AsrStatus`, `AsrEvent`, and slim `SttStatus` (language + ready state only). Update `contracts.ts`, `operations.ts`, `nativeBridge.ts`, `installPreviewHost.ts`.
- [ ] **Step 2:** Implement `pcmStream.ts` (AudioWorklet downsampler to 16 kHz mono s16le with `ScriptProcessorNode` fallback and `encodeBase64` helper) plus unit tests for resampling/encoding.
- [ ] **Step 3:** Run `npm run typecheck` and `npx vitest run` in `desktop/`. Commit and push.

---

### Task 5: Composer Live Real-Time Transcript Above `VoiceWave`

**Files:**
- Modify: `desktop/src/renderer/src/features/plugins/useVoice.ts`
- Modify: `desktop/src/renderer/src/features/chat/VoiceWave.tsx`
- Modify: `desktop/src/renderer/src/styles/dictation.css`
- Modify: `desktop/src/renderer/src/features/chat/Composer.tsx`
- Create: `desktop/src/renderer/src/features/chat/VoiceWave.test.tsx` (or pure helper tests)

**Interfaces:**
- Consumes: `window.prime.asr`, `window.prime.onEvent`, `openPcmStream`.
- Produces: `useVoice` returning `{ recording, transcribing, analyser, liveTranscript, toggle, finish, cancel }`; `VoiceWave` rendering the live transcript panel directly above the waveform bar.

- [ ] **Step 1:** Refactor `useVoice.ts` to stream real-time PCM chunks via `window.prime.asr({ action: 'start' | 'push' | 'finish' | 'cancel' })`, updating `liveTranscript: { committed, open, tail }` on `"asr"` events and cleaning up streams on unmount/cancel/error.
- [ ] **Step 2:** Enhance `VoiceWave.tsx` and `dictation.css` to render a polished live transcript region directly above the waveform bar (smooth reveal on first recognized words, committed words in primary ink, open/tail words in softer muted ink, bounded max height with auto-scroll and subtle top fade mask, reduced-motion support).
- [ ] **Step 3:** Wire `liveTranscript` in `Composer.tsx`, preserving per-chat draft routing and Enter-to-send-while-recording behavior.
- [ ] **Step 4:** Run `npm run typecheck` and `npx vitest run` in `desktop/`. Commit and push.

---

### Task 6: Voice Conversation (`useConversation`) & Simplified Voice Settings (`VoiceSettings`)

**Files:**
- Modify: `desktop/src/renderer/src/features/voice/useConversation.ts`
- Modify: `desktop/src/renderer/src/features/voice/ConversationCall.tsx`
- Modify: `desktop/src/renderer/src/features/voice/sttReady.ts`
- Modify: `desktop/src/renderer/src/features/settings/VoiceSettings.tsx`
- Delete/clean unused: legacy STT provider helpers, unused `ModelStatusCard.tsx` if only used for legacy STT
- Modify: `desktop/src/renderer/src/i18n/` (if needed for any new Russian/English strings)

**Interfaces:**
- Consumes: `window.prime.asr`, `openPcmStream`, simplified `stt` / `sttSelect` operations.
- Produces: Live Silero-VAD + `eou` conversation streaming and minimalist `VoiceSettings` showing only local model status and language selector (`Авто`, `Русский`, `Английский`, `Украинский`).

- [ ] **Step 1:** Migrate `useConversation.ts` and `ConversationCall.tsx` to `window.prime.asr` live streaming with `eou` end-of-turn handling and live transcript display.
- [ ] **Step 2:** Simplify `VoiceSettings.tsx` and `sttReady.ts`: remove Google, Whisper, Vosk, CUDA/Vulkan, and third-party model catalog; show built-in local `en-ru-uk` status + language dropdown (`Авто` default, `Русский`, `Английский`, `Украинский`).
- [ ] **Step 3:** Run `npm run typecheck` and `npx vitest run` in `desktop/`. Commit and push.

---

### Task 7: Full End-to-End Verification, Visual UI Audit & Release Hygiene

**Files:**
- Verify all modified files across `SpherePrime/prime-asr` and `SpherePrime/CLI`.

- [ ] **Step 1:** Run full test suites:
  - `cargo test` in `../prime-asr`
  - `cargo test` in `desktop/src-tauri`
  - `CGO_ENABLED=0 go test ./internal/voice/... ./internal/config/... ./internal/backend/... ./internal/server/... ./internal/ui/model/...`
  - `npm run typecheck` and `npm test` in `desktop/`
- [ ] **Step 2:** Start Vite dev server for Prime Desktop renderer and run a single `browser_run` verification scenario inspecting:
  - `VoiceSettings` (verify only Auto/RU/EN/UK language selector & local model status; zero mentions of Google/Vosk/Whisper/CUDA/Vulkan; overview + close-up screenshot).
  - Composer `VoiceWave` active recording state with live transcript above the waveform (verify committed vs partial styling, long-text scrolling without clipping or button overflow; overview + close-up screenshot).
- [ ] **Step 3:** Verify `git status`, `.gitignore` hygiene (no `.tzf.gz`, `.onnx.gz`, `.exe` tracked), `git log -n 10 --format=%B` (zero AI attribution trailers), and push `main` to `origin`.
