# Motion craft

How to build motion that feels expensive: precise, physical, quiet until it matters. These are techniques, not looks to copy. Read with `motion-and-3d.md` (motion spec, rules, 3D).

## 1. What separates crafted motion from template motion

- **One motion language.** A small set of durations, easings and distances used everywhere. Template motion uses one `0.5s ease` for everything.
- **Causality.** Things move from where they come from to where they go. A menu grows out of its button, a card expands into its detail view, a removed row collapses and its neighbours close the gap. Nothing appears from nowhere.
- **Short distances.** Elements travel 8–24px. Large moves are reserved for spatial changes (a panel entering, a view changing).
- **Asymmetric timing.** Exit is 60–75% of enter. People wait for what arrives, not for what leaves.
- **A lead and a follow.** The element the user acted on moves first; its context follows 30–60ms later; secondary content settles last. Never animate everything at once.
- **Stillness at rest.** After the intro the page is calm. Ambient loops exist only when they carry meaning and can be paused.
- **Physical response to input.** Press feedback in the same frame, release with a slight spring, drag that follows the pointer 1:1 and settles with momentum.

## 2. Motion tokens

```css
:root {
  --ease-out-quart: cubic-bezier(0.25, 1, 0.5, 1);
  --ease-out-expo: cubic-bezier(0.16, 1, 0.3, 1);
  --ease-in-out-quart: cubic-bezier(0.76, 0, 0.24, 1);
  --ease-in-quad: cubic-bezier(0.5, 0, 0.75, 0);
  --spring-snappy: linear(0, 0.081 4.2%, 0.251 8.3%, 0.439 12.5%, 0.608 16.7%, 0.745 20.8%, 0.846 25%, 0.916 29.2%, 0.962 33.3%, 0.989 37.5%, 1.003 41.7%, 1.009 45.8%, 1.011 50%, 1.01 54.2%, 1.006 62.5%, 1.003 70.8%, 1);
  --spring-soft: linear(0, 0.108 4.2%, 0.332 8.3%, 0.571 12.5%, 0.77 16.7%, 0.912 20.8%, 0.999 25%, 1.041 29.2%, 1.054 33.3%, 1.05 37.5%, 1.038 41.7%, 1.025 45.8%, 1.014 50%, 1.006 54.2%, 1.001 58.3%, 0.997 66.7%, 0.998 75%, 1);
  --dur-press: 90ms;
  --dur-micro: 160ms;
  --dur-ui: 240ms;
  --dur-panel: 360ms;
  --dur-scene: 600ms;
  --spring-snappy-dur: 420ms;
  --spring-soft-dur: 700ms;
  --rise: 16px;
}
```

`--spring-snappy` overshoots about 1% and suits controls; `--spring-soft` overshoots about 5% and suits objects that land (toasts, dropped cards, sheets). Use springs only where the subject is physical; ease-out curves elsewhere. Browsers without `linear()` ignore the declaration, so declare a `cubic-bezier` fallback first.

| Change | Duration | Easing | Movement |
| --- | --- | --- | --- |
| Press | 90ms down, spring up | `--ease-out-quart`, `--spring-snappy` | `scale(0.97)` |
| Hover | 160ms | `--ease-out-quart` | lift 2px, shadow via pseudo-element opacity |
| Tooltip | 120ms in, 80ms out | ease-out / ease-in | 4px from the anchor; 300ms open delay, none when moving between tooltips |
| Menu, popover | 180ms in, 120ms out | `--ease-out-quart` / `--ease-in-quad` | `scale(0.96)` → 1 from the trigger side |
| Dialog | 260ms in, 160ms out | `--ease-out-expo` / `--ease-in-quad` | 12px rise + `scale(0.98)`; backdrop fades 200ms |
| Drawer, sheet | 360ms in, 240ms out | `--ease-out-expo` / `--ease-in-quad` | full travel; follows drag |
| Toast | 700ms | `--spring-soft` | enters from its edge; timer pauses on hover and focus |
| View change | 280–450ms | `--ease-in-out-quart` | View Transition (§5) |
| List reorder | 320ms | `--ease-out-quart` | FLIP (§4) |
| Hero intro | ≤ 1.2s total | `--ease-out-expo` | line reveal, 70–90ms stagger (§7) |

