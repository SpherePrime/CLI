# Motion, 3D, and 2D art

## Motion spec

Write one line per important animation before building it:

| Field | Example |
| --- | --- |
| Trigger | Click "Disassemble" |
| Purpose | Show how 15 modules fit together |
| Subject and path | Each module moves along its assembly axis, 0.6–1.2 units |
| Duration / delay / stagger | 700ms, stagger 40ms outer → inner |
| Easing | `power3.out` in, `power2.inOut` back |
| Rest state | Exploded view, labels fade in after 200ms |
| Interruption | Clicking again reverses from the current progress |
| Reduced motion | Instant switch with a 150ms cross-fade, labels shown |
| Cost | Transforms only, no shadow map updates during motion |

Rules:

- Motion explains causality, hierarchy, state, or space. If removing an animation loses nothing, remove it.
- A few deliberate moments beat animating every section on scroll.
- Animate `transform` and `opacity`, or renderer-native properties. No global `will-change`.
- Time-based, not frame-based (`delta` from the clock).
- Interruptible: any animation can be reversed or replaced mid-flight without jumps.
- Ambient loops need a reason and a pause path; pause in hidden tabs (`visibilitychange`) and offscreen (`IntersectionObserver`).
- Audio is opt-in, user-initiated, adjustable, and never required to understand content.
- `prefers-reduced-motion: reduce` in both CSS and JS: remove parallax, camera drift, autoplay loops, and scroll scrubbing; keep state changes as instant or cross-fade.

Test: repeated triggers, rapid toggling, interruption, resize mid-animation, touch, back/forward navigation, hidden tab, reduced motion.

## 3D

Use 3D when volume, material, spatial relationship, assembly, or direct manipulation is the subject. A strong photo or illustration is often better.

Design the object before the renderer:

- **Form:** recognizable silhouette, correct proportions and scale, intentional detail budget.
- **Material:** response appropriate to the subject (paper, metal, glass, fabric, ceramic, plastic). Metals need an environment map to read as metal.
- **Light:** key direction, softness, environment, contact shadow, separation from background, readable contrast with overlaid UI.
- **Camera:** position, FOV, framing on desktop and on a 390px portrait screen, orbit limits, initial pose, reset.
- **Source:** licensed model with attribution, custom model, or procedural geometry. Compress (Draco/Meshopt, KTX2) and record the source.
- **Interaction:** input mapping, hover/selected states, transitions, reset, loading progress, error, non-WebGL fallback.

Renderer checklist:

- Check the installed library version and its docs before using APIs.
- Size renderer to the container with `ResizeObserver`; set drawing-buffer size and camera aspect on resize.
- Cap `devicePixelRatio` (≤ 2, lower on weak GPUs); render on demand when the scene is static.
- Pause when offscreen or hidden; lazy-load heavy scenes below the fold.
- Dispose geometries, materials, textures, render targets, controls, observers, and listeners on teardown.
- Profile before adding shadows, post-processing, large textures, or a second canvas.
- Labels, controls, and essential information live in semantic HTML, never only in canvas.
- Touch: one-finger page scroll still works; rotation requires a deliberate gesture or a dedicated area; `touch-action` set accordingly.
- Fallback: a poster image plus the same information and controls when WebGL or the asset fails.

QA for 3D adds: framing at every test width, resize while animating, context loss, slow-network loading state, GPU memory after repeated open/close, frame time while interacting.

## 2D art, posters, and graphics

- Write a short visual philosophy first (4–6 paragraphs): movement, form and space, color and material, scale and rhythm, composition, hierarchy. Then make the piece express it.
- Minimal text, clear margins, deliberate negative space, one focal point.
- Do not imitate a living artist's signature style.
- Refine the existing composition in a second pass instead of adding elements.
- Local fonts are in `assets/fonts/` with their licenses; always declare a fallback.
- Export checks: correct dimensions and DPI for the destination, color profile (sRGB for screen), text converted or embedded for print, file size reasonable.

## Slack GIFs

Tools in `assets/gif-tools/` (`gif_builder.py`, `frame_composer.py`, `easing.py`, `validators.py`).

- Emoji: 128 × 128, under ~3s, 48–128 colors. Message GIF: 480 × 480.
- 10–30 FPS; use easing, thick lines, layered shapes, strong contrast.
- Run `validators.validate_gif()` on the output and report its result.
