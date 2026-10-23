# Renderer and native contract

Status: completed for renderer bridge and standalone renderer build. Full native feature parity is tracked separately.

Implemented: independent Vite build for the existing renderer; complete compile-time DesktopApi method manifest; native Tauri request/event adapter; original operation routing reused; Electron renderer still uses its existing preload.

Checks: TypeScript passed as part of native:ui; three bridge tests passed (method coverage, operation routing/errors, subscription cleanup); actual native renderer production build passed.

Remaining: run real native window, verify visuals and all native command implementations; method coverage is not feature parity.
