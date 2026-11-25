Create, inspect, update, or fill templates in Microsoft Word (.docx) documents natively with full UTF-8 and Cyrillic support.

## Actions

- `create`: Build a styled `.docx` file at `output_path` from structured `blocks` (`heading`, `paragraph`, `bullets`, `numbered_list`, `table`, `callout`, `divider`, `image`, `page_break`) with optional `title`, `subtitle`, `author`, `subject`, `header`, `footer`, and `page_numbers`.
- `inspect`: Extract metadata, headings, paragraphs, tables, headers, footers, comments, and `{{placeholder}}` names from an existing `.docx` (`path` or `input_path`). Read-only; does not require write permission.
- `update`: Modify an existing `.docx` (`input_path`, optional `output_path` defaulting to in-place) with exact text `replacements`, zero-based `table_updates`, and `append_blocks`, preserving existing formatting and XML parts.
- `fill_template`: Replace `{{key}}` placeholders across body, headers, footers, and split XML runs from `input_path` into `output_path` using `values`.

## Output

Mutating actions request write permission before touching disk and return JSON with `path`, `size_bytes`, counts, `summary`, and `note`. Always include the exact output file path in your final reply to the user.
