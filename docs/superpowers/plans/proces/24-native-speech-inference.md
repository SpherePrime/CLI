# Native speech inference repair

The silent waveform failure was reproduced through `SpeechService.request(speak)` with cached, checksum-verified Supertonic assets and the pinned ONNX Runtime 1.22.0 CPU DLL. No model downloads, microphone access or audio playback were used.

The original TypeScript production implementation was also executed repeatedly against the same assets, voice F1, text `Привет.` and speed 1.6. Of eight independent runs, five produced a peak around 0.00001069, which becomes fully silent PCM16; two produced normal waveform energy and one was quiet. Increasing denoising steps did not reliably eliminate silence. This is stochastic model behavior, not a confirmed Rust runtime or graph optimizer defect. Experimental graph optimization changes were reverted; all four ONNX sessions retain Level3.

Production now checks samples using exactly the PCM16 quantization used by the WAV encoder. A completely silent result is synthesized once more with fresh Gaussian noise. Two silent results return structured `SPEECH_SILENT` instead of a silent WAV. Errors and cancellation propagate immediately. This adds at most one additional inference for a silent result; it cannot guarantee that this model will produce audible speech on every request.

The actual cached inference test uses a deterministic internal Gaussian RNG seed available only in test builds. Model loading, duration prediction, text encoding, five denoising steps, vocoding, bounded silence retry, WAV encoding and the speech worker remain the production pipeline. Production retains `rand::rng()` and accepts no seed override. PCM energy thresholds, speed comparison, idle unloading and shutdown assertions remain unchanged.

Validation on Windows x64:

- `cargo test --lib speech::tests -- --nocapture`: 7 passed, 0 failed, including actual cached inference and the focused PCM rounding/retry-limit/cancellation regression.
- Three additional independent executions of the cached inference test binary: 3 passed, 0 failed (15.45, 19.94 and 21.82 seconds under concurrent build load). Each execution checks unchanged PCM energy, speed-dependent duration, idle release and shutdown assertions.
- `rustfmt --edition 2021 --check` for the four modified speech modules: passed.
- `git diff --check`: passed.

Temporary tensor diagnostics, comparison scripts and task logs were removed. No graph optimization or PCM-energy threshold changes remain.
