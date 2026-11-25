package xlsx

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/internal/documents/docutil"
	"github.com/SpherePrime/CLI/vendordeps/xuri/excelize/v2"
)

type CellStyle struct {
	Bold          bool    `json:"bold,omitempty"`
	Italic        bool    `json:"italic,omitempty"`
	Underline     bool    `json:"underline,omitempty"`
	FontFamily    string  `json:"font_family,omitempty"`
	FontSize      float64 `json:"font_size,omitempty"`
	FontColor     string  `json:"font_color,omitempty"`
	Fill          string  `json:"fill,omitempty"`
	Border        bool    `json:"border,omitempty"`
	BorderStyle   string  `json:"border_style,omitempty"`
	BorderColor   string  `json:"border_color,omitempty"`
	Align         string  `json:"align,omitempty"`
	VerticalAlign string  `json:"vertical_align,omitempty"`
	WrapText      bool    `json:"wrap_text,omitempty"`
	NumFmt        string  `json:"num_fmt,omitempty"`
}

type StyledRange struct {
	Range string    `json:"range"`
	Style CellStyle `json:"style"`
}

type ChartSeriesSpec struct {
	Name       string `json:"name,omitempty"`
	Categories string `json:"categories,omitempty"`
	Values     string `json:"values"`
}

type ChartSpec struct {
	Type           string            `json:"type"`
	Cell           string            `json:"cell"`
	Title          string            `json:"title,omitempty"`
	Series         []ChartSeriesSpec `json:"series"`
	Width          uint              `json:"width,omitempty"`
	Height         uint              `json:"height,omitempty"`
	LegendPosition string            `json:"legend_position,omitempty"`
	ShowDataLabels bool              `json:"show_data_labels,omitempty"`
}

type SheetSpec struct {
	Name        string             `json:"name"`
	Headers     []string           `json:"headers,omitempty"`
	HeaderStyle *CellStyle         `json:"header_style,omitempty"`
	Rows        [][]any            `json:"rows,omitempty"`
	Cells       map[string]any     `json:"cells,omitempty"`
	Styles      []StyledRange      `json:"styles,omitempty"`
	ColWidths   map[string]float64 `json:"col_widths,omitempty"`
	RowHeights  map[int]float64    `json:"row_heights,omitempty"`
	AutoFit     *bool              `json:"auto_fit,omitempty"`
	FreezePane  string             `json:"freeze_pane,omitempty"`
	AutoFilter  string             `json:"auto_filter,omitempty"`
	Merges      []string           `json:"merges,omitempty"`
	Charts      []ChartSpec        `json:"charts,omitempty"`
}

type CreateOptions struct {
	OutputPath  string      `json:"output_path"`
	WorkingDir  string      `json:"working_dir,omitempty"`
	Title       string      `json:"title,omitempty"`
	Author      string      `json:"author,omitempty"`
	Subject     string      `json:"subject,omitempty"`
	ActiveSheet string      `json:"active_sheet,omitempty"`
	Sheets      []SheetSpec `json:"sheets"`
}

type CellEdit struct {
	Sheet string     `json:"sheet"`
	Cell  string     `json:"cell"`
	Value any        `json:"value"`
	Style *CellStyle `json:"style,omitempty"`
}

type AppendRowsEdit struct {
	Sheet string  `json:"sheet"`
	Rows  [][]any `json:"rows"`
}

type SheetStyleEdit struct {
	Sheet string    `json:"sheet"`
	Range string    `json:"range"`
	Style CellStyle `json:"style"`
}

type RenameSheetEdit struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type ColWidthEdit struct {
	Sheet  string  `json:"sheet"`
	Column string  `json:"column"`
	Width  float64 `json:"width"`
}

type RowHeightEdit struct {
	Sheet  string  `json:"sheet"`
	Row    int     `json:"row"`
	Height float64 `json:"height"`
}

type MergeEdit struct {
	Sheet string `json:"sheet"`
	Range string `json:"range"`
}

type ChartEdit struct {
	Sheet string    `json:"sheet"`
	Chart ChartSpec `json:"chart"`
}