## 3. Enter and exit in CSS

Animate overlays in and out without JavaScript. `@starting-style` defines the first frame; `allow-discrete` keeps `display` and the top layer alive during the exit.

```css
.menu[popover],
dialog.panel {
  opacity: 1;
  transform: none;
  transition:
    opacity var(--dur-ui) var(--ease-out-quart),
    transform var(--dur-ui) var(--ease-out-quart),
    overlay var(--dur-ui) allow-discrete,
    display var(--dur-ui) allow-discrete;
}
@starting-style {
  .menu[popover]:popover-open { opacity: 0; transform: translateY(-4px) scale(0.96); }
  dialog.panel[open] { opacity: 0; transform: translateY(12px) scale(0.98); }
}
.menu[popover]:not(:popover-open),
dialog.panel:not([open]) {
  opacity: 0;
  transform: translateY(4px) scale(0.99);
  transition-duration: var(--dur-micro);
  transition-timing-function: var(--ease-in-quad);
}
dialog.panel::backdrop { background: oklch(15% 0.02 260 / 0.5); transition: opacity 200ms, overlay 200ms allow-discrete, display 200ms allow-discrete; }
@starting-style { dialog.panel[open]::backdrop { opacity: 0; } }
```

- **Origin.** Set `transform-origin` to the trigger side (`top left` under a left-aligned button). Menus grow out of what opened them.
- **Height to auto.** Where `interpolate-size: allow-keywords` is supported, transition `height` to `auto`. Elsewhere use the grid row trick:

```css
.collapse { display: grid; grid-template-rows: 0fr; transition: grid-template-rows var(--dur-panel) var(--ease-out-quart); }
.collapse[data-open] { grid-template-rows: 1fr; }
.collapse > .inner { overflow: hidden; min-height: 0; }
```

- **Removal.** Fade and shrink the leaving item, then FLIP the neighbours into place, so the gap closes instead of jumping.
- **Fallback.** Without support the element simply appears. Never hide content behind an animation that may not run.

## 4. Layout changes with FLIP

Measure, change, invert, play. Use it for filters that regroup cards, sorting, list insertions and expanding cards.

```js
export function flip(elements, mutate, { duration = 320, easing = 'cubic-bezier(0.25, 1, 0.5, 1)' } = {}) {
  const before = new Map([...elements].map((element) => [element, element.getBoundingClientRect()]))
  mutate()
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) return
  for (const element of elements) {
    const first = before.get(element)
    const last = element.getBoundingClientRect()
    const dx = first.left - last.left
    const dy = first.top - last.top
    if (!dx && !dy) continue
    element.animate(
      [{ transform: `translate(${dx}px, ${dy}px)` }, { transform: 'none' }],
      { duration, easing, composite: 'add' }
    )
  }
}
```

- Translate only; scaling an element also scales its text. For size changes, animate the container with a View Transition or counter-scale the children.
- New items enter with opacity and a 0.96 scale after the others have moved (delay ≈ 60% of the move).
- In React, use the animation library already in the project (for example a `layout` prop) instead of hand-written FLIP.

## 5. View Transitions

Continuity between states and pages with real morphs.

```js
export function transition(update) {
  if (!document.startViewTransition || matchMedia('(prefers-reduced-motion: reduce)').matches) return update()
  return document.startViewTransition(update)
}
```

```css
::view-transition-old(root),
::view-transition-new(root) { animation-duration: 280ms; animation-timing-function: var(--ease-out-quart); }
.product-thumb.is-active,
.product-hero { view-transition-name: product-image; }
::view-transition-group(product-image) { animation-duration: 420ms; animation-timing-function: var(--ease-in-out-quart); }
```

- Name only the few elements that need continuity: every name is captured as a snapshot. Names must be unique on the page at the moment of capture.
- Multi-page sites: add `@view-transition { navigation: auto; }` to both pages.
- React: run the state update synchronously inside the callback (`flushSync`) so the new DOM exists when the snapshot is taken.
- Give the old view a directional exit (slide 24px, fade) and the new one the matching entrance, so navigation has a direction: forward goes left, back goes right.

