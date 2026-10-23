# Native engine resources

Status: in progress.

Package the Go library as an application resource and resolve it through the native resource directory on installed systems. Development checkout fallback must remain limited to debug builds. Add a checksum and ABI manifest before installer verification.

Implemented Tauri resource-directory resolution, a debug-only checkout/override fallback, an ABI/version/platform/architecture/SHA-256 manifest, streamed checksum verification and native build preparation. The library resource directory is included in the preview bundle configuration.

Checks: manifest validation and changed-library rejection passed in the combined Windows run; the native library and manifest were regenerated successfully. The real app loaded its engine and completed chat/image/PTY/reload/shutdown checks. Installed Windows/Linux resource layout still requires installer verification. Production Electron packaging remains active until capability parity and upgrade checks pass.