type UpdateOptions struct {
	InputPath    string            `json:"input_path"`
	OutputPath   string            `json:"output_path,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	AddSheets    []SheetSpec       `json:"add_sheets,omitempty"`
	RenameSheets []RenameSheetEdit `json:"rename_sheets,omitempty"`
	DeleteSheets []string          `json:"delete_sheets,omitempty"`
	SetCells     []CellEdit        `json:"set_cells,omitempty"`
	AppendRows   []AppendRowsEdit  `json:"append_rows,omitempty"`
	ApplyStyles  []SheetStyleEdit  `json:"apply_styles,omitempty"`
	ColWidths    []ColWidthEdit    `json:"col_widths,omitempty"`
	RowHeights   []RowHeightEdit   `json:"row_heights,omitempty"`
	Merges       []MergeEdit       `json:"merges,omitempty"`
	AddCharts    []ChartEdit       `json:"add_charts,omitempty"`
	ActiveSheet  string            `json:"active_sheet,omitempty"`
}

type Result struct {
	Path        string   `json:"path"`
	SizeBytes   int64    `json:"size_bytes"`
	Sheets      []string `json:"sheets"`
	ActiveSheet string   `json:"active_sheet,omitempty"`
	SheetCount  int      `json:"sheet_count"`
}

type Metadata struct {
	Title    string `json:"title,omitempty"`
	Author   string `json:"author,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Created  string `json:"created,omitempty"`
	Modified string `json:"modified,omitempty"`
}

type FormulaError struct {
	Sheet   string `json:"sheet"`
	Cell    string `json:"cell"`
	Formula string `json:"formula,omitempty"`
	Error   string `json:"error"`
}

type SheetSummary struct {
	Name       string `json:"name"`
	RowCount   int    `json:"row_count"`
	ColCount   int    `json:"col_count"`
	Dimensions string `json:"dimensions,omitempty"`
}

type InspectResult struct {
	Path           string            `json:"path"`
	SizeBytes      int64             `json:"size_bytes"`
	Sheets         []string          `json:"sheets"`
	ActiveSheet    string            `json:"active_sheet"`
	Metadata       Metadata          `json:"metadata"`
	Sheet          string            `json:"sheet"`
	Range          string            `json:"range,omitempty"`
	Dimensions     string            `json:"dimensions,omitempty"`
	RowCount       int               `json:"row_count"`
	ColCount       int               `json:"col_count"`
	Rows           [][]string        `json:"rows"`
	Formulas       map[string]string `json:"formulas,omitempty"`
	FormulaErrors  []FormulaError    `json:"formula_errors,omitempty"`
	MergedCells    []string          `json:"merged_cells,omitempty"`
	SheetSummaries []SheetSummary    `json:"sheet_summaries,omitempty"`
}

var knownFormulaErrors = []string{
	"#REF!",
	"#DIV/0!",
	"#VALUE!",
	"#NAME?",
	"#N/A",
	"#NUM!",
	"#NULL!",
	"#SPILL!",
	"#CALC!",
}

func resolvePath(workingDir, rawPath string) (string, error) {
	wd := strings.TrimSpace(workingDir)
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			wd = ""
		}
	}
	return docutil.ResolvePath(wd, rawPath)
}

func normalizeHexColor(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return ""
	}
	for i := range 6 {
		c := s[i]
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isHex {
			return ""
		}
	}
	return strings.ToUpper(s)
}

func mapBorderStyle(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "thin":
		return 1
	case "medium":
		return 2
	case "dashed":
		return 3
	case "dotted":
		return 4
	case "thick":
		return 5
	case "double":
		return 6
	default:
		return 1
	}
}

func mapHorizontalAlign(align string) string {
	switch strings.ToLower(strings.TrimSpace(align)) {
	case "left":
		return "left"
	case "center":
		return "center"
	case "right":
		return "right"
	case "justify":
		return "justify"
	case "fill":
		return "fill"
	case "distributed":
		return "distributed"
	default:
		return ""
	}
}