## 6. Scroll-driven motion

Scroll reveals the story; it never takes control of scrolling.

```css
@supports (animation-timeline: view()) {
  .reveal {
    animation: reveal linear both;
    animation-timeline: view();
    animation-range: entry 10% cover 30%;
  }
}
@keyframes reveal { from { opacity: 0; transform: translateY(var(--rise)); } }
.progress { transform-origin: left; animation: grow linear both; animation-timeline: scroll(root); }
@keyframes grow { from { transform: scaleX(0); } }
```

- **CSS first.** Scroll timelines run off the main thread. Without support, content stays visible.
- **JS reveal fallback.** `IntersectionObserver` with threshold 0.15–0.25 and `rootMargin: '0px 0px -10% 0px'`; add the revealed class once and stop observing. Content never replays on scroll up.
- **Pinned storytelling.** A tall section (`height: 300vh`) holds a `position: sticky; top: 0; height: 100svh` stage. Map progress to a few discrete steps, each changing one thing. All step text stays in the DOM in reading order.
- **Parallax** only on decorative layers, with a 5–15% speed difference, `transform` only. Off for touch devices, small screens and reduced motion.
- **Never scroll-jack.** No wheel interception and no smooth-scroll library by default. Native momentum, anchors, find-in-page and keyboard scrolling must keep working.
- **Reveal what matters.** Apply reveals to section headings and key visuals, not to every paragraph and card.

## 7. Text in motion

- **Line mask reveal** for one display headline:
  1. Wait for `document.fonts.ready`.
  2. Split the headline into rendered lines.
  3. Wrap each line in a span with `overflow: clip` (add `padding-bottom: 0.1em` so descenders survive).
  4. Animate the inner span from `translateY(105%)` to `0`: 700ms `--ease-out-expo`, 70–90ms stagger.

  Re-split after a debounced resize. Put the full text in the heading's `aria-label` and mark the split spans `aria-hidden="true"`.
- **Per-character motion** only for one short word with a reason. Never for paragraphs.
- **No typewriter or scrambled-text headlines** unless the subject is literally a terminal or a cipher.
- **Variable fonts:** animate weight or width on hover only for a single display word. It triggers layout every frame, so keep it small or fake it with `transform`.
- **Numbers:** tween the value with `requestAnimationFrame` and ease-out over 400–800ms. Format with `Intl.NumberFormat` and use `font-variant-numeric: tabular-nums` so the width does not jitter. Animate the change from the old value, not a count-up from zero on every view. Only real numbers.

## 8. Micro-interactions that feel built

- **Buttons:** pressing scales to 0.97 in 90ms and releasing springs back. Hover shifts the background lightness 4–6% and fades in a stronger shadow on a pseudo-element. A directional icon nudges 2px its way (an arrow moves right).
- **Links:** the underline draws from the start edge (`background-size` from `0 1px` to `100% 1px`) or thickens (`text-decoration-thickness`). Color alone is not a hover state.
- **Switches:** the thumb travels on `--spring-snappy` while the track color cross-fades. Checkmarks draw with `stroke-dashoffset` in 200ms.
- **Focus:** the ring arrives with `outline-offset` going from 0 to 2px in 120ms. Never animate focus away.
- **Cards:** lift 2–4px; an image inside a clipped frame scales to 1.03 over 600ms ease-out. The whole card is one link (stretched pseudo-element) with a visible focus ring.
- **Loading:** a skeleton mirrors the final layout with at most one low-contrast shimmer (about 1.6s) or none. Spinners appear only after 300ms. Prefer optimistic updates, and progress bars that reflect real progress.
- **Copy and save actions:** the icon morphs into a check for 1.5s and an `aria-live` region announces the result.
- **Drag:** the picked item lifts (scale 1.02, the largest shadow), the others make room with FLIP, and the drop settles on `--spring-soft`.
- **Errors:** a field shakes at most once (3 cycles, 4px, 300ms) and the message appears next to it. Never shake the whole form.

## 9. Pointer-reactive detail

Desktop only, opt-in, and only where the surface is the subject.

