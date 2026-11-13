# Quality gate — definition of done

A design is done when every **P0** and **P1** criterion below passes with evidence, and every open **P2** is listed in the report. A green build, a passing lint, or "looks fine on my screen" is not evidence.

Severity:

- **P0 — blocker.** Broken, unreadable, unreachable, or misleading. Never ship.
- **P1 — must fix.** Visibly unpolished or excludes some users. Ship only with the user's explicit acceptance.
- **P2 — polish.** Noticeable to a careful eye. Fix when cheap; list the rest.

## 1. Test matrix

Run every check at each width. Height matters for the first screen and sticky elements.

| Width × height | Represents | Why it breaks |
| --- | --- | --- |
| 320 × 640 | Small Android, iPhone SE zoomed | Long words, buttons in a row, nav overflow |
| 390 × 844 | Typical phone | Default mobile composition |
| 768 × 1024 | Tablet portrait, split screen | The "in-between" layout nobody designs |
| 1024 × 768 | Tablet landscape, small laptop | Desktop nav squeezed, sidebars collide |
| 1280 × 800 | Laptop | Most common desktop |
| 1440 × 900 | Large laptop | Reference desktop |
| 1920 × 1080 | Desktop monitor | Line length, empty edges, tiny hero |
| 2560 × 1440 | Wide monitor (spot check) | Content floating in a void, upscaled images |

Also: browser zoom 200% at 1280 (WCAG 1.4.4), 400% zoom or 320 CSS px reflow (WCAG 1.4.10), dark scheme if supported, `prefers-reduced-motion: reduce`, and at least one real touch interaction or touch emulation.

Run the automated pass first:

```bash
cd scripts && npm install && node audit.mjs http://localhost:5173 --dark --out qa-report
```

Then open every screenshot it produced. Automation finds mechanical defects; your eyes judge the rest.

In Prime Desktop, also run the interaction pass as one `browser_run` scenario: all widths above, every control, overlays, inputs and states, with a screenshot at each state. Run it in one call, review the log and screenshots, fix, and rerun the same scenario. Do not click through the page with separate browser_* calls.

Close-ups are part of the gate. A full-page screenshot reaches the model heavily downscaled, so it shows composition only. Capture details separately: each section and component by selector with `padding` 16-32 px, `zoom: 2` for small text, icons, borders and focus rings, `clip` for the places where blocks meet and for the right edge at 320/390 px, and `fullPage` with `tiles` for long pages. Hover, focus, open and error states get close-ups too. Every visual defect is confirmed on a close-up and cites it.

## 2. Layout integrity — P0

Nothing may collide, spill, or hide. Each criterion is binary.

| ID | Criterion | How to verify |
| --- | --- | --- |
| L1 | No horizontal page scroll at any width ≥ 320px | `document.documentElement.scrollWidth <= clientWidth`; audit rule `horizontal-overflow` |
| L2 | No text overlaps other text, controls, or icons (no наезжание) | Audit rule `overlap`; visual pass on every screenshot, especially headings next to images and absolutely positioned badges |
| L3 | No text is cut mid-glyph or mid-line by `overflow: hidden`, fixed heights, or `max-height` | Audit rule `clipped-text`; test with the longest real string ×1.5 |
| L4 | Nothing important is covered by sticky headers, cookie bars, chat widgets, or floating buttons — on load, after anchor jumps, and when focused | Audit rules `occluded`, `focus-obscured`; click every in-page anchor |
| L5 | Modals, menus, popovers, tooltips fit inside the viewport and scroll internally when taller than it | Open each at 320 × 640 |
| L6 | No layout shift after load: CLS ≤ 0.1, media reserves its box, fonts do not reflow headings by more than a line | Audit metric `CLS`; reload with cache disabled |
| L7 | Mobile viewport height uses `dvh`/`svh` or JS-measured height, never raw `100vh` for full-screen sections | Check on mobile emulation with the URL bar visible |
| L8 | Safe areas: fixed bars respect `env(safe-area-inset-*)`; content is not under the notch or home indicator | Inspect fixed elements; `viewport-fit=cover` only with insets applied |
| L9 | Grid/flex children cannot blow out their track: `min-width: 0` on flex/grid text children, `minmax(0, 1fr)` columns | Long URL / unbroken string test |
| L10 | z-index is a defined scale (base, sticky, dropdown, overlay, modal, toast); no stacking-context surprises | Search for z-index values; open dropdown inside sticky header and inside modal |

