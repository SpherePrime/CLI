# Disk-backed native attachments

Status: implemented on the engine boundary with focused tests on both sides.

The preview no longer carries attachment bytes through the ABI request body. The Rust host stores each attachment in the private application data directory (`<data>/attachments/<uuid>.bin`), keeps only the descriptor, file name, MIME type and an image preview in memory, and resolves send requests into `{file_path, file_name, mime_type}` references without content.

The embedded engine receives the trusted attachment root as an engine option. The Go backend resolves every attachment with a file path and no content by verifying the path stays inside that root after symlink resolution, rejecting non-regular files and reads beyond 20 MiB, and loading the bytes with a bounded reader before the agent consumes them. A backend without a configured root rejects every file reference, so the ordinary HTTP API can never read arbitrary files through this mechanism. Reference resolution happens before run acceptance, so recovery of interrupted runs re-resolves references against the same root after a restart.

Startup clears stale files from the attachment directory, and release deletes the backing file immediately. Image previews above 8 MiB are omitted instead of being embedded as data URLs.

Validation: three focused Rust tests cover resolve-without-content, release deleting the backing file, stale-file startup cleanup and the reference staying inside the private root. Seven Go tests cover trusted reference reads, rejection outside the root, rejection without a configured root, path escape rejection, the symlink vector where the platform allows file symlinks, oversized rejection, resolved bytes reaching the coordinator, and inline content pass-through. Live provider delivery through the native UI smoke continues to pass. Migration of the old in-memory preview behavior above 8 MiB is the only deviation and is documented here.
