# Local build verification

## Implemented and checked

- Strict TypeScript compilation and production Electron/Vite build.
- SSE UTF-8/chunk boundaries, nested credential redaction, path traversal/junction rejection, route validation and authoritative activity tracking tests.
- Real local named-pipe HTTP/SSE integration test.
- Electron launch with a bundled Prime process and Cyrillic workspace path.
- Real fixture-provider round trip through Prime, renderer reload, actual tool permission and file creation, theme switching.
- Go backend tests and vet for durable run storage, pending requests, plugins, voice upload, model reasoning, SDK and Windows pipe ACL.
- Runtime manifest ties the bundled binary to its exact Git revision and SHA-256.
- Brand geometry and colors taken from the public SpherePrime site; fonts and assets are local.
- The same full Electron scenario passed against `release/win-unpacked/PrimeDesktop.exe`, using its packaged assets and bundled runtime rather than development files.
- Structured custom-command arguments and MCP prompt preparation, explicit channel opt-in and workspace-scoped channel display are implemented.
- SQLite migration backup and newer-schema rejection passed Go tests. These do not substitute for a full installer upgrade test on a separate VM.

## Public release requirements

- No public Desktop repository or update-feed destination has been assigned. The updater reports an unavailable feed; no release was published.
- The local Windows installer is not digitally signed. Production signing needs a certificate and release secrets.
- Clean-VM installation/upgrade/downgrade, physical microphone, sleep/wake and multi-process external-CLI database access need separate acceptance evidence.
- Backend event replay remains unsupported. Recovery uses current persistent message/session snapshots plus buffered live events and pending-request snapshots. No replay guarantee is advertised by capabilities.
- Remaining functional acceptance includes complete Git worktree-status discovery, English interface localization and long-history virtualization/performance checks. Current history remains available through the older-message control.
- Existing-data upgrade/downgrade behavior needs installer-level acceptance before automatic deployment over established user installations.

The application is a functional local build. These remaining items prevent calling it a completed production release matching every criterion in the original comprehensive plan.
