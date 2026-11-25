---
name: prime-pdf
description: Use when creating, inspecting, merging, splitting, or rotating PDF documents (.pdf), including reports, proposals, summaries, invoices, or attached PDF files.
---

# Prime PDF

Use the native `pdf_document` tool for all PDF generation, inspection, and manipulation. It produces UTF-8 / Cyrillic-safe PDFs natively without external dependencies.

## Actions & Key Parameters

- `create`: Requires `output_path` and structured `blocks`. Supports `title`, `subtitle`, `author`, `subject`, `page_size` (`A4` or `Letter`), `orientation` (`portrait` or `landscape`), `margins` (`{top, right, bottom, left}` in points), `header`, `footer`, and `page_numbers`.
- `inspect`: Read-only check of metadata, page count, dimensions, rotation, and extracted text. Accepts `path` (or `input_path`) and optional `page_range` (e.g. `"1-3,5"`).
- `merge`: Combine ordered `input_paths` into `output_path`.
- `split`: Extract pages from `input_path` using either `page_range` into `output_path` / `output_dir`, or fixed chunks via `span` (>= 1) into `output_dir`.
- `rotate`: Rotate all or `page_range` pages of `input_path` by `angle` (multiple of 90) to `output_path` (defaults to `input_path` in-place).

## Content Blocks (`blocks`)

Each item in `blocks` uses `type`:

- `heading`: `text`, `level` (`1`, `2`, or `3`)
- `paragraph`: `text`, optional `align` (`left`, `center`, `right`, `justify`)
- `bullet_list`: `items` (`[]string`)
- `numbered_list`: `items` (`[]string`)
- `table`: `headers` (`[]string`), `rows` (`[][]string`), optional `col_widths` (`[]float64`)
- `callout`: `title`, `text`, optional `tone` (`info`, `warning`, `success`, `error` / `danger`)
- `image`: `image_path`, optional `width`, `height`, `caption`
- `divider`: horizontal section separator
- `page_break`: force start of a new page

## Layout & Quality Rules

1. **Clear hierarchy:** Set `title` and optional `subtitle` for the cover/header band, use `heading` levels 1–3 consistently, and separate major sections with `divider` or `page_break` only where natural.
2. **Readable tables:** Keep cell text concise. For tables with 5+ columns or long labels, pass explicit `col_widths` or switch `orientation` to `"landscape"`.
3. **Cyrillic & UTF-8 safety:** Write natural Unicode text directly (Cyrillic, Latin, punctuation, currency symbols); never transliterate or use ASCII fallbacks.
4. **Running chrome:** Enable `page_numbers: true` and set concise `header` / `footer` text on multi-page documents.

## Working with Attached or Existing PDFs

1. Always call `pdf_document` with `action: "inspect"` and `input_path` (or `path`) first to read page count, orientation, and text.
2. For structural edits on existing PDFs, use `split`, `rotate`, or `merge` with `input_path` / `input_paths` and a clear `output_path` (avoid overwriting an original attached file unless the user asked for in-place modification).
3. When the user asks to rewrite or reformat content from an attached PDF, extract content with `inspect` and generate a clean new file with `create`.

## Verification & Final Reply

1. After every write (`create`, `merge`, `split`, `rotate`), run `pdf_document` with `action: "inspect"` on the generated file to verify `page_count`, metadata, and extracted text (including Cyrillic characters).
2. Always cite the exact output file path on its own line in your final reply.
