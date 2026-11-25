Create, inspect, update, or fill templates in Microsoft PowerPoint (.pptx) presentations natively with full UTF-8 and Cyrillic support.

## Actions

- `create`: Build a styled `.pptx` presentation at `output_path` from `slides` (`title`, `section`, `bullets`, `two_column`, `table`, `metrics`, `image`, `blank` layouts), theme colors (`primary_color`, `accent_color`, `background_color`, `text_color`), `font_family`, `aspect_ratio` (`16:9` or `4:3`), `slide_numbers`, and speaker `notes`.
- `inspect`: Extract metadata, slide dimensions, slide titles, text, tables, speaker notes, and `{{placeholder}}` names from an existing `.pptx` (`path` or `input_path`). Read-only; does not require write permission.
- `update`: Modify an existing `.pptx` (`input_path`, optional `output_path` defaulting to in-place) with text `replacements`, `notes_updates`, and `append_slides`, preserving existing slide formatting and media.
- `fill_template`: Replace `{{key}}` placeholders across slides, tables, and notes (including split XML runs) from `input_path` into `output_path` using `values`.

## Output

Mutating actions request write permission before touching disk and return JSON with `path`, `size_bytes`, `slide_count`, `summary`, and `note`. Always include the exact output file path in your final reply to the user.
