Create, inspect, merge, split, or rotate PDF documents natively with full UTF-8 and Cyrillic support.

## Actions

- `create`: Generate a formatted PDF from structured `blocks` (`heading`, `paragraph`, `bullets`, `numbered_list`, `table`, `callout`, `divider`, `image`, `page_break`) with optional `title`, `subtitle`, `author`, `subject`, `page_size` (`A4` or `Letter`), `orientation` (`portrait` or `landscape`), `margins`, `header`, `footer`, and `page_numbers`. Requires `output_path`.
- `inspect`: Read metadata, page dimensions, rotation, and extracted text from an existing PDF (`path` or `input_path`, optional `page_range`). Read-only; does not require write permission.
- `merge`: Combine multiple PDFs (`input_paths`) into `output_path`.
- `split`: Extract selected pages (`page_range` into `output_path` or `output_dir`) or split into fixed-size chunks (`span` pages per file into `output_dir`) from `input_path`.
- `rotate`: Rotate all or selected pages (`page_range`) of `input_path` by `angle` (multiple of 90 degrees) and save to `output_path` (defaults to `input_path` in-place).

## Output

Mutating actions request write permission before touching disk and return JSON with `path`, `size_bytes`, `page_count` (or `files`), `summary`, and `note`. Always include the exact output file path in your final reply to the user.
