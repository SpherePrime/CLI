---
name: prime-pptx
description: Use when creating, inspecting, updating, or filling templates in Microsoft PowerPoint presentations (.pptx), pitch decks, reports, or attached slide decks.
---

# Prime PowerPoint (.pptx)

Use the native `powerpoint_presentation` tool for all `.pptx` creation, inspection, targeted editing, and template filling with full UTF-8 and Cyrillic support.

## Actions & Key Parameters

- `create`: Requires `output_path` and `slides`. Supports `title`, `author`, `subject`, `aspect_ratio` (`16:9` or `4:3`), theme hex colors (`primary_color`, `accent_color`, `background_color`, `text_color`), `font_family`, and `slide_numbers`.
- `inspect`: Read-only extraction of `slide_count`, `dimensions`, `metadata`, `slides` (number, title, texts, tables, speaker notes), and `{{placeholder}}` names from `path` (or `input_path`).
- `update`: Modify an existing `.pptx` at `input_path` (writes to `output_path`, or in-place if omitted) while preserving existing slide layouts and media. Supports `replacements` (exact string map), `notes_updates` (`[{slide, notes}]`, 1-based slide index), `append_slides` (`[]SlideSpec`), and optional theme / `slide_numbers` for appended slides.
- `fill_template`: Replace `{{key}}` placeholders from `input_path` into `output_path` using `values` across slides, tables, and notes (including split XML runs).

## Slide Definitions (`slides` / `append_slides`)

Each slide supports:

- `layout`: `title`, `title_content`, `two_column`, `table`, `metrics`, `image_text`, or `blank` (inferred automatically from populated fields if omitted)
- `title`, `subtitle`, `body`, `bullets` (`[]string`)
- `left`, `right`: `{heading, body, bullets}` for `two_column`
- `table`: `{headers, rows, col_widths, header_fill}` for `table`
- `metrics`: `[{label, value, delta, color}]` for `metrics` KPI cards
- `image_path`, `image_caption` for `image_text`
- `notes`: speaker notes text
- `background_color`: optional per-slide hex override

## Slide Design & Density Rules

1. **Clear deck arc:** Start with a `title` slide, vary layouts (`title_content`, `two_column`, `metrics`, `table`, `image_text`) to match the message, and end with concrete takeaways or next steps.
2. **Enforce visual density limits:**
   - Keep `title_content` slides to 3–5 concise bullets (short scannable phrases, not full paragraphs).
   - Keep `metrics` slides to 2–4 KPI cards with short labels and clear values/deltas.
   - Keep `table` slides compact (ideally <= 5 columns and <= 6 rows) with brief cell values.
   - Move detailed commentary, citations, and talking points into `notes`.
3. **Cohesive theme:** Pick one clean palette (`primary_color`, `accent_color`, `background_color`, `text_color`) with strong contrast and enable `slide_numbers: true` for decks of 4+ slides.

## Working with Attached `.pptx` Files

1. Always run `inspect` with `input_path` (or `path`) first to review existing slide numbers, titles, text, notes, and `placeholders`.
2. Use `fill_template` with `input_path`, `output_path`, and `values` whenever `{{placeholder}}` tokens are present.
3. Use `update` with `input_path`, `replacements`, `notes_updates`, and/or `append_slides` to edit an existing deck without losing its design. Prefer a separate `output_path` unless in-place editing was requested.

## Verification & Final Reply

1. After creating, updating, or filling a `.pptx`, run `powerpoint_presentation` with `action: "inspect"` on the output file to verify `slide_count`, titles, tables, speaker notes, and (for templates) zero remaining `placeholders`.
2. Always cite the exact output file path on its own line in your final reply.
