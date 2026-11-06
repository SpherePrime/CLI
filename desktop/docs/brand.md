# SpherePrime brand

Source: https://sphereprime.ru/ — inspected 17 October 2026.

Desktop uses the site's existing three-bar SVG mark, taken from the rendered `.pr-brand-symbol` CSS mask, with the original `translate(8 2) skewX(-20)` geometry. Inter is the interface font; JetBrains Mono is used for code and small technical labels. Font files are bundled locally.

Dark palette: background #101010, foreground #F1EFE9, orange #FF6A35, secondary orange #FF8C60, panels #191919. Light mode uses warm off-white surfaces and a darker orange for readable text. The app adapts the site's visual language to a dense desktop workspace.

## Desktop application icon

`desktop/build/icon.svg` is the source: a dark rounded square, a restrained application-window outline, and the orange three-bar mark. The window frame distinguishes Desktop from CLI branding. The mark keeps its original bar proportions, rounded ends and skew; it remains the dominant feature at taskbar sizes. The icon does not change with the interface theme.

From `desktop`, run `node scripts/build-icons.mjs` to regenerate the tracked assets with the existing Sharp dependency:

- `build/icon.png`: 512 × 512 RGBA for Linux packaging and the native application icon.
- `build/icon.ico`: RGBA PNG frames at 16, 20, 24, 32, 40, 48, 64, 128 and 256 pixels for Windows Explorer, taskbar and high-DPI displays.

Both `src-tauri/tauri.conf.json` and `installer/src-tauri/tauri.conf.json` reference these same assets. The native NSIS installer also uses `build/icon.ico`. The custom setup executable, installed application and subsequent release/update payloads inherit this branding through the existing native packaging pipeline. Release scripts do not generate separate icons. Regenerate and include both binary assets whenever the SVG changes; `npm run test:packaging` verifies source parity, ICO frames and configuration paths before release.

CLI notification icons and in-app brand marks are separate and unchanged. macOS packaging is not currently configured; an ICNS asset must be added before supporting that target. Existing Windows shortcuts may retain a cached icon until the shell refreshes its icon cache.
