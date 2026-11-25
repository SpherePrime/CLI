package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/documents/docutil"
	"github.com/SpherePrime/CLI/internal/documents/xlsx"
	"github.com/SpherePrime/CLI/internal/fsext"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const ExcelWorkbookToolName = "excel_workbook"

//go:embed excel_workbook.md
var excelWorkbookDescription string

type ExcelWorkbookParams struct {
	Action       string                 `json:"action" enum:"create,inspect,update" description:"Operation to perform: create, inspect, or update"`
	Path         string                 `json:"path,omitempty" description:"Target file path (used by inspect, or fallback for input_path/output_path)"`
	InputPath    string                 `json:"input_path,omitempty" description:"Input XLSX file path for inspect or update"`
	OutputPath   string                 `json:"output_path,omitempty" description:"Output XLSX file path for create or update (defaults to input_path in-place)"`
	Sheet        string                 `json:"sheet,omitempty" description:"Sheet name to inspect (defaults to active or first sheet)"`
	Range        string                 `json:"range,omitempty" description:"Optional A1 range to inspect, e.g. A1:D10"`
	Title        string                 `json:"title,omitempty" description:"Workbook title metadata for create"`
	Author       string                 `json:"author,omitempty" description:"Workbook author metadata for create"`
	Subject      string                 `json:"subject,omitempty" description:"Workbook subject metadata for create"`
	ActiveSheet  string                 `json:"active_sheet,omitempty" description:"Active sheet name for create or update"`
	Sheets       []xlsx.SheetSpec       `json:"sheets,omitempty" description:"Worksheet definitions for create"`
	AddSheets    []xlsx.SheetSpec       `json:"add_sheets,omitempty" description:"New worksheets to add during update"`
	RenameSheets []xlsx.RenameSheetEdit `json:"rename_sheets,omitempty" description:"Worksheets to rename during update"`
	DeleteSheets []string               `json:"delete_sheets,omitempty" description:"Worksheet names to delete during update"`
	SetCells     []xlsx.CellEdit        `json:"set_cells,omitempty" description:"Cell value/formula/style updates during update"`
	AppendRows   []xlsx.AppendRowsEdit  `json:"append_rows,omitempty" description:"Rows to append to worksheets during update"`
	ApplyStyles  []xlsx.SheetStyleEdit  `json:"apply_styles,omitempty" description:"Range styles to apply during update"`
	ColWidths    []xlsx.ColWidthEdit    `json:"col_widths,omitempty" description:"Column width updates during update"`
	RowHeights   []xlsx.RowHeightEdit   `json:"row_heights,omitempty" description:"Row height updates during update"`
	Merges       []xlsx.MergeEdit       `json:"merges,omitempty" description:"Cell range merges during update"`
	AddCharts    []xlsx.ChartEdit       `json:"add_charts,omitempty" description:"Charts to add during update"`
}

type excelWriteToolResult struct {
	Action      string   `json:"action"`
	Path        string   `json:"path"`
	SizeBytes   int64    `json:"size_bytes"`
	Sheets      []string `json:"sheets"`
	ActiveSheet string   `json:"active_sheet,omitempty"`
	SheetCount  int      `json:"sheet_count"`
	Summary     string   `json:"summary"`
	Note        string   `json:"note"`
}

type excelWorkbookTool struct {
	fantasy.AgentTool
}

func (t excelWorkbookTool) Info() fantasy.ToolInfo {
	info := t.AgentTool.Info()
	patchExcelWorkbookParameters(info.Parameters)
	return info
}

func excelCellValueSchema() map[string]any {
	return map[string]any{
		"type": []string{"string", "number", "boolean", "null"},
	}
}

func patchExcelSheetSpecSchema(arrayParam any) {
	arr, ok := arrayParam.(map[string]any)
	if !ok {
		return
	}
	items, ok := arr["items"].(map[string]any)
	if !ok {
		return
	}
	props, ok := items["properties"].(map[string]any)
	if !ok {
		return
	}
	patchExcelRowsSchema(props["rows"])
	if cells, ok := props["cells"].(map[string]any); ok {
		cells["properties"] = map[string]any{
			"*": excelCellValueSchema(),
		}
		cells["additionalProperties"] = excelCellValueSchema()
	}
}

func patchExcelRowsSchema(rowsParam any) {
	rows, ok := rowsParam.(map[string]any)
	if !ok {
		return
	}
	rowItems, ok := rows["items"].(map[string]any)
	if !ok {
		return
	}
	rowItems["items"] = excelCellValueSchema()
}

func patchExcelWorkbookParameters(params map[string]any) {
	if params == nil {
		return
	}
	patchExcelSheetSpecSchema(params["sheets"])
	patchExcelSheetSpecSchema(params["add_sheets"])
	if appendRows, ok := params["append_rows"].(map[string]any); ok {
		if items, ok := appendRows["items"].(map[string]any); ok {
			if props, ok := items["properties"].(map[string]any); ok {
				patchExcelRowsSchema(props["rows"])
			}
		}
	}
	if setCells, ok := params["set_cells"].(map[string]any); ok {
		if items, ok := setCells["items"].(map[string]any); ok {
			if props, ok := items["properties"].(map[string]any); ok {
				valSchema := excelCellValueSchema()
				if existing, ok := props["value"].(map[string]any); ok {
					if desc, ok := existing["description"].(string); ok && desc != "" {
						valSchema["description"] = desc
					}
				}
				props["value"] = valSchema
			}
		}
	}
}