## 3. Content resilience — P0/P1

Design for the content you will get, not the content in the mockup.

| ID | Criterion | Severity |
| --- | --- | --- |
| C1 | Long strings: 40-character word, long email, URL, German compound noun, unbroken number — wrap or truncate deliberately (`overflow-wrap: anywhere`, `text-overflow` + title/tooltip) | P0 |
| C2 | Short and missing content: one-word title, empty description, missing image, zero items — layout still holds and shows a meaningful empty state | P0 |
| C3 | Localization: copy 30–40% longer (DE/RU/FI) still fits buttons, tabs, nav; RTL mirrors if the product supports it | P1 |
| C4 | Numbers: tabular figures in tables and counters (`font-variant-numeric: tabular-nums`); values do not jitter while updating | P1 |
| C5 | Truncation is signalled (ellipsis, fade, "Show more"); full text is reachable | P1 |
| C6 | Dynamic states exist and are designed: loading (skeleton matching final layout), empty, error with recovery action, success, partial data, permission denied, offline | P0 for primary flow |
| C7 | No lorem ipsum, fake metrics, placeholder avatars, or `TODO` copy in the shipped result | P0 |

## 4. Accessibility — P0/P1

Baseline: WCAG 2.2 AA.

| ID | Criterion | Threshold |
| --- | --- | --- |
| A1 | Text contrast | ≥ 4.5:1 body, ≥ 3:1 for ≥ 24px or ≥ 18.66px bold; audit rule `contrast` |
| A2 | Non-text contrast: icons, input borders, focus rings, chart lines, selected states | ≥ 3:1 against adjacent colors |
| A3 | Text over images/video/gradients | Measured at the worst point, or a scrim/solid plate guarantees it; audit rule `contrast-manual` lists candidates |
| A4 | Every interactive element is keyboard reachable in visual order, operable, and escapable | Audit focus walk; manual Tab through primary flow |
| A5 | Visible focus indicator on every focusable element | ≥ 2px, ≥ 3:1 contrast, not removed without replacement; audit rule `focus-indicator` |
| A6 | Focus not obscured by sticky/fixed layers | WCAG 2.4.11; audit rule `focus-obscured` |
| A7 | Touch targets | ≥ 24 × 24 CSS px or spaced (WCAG 2.5.8); 44 × 44 on touch-first UI |
| A8 | Accessible names for icon buttons, inputs, links | Audit rule `accessible-name`; visible label preferred over placeholder |
| A9 | Semantic structure | One h1, no skipped heading levels, landmarks (`header`, `nav`, `main`, `footer`), lists for lists, tables for tabular data |
| A10 | Color is never the only signal | Errors, status, chart series also differ by text, icon, or pattern |
| A11 | Motion | `prefers-reduced-motion` removes non-essential motion; nothing flashes > 3/s; autoplay > 5s can be paused |
| A12 | Zoom and reflow | 200% zoom and 320px reflow keep all content and functions without two-axis scrolling |
| A13 | Forms | Labels, autocomplete, inputmode, error text tied with `aria-describedby`, errors announced, input font ≥ 16px on mobile |

## 5. Visual quality — P1/P2

These need eyes. Check each on the desktop and the 390px screenshots.

