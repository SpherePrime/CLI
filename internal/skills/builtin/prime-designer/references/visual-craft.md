# Visual craft

How to move a correct design to a memorable one. Use these as decisions to make, not defaults to paste.

## 1. Token foundation

Define tokens by **role** before writing component styles. Components reference roles, never raw values.

```css
:root {
  --color-bg: oklch(98.5% 0.006 85);
  --color-surface: oklch(100% 0 0);
  --color-surface-raised: oklch(99% 0.004 85);
  --color-text: oklch(22% 0.02 260);
  --color-text-muted: oklch(45% 0.02 260);
  --color-border: oklch(88% 0.01 260);
  --color-accent: oklch(52% 0.17 255);
  --color-accent-contrast: oklch(99% 0 0);
  --color-focus: oklch(60% 0.2 255);
  --color-success: oklch(55% 0.14 150);
  --color-warning: oklch(70% 0.15 75);
  --color-danger: oklch(55% 0.2 25);

  --space-1: 4px; --space-2: 8px; --space-3: 12px; --space-4: 16px;
  --space-5: 24px; --space-6: 32px; --space-7: 48px; --space-8: 64px; --space-9: 96px; --space-10: 128px;

  --radius-sm: 4px; --radius-md: 8px; --radius-lg: 16px; --radius-full: 999px;

  --shadow-1: 0 1px 2px oklch(20% 0.02 260 / 0.06), 0 1px 1px oklch(20% 0.02 260 / 0.04);
  --shadow-2: 0 4px 12px oklch(20% 0.02 260 / 0.08), 0 2px 4px oklch(20% 0.02 260 / 0.05);
  --shadow-3: 0 16px 40px oklch(20% 0.02 260 / 0.14), 0 4px 12px oklch(20% 0.02 260 / 0.06);

  --ease-out: cubic-bezier(0.22, 1, 0.36, 1);
  --ease-in-out: cubic-bezier(0.65, 0, 0.35, 1);
  --duration-fast: 150ms; --duration-base: 240ms; --duration-slow: 480ms;

  --z-sticky: 100; --z-dropdown: 200; --z-overlay: 300; --z-modal: 400; --z-toast: 500;
}
```

The values above are a shape to follow. Derive the actual hues, scale, and character from the subject.

## 2. Color

