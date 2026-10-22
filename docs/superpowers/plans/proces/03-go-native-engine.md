# Go native engine

Status: Windows implementation and native ABI smoke checks completed. Other target platforms remain open.

- Added an in-process server constructor that uses the existing protocol routes without binding a listener.
- Added opaque engine and request handles, ABI versioning, asynchronous requests, incremental response events, cancellation, explicit C buffer freeing and shutdown.
- Response queues are bounded to 64 events and 32 KiB chunks; requests are limited to 256 handles per engine and 32 MiB bodies.
- Added lifecycle, streaming-before-completion, backpressure cancellation and input limit checks.
- Added the native library build script for Windows, Linux and macOS. Cross builds require the appropriate C toolchain.
- Added explicit backend cleanup of workspace holds and timers, plus a guard against repeated teardown after removal.

Validation:

- `go test ./internal/desktopengine` passed: streaming delivery before handler completion, backpressure cancellation, shutdown, request limits, real route dispatch and UTF-8 chunk boundaries.
- Focused backend teardown/idle/shutdown tests passed, including the new repeated-close/timer cleanup check.
- Focused server route/docs/recovery tests passed.
- Windows amd64 `go build -buildmode=c-shared` passed using llvm-mingw 20261020. Release DLL size: 81,985,536 bytes.
- Native DLL smoke passed through Python ctypes with isolated configuration/data: three create/request/read/free/release/shutdown cycles and four real routes per cycle.
- Native workspace smoke passed: create SQLite workspace, open SSE, receive headers immediately, cancel/release SSE, shutdown, recreate engine and reopen the same database; repeated twice.
- Full server tests reported STT fixture cleanup failures because Windows held `prime.db` open. No claim is made that the full server suite passes.

Remaining: Linux and ARM64 builds, platform packaging/runtime dependencies, long-running memory measurements, and end-to-end renderer/Rust integration. The native host starts no listener or CLI process; tools may still start their normal user-command processes.
