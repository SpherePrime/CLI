Create, inspect, or update Microsoft Excel (.xlsx) workbooks natively with formulas, formatting, charts, and full UTF-8 and Cyrillic support.

## Actions

- `create`: Create an `.xlsx` workbook at `output_path` from `sheets`, supporting `headers`, `header_style`, `rows`, `cells`, Excel formulas (`=SUM(...)`, etc.), `styles`, `col_widths`, `row_heights`, `auto_fit`, `freeze_pane`, `auto_filter`, `merges`, and `charts`.
- `inspect`: Read sheet list, sheet summaries, dimensions, values, formulas, formula errors, and merged cells from an existing `.xlsx` (`path` or `input_path`, optional `sheet` and `range`). Read-only; does not require write permission.
- `update`: Modify an existing `.xlsx` (`input_path`, optional `output_path` defaulting to in-place) by adding/renaming/deleting sheets (`add_sheets`, `rename_sheets`, `delete_sheets`), setting cells (`set_cells`), appending rows (`append_rows`), applying styles (`apply_styles`), adjusting `col_widths`/`row_heights`, merging cells (`merges`), adding charts (`add_charts`), or setting `active_sheet`.

## Output

Mutating actions request write permission before touching disk and return JSON with `path`, `size_bytes`, `sheets`, `active_sheet`, `sheet_count`, `summary`, and `note`. Always include the exact output file path in your final reply to the user.
