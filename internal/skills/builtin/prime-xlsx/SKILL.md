---
name: prime-xlsx
description: Use when creating, inspecting, or updating Microsoft Excel workbooks (.xlsx), spreadsheets, financial models, trackers, budgets, data tables, or attached Excel files.
---

# Prime Excel (.xlsx)

Use the native `excel_workbook` tool for all `.xlsx` creation, inspection, and editing with live Excel formulas, number formatting, styling, charts, and full UTF-8 / Cyrillic support.

## Actions & Key Parameters

- `create`: Requires `output_path` and `sheets`. Supports `title`, `author`, `subject`, and `active_sheet`.
- `inspect`: Read-only inspection of `sheets`, `active_sheet`, `sheet_summaries`, `dimensions`, `rows`, `formulas`, `formula_errors`, and `merged_cells`. Accepts `path` (or `input_path`), optional `sheet`, and optional `range` (e.g. `"A1:F30"`).
- `update`: Modify an existing `.xlsx` at `input_path` (writes to `output_path`, or in-place if omitted). Supports `add_sheets`, `rename_sheets` (`[{from, to}]`), `delete_sheets`, `set_cells` (`[{sheet, cell, value, style}]`), `append_rows` (`[{sheet, rows}]`), `apply_styles` (`[{sheet, range, style}]`), `col_widths` (`[{sheet, column, width}]`), `row_heights` (`[{sheet, row, height}]`), `merges` (`[{sheet, range}]`), `add_charts` (`[{sheet, chart}]`), and `active_sheet`.

## Sheet Definition (`sheets` / `add_sheets`)

Each sheet object supports:

- `name`: worksheet name
- `headers`: column header strings (row 1) and optional `header_style`
- `rows`: 2D array of values (strings, numbers, booleans, or formula strings starting with `=`)
- `cells`: map of A1 cell coordinates to values/formulas for non-tabular or summary cells
- `styles`: `[{range, style}]` where `style` supports `bold`, `italic`, `underline`, `font_family`, `font_size`, `font_color`, `fill`, `border`, `border_style` (`thin`, `medium`, `dashed`, `dotted`, `thick`, `double`), `border_color`, `align`, `vertical_align`, `wrap_text`, `num_fmt`
- `col_widths` (`{"A": 24}`), `row_heights` (`{1: 24}`), `auto_fit` (`true`/`false`)
- `freeze_pane`: top-left unfrozen cell (e.g. `"A2"` freezes row 1)
- `auto_filter`: filter range (e.g. `"A1:F25"`)
- `merges`: list of A1 ranges (e.g. `["A1:D1"]`)
- `charts`: `[{type, cell, title, series: [{name, categories, values}], width, height, legend_position, show_data_labels}]`
  - `type`: `col`, `col_stacked`, `bar`, `bar_stacked`, `line`, `pie`, `area`, `doughnut`
  - `legend_position`: `none`, `top`, `bottom`, `left`, `right`, `top_right`

## Spreadsheet Quality Rules

1. **Always use live Excel formulas:** Put `=SUM(...)`, `=AVERAGE(...)`, `=IF(...)`, `=SUMIF(...)`, or cross-sheet references (`='Sales'!C12`) in total, margin, variance, and KPI cells—never hardcode calculated totals.
2. **Clean number formats:** Apply `num_fmt` via `styles` or `apply_styles` (e.g. `#,##0`, `#,##0.00`, `0.0%`, `yyyy-mm-dd`, `$#,##0.00` or `#,##0.00 "₽"`).
3. **Usability & layout:** Set `freeze_pane: "A2"` and `auto_filter` on data tables, enable `auto_fit: true` (or explicit `col_widths`), and place charts in empty cells beside or below data without overlapping tables.
4. **Attached `.xlsx` files:** Always call `inspect` with `input_path` first (check `sheets`, `sheet_summaries`, and cell coordinates) before calling `update` with `input_path`. Write to a new `output_path` unless in-place editing was requested.

## Verification & Final Reply

1. After every `create` or `update`, run `excel_workbook` with `action: "inspect"` on every modified sheet and confirm `formula_errors` is empty (zero `#REF!`, `#DIV/0!`, `#VALUE!`, `#NAME?`, `#N/A`, `#NUM!`, `#NULL!`, `#SPILL!`, `#CALC!`).
2. Always cite the exact output file path on its own line in your final reply.