func mapVerticalAlign(align string) string {
	switch strings.ToLower(strings.TrimSpace(align)) {
	case "top":
		return "top"
	case "center", "middle":
		return "center"
	case "bottom":
		return "bottom"
	case "justify":
		return "justify"
	case "distributed":
		return "distributed"
	default:
		return ""
	}
}

func mapChartType(kind string) (excelize.ChartType, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "col", "column", "bar_vertical":
		return excelize.Col, nil
	case "col_stacked", "column_stacked":
		return excelize.ColStacked, nil
	case "bar", "bar_horizontal":
		return excelize.Bar, nil
	case "bar_stacked":
		return excelize.BarStacked, nil
	case "line":
		return excelize.Line, nil
	case "pie":
		return excelize.Pie, nil
	case "area":
		return excelize.Area, nil
	case "doughnut", "donut":
		return excelize.Doughnut, nil
	default:
		return 0, fmt.Errorf("unsupported chart type %q (supported: col, bar, line, pie)", kind)
	}
}

func mapChartLegendPosition(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "bottom":
		return "bottom", nil
	case "none":
		return "none", nil
	case "top":
		return "top", nil
	case "left":
		return "left", nil
	case "right":
		return "right", nil
	case "top_right", "top-right":
		return "top_right", nil
	default:
		return "", fmt.Errorf("unsupported legend_position %q (supported: none, top, bottom, left, right, top_right)", raw)
	}
}

func splitRangeRef(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", errors.New("cell range is required")
	}
	parts := strings.Split(trimmed, ":")
	switch len(parts) {
	case 1:
		cell := strings.TrimSpace(parts[0])
		if _, _, err := excelize.CellNameToCoordinates(cell); err != nil {
			return "", "", fmt.Errorf("invalid cell reference %q: %w", cell, err)
		}
		return cell, cell, nil
	case 2:
		topLeft := strings.TrimSpace(parts[0])
		bottomRight := strings.TrimSpace(parts[1])
		if _, _, err := excelize.CellNameToCoordinates(topLeft); err != nil {
			return "", "", fmt.Errorf("invalid range start %q: %w", topLeft, err)
		}
		if _, _, err := excelize.CellNameToCoordinates(bottomRight); err != nil {
			return "", "", fmt.Errorf("invalid range end %q: %w", bottomRight, err)
		}
		return topLeft, bottomRight, nil
	default:
		return "", "", fmt.Errorf("invalid range %q", raw)
	}
}

func parseRangeCoordinates(raw string) (int, int, int, int, error) {
	topLeft, bottomRight, err := splitRangeRef(raw)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	c1, r1, err := excelize.CellNameToCoordinates(topLeft)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	c2, r2, err := excelize.CellNameToCoordinates(bottomRight)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if c2 < c1 {
		c1, c2 = c2, c1
	}
	if r2 < r1 {
		r1, r2 = r2, r1
	}
	return c1, r1, c2, r2, nil
}

func setCellValueOrFormula(f *excelize.File, sheet, cell string, value any) error {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "=") && len(strings.TrimSpace(v)) > 1 {
			return f.SetCellFormula(sheet, cell, strings.TrimPrefix(v, "="))
		}
		return f.SetCellValue(sheet, cell, v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return f.SetCellValue(sheet, cell, i)
		}
		if fl, err := v.Float64(); err == nil {
			return f.SetCellValue(sheet, cell, fl)
		}
		return f.SetCellValue(sheet, cell, v.String())
	default:
		return f.SetCellValue(sheet, cell, value)
	}
}

