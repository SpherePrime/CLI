# Native window chrome

Status: in progress.

Added main-view-only Rust commands for minimize, maximize/restore, dragging and close. Existing titlebar reserves the same 138 px for the controls; existing Electron rendering remains unchanged. Close uses the existing native unsaved-editor confirmation lifecycle.

Validation: renderer typecheck and production UI build passed. The native application compiled and displayed the custom controls with native decorations disabled. A real WebView invocation of the close command shut down the app and engine during the smoke fixture. Native command errors feed the existing application error display. Minimize/maximize and pointer dragging still need manual interaction verification.
