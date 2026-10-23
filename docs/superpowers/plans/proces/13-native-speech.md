# Native speech

Status: Windows x64 implementation and six focused tests passed, including real cached-model inference.

Scope: Supertonic 3 through in-process ONNX Runtime, existing pinned model/voice assets and hashes, text normalization and language detection, imported voice styles, speed control, PCM WAV output, bounded worker queue and ten-minute idle session release. STT remains in the retained Go engine routes.

The complete Supertonic model cache is available locally. A pinned official Windows x64 CPU runtime 1.22.0 archive was fetched to establish independent runtime packaging and validated library hashes; no models were downloaded.

Validation: six focused Rust tests passed in 24.92 seconds. A Russian utterance was synthesized through the actual cached duration predictor, text encoder, five vector-estimator steps and vocoder using the pinned CPU ONNX Runtime. The fixture verified non-silent PCM WAV output and increased duration at speed 0.7 versus 1.6. Session unloading was verified with the same idle-timer mechanism shortened to 200 ms for the test; production is configured for ten minutes.

Additional checks passed for NFKD text normalization, language detection, UTF-16 index mapping, WAV headers/clipping/rounding, imported voice validation/removal/path restrictions, asset content hashes, and prompt cancellation of a stalled native HTTP download. Shutdown also terminates the active ONNX RunOptions. No audio playback, microphone access or model download was used.

Runtime provisioning uses the official Windows x64 ONNX Runtime 1.22.0 archive SHA-256 `174c616efc0271194488642a72f1a514e01487da4dfe84c49296d66e40ebe0da`; its 12,418,080-byte CPU library is pinned to `579b636403983254346a5c1d80bd28f1519cd1e284cd204f8d4ff41f8d711559`. The library can be bundled under speech-runtime/ or fetched into the existing speech cache. No Node/Python helper process is used.

Host module/state/command/shutdown wiring is present. Application-level UI verification and in-flight inference shutdown tests remain open. Compilation emitted existing remote dead-code and OpenSSL debug-symbol linker warnings, with no test failures.

Verified runtime provisioning for additional platforms remains a platform gate.