func buildExcelizeStyle(base *excelize.Style, patch CellStyle) *excelize.Style {
	out := &excelize.Style{}
	if base != nil {
		*out = *base
		if base.Font != nil {
			fc := *base.Font
			out.Font = &fc
		}
		if base.Alignment != nil {
			ac := *base.Alignment
			out.Alignment = &ac
		}
		if len(base.Border) > 0 {
			out.Border = append([]excelize.Border(nil), base.Border...)
		}
		if len(base.Fill.Color) > 0 {
			out.Fill.Color = append([]string(nil), base.Fill.Color...)
		}
		if base.CustomNumFmt != nil {
			if *base.CustomNumFmt == "" {
				out.CustomNumFmt = nil
			} else {
				cp := *base.CustomNumFmt
				out.CustomNumFmt = &cp
			}
		}
		if out.Fill.Type == "" {
			out.Fill = excelize.Fill{}
		}
	}

	fontColor := normalizeHexColor(patch.FontColor)
	hasFont := patch.Bold || patch.Italic || patch.Underline ||
		strings.TrimSpace(patch.FontFamily) != "" || patch.FontSize > 0 || fontColor != ""
	if hasFont {
		if out.Font == nil {
			out.Font = &excelize.Font{}
		}
		if patch.Bold {
			out.Font.Bold = true
		}
		if patch.Italic {
			out.Font.Italic = true
		}
		if patch.Underline {
			out.Font.Underline = "single"
		}
		if fam := strings.TrimSpace(patch.FontFamily); fam != "" {
			out.Font.Family = fam
		}
		if patch.FontSize > 0 {
			out.Font.Size = patch.FontSize
		}
		if fontColor != "" {
			out.Font.Color = fontColor
			out.Font.ColorTheme = nil
			out.Font.ColorIndexed = 0
			out.Font.ColorTint = 0
		}
	}

	if fillColor := normalizeHexColor(patch.Fill); fillColor != "" {
		out.Fill = excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{fillColor},
		}
	}

	if patch.Border || strings.TrimSpace(patch.BorderStyle) != "" {
		borderColor := normalizeHexColor(patch.BorderColor)
		if borderColor == "" {
			borderColor = "D9D9D9"
		}
		borderStyle := mapBorderStyle(patch.BorderStyle)
		out.Border = []excelize.Border{
			{Type: "left", Color: borderColor, Style: borderStyle},
			{Type: "top", Color: borderColor, Style: borderStyle},
			{Type: "right", Color: borderColor, Style: borderStyle},
			{Type: "bottom", Color: borderColor, Style: borderStyle},
		}
	}

	hAlign := mapHorizontalAlign(patch.Align)
	vAlign := mapVerticalAlign(patch.VerticalAlign)
	if hAlign != "" || vAlign != "" || patch.WrapText {
		if out.Alignment == nil {
			out.Alignment = &excelize.Alignment{}
		}
		if hAlign != "" {
			out.Alignment.Horizontal = hAlign
		}
		if vAlign != "" {
			out.Alignment.Vertical = vAlign
		}
		if patch.WrapText {
			out.Alignment.WrapText = true
		}
	}

	if nf := strings.TrimSpace(patch.NumFmt); nf != "" {
		out.NumFmt = 0
		out.CustomNumFmt = &nf
	}

	return out
}

func applyRangeStyle(f *excelize.File, sheet, rangeRef string, patch CellStyle) error {
	c1, r1, c2, r2, err := parseRangeCoordinates(rangeRef)
	if err != nil {
		return err
	}

	hasExisting := false
	for r := r1; r <= r2 && !hasExisting; r++ {
		for c := c1; c <= c2; c++ {
			cell, err := excelize.CoordinatesToCellName(c, r)
			if err != nil {
				return err
			}
			id, err := f.GetCellStyle(sheet, cell)
			if err != nil {
				return err
			}
			if id != 0 {
				hasExisting = true
				break
			}
		}
	}

	if !hasExisting {
		topLeft, _ := excelize.CoordinatesToCellName(c1, r1)
		bottomRight, _ := excelize.CoordinatesToCellName(c2, r2)
		styleID, err := f.NewStyle(buildExcelizeStyle(nil, patch))
		if err != nil {
			return fmt.Errorf("create style: %w", err)
		}
		return f.SetCellStyle(sheet, topLeft, bottomRight, styleID)
	}

	cache := make(map[int]int)
	for r := r1; r <= r2; r++ {
		for c := c1; c <= c2; c++ {
			cell, err := excelize.CoordinatesToCellName(c, r)
			if err != nil {
				return err
			}
			existingID, err := f.GetCellStyle(sheet, cell)
			if err != nil {
				return err
			}
			mergedID, ok := cache[existingID]
			if !ok {
				var base *excelize.Style
				if existingID != 0 {
					base, err = f.GetStyle(existingID)
					if err != nil {
						return fmt.Errorf("read existing style %d: %w", existingID, err)
					}
				}
				mergedID, err = f.NewStyle(buildExcelizeStyle(base, patch))
				if err != nil {
					return fmt.Errorf("create merged style: %w", err)
				}
				cache[existingID] = mergedID
			}
			if err := f.SetCellStyle(sheet, cell, cell, mergedID); err != nil {
				return err
			}
		}
	}
	return nil
}

