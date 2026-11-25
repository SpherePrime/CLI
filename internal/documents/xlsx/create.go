package xlsx

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/xuri/excelize/v2"
)

func Create(opts CreateOptions) (*Result, error) {
	outPath, err := resolvePath(opts.WorkingDir, opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve output path: %w", err)
	}
	if len(opts.Sheets) == 0 {
		return nil, errors.New("at least one sheet is required")
	}

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if opts.Title != "" || opts.Author != "" || opts.Subject != "" {
		if err := f.SetDocProps(&excelize.DocProperties{
			Title:   opts.Title,
			Creator: opts.Author,
			Subject: opts.Subject,
		}); err != nil {
			return nil, fmt.Errorf("set workbook metadata: %w", err)
		}
	}

	defaultSheet := f.GetSheetName(0)
	seen := make(map[string]bool, len(opts.Sheets))
	for i, spec := range opts.Sheets {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			name = fmt.Sprintf("Sheet%d", i+1)
		}
		lower := strings.ToLower(name)
		if seen[lower] {
			return nil, fmt.Errorf("duplicate sheet name %q", name)
		}
		seen[lower] = true

		if i == 0 {
			if name != defaultSheet {
				if err := f.SetSheetName(defaultSheet, name); err != nil {
					return nil, fmt.Errorf("rename default sheet to %q: %w", name, err)
				}
			}
		} else {
			if _, err := f.NewSheet(name); err != nil {
				return nil, fmt.Errorf("create sheet %q: %w", name, err)
			}
		}

		spec.Name = name
		if err := populateSheet(f, spec); err != nil {
			return nil, fmt.Errorf("populate sheet %q: %w", name, err)
		}
	}

	if active := strings.TrimSpace(opts.ActiveSheet); active != "" {
		idx, err := f.GetSheetIndex(active)
		if err != nil {
			return nil, fmt.Errorf("active sheet %q: %w", active, err)
		}
		if idx < 0 {
			return nil, fmt.Errorf("active sheet %q not found", active)
		}
		f.SetActiveSheet(idx)
	} else {
		f.SetActiveSheet(0)
	}

	return saveWorkbookAtomic(f, outPath)
}

func populateSheet(f *excelize.File, spec SheetSpec) error {
	sheet := spec.Name
	rowCursor := 1

	if len(spec.Headers) > 0 {
		for colIdx, h := range spec.Headers {
			cell, err := excelize.CoordinatesToCellName(colIdx+1, rowCursor)
			if err != nil {
				return err
			}
			if err := f.SetCellValue(sheet, cell, h); err != nil {
				return err
			}
		}
		endCell, err := excelize.CoordinatesToCellName(len(spec.Headers), rowCursor)
		if err != nil {
			return err
		}
		headerStyle := CellStyle{
			Bold:   true,
			Fill:   "#D9E1F2",
			Border: true,
			Align:  "center",
		}
		if spec.HeaderStyle != nil {
			headerStyle = *spec.HeaderStyle
		}
		if err := applyRangeStyle(f, sheet, "A1:"+endCell, headerStyle); err != nil {
			return err
		}
		rowCursor++
	}

	for _, row := range spec.Rows {
		for colIdx, val := range row {
			cell, err := excelize.CoordinatesToCellName(colIdx+1, rowCursor)
			if err != nil {
				return err
			}
			if err := setCellValueOrFormula(f, sheet, cell, val); err != nil {
				return err
			}
		}
		rowCursor++
	}

	if len(spec.Cells) > 0 {
		keys := make([]string, 0, len(spec.Cells))
		for k := range spec.Cells {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, cellRef := range keys {
			cell := strings.TrimSpace(cellRef)
			if _, _, err := excelize.CellNameToCoordinates(cell); err != nil {
				return fmt.Errorf("invalid cell %q: %w", cellRef, err)
			}
			if err := setCellValueOrFormula(f, sheet, cell, spec.Cells[cellRef]); err != nil {
				return err
			}
		}
	}

	for _, mergeRef := range spec.Merges {
		topLeft, bottomRight, err := splitRangeRef(mergeRef)
		if err != nil {
			return fmt.Errorf("merge %q: %w", mergeRef, err)
		}
		if err := f.MergeCell(sheet, topLeft, bottomRight); err != nil {
			return fmt.Errorf("merge %q: %w", mergeRef, err)
		}
	}

	for _, sr := range spec.Styles {
		if err := applyRangeStyle(f, sheet, sr.Range, sr.Style); err != nil {
			return fmt.Errorf("style %q: %w", sr.Range, err)
		}
	}

	if spec.AutoFit == nil || *spec.AutoFit {
		if err := applyAutoFitColumns(f, sheet, spec.ColWidths); err != nil {
			return fmt.Errorf("auto-fit columns: %w", err)
		}
	}

	if len(spec.ColWidths) > 0 {
		cols := make([]string, 0, len(spec.ColWidths))
		for c := range spec.ColWidths {
			cols = append(cols, c)
		}
		sort.Strings(cols)
		for _, col := range cols {
			colName := strings.ToUpper(strings.TrimSpace(col))
			width := spec.ColWidths[col]
			if colName == "" || width <= 0 {
				continue
			}
			if err := f.SetColWidth(sheet, colName, colName, width); err != nil {
				return fmt.Errorf("set column width %q: %w", colName, err)
			}
		}
	}

	if len(spec.RowHeights) > 0 {
		rowNums := make([]int, 0, len(spec.RowHeights))
		for r := range spec.RowHeights {
			rowNums = append(rowNums, r)
		}
		sort.Ints(rowNums)
		for _, rowNum := range rowNums {
			height := spec.RowHeights[rowNum]
			if rowNum <= 0 || height <= 0 {
				continue
			}
			if err := f.SetRowHeight(sheet, rowNum, height); err != nil {
				return fmt.Errorf("set row height %d: %w", rowNum, err)
			}
		}
	}

	if strings.TrimSpace(spec.FreezePane) != "" {
		if err := applyFreezePane(f, sheet, spec.FreezePane); err != nil {
			return fmt.Errorf("freeze pane: %w", err)
		}
	}

	if af := strings.TrimSpace(spec.AutoFilter); af != "" {
		if _, _, err := splitRangeRef(af); err != nil {
			return fmt.Errorf("auto_filter %q: %w", af, err)
		}
		if err := f.AutoFilter(sheet, af, nil); err != nil {
			return fmt.Errorf("auto_filter %q: %w", af, err)
		}
	}

	for _, chart := range spec.Charts {
		if err := addChartToSheet(f, sheet, chart); err != nil {
			return fmt.Errorf("add chart: %w", err)
		}
	}

	return nil
}
