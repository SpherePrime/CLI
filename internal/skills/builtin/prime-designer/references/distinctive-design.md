# Distinctive design

How to make an interface look authored instead of generated. These are techniques, not references: never reproduce a known site. Every decision comes from the subject, the content and the user.

## 1. Derive the visual language from the subject

Do this before any code, in a few minutes:

1. **List the subject's real artifacts:** materials, tools, documents, places, sounds, rituals, time of day. For a bakery: flour dust, kraft paper, oven timers, chalk price boards, early light. For a logistics tool: waybills, route maps, barcodes, shift schedules.
2. **Extract qualities from them:**
   - shape: soft loaf versus sharp ticket edge;
   - texture: matte, grainy, glossy, printed;
   - type cues: stencil, receipt mono, hand-painted, engraved, technical;
   - color sources: sample from photographs of the material, not from trend palettes;
   - tempo: slow and warm, or fast and exact.
3. **Turn two or three of them into system decisions:** the radius scale, the type pairing, the palette, the motion character, one signature component (for example, prices set like chalk-board numerals, or progress drawn as a route).
4. **Write the character** as 3–5 traits a competitor could not claim. "Modern, clean, friendly" fails; "floury, early-morning, hand-counted" works.
5. **Write the signature** in one sentence: the one thing someone remembers after closing the tab.

## 2. Composition

- **Order at 375 px first.** Sketch the phone order for content priority, then compose the desktop with scale contrast instead of stretching the phone layout.
- **Hero structures.** Pick by content, not habit:
  - **Statement and proof:** one large sentence of real value (display 56–120px), and directly under it the evidence (a real product crop, a real photo, a real number with its source) large enough to read.
  - **Object-led:** one subject image or render fills 50–70% of the viewport. Text sits in its negative space, aligned to the object's lines.
  - **Index-led:** for content-rich sites the hero is the index itself, a typographic list of real items with previews on hover and focus.
  - **Tool-led:** the product's main interaction works in the first viewport: search, configurator, calculator, booking.
- **Grid with intent.** On a 12-column grid prefer spans like 7/5, 8/4 and 5/7 with offsets over 6/6 everywhere. Keep at most three strong vertical alignment lines per screen.
- **Rhythm.** Alternate dense and airy sections, and vary their structure. Five "title + three cards" sections in a row read as a template.
- **Scale contrast.** One very large element per screen against small, precise detail: captions and labels at 12–13px with good tracking.
- **Deliberate overlap.** Let one element cross a boundary (an image over a section edge, a headline across an image), with a z-index from the scale, checked at every width.
- **Full-bleed moments.** One or two per page, where the subject deserves the whole width.
- **Whitespace as material.** The most important thing gets the largest margin around it.

## 3. Typography that carries the design

- **Display type is the image.** Set it large with tight leading (0.95–1.1) and tracking (−0.02 to −0.04em), optical kerning, and `text-wrap: balance`.
- **Pairing.** Contrast in structure (a serif or slab display with a grotesk text face, mono only for real data), never two similar sans-serifs.
- **Emphasis.** Inside a headline, use weight or a true italic, not a second color.
- **One label style.** One size, tracking and case for labels and metadata, used where it carries information. Do not put uppercase mono above every heading.
- **Micro-typography:** real quotes and dashes, non-breaking spaces before units, tabular numerals in tables and oldstyle numerals in running text when the font has them, hanging punctuation for pull quotes, a measure of 45–75 characters.
- **Coverage.** Check that the font covers Cyrillic and every weight you use. Prefer the fonts in `assets/fonts/` or the project's own.

## 4. Color with authorship

- **Sources.** Derive the palette from the subject's photos: one dominant hue family, one accent reserved for action, neutrals tinted toward the dominant hue.
- **Color as structure.** Build with blocks (a section on a deep surface, a colored band behind a key fact), not decorative gradients.
- **Gradients** only when they model something real (light, sky, a material), with no more than 8% lightness difference. Add fine noise only when banding is visible.
- **Dark themes:** off-black surfaces (OKLCH L 14–20%) that lighten as they rise, desaturated accents, no glow.
- **Tuning.** Check every pair with `scripts/contrast.mjs`; tune lightness, not hue, to pass.

## 5. Imagery and graphics

- **Priority:** the user's real photos first, then carefully chosen stock, then generated images. Keep one grade for all of them (contrast, temperature, saturation) and one crop logic.
- **Generated images.** Write the full prompt: subject, setting, light, lens, composition, the project palette and the place the image goes. Reject results with artifacts: garbled text, wrong hands, plastic skin, impossible reflections, melted edges.
- **Illustration** from one hand: the same stroke, palette, perspective and level of detail.
- **Icons** from one family, with the stroke weight matched to the adjacent text weight.
- **Drawn detail.** Custom SVG dividers and diagrams drawn from the real data or process add more authorship than any effect.

## 6. The detail layer

What makes a page feel finished:

- **States.** Every state is designed: hover, focus, active, disabled, loading, empty, error, success. Focus rings match the brand.
- **Radii** nest (inner = outer − padding). Hairlines are 1px in a tinted color, not grey.
- **Browser chrome:** brand-colored `::selection`, `accent-color` for native controls, a subtle `scrollbar-color`, a favicon, an Open Graph image and a `theme-color` meta.
- **Loading without jumps:**
  - fonts: `font-display: swap` with a metric-matched fallback (`size-adjust`, `ascent-override`);
  - images: explicit dimensions and a dominant-color or blurred placeholder;
  - no layout shift when content arrives.
- **Edge pages:** 404, error and empty pages get the same care as the home page, each with one helpful next action.
- **Cursor:** a pointer only on things that act. Text stays selectable.

## 7. Generic moves and what to do instead

| Generic move | Do instead |
| --- | --- |
| Gradient hero with a centered headline and two buttons | A hero structure from §2 with real evidence and one primary action |
| Three equal feature cards with icons | One large feature with a real screenshot or demo plus compact secondary items, or a numbered list with inline proof |
| Grey logo strip "trusted by" | Only real logos and quotes with names; otherwise omit the section |
| Glass panels over blurred blobs | Solid surfaces with elevation from the token scale |
| Fade-up on every section | One intro sequence and motion only where state changes (`motion-craft.md`) |
| An icon or emoji in every heading | Typography and real imagery |
| An uppercase label above every heading | One label style, used where it carries information |
| Everything at 16px radius | A radius scale tied to the character (sharp for technical, soft for warm), nested |
| Abstract 3D blobs and shapes | The real subject, its data or its process |
| "Unlock the power of…" copy | Specific copy: facts, names, numbers and outcomes the user gave |
| Counters counting up to made-up stats | Real numbers with a source, or nothing |
| Neon glow on a dark theme | Calm, desaturated dark surfaces |
| Everything centered | Left-aligned reading; centering only for one short statement |
| The same section layout repeated | Rhythm and structure that vary with the content |
| Generic illustrated people | Photos of the real thing or consistent custom graphics |

## 8. Self-critique loop

After the first working build, take screenshots at 1440 and 390 px (overview plus close-ups) and critique them as a demanding art director:

1. What will someone remember? If nothing, strengthen the signature.
2. What looks like a template? Replace it using §7.
3. Where does the eye land first? It must be the primary message or action.
4. What is noise? Remove it, then check that nothing got worse.
5. Is every detail in §6 done?

Iterate at least once and note what changed and why.