func quoteSheetName(sheet string) string {
	escaped := strings.ReplaceAll(sheet, "'", "''")
	return "'" + escaped + "'"
}

func formatAbsoluteCell(cell string) (string, bool) {
	clean := strings.ReplaceAll(strings.TrimSpace(cell), "$", "")
	col, row, err := excelize.SplitCellName(clean)
	if err != nil || col == "" || row <= 0 {
		return "", false
	}
	return "$" + strings.ToUpper(col) + "$" + strconv.Itoa(row), true
}

func qualifyChartRange(sheet, raw string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "="))
	if trimmed == "" || strings.Contains(trimmed, "!") {
		return trimmed
	}
	parts := strings.Split(trimmed, ":")
	if len(parts) == 1 {
		if abs, ok := formatAbsoluteCell(parts[0]); ok {
			return quoteSheetName(sheet) + "!" + abs
		}
		return trimmed
	}
	if len(parts) == 2 {
		abs1, ok1 := formatAbsoluteCell(parts[0])
		abs2, ok2 := formatAbsoluteCell(parts[1])
		if ok1 && ok2 {
			return quoteSheetName(sheet) + "!" + abs1 + ":" + abs2
		}
	}
	return trimmed
}

func qualifyChartName(sheet, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.Contains(trimmed, "!") {
		return strings.TrimPrefix(trimmed, "=")
	}
	if abs, ok := formatAbsoluteCell(trimmed); ok {
		return quoteSheetName(sheet) + "!" + abs
	}
	return trimmed
}

func addChartToSheet(f *excelize.File, sheet string, spec ChartSpec) error {
	chartType, err := mapChartType(spec.Type)
	if err != nil {
		return err
	}
	anchor := strings.TrimSpace(spec.Cell)
	if anchor == "" {
		return errors.New("chart anchor cell is required")
	}
	if _, _, err := excelize.CellNameToCoordinates(anchor); err != nil {
		return fmt.Errorf("invalid chart cell %q: %w", anchor, err)
	}
	if len(spec.Series) == 0 {
		return errors.New("chart requires at least one series")
	}

	series := make([]excelize.ChartSeries, 0, len(spec.Series))
	for _, s := range spec.Series {
		vals := qualifyChartRange(sheet, s.Values)
		if vals == "" {
			return errors.New("chart series values range is required")
		}
		series = append(series, excelize.ChartSeries{
			Name:       qualifyChartName(sheet, s.Name),
			Categories: qualifyChartRange(sheet, s.Categories),
			Values:     vals,
		})
	}

	width := spec.Width
	if width == 0 {
		width = 480
	}
	height := spec.Height
	if height == 0 {
		height = 280
	}

	legendPos, err := mapChartLegendPosition(spec.LegendPosition)
	if err != nil {
		return err
	}

	chart := &excelize.Chart{
		Type:   chartType,
		Series: series,
		Dimension: excelize.ChartDimension{
			Width:  width,
			Height: height,
		},
		Legend: excelize.ChartLegend{
			Position: legendPos,
		},
		PlotArea: excelize.ChartPlotArea{
			ShowVal: spec.ShowDataLabels,
		},
	}
	if title := strings.TrimSpace(spec.Title); title != "" {
		chart.Title = excelize.ChartTitle{
			Paragraph: []excelize.RichTextRun{{Text: title}},
		}
	}

	return f.AddChart(sheet, anchor, chart)
}

