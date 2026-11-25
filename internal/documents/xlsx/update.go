package xlsx

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/xuri/excelize/v2"
)

func Update(opts UpdateOptions) (*Result, error) {
	inPath, err := resolvePath(opts.WorkingDir, opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve input path: %w", err)
	}
	if _, err := os.Stat(inPath); err != nil {
		return nil, fmt.Errorf("input file not found: %w", err)
	}

	rawOut := strings.TrimSpace(opts.OutputPath)
	if rawOut == "" {
		rawOut = opts.InputPath
	}
	outPath, err := resolvePath(opts.WorkingDir, rawOut)
	if err != nil {
		return nil, fmt.Errorf("resolve output path: %w", err)
	}

	f, err := excelize.OpenFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() { _ = f.Close() }()

	for _, spec := range opts.AddSheets {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			return nil, errors.New("new sheet name is required")
		}
		if idx, err := f.GetSheetIndex(name); err != nil {
			return nil, fmt.Errorf("invalid sheet name %q: %w", name, err)
		} else if idx >= 0 {
			return nil, fmt.Errorf("sheet %q already exists", name)
		}
		if _, err := f.NewSheet(name); err != nil {
			return nil, fmt.Errorf("add sheet %q: %w", name, err)
		}
		spec.Name = name
		if err := populateSheet(f, spec); err != nil {
			return nil, fmt.Errorf("populate added sheet %q: %w", name, err)
		}
	}

	renamed := make(map[string]string, len(opts.RenameSheets))
	for _, ren := range opts.RenameSheets {
		from := strings.TrimSpace(ren.From)
		to := strings.TrimSpace(ren.To)
		if from == "" || to == "" {
			return nil, errors.New("rename_sheets requires non-empty from and to names")
		}
		fromIdx, err := f.GetSheetIndex(from)
		if err != nil || fromIdx < 0 {
			return nil, fmt.Errorf("cannot rename sheet %q: sheet not found", from)
		}
		if from != to {
			if toIdx, err := f.GetSheetIndex(to); err != nil {
				return nil, fmt.Errorf("invalid target sheet name %q: %w", to, err)
			} else if toIdx >= 0 {
				return nil, fmt.Errorf("cannot rename sheet %q to %q: target already exists", from, to)
			}
			if err := f.SetSheetName(from, to); err != nil {
				return nil, fmt.Errorf("rename sheet %q to %q: %w", from, to, err)
			}
		}
		renamed[from] = to
	}

	resolveTargetSheet := func(raw string) (string, error) {
		name := strings.TrimSpace(raw)
		if name == "" {
			active := f.GetSheetName(f.GetActiveSheetIndex())
			if active != "" {
				return active, nil
			}
			sheets := f.GetSheetList()
			if len(sheets) == 0 {
				return "", errors.New("workbook has no sheets")
			}
			return sheets[0], nil
		}
		if idx, err := f.GetSheetIndex(name); err == nil && idx >= 0 {
			return name, nil
		}
		if mapped, ok := renamed[name]; ok {
			if idx, err := f.GetSheetIndex(mapped); err == nil && idx >= 0 {
				return mapped, nil
			}
		}
		return "", fmt.Errorf("sheet %q not found", name)
	}

	for _, delName := range opts.DeleteSheets {
		name, err := resolveTargetSheet(delName)
		if err != nil {
			return nil, fmt.Errorf("delete sheet %q: %w", delName, err)
		}
		if len(f.GetSheetList()) <= 1 {
			return nil, fmt.Errorf("cannot delete the last remaining sheet %q", name)
		}
		if err := f.DeleteSheet(name); err != nil {
			return nil, fmt.Errorf("delete sheet %q: %w", name, err)
		}
	}

	for _, edit := range opts.SetCells {
		sheet, err := resolveTargetSheet(edit.Sheet)
		if err != nil {
			return nil, fmt.Errorf("set cell %q: %w", edit.Cell, err)
		}
		cell := strings.TrimSpace(edit.Cell)
		if _, _, err := excelize.CellNameToCoordinates(cell); err != nil {
			return nil, fmt.Errorf("invalid cell %q: %w", edit.Cell, err)
		}
		if err := setCellValueOrFormula(f, sheet, cell, edit.Value); err != nil {
			return nil, fmt.Errorf("set cell %s!%s: %w", sheet, cell, err)
		}
		if edit.Style != nil {
			if err := applyRangeStyle(f, sheet, cell, *edit.Style); err != nil {
				return nil, fmt.Errorf("style cell %s!%s: %w", sheet, cell, err)
			}
		}
	}

	for _, app := range opts.AppendRows {
		if len(app.Rows) == 0 {
			continue
		}
		sheet, err := resolveTargetSheet(app.Sheet)
		if err != nil {
			return nil, fmt.Errorf("append rows: %w", err)
		}
		existing, err := f.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("read rows of %q: %w", sheet, err)
		}
		nextRow := len(existing) + 1
		for _, row := range app.Rows {
			for colIdx, val := range row {
				cell, err := excelize.CoordinatesToCellName(colIdx+1, nextRow)
				if err != nil {
					return nil, err
				}
				if err := setCellValueOrFormula(f, sheet, cell, val); err != nil {
					return nil, fmt.Errorf("append cell %s!%s: %w", sheet, cell, err)
				}
			}
			nextRow++
		}
	}

	for _, st := range opts.ApplyStyles {
		sheet, err := resolveTargetSheet(st.Sheet)
		if err != nil {
			return nil, fmt.Errorf("apply style: %w", err)
		}
		if err := applyRangeStyle(f, sheet, st.Range, st.Style); err != nil {
			return nil, fmt.Errorf("apply style %s!%s: %w", sheet, st.Range, err)
		}
	}

	for _, cw := range opts.ColWidths {
		sheet, err := resolveTargetSheet(cw.Sheet)
		if err != nil {
			return nil, fmt.Errorf("set col width: %w", err)
		}
		col := strings.ToUpper(strings.TrimSpace(cw.Column))
		if col == "" || cw.Width <= 0 {
			return nil, fmt.Errorf("invalid column width for %q", cw.Column)
		}
		if err := f.SetColWidth(sheet, col, col, cw.Width); err != nil {
			return nil, fmt.Errorf("set col width %s!%s: %w", sheet, col, err)
		}
	}

	for _, rh := range opts.RowHeights {
		sheet, err := resolveTargetSheet(rh.Sheet)
		if err != nil {
			return nil, fmt.Errorf("set row height: %w", err)
		}
		if rh.Row <= 0 || rh.Height <= 0 {
			return nil, fmt.Errorf("invalid row height for row %d", rh.Row)
		}
		if err := f.SetRowHeight(sheet, rh.Row, rh.Height); err != nil {
			return nil, fmt.Errorf("set row height %s row %d: %w", sheet, rh.Row, err)
		}
	}

	for _, m := range opts.Merges {
		sheet, err := resolveTargetSheet(m.Sheet)
		if err != nil {
			return nil, fmt.Errorf("merge cells: %w", err)
		}
		topLeft, bottomRight, err := splitRangeRef(m.Range)
		if err != nil {
			return nil, fmt.Errorf("merge %q: %w", m.Range, err)
		}
		if err := f.MergeCell(sheet, topLeft, bottomRight); err != nil {
			return nil, fmt.Errorf("merge %s!%s: %w", sheet, m.Range, err)
		}
	}

	for _, ch := range opts.AddCharts {
		sheet, err := resolveTargetSheet(ch.Sheet)
		if err != nil {
			return nil, fmt.Errorf("add chart: %w", err)
		}
		if err := addChartToSheet(f, sheet, ch.Chart); err != nil {
			return nil, fmt.Errorf("add chart on %q: %w", sheet, err)
		}
	}

	if active := strings.TrimSpace(opts.ActiveSheet); active != "" {
		sheet, err := resolveTargetSheet(active)
		if err != nil {
			return nil, fmt.Errorf("set active sheet: %w", err)
		}
		idx, err := f.GetSheetIndex(sheet)
		if err != nil || idx < 0 {
			return nil, fmt.Errorf("active sheet %q not found", sheet)
		}
		f.SetActiveSheet(idx)
	}

	return saveWorkbookAtomic(f, outPath)
}
