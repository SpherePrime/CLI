# Native UI integration

Status: Windows smoke passed.

Added a repeatable isolated native application fixture with a local compatible provider. It verifies native startup, workspace/session creation, streamed response completion, image delivery to the provider, history after renderer reload, actual PTY command execution and graceful native close. No model service or user credentials are used.

The Windows run passed. Its process tree contains the native app and WebView2 processes, with no local CLI server executable. The short development fixture measured 641,748,992 bytes of summed working sets across the app tree; this includes shared pages and development tooling effects and is not a long-run performance baseline or a comparison with Electron.

Remaining: broader concurrent-chat/browser/voice workloads, Linux renderer parity, production startup measurements, installed clean/upgrade checks and the complete compatibility manifest.