func applyFreezePane(f *excelize.File, sheet, cellRef string) error {
	trimmed := strings.TrimSpace(cellRef)
	if trimmed == "" {
		return nil
	}
	col, row, err := excelize.CellNameToCoordinates(trimmed)
	if err != nil {
		return fmt.Errorf("invalid freeze_pane cell %q: %w", trimmed, err)
	}
	xSplit := col - 1
	ySplit := row - 1
	if xSplit == 0 && ySplit == 0 {
		return nil
	}
	activePane := "bottomRight"
	if xSplit == 0 {
		activePane = "bottomLeft"
	} else if ySplit == 0 {
		activePane = "topRight"
	}
	topLeft := strings.ToUpper(trimmed)
	return f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      xSplit,
		YSplit:      ySplit,
		TopLeftCell: topLeft,
		ActivePane:  activePane,
		Selection: []excelize.Selection{
			{
				SQRef:      topLeft,
				ActiveCell: topLeft,
				Pane:       activePane,
			},
		},
	})
}

func applyAutoFitColumns(f *excelize.File, sheet string, explicit map[string]float64) error {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return err
	}

	type mergeSpan struct {
		c1, r1, c2, r2 int
	}
	var wideMerges []mergeSpan
	if merges, err := f.GetMergeCells(sheet); err == nil {
		for _, m := range merges {
			c1, r1, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
			c2, r2, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
			if err1 == nil && err2 == nil && c2 > c1 {
				wideMerges = append(wideMerges, mergeSpan{c1: c1, r1: r1, c2: c2, r2: r2})
			}
		}
	}
	inWideMerge := func(col, row int) bool {
		for _, ms := range wideMerges {
			if col >= ms.c1 && col <= ms.c2 && row >= ms.r1 && row <= ms.r2 {
				return true
			}
		}
		return false
	}

	maxLen := make(map[int]int)
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			if inWideMerge(colIdx+1, rowIdx+1) {
				continue
			}
			if val == "" {
				if cellRef, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1); err == nil {
					if formula, _ := f.GetCellFormula(sheet, cellRef); formula != "" {
						if calcVal, err := f.CalcCellValue(sheet, cellRef); err == nil {
							val = calcVal
						}
					}
				}
			}
			for _, line := range strings.Split(val, "\n") {
				n := utf8.RuneCountInString(line)
				if n > maxLen[colIdx] {
					maxLen[colIdx] = n
				}
			}
		}
	}

	explicitUpper := make(map[string]bool, len(explicit))
	for k := range explicit {
		explicitUpper[strings.ToUpper(strings.TrimSpace(k))] = true
	}

	for colIdx, n := range maxLen {
		colName, err := excelize.ColumnNumberToName(colIdx + 1)
		if err != nil {
			continue
		}
		if explicitUpper[colName] {
			continue
		}
		width := min(max(float64(n)*1.15+3.0, 10.0), 60.0)
		if err := f.SetColWidth(sheet, colName, colName, width); err != nil {
			return err
		}
	}
	return nil
}

func saveWorkbookAtomic(f *excelize.File, outPath string) (*Result, error) {
	if err := f.UpdateLinkedValue(); err != nil {
		return nil, fmt.Errorf("update linked values: %w", err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("serialize xlsx: %w", err)
	}
	data := buf.Bytes()
	if err := docutil.WriteFileAtomic(outPath, data); err != nil {
		return nil, fmt.Errorf("write xlsx: %w", err)
	}
	sheets := f.GetSheetList()
	activeSheet := f.GetSheetName(f.GetActiveSheetIndex())
	return &Result{
		Path:        outPath,
		SizeBytes:   int64(len(data)),
		Sheets:      sheets,
		ActiveSheet: activeSheet,
		SheetCount:  len(sheets),
	}, nil
}

func detectExactFormulaError(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	for _, code := range knownFormulaErrors {
		if trimmed == code {
			return code
		}
	}
	return ""
}

func detectFormulaErrorInMessage(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	for _, code := range knownFormulaErrors {
		if trimmed == code || strings.Contains(trimmed, code) {
			return code
		}
	}
	return ""
}
