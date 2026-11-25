---
name: prime-docx
description: Use when creating, inspecting, updating, or filling templates in Microsoft Word documents (.docx), including reports, contracts, letters, specifications, or attached Word files.
---

# Prime Word (.docx)

Use the native `word_document` tool for all `.docx` creation, inspection, targeted editing, and template filling with full UTF-8 and Cyrillic support.

## Actions & Key Parameters

- `create`: Requires `output_path` and `blocks`. Supports `title`, `subtitle`, `author`, `subject`, `header`, `footer`, and `page_numbers`.
- `inspect`: Read-only extraction of `metadata`, `headings`, `paragraphs`, `tables`, `headers`, `footers`, `comments`, and `{{placeholder}}` names from `path` (or `input_path`).
- `update`: Edit an existing `.docx` at `input_path` (writes to `output_path`, or in-place if omitted) while preserving existing XML styles and media. Supports `replacements` (exact string map), `table_updates` (`[{table_index, row, col, value}]`, 0-based), and `append_blocks`.
- `fill_template`: Replace `{{key}}` placeholders from `input_path` into `output_path` using `values` (map of placeholder name to replacement string) across body, tables, headers, footers, and split XML runs.

## Content Blocks (`blocks` and `append_blocks`)

Each block uses `type`:

- `heading`: `text`, `level` (`1`–`3`)
- `paragraph`: `text` or formatted `runs` (`[{text, bold, italic, underline, color, font_size, font_family}]`), optional `align` (`left`, `center`, `right`, `justify`)
- `bullet_list`: `items` (`[]string`)
- `numbered_list`: `items` (`[]string`)
- `table`: `headers`, `rows`, optional `col_widths`, `header_fill` (6-digit hex)
- `callout`: `title`, `text`, optional `tone` (`info`, `warning`, `success`, `danger`)
- `image`: `image_path`, optional `width`, `height`, `caption`
- `divider`: horizontal rule
- `page_break`: page break

## Authoring & Editing Rules

1. **Structured documents:** Use `title`/`subtitle`, logical `heading` levels, concise `bullet_list`/`numbered_list`, styled `table` blocks for structured data, and `header`/`footer`/`page_numbers` on multi-page documents.
2. **Attached `.docx` files:** Always run `inspect` with `input_path` (or `path`) first to see existing headings, table indices, comments, and `placeholders`.
3. **Preserve existing formatting:**
   - If the document contains `{{placeholder}}` tokens, prefer `action: "fill_template"` with `input_path`, `output_path`, and `values`.
   - For surgical edits to existing text, table cells, or adding new sections, use `action: "update"` with `input_path`, `replacements`, `table_updates`, and/or `append_blocks`.
   - Write to a distinct `output_path` unless the user explicitly asks to overwrite the input file in-place.

## Verification & Final Reply

1. After creating, updating, or filling a `.docx`, run `word_document` with `action: "inspect"` on the output file to verify headings, tables, text integrity, and (for templates) that `placeholders` is empty.
2. Always cite the exact output file path on its own line in your final reply.
