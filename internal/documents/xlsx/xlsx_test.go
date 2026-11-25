package xlsx

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/xuri/excelize/v2"
)

func TestCreateAndInspectMultiSheetCyrillicWithFormulasAndChart(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.xlsx")

	res, err := Create(CreateOptions{
		OutputPath:  outPath,
		Title:       "Квартальный отчёт",
		Author:      "Аналитика",
		Subject:     "Продажи",
		ActiveSheet: "Продажи",
		Sheets: []SheetSpec{
			{
				Name: "Продажи",
				Rows: [][]any{
					{"Отчёт по продажам за Q1"},
					{"Категория", "Выручка", "Доля"},
					{"Ноутбуки", 120000.50, "=B3/$B$6"},
					{"Мониторы", 45000.00, "=B4/$B$6"},
					{"Клавиатуры", 15000.00, "=B5/$B$6"},
					{"Итого", "=SUM(B3:B5)", "=SUM(C3:C5)"},
				},
				Merges: []string{"A1:C1"},
				Styles: []StyledRange{
					{
						Range: "A1:C1",
						Style: CellStyle{
							Bold:          true,
							FontSize:      14,
							FontColor:     "#FFFFFF",
							Fill:          "#1F4E78",
							Align:         "center",
							VerticalAlign: "center",
						},
					},
					{
						Range: "A2:C2",
						Style: CellStyle{
							Bold:   true,
							Fill:   "#D9E1F2",
							Border: true,
							Align:  "center",
						},
					},
					{
						Range: "B3:B6",
						Style: CellStyle{
							NumFmt: "#,##0.00",
							Border: true,
						},
					},
					{
						Range: "C3:C6",
						Style: CellStyle{
							NumFmt: "0.0%",
							Border: true,
						},
					},
					{
						Range: "A6:C6",
						Style: CellStyle{
							Bold:   true,
							Fill:   "#F2F2F2",
							Border: true,
						},
					},
				},
				ColWidths:  map[string]float64{"A": 22},
				RowHeights: map[int]float64{1: 28, 2: 22},
				FreezePane: "A3",
				AutoFilter: "A2:C5",
				Charts: []ChartSpec{
					{
						Type:  "col",
						Cell:  "E2",
						Title: "Выручка по категориям",
						Series: []ChartSeriesSpec{
							{
								Name:       "B2",
								Categories: "A3:A5",
								Values:     "B3:B5",
							},
						},
						Width:          480,
						Height:         280,
						LegendPosition: "bottom",
					},
				},
			},
			{
				Name:    "Сводка",
				Headers: []string{"Показатель", "Значение"},
				HeaderStyle: &CellStyle{
					Bold:      true,
					FontColor: "#FFFFFF",
					Fill:      "#2F5597",
					Align:     "left",
				},
				Rows: [][]any{
					{"Общая выручка", "='Продажи'!B6"},
					{"Активен", true},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if res.Path != outPath {
		t.Fatalf("unexpected path: got %q want %q", res.Path, outPath)
	}
	if res.SizeBytes <= 0 {
		t.Fatalf("expected positive file size, got %d", res.SizeBytes)
	}
	if res.SheetCount != 2 {
		t.Fatalf("expected 2 sheets, got %d", res.SheetCount)
	}

	info, err := Inspect(outPath, "Продажи", "")
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	if len(info.Sheets) != 2 || info.Sheets[0] != "Продажи" || info.Sheets[1] != "Сводка" {
		t.Fatalf("unexpected sheet list: %v", info.Sheets)
	}
	if info.ActiveSheet != "Продажи" {
		t.Fatalf("unexpected active sheet: %q", info.ActiveSheet)
	}
	if info.RowCount < 6 || info.ColCount < 3 {
		t.Fatalf("unexpected dimensions: rows=%d cols=%d", info.RowCount, info.ColCount)
	}
	if info.Rows[0][0] != "Отчёт по продажам за Q1" {
		t.Fatalf("unexpected title cell: %q", info.Rows[0][0])
	}
	if info.Rows[2][0] != "Ноутбуки" {
		t.Fatalf("unexpected Cyrillic cell: %q", info.Rows[2][0])
	}
	formulaB6 := info.Formulas["B6"]
	if !strings.Contains(formulaB6, "SUM(B3:B5)") {
		t.Fatalf("expected SUM(B3:B5) formula in B6, got %q", formulaB6)
	}
	if len(info.FormulaErrors) != 0 {
		t.Fatalf("expected zero formula errors, got %+v", info.FormulaErrors)
	}
	if len(info.MergedCells) == 0 || info.MergedCells[0] != "A1:C1" {
		t.Fatalf("expected merged cell A1:C1, got %v", info.MergedCells)
	}

	subInfo, err := Inspect(outPath, "Продажи", "A2:B4")
	if err != nil {
		t.Fatalf("Inspect with range failed: %v", err)
	}
	if len(subInfo.Rows) != 3 || len(subInfo.Rows[0]) != 2 {
		t.Fatalf("expected 3x2 range rows, got %v", subInfo.Rows)
	}
	if subInfo.Rows[0][0] != "Категория" || subInfo.Rows[1][0] != "Ноутбуки" {
		t.Fatalf("unexpected range values: %v", subInfo.Rows)
	}

	summaryInfo, err := Inspect(outPath, "Сводка", "")
	if err != nil {
		t.Fatalf("Inspect summary sheet failed: %v", err)
	}
	if !strings.Contains(summaryInfo.Formulas["B2"], "B6") {
		t.Fatalf("expected cross-sheet formula in Сводка!B2, got %q", summaryInfo.Formulas["B2"])
	}
	if len(summaryInfo.FormulaErrors) != 0 {
		t.Fatalf("expected zero formula errors on summary sheet, got %+v", summaryInfo.FormulaErrors)
	}

	f, err := excelize.OpenFile(outPath)
	if err != nil {
		t.Fatalf("excelize.OpenFile failed: %v", err)
	}
	defer f.Close()

	panes, err := f.GetPanes("Продажи")
	if err != nil {
		t.Fatalf("GetPanes failed: %v", err)
	}
	if !panes.Freeze || panes.YSplit != 2 {
		t.Fatalf("expected freeze pane at row 2 (A3), got %+v", panes)
	}
	styleID, err := f.GetCellStyle("Продажи", "A1")
	if err != nil || styleID == 0 {
		t.Fatalf("expected non-default style on A1, got id=%d err=%v", styleID, err)
	}
	widthA, err := f.GetColWidth("Продажи", "A")
	if err != nil || widthA < 21.5 {
		t.Fatalf("expected col A width ~22, got %v err=%v", widthA, err)
	}
	b6StyleID, err := f.GetCellStyle("Продажи", "B6")
	if err != nil || b6StyleID == 0 {
		t.Fatalf("expected merged style on B6, got id=%d err=%v", b6StyleID, err)
	}
	b6Style, err := f.GetStyle(b6StyleID)
	if err != nil {
		t.Fatalf("GetStyle(B6) failed: %v", err)
	}
	if b6Style.Font == nil || !b6Style.Font.Bold {
		t.Fatalf("expected B6 bold preserved in merged style, got %+v", b6Style.Font)
	}
	if b6Style.CustomNumFmt == nil || *b6Style.CustomNumFmt != "#,##0.00" {
		t.Fatalf("expected B6 custom number format preserved in merged style, got %+v", b6Style.CustomNumFmt)
	}

	zr, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatalf("open xlsx zip failed: %v", err)
	}
	defer zr.Close()
	hasChartXML := false
	for _, entry := range zr.File {
		if strings.HasPrefix(entry.Name, "xl/charts/chart") && strings.HasSuffix(entry.Name, ".xml") {
			hasChartXML = true
			break
		}
	}
	if !hasChartXML {
		t.Fatal("expected xl/charts/chart*.xml entry inside xlsx zip")
	}
}

func TestUpdateInPlacePreservesDataAndStyles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "budget.xlsx")

	_, err := Create(CreateOptions{
		OutputPath: path,
		Sheets: []SheetSpec{
			{
				Name:    "Бюджет",
				Headers: []string{"Статья", "Сумма"},
				HeaderStyle: &CellStyle{
					Bold:      true,
					FontColor: "#FFFFFF",
					Fill:      "#1F4E78",
				},
				Rows: [][]any{
					{"Аренда", 50000},
					{"Зарплата", 120000},
					{"Итого", "=SUM(B2:B3)"},
				},
				Styles: []StyledRange{
					{
						Range: "B2:B4",
						Style: CellStyle{NumFmt: "#,##0"},
					},
				},
			},
			{
				Name:    "Заметки",
				Headers: []string{"Комментарий"},
				Rows: [][]any{
					{"Черновик"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	origFile, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("open original failed: %v", err)
	}
	origHeaderStyleID, err := origFile.GetCellStyle("Бюджет", "A1")
	_ = origFile.Close()
	if err != nil || origHeaderStyleID == 0 {
		t.Fatalf("expected non-zero original header style, got %d err=%v", origHeaderStyleID, err)
	}

	updRes, err := Update(UpdateOptions{
		InputPath:  path,
		OutputPath: path,
		SetCells: []CellEdit{
			{Sheet: "Бюджет", Cell: "B2", Value: 55000},
			{Sheet: "Бюджет", Cell: "B4", Value: "=SUM(B2:B3)"},
			{Sheet: "Бюджет", Cell: "C4", Value: "=AVERAGE(B2:B3)"},
		},
		AppendRows: []AppendRowsEdit{
			{
				Sheet: "Бюджет",
				Rows: [][]any{
					{"Маркетинг", 25000},
				},
			},
		},
		AddSheets: []SheetSpec{
			{
				Name:    "Прогноз",
				Headers: []string{"Месяц", "План"},
				Rows: [][]any{
					{"Апрель", 210000},
				},
			},
		},
		RenameSheets: []RenameSheetEdit{
			{From: "Заметки", To: "Комментарии"},
		},
		ApplyStyles: []SheetStyleEdit{
			{
				Sheet: "Бюджет",
				Range: "C4",
				Style: CellStyle{Bold: true, NumFmt: "#,##0.00"},
			},
		},
		ColWidths: []ColWidthEdit{
			{Sheet: "Бюджет", Column: "C", Width: 18},
		},
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updRes.SheetCount != 3 {
		t.Fatalf("expected 3 sheets after update, got %d", updRes.SheetCount)
	}

	info, err := Inspect(path, "Бюджет", "")
	if err != nil {
		t.Fatalf("Inspect after update failed: %v", err)
	}
	expectedSheets := []string{"Бюджет", "Комментарии", "Прогноз"}
	if len(info.Sheets) != len(expectedSheets) {
		t.Fatalf("unexpected sheets: %v", info.Sheets)
	}
	for i, s := range expectedSheets {
		if info.Sheets[i] != s {
			t.Fatalf("sheet[%d]: got %q want %q", i, info.Sheets[i], s)
		}
	}
	if info.Rows[1][0] != "Аренда" || !strings.Contains(info.Rows[1][1], "55") {
		t.Fatalf("expected updated row 2, got %v", info.Rows[1])
	}
	if info.Rows[2][0] != "Зарплата" {
		t.Fatalf("expected preserved row 3, got %v", info.Rows[2])
	}
	if len(info.Rows) < 5 || info.Rows[4][0] != "Маркетинг" {
		t.Fatalf("expected appended row 5, got %v", info.Rows)
	}
	if !strings.Contains(info.Formulas["C4"], "AVERAGE(B2:B3)") {
		t.Fatalf("expected new formula in C4, got %q", info.Formulas["C4"])
	}
	if len(info.FormulaErrors) != 0 {
		t.Fatalf("expected zero formula errors, got %+v", info.FormulaErrors)
	}

	afterFile, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("reopen updated failed: %v", err)
	}
	defer afterFile.Close()

	afterHeaderStyleID, err := afterFile.GetCellStyle("Бюджет", "A1")
	if err != nil || afterHeaderStyleID != origHeaderStyleID {
		t.Fatalf("expected original header style %d preserved, got %d err=%v", origHeaderStyleID, afterHeaderStyleID, err)
	}
}

func TestInspectDetectsFormulaErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "errors.xlsx")

	_, err := Create(CreateOptions{
		OutputPath: path,
		Sheets: []SheetSpec{
			{
				Name: "Ошибки",
				Rows: [][]any{
					{"Тип", "Значение"},
					{"Деление на ноль", "=1/0"},
					{"Сломанная ссылка", "#REF!"},
					{"Неизвестное имя", "#NAME?"},
					{"Нет значения", "=NA()"},
					{"Описание", "Примечание: код #N/A означает отсутствие данных"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	info, err := Inspect(path, "Ошибки", "")
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	found := make(map[string]string)
	for _, fe := range info.FormulaErrors {
		found[fe.Cell] = fe.Error
	}
	if found["B2"] != "#DIV/0!" {
		t.Fatalf("expected #DIV/0! at B2, got %+v", info.FormulaErrors)
	}
	if found["B3"] != "#REF!" {
		t.Fatalf("expected #REF! at B3, got %+v", info.FormulaErrors)
	}
	if found["B4"] != "#NAME?" {
		t.Fatalf("expected #NAME? at B4, got %+v", info.FormulaErrors)
	}
	if found["B5"] != "#N/A" {
		t.Fatalf("expected #N/A at B5, got %+v", info.FormulaErrors)
	}
	if _, falsePositive := found["B6"]; falsePositive {
		t.Fatalf("did not expect false positive on plain text cell B6, got %+v", info.FormulaErrors)
	}
}

func TestChartTypesAndSheetDeletion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "charts.xlsx")

	_, err := Create(CreateOptions{
		OutputPath: path,
		Sheets: []SheetSpec{
			{
				Name:    "Данные",
				Headers: []string{"Месяц", "План", "Факт"},
				Rows: [][]any{
					{"Янв", 100, 110},
					{"Фев", 120, 125},
					{"Мар", 140, 150},
				},
				Charts: []ChartSpec{
					{
						Type:  "bar",
						Cell:  "E2",
						Title: "План vs Факт",
						Series: []ChartSeriesSpec{
							{Name: "B1", Categories: "A2:A4", Values: "B2:B4"},
							{Name: "C1", Categories: "A2:A4", Values: "C2:C4"},
						},
					},
					{
						Type:  "line",
						Cell:  "E18",
						Title: "Динамика",
						Series: []ChartSeriesSpec{
							{Name: "Факт", Categories: "A2:A4", Values: "C2:C4"},
						},
					},
					{
						Type:  "pie",
						Cell:  "M2",
						Title: "Структура",
						Series: []ChartSeriesSpec{
							{Name: "C1", Categories: "A2:A4", Values: "C2:C4"},
						},
						ShowDataLabels: true,
					},
				},
			},
			{
				Name: "Временный",
				Rows: [][]any{{"удалить"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Create with multiple chart types failed: %v", err)
	}

	res, err := Update(UpdateOptions{
		InputPath:    path,
		DeleteSheets: []string{"Временный"},
		Merges:       []MergeEdit{{Sheet: "Данные", Range: "A6:C6"}},
		SetCells:     []CellEdit{{Sheet: "Данные", Cell: "A6", Value: "Итоговый комментарий"}},
		RowHeights:   []RowHeightEdit{{Sheet: "Данные", Row: 6, Height: 24}},
		AddCharts: []ChartEdit{
			{
				Sheet: "Данные",
				Chart: ChartSpec{
					Type:  "col",
					Cell:  "M18",
					Title: "Дополнительный график",
					Series: []ChartSeriesSpec{
						{Name: "B1", Categories: "A2:A4", Values: "B2:B4"},
					},
				},
			},
		},
		ActiveSheet: "Данные",
	})
	if err != nil {
		t.Fatalf("Update with delete/merge/chart failed: %v", err)
	}
	if res.SheetCount != 1 || res.Sheets[0] != "Данные" {
		t.Fatalf("expected only Данные after delete, got %v", res.Sheets)
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}
	defer zr.Close()
	chartCount := 0
	for _, entry := range zr.File {
		if strings.HasPrefix(entry.Name, "xl/charts/chart") && strings.HasSuffix(entry.Name, ".xml") {
			chartCount++
		}
	}
	if chartCount != 4 {
		t.Fatalf("expected 4 chart XML parts (bar, line, pie, col), got %d", chartCount)
	}
}

func TestErrorHandling(t *testing.T) {
	dir := t.TempDir()

	if _, err := Create(CreateOptions{
		OutputPath: "",
		Sheets:     []SheetSpec{{Name: "Лист1"}},
	}); err == nil {
		t.Fatal("expected error for empty output path in Create")
	}

	missingPath := filepath.Join(dir, "does-not-exist.xlsx")
	if _, err := Inspect(missingPath, "", ""); err == nil {
		t.Fatal("expected error for missing file in Inspect")
	}
	if _, err := Update(UpdateOptions{
		InputPath:  missingPath,
		OutputPath: filepath.Join(dir, "out.xlsx"),
	}); err == nil {
		t.Fatal("expected error for missing input file in Update")
	}

	singlePath := filepath.Join(dir, "single.xlsx")
	if _, err := Create(CreateOptions{
		OutputPath: singlePath,
		Sheets:     []SheetSpec{{Name: "Единственный", Rows: [][]any{{"A", 1}}}},
	}); err != nil {
		t.Fatalf("Create single sheet failed: %v", err)
	}

	if _, err := Update(UpdateOptions{
		InputPath:    singlePath,
		OutputPath:   singlePath,
		DeleteSheets: []string{"Единственный"},
	}); err == nil {
		t.Fatal("expected error when deleting the last remaining sheet")
	}

	if _, err := os.Stat(singlePath); err != nil {
		t.Fatalf("original file should still exist after failed update: %v", err)
	}
}

func TestRenameSheetsCaseHandling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rename_case.xlsx")

	_, err := Create(CreateOptions{
		OutputPath: path,
		Sheets: []SheetSpec{
			{Name: "Data", Rows: [][]any{{"Key", 1}}},
			{Name: "Notes", Rows: [][]any{{"Initial"}}},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	res, err := Update(UpdateOptions{
		InputPath:  path,
		OutputPath: path,
		RenameSheets: []RenameSheetEdit{
			{From: "notes", To: "Comments"},
		},
		SetCells: []CellEdit{
			{Sheet: "notes", Cell: "A2", Value: "Updated via old case"},
		},
	})
	if err != nil {
		t.Fatalf("case-mismatched RenameSheets failed: %v", err)
	}
	if len(res.Sheets) != 2 || res.Sheets[0] != "Data" || res.Sheets[1] != "Comments" {
		t.Fatalf("expected sheets [Data Comments], got %v", res.Sheets)
	}

	info, err := Inspect(path, "comments", "")
	if err != nil {
		t.Fatalf("Inspect case-mismatched sheet failed: %v", err)
	}
	if info.Sheet != "Comments" {
		t.Fatalf("expected canonical sheet name Comments, got %q", info.Sheet)
	}
	if len(info.Rows) < 2 || info.Rows[1][0] != "Updated via old case" {
		t.Fatalf("expected updated cell in renamed sheet, got %v", info.Rows)
	}

	caseOnlyPath := filepath.Join(dir, "rename_case_only.xlsx")
	_, err = Create(CreateOptions{
		OutputPath: caseOnlyPath,
		Sheets: []SheetSpec{
			{Name: "Data", Rows: [][]any{{"Key", 1}}},
			{Name: "notes", Rows: [][]any{{"Lower"}}},
		},
	})
	if err != nil {
		t.Fatalf("Create for case-only rename failed: %v", err)
	}

	caseRes, err := Update(UpdateOptions{
		InputPath:  caseOnlyPath,
		OutputPath: caseOnlyPath,
		RenameSheets: []RenameSheetEdit{
			{From: "notes", To: "Notes"},
		},
		SetCells: []CellEdit{
			{Sheet: "NOTES", Cell: "A2", Value: "Case-only OK"},
		},
	})
	if err != nil {
		t.Fatalf("case-only RenameSheets failed: %v", err)
	}
	if len(caseRes.Sheets) != 2 || caseRes.Sheets[0] != "Data" || caseRes.Sheets[1] != "Notes" {
		t.Fatalf("expected sheets [Data Notes], got %v", caseRes.Sheets)
	}

	caseInfo, err := Inspect(caseOnlyPath, "notes", "")
	if err != nil {
		t.Fatalf("Inspect after case-only rename failed: %v", err)
	}
	if caseInfo.Sheet != "Notes" {
		t.Fatalf("expected canonical sheet name Notes, got %q", caseInfo.Sheet)
	}
	if len(caseInfo.Rows) < 2 || caseInfo.Rows[1][0] != "Case-only OK" {
		t.Fatalf("expected updated cell after case-only rename, got %v", caseInfo.Rows)
	}

	if _, err := Update(UpdateOptions{
		InputPath:  caseOnlyPath,
		OutputPath: caseOnlyPath,
		RenameSheets: []RenameSheetEdit{
			{From: "Notes", To: "data"},
		},
	}); err == nil {
		t.Fatal("expected collision error when renaming Notes to existing sheet data")
	}
}

func TestDeleteSheetsRejectsEmptyName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "delete_empty.xlsx")

	_, err := Create(CreateOptions{
		OutputPath: path,
		Sheets: []SheetSpec{
			{Name: "SheetA", Rows: [][]any{{"A1"}}},
			{Name: "SheetB", Rows: [][]any{{"B1"}}},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := Update(UpdateOptions{
		InputPath:    path,
		OutputPath:   path,
		DeleteSheets: []string{""},
	}); err == nil {
		t.Fatal("expected error for empty sheet name in DeleteSheets")
	}

	if _, err := Update(UpdateOptions{
		InputPath:    path,
		OutputPath:   path,
		DeleteSheets: []string{"   "},
	}); err == nil {
		t.Fatal("expected error for whitespace sheet name in DeleteSheets")
	}

	info, err := Inspect(path, "", "")
	if err != nil {
		t.Fatalf("Inspect after rejected delete failed: %v", err)
	}
	if len(info.Sheets) != 2 || info.Sheets[0] != "SheetA" || info.Sheets[1] != "SheetB" {
		t.Fatalf("expected all sheets intact [SheetA SheetB], got %v", info.Sheets)
	}
}

func TestInspectDetectsCachedFormulaErrorWhenCalcFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cached_formula_errors.xlsx")

	_, err := Create(CreateOptions{
		OutputPath: path,
		Sheets: []SheetSpec{
			{
				Name: "Data",
				Rows: [][]any{
					{"Label", "Result"},
					{"UnsupportedLambda", "=LAMBDA(x,x+1)(2)"},
					{"UnsupportedArray", "={1,2}"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("zip.NewReader failed: %v", err)
	}
	var outBuf bytes.Buffer
	zw := zip.NewWriter(&outBuf)
	patched := false
	for _, entry := range zr.File {
		rc, err := entry.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", entry.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", entry.Name, err)
		}
		if entry.Name == "xl/worksheets/sheet1.xml" {
			xmlStr := string(data)
			oldB2 := `<c r="B2" t="str"><f>LAMBDA(x,x+1)(2)</f></c>`
			newB2 := `<c r="B2" t="e"><f>LAMBDA(x,x+1)(2)</f><v>#NAME?</v></c>`
			oldB3 := `<c r="B3" t="str"><f>{1,2}</f></c>`
			newB3 := `<c r="B3" t="e"><f>{1,2}</f><v>#REF!</v></c>`
			if !strings.Contains(xmlStr, oldB2) || !strings.Contains(xmlStr, oldB3) {
				t.Fatalf("unexpected worksheet xml structure: %s", xmlStr)
			}
			xmlStr = strings.Replace(xmlStr, oldB2, newB2, 1)
			xmlStr = strings.Replace(xmlStr, oldB3, newB3, 1)
			data = []byte(xmlStr)
			patched = true
		}
		w, err := zw.Create(entry.Name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", entry.Name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write zip entry %s: %v", entry.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if !patched {
		t.Fatal("failed to locate xl/worksheets/sheet1.xml to patch")
	}
	if err := os.WriteFile(path, outBuf.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile patched xlsx failed: %v", err)
	}

	checkFile, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	for _, cell := range []string{"B2", "B3"} {
		calcVal, calcErr := checkFile.CalcCellValue("Data", cell)
		if detectExactFormulaError(calcVal) != "" || (calcErr != nil && detectFormulaErrorInMessage(calcErr.Error()) != "") {
			_ = checkFile.Close()
			t.Fatalf("expected CalcCellValue(%s) to fail without #... token, got val=%q err=%v", cell, calcVal, calcErr)
		}
	}
	_ = checkFile.Close()

	info, err := Inspect(path, "Data", "")
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	found := make(map[string]FormulaError)
	for _, fe := range info.FormulaErrors {
		found[fe.Cell] = fe
	}
	if fe, ok := found["B2"]; !ok || fe.Error != "#NAME?" || !strings.Contains(fe.Formula, "LAMBDA") {
		t.Fatalf("expected #NAME? formula error for B2, got %+v", info.FormulaErrors)
	}
	if fe, ok := found["B3"]; !ok || fe.Error != "#REF!" || !strings.Contains(fe.Formula, "{1,2}") {
		t.Fatalf("expected #REF! formula error for B3, got %+v", info.FormulaErrors)
	}
}

func TestChartLegendPositionValidation(t *testing.T) {
	dir := t.TempDir()

	invalidPath := filepath.Join(dir, "invalid_legend.xlsx")
	_, err := Create(CreateOptions{
		OutputPath: invalidPath,
		Sheets: []SheetSpec{
			{
				Name:    "Sales",
				Headers: []string{"Month", "Amount"},
				Rows: [][]any{
					{"Jan", 10},
					{"Feb", 20},
				},
				Charts: []ChartSpec{
					{
						Type:           "col",
						Cell:           "D2",
						LegendPosition: "diagonal",
						Series: []ChartSeriesSpec{
							{Name: "B1", Categories: "A2:A3", Values: "B2:B3"},
						},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid LegendPosition")
	}

	validPath := filepath.Join(dir, "valid_legend.xlsx")
	_, err = Create(CreateOptions{
		OutputPath: validPath,
		Sheets: []SheetSpec{
			{
				Name:    "Sales",
				Headers: []string{"Month", "Amount"},
				Rows: [][]any{
					{"Jan", 10},
					{"Feb", 20},
				},
				Charts: []ChartSpec{
					{
						Type:           "col",
						Cell:           "D2",
						LegendPosition: "top-right",
						Series: []ChartSeriesSpec{
							{Name: "B1", Categories: "A2:A3", Values: "B2:B3"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Create with top-right LegendPosition failed: %v", err)
	}

	zr, err := zip.OpenReader(validPath)
	if err != nil {
		t.Fatalf("open valid_legend zip failed: %v", err)
	}
	defer zr.Close()
	foundLegendTR := false
	for _, entry := range zr.File {
		if strings.HasPrefix(entry.Name, "xl/charts/chart") && strings.HasSuffix(entry.Name, ".xml") {
			rc, err := entry.Open()
			if err != nil {
				t.Fatalf("open chart xml: %v", err)
			}
			data, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Fatalf("read chart xml: %v", err)
			}
			if strings.Contains(string(data), `legendPos val="tr"`) {
				foundLegendTR = true
			}
		}
	}
	if !foundLegendTR {
		t.Fatal("expected chart XML to contain <c:legendPos val=\"tr\"> for top-right legend")
	}
}

func TestFontColorPatchClearsThemeIndexedTint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "theme_font.xlsx")

	f := excelize.NewFile()
	theme := 4
	baseStyleID, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:         true,
			ColorTheme:   &theme,
			ColorIndexed: 10,
			ColorTint:    0.4,
		},
	})
	if err != nil {
		_ = f.Close()
		t.Fatalf("NewStyle failed: %v", err)
	}
	if err := f.SetCellValue("Sheet1", "A1", "Themed"); err != nil {
		_ = f.Close()
		t.Fatalf("SetCellValue failed: %v", err)
	}
	if err := f.SetCellStyle("Sheet1", "A1", "A1", baseStyleID); err != nil {
		_ = f.Close()
		t.Fatalf("SetCellStyle failed: %v", err)
	}
	if err := f.SaveAs(path); err != nil {
		_ = f.Close()
		t.Fatalf("SaveAs failed: %v", err)
	}
	_ = f.Close()

	_, err = Update(UpdateOptions{
		InputPath:  path,
		OutputPath: path,
		ApplyStyles: []SheetStyleEdit{
			{
				Sheet: "Sheet1",
				Range: "A1",
				Style: CellStyle{FontColor: "#CC0000"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Update ApplyStyles failed: %v", err)
	}

	reopened, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	defer reopened.Close()

	patchedID, err := reopened.GetCellStyle("Sheet1", "A1")
	if err != nil || patchedID == 0 {
		t.Fatalf("GetCellStyle failed: id=%d err=%v", patchedID, err)
	}
	st, err := reopened.GetStyle(patchedID)
	if err != nil {
		t.Fatalf("GetStyle failed: %v", err)
	}
	if st.Font == nil {
		t.Fatal("expected non-nil Font on patched style")
	}
	if !st.Font.Bold {
		t.Fatal("expected Bold preserved after FontColor patch")
	}
	if st.Font.Color != "CC0000" {
		t.Fatalf("expected Font.Color CC0000, got %q", st.Font.Color)
	}
	if st.Font.ColorTheme != nil {
		t.Fatalf("expected Font.ColorTheme nil, got %v", *st.Font.ColorTheme)
	}
	if st.Font.ColorIndexed != 0 {
		t.Fatalf("expected Font.ColorIndexed 0, got %d", st.Font.ColorIndexed)
	}
	if st.Font.ColorTint != 0 {
		t.Fatalf("expected Font.ColorTint 0, got %v", st.Font.ColorTint)
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open styles zip failed: %v", err)
	}
	defer zr.Close()
	foundCleanRGB := false
	for _, entry := range zr.File {
		if entry.Name == "xl/styles.xml" {
			rc, err := entry.Open()
			if err != nil {
				t.Fatalf("open styles.xml: %v", err)
			}
			data, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Fatalf("read styles.xml: %v", err)
			}
			if strings.Contains(string(data), `<color rgb="FFCC0000"></color>`) || strings.Contains(string(data), `<color rgb="FFCC0000"/>`) {
				foundCleanRGB = true
			}
		}
	}
	if !foundCleanRGB {
		t.Fatal("expected xl/styles.xml to contain clean RGB-only color element for FFCC0000")
	}
}
