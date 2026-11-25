package xlsx

import (
	"fmt"
	"os"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/xuri/excelize/v2"
)

func Inspect(path string, sheet string, cellRange string) (*InspectResult, error) {
	resolved, err := resolvePath("", path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	stat, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("stat xlsx: %w", err)
	}

	f, err := excelize.OpenFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	activeSheet := f.GetSheetName(f.GetActiveSheetIndex())

	var meta Metadata
	if props, err := f.GetDocProps(); err == nil && props != nil {
		meta = Metadata{
			Title:    props.Title,
			Author:   props.Creator,
			Subject:  props.Subject,
			Created:  props.Created,
			Modified: props.Modified,
		}
	}

	summaries := make([]SheetSummary, 0, len(sheets))
	for _, s := range sheets {
		rows, err := f.GetRows(s)
		if err != nil {
			continue
		}
		rc := len(rows)
		cc := 0
		for _, row := range rows {
			if len(row) > cc {
				cc = len(row)
			}
		}
		dim := ""
		if rc > 0 && cc > 0 {
			if endCell, err := excelize.CoordinatesToCellName(cc, rc); err == nil {
				dim = "A1:" + endCell
			}
		}
		summaries = append(summaries, SheetSummary{
			Name:       s,
			RowCount:   rc,
			ColCount:   cc,
			Dimensions: dim,
		})
	}

	targetSheet := strings.TrimSpace(sheet)
	if targetSheet == "" {
		if activeSheet != "" {
			targetSheet = activeSheet
		} else if len(sheets) > 0 {
			targetSheet = sheets[0]
		}
	}
	if targetSheet == "" {
		return nil, fmt.Errorf("workbook %q has no sheets", resolved)
	}
	idx, err := f.GetSheetIndex(targetSheet)
	if err != nil || idx < 0 {
		return nil, fmt.Errorf("sheet %q not found", targetSheet)
	}

	allRows, err := f.GetRows(targetSheet)
	if err != nil {
		return nil, fmt.Errorf("read rows from %q: %w", targetSheet, err)
	}
	fullRowCount := len(allRows)
	fullColCount := 0
	for _, row := range allRows {
		if len(row) > fullColCount {
			fullColCount = len(row)
		}
	}

	startCol, startRow := 1, 1
	endCol, endRow := fullColCount, fullRowCount
	trimmedRange := strings.TrimSpace(cellRange)
	if trimmedRange != "" {
		c1, r1, c2, r2, err := parseRangeCoordinates(trimmedRange)
		if err != nil {
			return nil, fmt.Errorf("parse cell range %q: %w", trimmedRange, err)
		}
		startCol, startRow, endCol, endRow = c1, r1, c2, r2
	}

	formulas := make(map[string]string)
	var formulaErrors []FormulaError
	var outRows [][]string

	if endRow >= startRow && endCol >= startCol && endRow > 0 && endCol > 0 {
		outRows = make([][]string, 0, endRow-startRow+1)
		for r := startRow; r <= endRow; r++ {
			rowOut := make([]string, 0, endCol-startCol+1)
			for c := startCol; c <= endCol; c++ {
				val := ""
				if r-1 < len(allRows) && c-1 < len(allRows[r-1]) {
					val = allRows[r-1][c-1]
				}
				cellRef, err := excelize.CoordinatesToCellName(c, r)
				if err != nil {
					rowOut = append(rowOut, val)
					continue
				}

				formulaText, _ := f.GetCellFormula(targetSheet, cellRef)
				if formulaText != "" {
					normalizedFormula := formulaText
					if !strings.HasPrefix(normalizedFormula, "=") {
						normalizedFormula = "=" + normalizedFormula
					}
					formulas[cellRef] = normalizedFormula

					calcVal, calcErr := f.CalcCellValue(targetSheet, cellRef)
					errCode := detectExactFormulaError(calcVal)
					if errCode == "" && calcErr != nil {
						errCode = detectFormulaErrorInMessage(calcErr.Error())
					}
					if errCode != "" {
						formulaErrors = append(formulaErrors, FormulaError{
							Sheet:   targetSheet,
							Cell:    cellRef,
							Formula: normalizedFormula,
							Error:   errCode,
						})
						if val == "" {
							val = errCode
						}
					} else if val == "" && calcErr == nil && calcVal != "" {
						val = calcVal
					}
				} else if errCode := detectExactFormulaError(val); errCode != "" {
					formulaErrors = append(formulaErrors, FormulaError{
						Sheet: targetSheet,
						Cell:  cellRef,
						Error: errCode,
					})
				}

				rowOut = append(rowOut, val)
			}
			outRows = append(outRows, rowOut)
		}
	}

	dimensions := ""
	if fullRowCount > 0 && fullColCount > 0 {
		if endRef, err := excelize.CoordinatesToCellName(fullColCount, fullRowCount); err == nil {
			dimensions = "A1:" + endRef
		}
	}

	var mergedCells []string
	if merges, err := f.GetMergeCells(targetSheet); err == nil {
		for _, m := range merges {
			ref := strings.TrimSpace(m.GetStartAxis()) + ":" + strings.TrimSpace(m.GetEndAxis())
			if ref != ":" {
				mergedCells = append(mergedCells, ref)
			}
		}
	}

	if len(formulas) == 0 {
		formulas = nil
	}

	return &InspectResult{
		Path:           resolved,
		SizeBytes:      stat.Size(),
		Sheets:         sheets,
		ActiveSheet:    activeSheet,
		Metadata:       meta,
		Sheet:          targetSheet,
		Range:          trimmedRange,
		Dimensions:     dimensions,
		RowCount:       fullRowCount,
		ColCount:       fullColCount,
		Rows:           outRows,
		Formulas:       formulas,
		FormulaErrors:  formulaErrors,
		MergedCells:    mergedCells,
		SheetSummaries: summaries,
	}, nil
}
