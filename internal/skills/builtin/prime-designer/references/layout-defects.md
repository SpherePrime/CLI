# Layout defect catalog

Each entry: what you see, why it happens, how to catch it, how to fix it at the root. Audit rule names refer to `scripts/layout-audit.js`.

## Overlap and collision

### Text runs into text or controls
- **Symptom:** heading letters touch an image, a badge sits on a title, two absolutely positioned labels stack, a button covers a paragraph at one width.
- **Cause:** `position: absolute` used for layout; negative margins; fixed heights; `transform` offsets that ignore content length; grid items placed into the same area.
- **Detect:** `overlap` rule; screenshot at every width; longest-content test.
- **Fix:** put elements into flow (grid areas, flex). Reserve absolute positioning for decoration and overlays that cannot collide. When overlap is the design (text over image), give the text its own grid cell stacked with `grid-area: 1 / 1` and constrain it with padding plus a scrim.

### Sticky header hides content
- **Symptom:** hero title under the header on load; anchor links land with the heading hidden; focused field under the header.
- **Cause:** fixed header without offset; no `scroll-margin-top`.
- **Detect:** `occluded`, `focus-obscured`; click every in-page link.
- **Fix:** prefer `position: sticky` (stays in flow). For fixed headers add body/top padding equal to header height via a CSS variable; `scroll-padding-top: var(--header-h)` on `html`.

### Floating widgets collide
- **Symptom:** cookie bar covers the CTA; chat bubble covers the last button; toast covers the submit button on mobile.
- **Detect:** `floating-coverage` (> 30% of viewport), screenshots at 320 and 390.
- **Fix:** reserve space (`padding-bottom` equal to bar height while visible); stack floating layers in one corner container; move toasts away from primary actions.

### Dropdown or tooltip clipped
- **Symptom:** menu cut by a card edge; tooltip off-screen.
- **Cause:** ancestor with `overflow: hidden` or a transform creating a containing block.
- **Fix:** render overlays in a top-level layer (`popover` attribute, `<dialog>`, a portal); use anchor positioning or flip/shift logic.

## Overflow

### Horizontal page scroll on mobile
- **Cause:** fixed widths (`width: 1200px`), `100vw` inside a page with a vertical scrollbar, long unbroken strings, images without `max-width: 100%`, negative margins, off-canvas menus positioned with `left` instead of `transform`, `white-space: nowrap` on headings.
- **Detect:** `horizontal-overflow` lists the outermost culprits.
- **Fix:** `max-width: 100%` for media; `width: 100%` instead of `100vw`; `overflow-wrap: anywhere` on user content; `min-width: 0` on flex/grid children; off-canvas via `transform` and `overflow-x: clip` on its wrapper. Do not paper over with `overflow-x: hidden` on `body`; it breaks `position: sticky` and hides the real culprit.

### Flex or grid item blows out
- **Symptom:** a long title pushes the sidebar off-screen; a table widens the page.
- **Cause:** default `min-width: auto` of flex/grid items.
- **Fix:** `min-width: 0` on the item, `grid-template-columns: minmax(0, 1fr)`; tables inside a scroll container with a visible shadow cue.

### Text clipped
- **Symptom:** descenders cut (g, y, p), the last line half visible, button label cut.
- **Cause:** fixed `height` on text containers; `line-height` smaller than font metrics with `overflow: hidden`; `max-height` truncation not aligned to line height.
- **Detect:** `clipped-text` (mid-line cut, error), `truncated-text` (clean line cut without signal, warning).
- **Fix:** use `min-height`, never `height`, on text containers; truncate with `-webkit-line-clamp` or ellipsis and offer the full text.

## Sizing and viewport

### `100vh` on mobile
- **Symptom:** full-screen hero taller than the screen; bottom CTA hidden under browser UI.
- **Fix:** `min-height: 100svh` (stable) or `100dvh` (dynamic). Keep `100vh` only as a fallback line before it.

### Notch and home indicator
- **Fix:** with `viewport-fit=cover`, pad fixed bars with `env(safe-area-inset-top/bottom)`.

### Content floating in a void on wide screens
- **Symptom:** 2560px monitor shows a 600px column with huge empty sides, or text lines 200 characters long.
- **Fix:** cap measure with `max-width: 68ch` for text; let the grid, imagery, or background use the width; fluid type with an upper bound via `clamp()`.

## Media

### Stretched or squashed images
- **Detect:** `img-distorted`.
- **Fix:** `object-fit: cover` with `object-position` set to the focal point; `aspect-ratio` on the box.

### Layout shift when media loads
- **Detect:** `img-dimensions`, CLS in the report.
- **Fix:** `width`/`height` attributes or `aspect-ratio`; reserve space for embeds, ads, and late banners.

### Blurry images
- **Detect:** `img-upscaled`.
- **Fix:** supply `srcset` with 2× candidates; SVG for logos and icons.

## Typography

### Orphan word in a heading
- **Detect:** `heading-widow`.
- **Fix:** `text-wrap: balance` for headings, `text-wrap: pretty` for paragraphs; adjust copy for display headlines.

### Lines too long or too short
- **Detect:** `line-length`.
- **Fix:** `max-width: 60–75ch` for body; on mobile reduce side padding before reducing font size.

### Font swap reflow
- **Symptom:** heading wraps differently after the web font loads; the page jumps.
- **Fix:** preload the display font, `font-display: optional` for decorative fonts, fallback with `size-adjust`, `ascent-override` matched to the web font.

### Tight leading in multi-line headings
- **Detect:** `tight-leading`.
- **Fix:** display line-height 1.05–1.2; check descender/ascender collisions in uppercase and in Cyrillic with diacritics.

## Interaction geometry

### Tiny or crowded targets
- **Detect:** `touch-target`, `touch-target-comfort`.
- **Fix:** enlarge the hit area with padding or a pseudo-element, not by scaling the icon; at least 8px between adjacent targets.

### Invisible focus stops
- **Symptom:** Tab lands on hidden mobile menu items or offscreen slides.
- **Detect:** `focus-invisible-stop`, `focus-offscreen`.
- **Fix:** `inert` or `hidden` on closed menus and inactive slides; `visibility: hidden` after the close transition ends.

### Hover-only content
- **Fix:** expose the same content via focus (`:focus-visible`, `:focus-within`) and tap; never hide primary actions behind hover.

## Stacking

### z-index war
- **Symptom:** dropdown behind the next section; modal under the sticky header.
- **Cause:** arbitrary values (`9999`), transforms or `opacity < 1` creating new stacking contexts.
- **Fix:** a token scale (`--z-sticky: 100; --z-dropdown: 200; --z-overlay: 300; --z-modal: 400; --z-toast: 500`); `isolation: isolate` on components that need a private stack.

## Pre-ship stress test

Paste into the real UI before calling it done:

- Title: `Internationalisierungsbeauftragtenstellvertreterin und Qualitätssicherung`
- Email: `alexandra.konstantinopolskaya-mueller@very-long-company-domain.example`
- URL: `https://example.com/a/very/long/path/without/any/breaks/to/test/wrapping?query=1`
- Number: `1 234 567 890,00 ₽` and `-0.000001`
- Empty: no title, no image, zero items
- One character: `A`
- Emoji and mixed scripts: `Café 東京 Москва 🚀`