func NewExcelWorkbookTool(permissions permission.Service, workingDir string) fantasy.AgentTool {
	base := fantasy.NewAgentTool(
		ExcelWorkbookToolName,
		excelWorkbookDescription,
		func(ctx context.Context, params ExcelWorkbookParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			action := strings.ToLower(strings.TrimSpace(params.Action))
			switch action {
			case "create":
				rawOut := firstTrimmed(params.OutputPath, params.Path)
				outPath, err := docutil.ResolvePath(workingDir, rawOut)
				if err != nil {
					return fantasy.NewTextErrorResponse("output_path is required: " + err.Error()), nil
				}
				if len(params.Sheets) == 0 {
					return fantasy.NewTextErrorResponse("sheets is required for create"), nil
				}
				allowed, err := requestDocumentWritePermission(
					ctx, permissions, call, workingDir,
					ExcelWorkbookToolName, action, outPath, "",
					fmt.Sprintf("Create Excel workbook %s", outPath),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				res, err := xlsx.Create(xlsx.CreateOptions{
					OutputPath:  outPath,
					WorkingDir:  workingDir,
					Title:       params.Title,
					Author:      params.Author,
					Subject:     params.Subject,
					ActiveSheet: params.ActiveSheet,
					Sheets:      params.Sheets,
				})
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to create Excel workbook: " + err.Error()), nil
				}
				fsext.InvalidateIndex(res.Path)
				return documentJSONResponse(excelWriteToolResult{
					Action:      action,
					Path:        res.Path,
					SizeBytes:   res.SizeBytes,
					Sheets:      res.Sheets,
					ActiveSheet: res.ActiveSheet,
					SheetCount:  res.SheetCount,
					Summary:     fmt.Sprintf("Created Excel workbook (%d sheet(s), %d bytes)", res.SheetCount, res.SizeBytes),
					Note:        documentCitePathNote,
				})

			case "inspect":
				rawIn := firstTrimmed(params.Path, params.InputPath)
				inPath, err := docutil.ResolvePath(workingDir, rawIn)
				if err != nil {
					return fantasy.NewTextErrorResponse("path is required: " + err.Error()), nil
				}
				res, err := xlsx.Inspect(inPath, params.Sheet, params.Range)
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to inspect Excel workbook: " + err.Error()), nil
				}
				return documentJSONResponse(res)

			case "update":
				rawIn := firstTrimmed(params.InputPath, params.Path)
				inPath, err := docutil.ResolvePath(workingDir, rawIn)
				if err != nil {
					return fantasy.NewTextErrorResponse("input_path is required: " + err.Error()), nil
				}
				outPath := inPath
				if strings.TrimSpace(params.OutputPath) != "" {
					outPath, err = docutil.ResolvePath(workingDir, params.OutputPath)
					if err != nil {
						return fantasy.NewTextErrorResponse("invalid output_path: " + err.Error()), nil
					}
				}
				allowed, err := requestDocumentWritePermission(
					ctx, permissions, call, workingDir,
					ExcelWorkbookToolName, action, outPath, inPath,
					fmt.Sprintf("Update Excel workbook %s -> %s", inPath, outPath),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				res, err := xlsx.Update(xlsx.UpdateOptions{
					InputPath:    inPath,
					OutputPath:   outPath,
					WorkingDir:   workingDir,
					AddSheets:    params.AddSheets,
					RenameSheets: params.RenameSheets,
					DeleteSheets: params.DeleteSheets,
					SetCells:     params.SetCells,
					AppendRows:   params.AppendRows,
					ApplyStyles:  params.ApplyStyles,
					ColWidths:    params.ColWidths,
					RowHeights:   params.RowHeights,
					Merges:       params.Merges,
					AddCharts:    params.AddCharts,
					ActiveSheet:  params.ActiveSheet,
				})
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to update Excel workbook: " + err.Error()), nil
				}
				fsext.InvalidateIndex(res.Path)
				return documentJSONResponse(excelWriteToolResult{
					Action:      action,
					Path:        res.Path,
					SizeBytes:   res.SizeBytes,
					Sheets:      res.Sheets,
					ActiveSheet: res.ActiveSheet,
					SheetCount:  res.SheetCount,
					Summary:     fmt.Sprintf("Updated Excel workbook (%d sheet(s), %d bytes)", res.SheetCount, res.SizeBytes),
					Note:        documentCitePathNote,
				})

			default:
				return fantasy.NewTextErrorResponse("action must be one of: create, inspect, update"), nil
			}
		},
	)
	return excelWorkbookTool{AgentTool: base}
}