| ID | Criterion | Severity |
| --- | --- | --- |
| V1 | **Hierarchy squint test.** Blur the screenshot (or squint): the primary message and the primary action are still the two most prominent things | P1 |
| V2 | **One signature.** Exactly one memorable idea dominates; everything else supports it | P1 |
| V3 | **Type scale.** 6–8 sizes from a ratio or a fluid scale; audit token report shows ≤ 10 distinct sizes | P1 |
| V4 | **Measure and leading.** Body 45–80 characters per line; body line-height 1.4–1.7, display 1.0–1.2; no heading with a single orphan word (`text-wrap: balance`) | P1 |
| V5 | **Spacing rhythm.** All spacing from one scale (4 or 8 base); related items closer than unrelated (proximity); section spacing visibly larger than component spacing | P1 |
| V6 | **Alignment.** Every element snaps to a grid line or a deliberate offset; no 1–3px near-misses (audit rule `alignment-near-miss`); optical alignment for icons, quotes, round shapes | P2 |
| V7 | **Color roles.** Palette defined by role; one accent used for action/emphasis only; neutrals tinted toward the brand hue rather than pure grey | P1 |
| V8 | **Consistency.** Same component looks and behaves the same everywhere: radius scale ≤ 4 values, 2–3 elevation levels, one icon family with one stroke weight | P1 |
| V9 | **Image craft.** Correct aspect ratio (no stretch), focal point kept in mobile crops, no upscaling blur, consistent treatment (color grade, corner radius) | P1 |
| V10 | **Edges and density.** No content touching the viewport edge on mobile (≥ 16px gutter); wide screens cap content width and use the extra space intentionally | P1 |
| V11 | **State polish.** Hover, active, focus, selected, disabled, loading look designed and consistent; disabled is distinguishable but not invisible | P1 |
| V12 | **Dark mode** (if supported). Not an inversion: elevated surfaces lighter, saturated accents toned down, images and shadows adapted, contrast re-measured | P1 |
| V13 | **Anti-slop pass.** Nothing generic: no default gradient blobs, no equal card grids by reflex, no meaningless eyebrows, no decorative motion. Would this design work unchanged for a different product? If yes, it is not done | P1 |
| V14 | **Static frame.** Freeze all animation: the composition still reads and still looks finished | P1 |

## 6. Interaction and motion — P0/P1

| ID | Criterion | Severity |
| --- | --- | --- |
| I1 | Every visible control does something real; no dead buttons or `href="#"` | P0 |
| I2 | Feedback within 100ms for every action (pressed state, spinner, optimistic update) | P1 |
| I3 | Repeated and interrupted actions are safe: double-click, rapid toggling, navigating mid-animation, back button | P0 |
| I4 | Animations: 150–300ms for UI transitions, 300–600ms for larger spatial moves; ease-out for entering, ease-in for leaving; nothing blocks input | P1 |
| I5 | Only `transform` and `opacity` animate in hot paths; no animated `width/height/top/left` on large elements | P1 |
| I6 | Scroll-linked effects do not hijack scrolling, do not break anchor links, and turn off with reduced motion | P0 |
| I7 | Hover-only affordances have a touch and keyboard equivalent | P0 |

## 7. Runtime and performance — P0/P1

| ID | Criterion | Threshold |
| --- | --- | --- |
| R1 | No console errors, no unhandled rejections, no failed requests | 0; audit rules `console-error`, `js-exception`, `failed-request` |
| R2 | LCP on a mid-range profile | ≤ 2.5s target; hero image preloaded and sized |
| R3 | CLS | ≤ 0.1 |
| R4 | INP / long tasks | Interactions respond < 200ms; no task > 200ms during interaction |
| R5 | Fonts | ≤ 2 families, subsetted, `font-display: swap` or `optional`, metric-matched fallback (`size-adjust`) to avoid reflow |
| R6 | Images | Modern formats, `srcset/sizes`, explicit dimensions, lazy below the fold, never lazy for LCP |
| R7 | 3D/canvas | Pixel ratio capped (≤ 2), paused offscreen and in hidden tabs, resources disposed, fallback without WebGL |

## 8. Evidence and report

Record every claim with its proof. Use [review-report.md](review-report.md). Rules:

- Visual claims cite a screenshot file and viewport.
- Interaction claims name the action performed and the observed result.
- Measured numbers come from a tool run in this session (audit report, Lighthouse, DevTools trace). Never estimate a score.
- Unchecked items are listed as **not verified** with the reason. Silence is not a pass.

## 9. Fix loop

1. Run the audit and the manual pass.
2. Fix P0 first, then P1. Fix root causes (a token, a component, a layout rule), not instances.
3. Re-run the full audit, not only the failing rule; fixes often move the defect to another width.
4. Repeat until P0 = 0 and P1 = 0 (or accepted by the user), then write the report.
