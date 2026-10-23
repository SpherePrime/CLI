# Native migration of Electron data

Status: Windows implementation and tests passed; macOS/Linux remain explicit unsupported results.

Scope: installed-application identity, data locations, SSH profiles, SSH credentials, sudo password and the migration marker.

Implemented `desktop/src-tauri/src/migration.rs`:

- Locates the Electron `userData` directory (`Prime Desktop` under APPDATA on Windows, the XDG config home on Linux, Application Support on macOS) and stops with an `absent` report when the legacy stores are not there.
- Imports `remote-profiles.json` into the native profile store: version and fingerprint validation, per-profile native validation, skip of existing identifiers, host fingerprint merge and an atomic persist that only writes when something was imported.
- Decrypts `remote-credentials.json` entries through Electron `safeStorage`. On Windows this is DPAPI: the encrypted blob is decrypted in memory, the payload binding is verified against the store entry, the profile's recomputed legacy binding is verified against the same value, and the credentials are immediately re-protected into the native keyring vault. No plaintext is written to disk at any point.
- Writes a `migration-complete.json` marker with the full report (counts and per-entry errors, never secret values) and skips future runs once it exists. Runs once on a background thread at native startup.
- macOS and Linux return an explicit unsupported result: Electron `safeStorage` keys on those platforms live in the OS keyring under the Electron application and cannot be read from Rust. Users re-save credentials once, which matches the plan's requirement not to assume old ciphertext is readable.

The native sudo settings store (`src/sudo.rs`, Linux-only, keyring service `io.sphereprime.native-preview.sudo`) publishes `PRIME_DESKTOP_SUDO_PASSWORD` before the engine loads so the existing Go sudo tool keeps working without a CLI server. Non-Linux builds remove the variable.

Validation: two DPAPI round-trip tests pass on Windows (protected bytes decrypt to the original plaintext; random bytes are rejected), plus the existing profile store round-trip test. The manifest test now also covers `sudoPassword`, `cancelProjectWait` (synchronous native open, no-op) and the explicit `update` unsupported arm. Windows/Chrome DPAPI validation against a real installed Electron release with existing secrets remains an upgrade-path check.