- **Light that follows the pointer.** Write `--x` and `--y` on `pointermove`, throttled with `requestAnimationFrame`. A pseudo-element draws `radial-gradient(400px at var(--x) var(--y), oklch(100% 0 0 / 0.06), transparent)`. Use it on one hero surface or one feature panel, not everywhere.
- **Tilt** up to 4–6° with `perspective: 1000px`, springing back on leave. Only for objects (a product, a card that stands for a physical thing), never for text blocks.
- **Not by default:** magnetic buttons, custom cursors and cursor trails. They belong only in an experimental piece the user asked for, and even then text and controls keep the native cursor.
- **Turn it all off** under `(hover: none)`, `(pointer: coarse)` and reduced motion.

## 10. Choosing the tool

| Need | Use |
| --- | --- |
| Hover, press, enter and exit | CSS transitions, `@starting-style` |
| Layout change, reorder | FLIP with the Web Animations API, or the project's animation library |
| Continuity between views | View Transitions API |
| Scroll-linked motion | CSS scroll timelines; a scroll-trigger library only for complex pinned sequences |
| Multi-step timelines | Web Animations API with `finished` promises, or a timeline library already in the project |
| Springs and gestures in React | The project's motion library (spring values, drag) |
| 3D | Three.js or its React renderer, see `motion-and-3d.md` |

Check the installed versions and their docs. Do not add a library for one hover effect.

## 11. Performance

- **Animate cheap properties:** `transform` and `opacity`. `filter`, `clip-path` and masks are acceptable on small areas once the Performance panel shows no long frames. Never animate `width`, `height`, `top`, `left` or `margin`; use transforms or FLIP.
- **Shadows:** fade a pseudo-element that already has the target shadow instead of animating `box-shadow`.
- **`will-change: transform`** only right before a known animation (on `pointerenter`); remove it on `finish`.
- **Blur:** no animated `backdrop-filter`, and no stacks of blurred layers.
- **Script work:** read layout, then write; use `requestAnimationFrame` for pointer-driven updates and passive listeners for scroll and touch.
- **Pausing:** pause offscreen work with `IntersectionObserver` and stop loops on `visibilitychange`.
- **Budget:** at 4× CPU throttling no interaction frame is longer than 16ms, and animations add no layout shift (CLS stays 0).

## 12. Reduced motion

Respect `prefers-reduced-motion: reduce` in CSS and in JavaScript (`matchMedia`, listening for changes), plus the product's own setting if it has one.

| Full motion | Reduced |
| --- | --- |
| Line reveal, rise-in | Appears at once, or a 120ms opacity fade |
| Parallax, scroll scrubbing, pinned scenes | Off; a static composition with all content |
| View Transition morph | 150ms cross-fade or instant |
| Spring, overshoot | Ease-out without overshoot, ≤ 150ms |
| Ambient loop, autoplay video | Paused, with a play control |
| Number tween | Final value at once |

## 13. Motion slop

Remove on sight:

- Fade-up on every section and card.
- The same duration and easing on everything.
- Bouncing or pulsing call-to-action buttons.
- Floating blobs and infinite gradient shifts.
- Typewriter headlines.
- Scroll-jacking and smooth-scroll hijacking.
- Parallax on everything.
- 3D card tilt on text.
- Cursor trails and magnetic buttons.
- Fake counters counting up.
- Glow pulses.
- Marquees of logos without a reason.
- Loaders shown for content that is already there.

## 14. Verifying motion

- **Slow motion.** Slow the timeline to 10–25% in the browser's animation tools. Check easing, overlap, stagger order and the end frame, which must not jump.
- **Interruption.** Re-trigger and reverse mid-flight, toggle rapidly, resize mid-animation, navigate back and forward.
- **Rest-state screenshots.** Judge screenshots at rest, before and after each transition. In a `browser_run` scenario, wait for animations to finish before a screenshot with an `evaluate` step: `Promise.all(document.getAnimations().map((animation) => animation.finished))`.
- **Reduced motion.** Content and meaning stay complete.
- **Performance.** At 4× CPU throttling, no long frames during interactions and no running animations offscreen (`document.getAnimations()` while scrolled away).