- **Work in OKLCH.** Equal lightness steps look equal; hue shifts do not change perceived brightness. Generate ramps by stepping L, keeping C moderate at extremes.
- **60–30–10.** Dominant neutral, secondary surface/brand, and an accent reserved for action and emphasis. If the accent appears on more than ~10% of the screen, it stops meaning "click here".
- **Tinted neutrals.** Pure grey looks dead next to a colored brand. Shift neutrals 2–5° chroma toward the brand hue.
- **Contrast by construction.** Pick text/background pairs with `scripts/contrast.mjs`; record the passing pairs next to the palette. A palette swatch that fails 3:1 against every other swatch is decoration only.
- **Dark mode is a separate design.** Surfaces get lighter as they rise; reduce accent chroma 10–20%; replace shadows with lighter surfaces and subtle borders; avoid pure black (#000) behind pure white text for long reading.
- **State colors** need a non-color partner: icon, label, or pattern.

## 3. Typography

- **Scale.** Pick a ratio by density: 1.125–1.2 for dense apps, 1.25 for products, 1.333–1.5 for editorial and marketing. Make it fluid with bounds:

```css
--step-0: clamp(1rem, 0.95rem + 0.25vw, 1.125rem);
--step-1: clamp(1.25rem, 1.15rem + 0.5vw, 1.5rem);
--step-2: clamp(1.563rem, 1.4rem + 0.8vw, 2rem);
--step-3: clamp(1.953rem, 1.7rem + 1.3vw, 2.75rem);
--step-4: clamp(2.441rem, 2rem + 2.2vw, 3.75rem);
--step-5: clamp(3.052rem, 2.4rem + 3.3vw, 5.5rem);
```

- **Pairing.** One family with strong weight range, or two with clear contrast in structure (serif display + grotesk text, mono for data). Never two similar sans-serifs.
- **Display type** wants tighter tracking (−0.01 to −0.03em) and tight leading (1.0–1.15). **Small caps and labels** want looser tracking (+0.04 to +0.08em). **Body** keeps default tracking, 1.5–1.7 leading.
- **Measure** 45–80 characters (`max-width: 65ch`).
- **Details that signal craft:** `text-wrap: balance` on headings, `text-wrap: pretty` on paragraphs, `font-variant-numeric: tabular-nums` in tables, real quotes and dashes (« », “ ”, —, –), non-breaking spaces between numbers and units, hanging punctuation for pull quotes, `font-optical-sizing: auto` for variable fonts.
- **Cyrillic and other scripts.** Verify the chosen font has full glyph coverage and matching weights; many display fonts ship Latin-only and silently fall back.

## 4. Space and composition

- **Spacing from one scale.** Inside a component: 4–16px. Between components: 24–48px. Between sections: 64–160px. If inner spacing ≥ outer spacing, grouping breaks.
- **Proximity carries meaning.** Label sits closer to its field than to the previous field. Heading sits closer to its paragraph than to the section above.
- **Grid.** 12 columns on desktop, 4–6 on mobile, with a defined gutter. Break the grid on purpose (one full-bleed image, one offset quote), not by accident.
- **Asymmetry with intent.** A centered stack of everything reads as a template. Shift weight: big type left, small details right; a dominant image and a quiet column.
- **Scale contrast.** Memorable layouts pair very large with very small. Timid layouts make everything medium.
- **Edges.** 16px minimum gutter on phones, 24–32px on tablets; on wide screens let the background, imagery, or a grid of content use the width while text keeps its measure.
- **Optical corrections.** Play icons nudged right, round shapes slightly larger than squares to look equal, quotes hung outside the text block, icon + label centered on the cap height, not the box.

## 5. Surfaces, depth, and detail

- **Elevation** has 2–3 levels and a meaning (resting, raised on hover, floating overlay). Shadows are layered (a tight contact shadow plus a soft ambient one) and tinted, never pure black at high opacity.
- **Borders** at 1px with low-contrast color separate surfaces in dense UIs better than shadows. Hairlines on high-DPI: `0.5px` only where supported and tested.
- **Radius** follows a scale and nests: inner radius = outer radius − padding.
- **Texture and grain** only when the subject is material (paper, film, print). Keep it below the threshold where it reduces text contrast.
- **Icons** from one family, one stroke width, sized on a 4px grid, aligned to text cap height.
- **Imagery** gets one consistent treatment: crop logic, color grade, corner radius, caption style.

## 6. Interaction polish

- **Every interactive element** has default, hover, active (pressed), focus-visible, disabled, and loading states designed together. Pressed state: slight scale (0.97–0.98) or darken; never move layout.
- **Feedback** within 100ms. Skeletons mirror the final layout; spinners appear only after ~300ms to avoid flicker.
- **Focus ring**: 2–3px, offset 2px, high contrast against both the element and the page; matches the brand rather than the browser default.
- **Cursor** and `touch-action` set correctly for draggable or zoomable areas.
- **Empty states** have a reason, an illustration or icon tied to the subject, and one next action.

## 7. Motion character

- **Durations:** micro 120–200ms, UI 200–300ms, spatial 300–600ms. Longer only for storytelling the user chose to watch.
- **Easing:** ease-out (fast start, gentle stop) for entering; ease-in for exiting; spring or overshoot only when the subject is physical or playful.
- **Choreography:** stagger related items 30–60ms; lead with the element the user acted on; never animate everything at once.
- **Purpose:** each animation explains a change (where something came from, where it went, what it is connected to). Decorative loops need a pause control and stop under reduced motion.
- **Implementation:** tokens, springs and recipes are in `motion-craft.md`.

## 8. Signature moves (pick one, adapt it)

Use one as the memorable idea, derived from the subject. Do not stack several.

- **Typographic hero:** oversized, tightly set display type with one word treated differently (italic, outline, color, cut by an image).
- **Material honesty:** the interface behaves like the subject's material (paper folds, metal snaps, liquid settles).
- **Data as image:** the real dataset drawn large becomes the hero visual.
- **Editorial grid:** magazine-style asymmetric columns, pull quotes, captions, and generous white space.
- **Object in space:** a single 3D or photographed object with real lighting, rotated or disassembled by the user.
- **Kinetic structure:** layout that rearranges meaningfully on interaction (filters that physically regroup cards, timeline that scrubs).
- **Restraint:** near-monochrome, one accent, perfect typography, nothing else. Hardest to do well; every detail shows.

## 9. Final polish pass

Run on the finished screen before the quality gate:

1. Remove one decorative element. Did anything get worse? If not, keep it removed.
2. Check the three largest type sizes: do they form a clear step?
3. Check every vertical gap between sections: are they from the scale and consistent?
4. Squint: is the primary action the strongest accent on screen?
5. Freeze motion: does the static frame still look composed?
6. Swap the logo for a competitor's: does the design still fit? If yes, push the signature further.
