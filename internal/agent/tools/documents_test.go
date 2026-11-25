package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/documents/docx"
	"github.com/SpherePrime/CLI/internal/documents/pdf"
	"github.com/SpherePrime/CLI/internal/documents/pptx"
	"github.com/SpherePrime/CLI/internal/documents/xlsx"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type docPermissionRecorder struct {
	*pubsub.Broker[permission.PermissionRequest]
	allow    bool
	requests []permission.CreatePermissionRequest
}

func (r *docPermissionRecorder) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	r.requests = append(r.requests, req)
	return r.allow, nil
}

func (r *docPermissionRecorder) Grant(req permission.PermissionRequest) bool           { return true }
func (r *docPermissionRecorder) Deny(req permission.PermissionRequest) bool            { return true }
func (r *docPermissionRecorder) GrantPersistent(req permission.PermissionRequest) bool { return true }
func (r *docPermissionRecorder) AutoApproveSession(sessionID string)                   {}
func (r *docPermissionRecorder) SetSkipRequests(skip bool)                             {}
func (r *docPermissionRecorder) SkipRequests() bool                                    { return false }
func (r *docPermissionRecorder) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

func docTestCtx() context.Context {
	return context.WithValue(context.Background(), SessionIDContextKey, "doc-test-session")
}

func runDocTool(t *testing.T, tool fantasy.AgentTool, name string, params any) (fantasy.ToolResponse, error) {
	t.Helper()
	raw, err := json.Marshal(params)
	require.NoError(t, err)
	return tool.Run(docTestCtx(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  name,
		Input: string(raw),
	})
}

func parseDocJSON(t *testing.T, content string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(content), &out), "expected JSON response, got: %s", content)
	return out
}

func TestDocumentPDFAllActions(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	perms := &docPermissionRecorder{allow: true}
	tool := NewPDFDocumentTool(perms, workingDir)

	require.Equal(t, PDFDocumentToolName, tool.Info().Name)
	_, err := fantasy.CanonicalToolSchema(tool.Info())
	require.NoError(t, err)

	resp, err := runDocTool(t, tool, PDFDocumentToolName, PDFDocumentParams{
		Action:      "create",
		OutputPath:  "reports/first.pdf",
		Title:       "Квартальный отчёт",
		Author:      "Тест",
		PageNumbers: true,
		Blocks: []pdf.Block{
			{Type: "heading", Level: 1, Text: "Раздел 1"},
			{Type: "paragraph", Text: "Привет мир из PDF"},
			{Type: "page_break"},
			{Type: "paragraph", Text: "Вторая страница"},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.Len(t, perms.requests, 1)
	require.Equal(t, "write", perms.requests[0].Action)
	require.Equal(t, PDFDocumentToolName, perms.requests[0].ToolName)

	created := parseDocJSON(t, resp.Content)
	expectedFirst := filepath.Join(workingDir, "reports", "first.pdf")
	require.Equal(t, expectedFirst, created["path"])
	require.Greater(t, created["size_bytes"].(float64), float64(0))
	require.Equal(t, float64(2), created["page_count"])
	require.NotEmpty(t, created["summary"])
	require.NotEmpty(t, created["note"])

	resp, err = runDocTool(t, tool, PDFDocumentToolName, PDFDocumentParams{
		Action:     "create",
		OutputPath: "reports/second.pdf",
		Blocks:     []pdf.Block{{Type: "paragraph", Text: "Third page content"}},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.Len(t, perms.requests, 2)

	beforeInspect := len(perms.requests)
	resp, err = runDocTool(t, tool, PDFDocumentToolName, PDFDocumentParams{
		Action: "inspect",
		Path:   "reports/first.pdf",
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.Equal(t, beforeInspect, len(perms.requests))
	inspected := parseDocJSON(t, resp.Content)
	require.Equal(t, expectedFirst, inspected["path"])
	require.Equal(t, float64(2), inspected["page_count"])

	resp, err = runDocTool(t, tool, PDFDocumentToolName, PDFDocumentParams{
		Action:     "merge",
		InputPaths: []string{"reports/first.pdf", "reports/second.pdf"},
		OutputPath: "reports/merged.pdf",
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	merged := parseDocJSON(t, resp.Content)
	require.Equal(t, filepath.Join(workingDir, "reports", "merged.pdf"), merged["path"])
	require.Equal(t, float64(3), merged["page_count"])
	require.Greater(t, merged["size_bytes"].(float64), float64(0))

	resp, err = runDocTool(t, tool, PDFDocumentToolName, PDFDocumentParams{
		Action:     "split",
		InputPath:  "reports/merged.pdf",
		PageRange:  "1-2",
		OutputPath: "reports/trimmed.pdf",
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	splitOut := parseDocJSON(t, resp.Content)
	require.Equal(t, filepath.Join(workingDir, "reports", "trimmed.pdf"), splitOut["path"])
	require.Greater(t, splitOut["size_bytes"].(float64), float64(0))
	files := splitOut["files"].([]any)
	require.Len(t, files, 1)

	resp, err = runDocTool(t, tool, PDFDocumentToolName, PDFDocumentParams{
		Action:     "rotate",
		InputPath:  "reports/trimmed.pdf",
		PageRange:  "1",
		Angle:      90,
		OutputPath: "reports/rotated.pdf",
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	rotated := parseDocJSON(t, resp.Content)
	require.Equal(t, filepath.Join(workingDir, "reports", "rotated.pdf"), rotated["path"])
	require.Equal(t, float64(2), rotated["page_count"])
	require.Greater(t, rotated["size_bytes"].(float64), float64(0))
}

func TestDocumentWordAllActions(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	perms := &docPermissionRecorder{allow: true}
	tool := NewWordDocumentTool(perms, workingDir)

	require.Equal(t, WordDocumentToolName, tool.Info().Name)
	_, err := fantasy.CanonicalToolSchema(tool.Info())
	require.NoError(t, err)

	resp, err := runDocTool(t, tool, WordDocumentToolName, WordDocumentParams{
		Action:     "create",
		OutputPath: "docs/contract.docx",
		Title:      "Договор {{client}}",
		Blocks: []docx.Block{
			{Type: "heading", Level: 1, Text: "Условия"},
			{Type: "paragraph", Text: "Клиент: {{client}}, сумма: {{amount}}"},
			{Type: "table", Headers: []string{"Позиция", "Цена"}, Rows: [][]string{{"Услуга", "100"}}},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	created := parseDocJSON(t, resp.Content)
	expectedPath := filepath.Join(workingDir, "docs", "contract.docx")
	require.Equal(t, expectedPath, created["path"])
	require.Greater(t, created["size_bytes"].(float64), float64(0))
	require.NotEmpty(t, created["note"])

	beforeInspect := len(perms.requests)
	resp, err = runDocTool(t, tool, WordDocumentToolName, WordDocumentParams{
		Action: "inspect",
		Path:   "docs/contract.docx",
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.Equal(t, beforeInspect, len(perms.requests))
	inspected := parseDocJSON(t, resp.Content)
	require.Equal(t, expectedPath, inspected["path"])
	require.Contains(t, inspected["placeholders"], "client")

	resp, err = runDocTool(t, tool, WordDocumentToolName, WordDocumentParams{
		Action:     "fill_template",
		InputPath:  "docs/contract.docx",
		OutputPath: "docs/filled.docx",
		Values:     map[string]string{"client": "ООО Ромашка", "amount": "50000"},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	filled := parseDocJSON(t, resp.Content)
	require.Equal(t, filepath.Join(workingDir, "docs", "filled.docx"), filled["path"])
	require.Greater(t, filled["replacements_applied"].(float64), float64(0))

	resp, err = runDocTool(t, tool, WordDocumentToolName, WordDocumentParams{
		Action:       "update",
		InputPath:    "docs/filled.docx",
		Replacements: map[string]string{"50000": "75000"},
		TableUpdates: []docx.TableUpdate{{TableIndex: 0, Row: 1, Col: 1, Value: "75000"}},
		AppendBlocks: []docx.Block{{Type: "paragraph", Text: "Дополнение"}},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	updated := parseDocJSON(t, resp.Content)
	require.Equal(t, filepath.Join(workingDir, "docs", "filled.docx"), updated["path"])
	require.Greater(t, updated["size_bytes"].(float64), float64(0))
}

func TestDocumentExcelAllActions(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	perms := &docPermissionRecorder{allow: true}
	tool := NewExcelWorkbookTool(perms, workingDir)

	require.Equal(t, ExcelWorkbookToolName, tool.Info().Name)
	_, err := fantasy.CanonicalToolSchema(tool.Info())
	require.NoError(t, err)

	resp, err := runDocTool(t, tool, ExcelWorkbookToolName, ExcelWorkbookParams{
		Action:     "create",
		OutputPath: "sheets/budget.xlsx",
		Title:      "Бюджет",
		Sheets: []xlsx.SheetSpec{
			{
				Name:    "Сводка",
				Headers: []string{"Статья", "Сумма"},
				Rows: [][]any{
					{"Разработка", 1000},
					{"Дизайн", 500},
					{"Итого", "=SUM(B2:B3)"},
				},
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	created := parseDocJSON(t, resp.Content)
	expectedPath := filepath.Join(workingDir, "sheets", "budget.xlsx")
	require.Equal(t, expectedPath, created["path"])
	require.Greater(t, created["size_bytes"].(float64), float64(0))
	require.Equal(t, float64(1), created["sheet_count"])
	require.NotEmpty(t, created["note"])

	beforeInspect := len(perms.requests)
	resp, err = runDocTool(t, tool, ExcelWorkbookToolName, ExcelWorkbookParams{
		Action: "inspect",
		Path:   "sheets/budget.xlsx",
		Sheet:  "Сводка",
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.Equal(t, beforeInspect, len(perms.requests))
	inspected := parseDocJSON(t, resp.Content)
	require.Equal(t, expectedPath, inspected["path"])
	require.Equal(t, "Сводка", inspected["sheet"])

	resp, err = runDocTool(t, tool, ExcelWorkbookToolName, ExcelWorkbookParams{
		Action:     "update",
		InputPath:  "sheets/budget.xlsx",
		SetCells:   []xlsx.CellEdit{{Sheet: "Сводка", Cell: "B3", Value: 750}},
		AppendRows: []xlsx.AppendRowsEdit{{Sheet: "Сводка", Rows: [][]any{{"Резерв", 250}}}},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	updated := parseDocJSON(t, resp.Content)
	require.Equal(t, expectedPath, updated["path"])
	require.Greater(t, updated["size_bytes"].(float64), float64(0))
}

func TestDocumentPowerPointAllActions(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	perms := &docPermissionRecorder{allow: true}
	tool := NewPowerPointPresentationTool(perms, workingDir)

	require.Equal(t, PowerPointPresentationToolName, tool.Info().Name)
	_, err := fantasy.CanonicalToolSchema(tool.Info())
	require.NoError(t, err)

	resp, err := runDocTool(t, tool, PowerPointPresentationToolName, PowerPointPresentationParams{
		Action:       "create",
		OutputPath:   "decks/pitch.pptx",
		Title:        "Презентация {{project}}",
		SlideNumbers: true,
		Slides: []pptx.SlideSpec{
			{Layout: "title", Title: "Проект {{project}}", Subtitle: "Обзор 2026"},
			{Layout: "bullets", Title: "Ключевые цели", Bullets: []string{"Скорость", "Качество"}, Notes: "Заметки спикера"},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	created := parseDocJSON(t, resp.Content)
	expectedPath := filepath.Join(workingDir, "decks", "pitch.pptx")
	require.Equal(t, expectedPath, created["path"])
	require.Greater(t, created["size_bytes"].(float64), float64(0))
	require.Equal(t, float64(2), created["slide_count"])
	require.NotEmpty(t, created["note"])

	beforeInspect := len(perms.requests)
	resp, err = runDocTool(t, tool, PowerPointPresentationToolName, PowerPointPresentationParams{
		Action: "inspect",
		Path:   "decks/pitch.pptx",
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.Equal(t, beforeInspect, len(perms.requests))
	inspected := parseDocJSON(t, resp.Content)
	require.Equal(t, expectedPath, inspected["path"])
	require.Equal(t, float64(2), inspected["slide_count"])
	require.Contains(t, inspected["placeholders"], "project")

	resp, err = runDocTool(t, tool, PowerPointPresentationToolName, PowerPointPresentationParams{
		Action:     "fill_template",
		InputPath:  "decks/pitch.pptx",
		OutputPath: "decks/pitch-filled.pptx",
		Values:     map[string]string{"project": "Альфа"},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	filled := parseDocJSON(t, resp.Content)
	require.Equal(t, filepath.Join(workingDir, "decks", "pitch-filled.pptx"), filled["path"])
	require.Greater(t, filled["replacements_applied"].(float64), float64(0))

	resp, err = runDocTool(t, tool, PowerPointPresentationToolName, PowerPointPresentationParams{
		Action:       "update",
		InputPath:    "decks/pitch-filled.pptx",
		NotesUpdates: []pptx.NotesUpdate{{Slide: 2, Notes: "Обновлённые заметки"}},
		AppendSlides: []pptx.SlideSpec{{Layout: "section", Title: "Итоги"}},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	updated := parseDocJSON(t, resp.Content)
	require.Equal(t, filepath.Join(workingDir, "decks", "pitch-filled.pptx"), updated["path"])
	require.Equal(t, float64(3), updated["slide_count"])
}

func TestDocumentPermissionGating(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	allowPerms := &docPermissionRecorder{allow: true}
	denyPerms := &docPermissionRecorder{allow: false}

	pdfSeed := NewPDFDocumentTool(allowPerms, workingDir)
	_, err := runDocTool(t, pdfSeed, PDFDocumentToolName, PDFDocumentParams{
		Action:     "create",
		OutputPath: "seed.pdf",
		Blocks:     []pdf.Block{{Type: "paragraph", Text: "Seed PDF"}},
	})
	require.NoError(t, err)

	docxSeed := NewWordDocumentTool(allowPerms, workingDir)
	_, err = runDocTool(t, docxSeed, WordDocumentToolName, WordDocumentParams{
		Action:     "create",
		OutputPath: "seed.docx",
		Blocks:     []docx.Block{{Type: "paragraph", Text: "Seed DOCX"}},
	})
	require.NoError(t, err)

	xlsxSeed := NewExcelWorkbookTool(allowPerms, workingDir)
	_, err = runDocTool(t, xlsxSeed, ExcelWorkbookToolName, ExcelWorkbookParams{
		Action:     "create",
		OutputPath: "seed.xlsx",
		Sheets:     []xlsx.SheetSpec{{Name: "Sheet1", Headers: []string{"A"}, Rows: [][]any{{"1"}}}},
	})
	require.NoError(t, err)

	pptxSeed := NewPowerPointPresentationTool(allowPerms, workingDir)
	_, err = runDocTool(t, pptxSeed, PowerPointPresentationToolName, PowerPointPresentationParams{
		Action:     "create",
		OutputPath: "seed.pptx",
		Slides:     []pptx.SlideSpec{{Layout: "title", Title: "Seed PPTX"}},
	})
	require.NoError(t, err)

	cases := []struct {
		name        string
		tool        fantasy.AgentTool
		toolName    string
		createInput any
		inspectIn   any
		blockedPath string
	}{
		{
			name:        "pdf",
			tool:        NewPDFDocumentTool(denyPerms, workingDir),
			toolName:    PDFDocumentToolName,
			createInput: PDFDocumentParams{Action: "create", OutputPath: "blocked.pdf", Blocks: []pdf.Block{{Type: "paragraph", Text: "nope"}}},
			inspectIn:   PDFDocumentParams{Action: "inspect", Path: "seed.pdf"},
			blockedPath: filepath.Join(workingDir, "blocked.pdf"),
		},
		{
			name:        "word",
			tool:        NewWordDocumentTool(denyPerms, workingDir),
			toolName:    WordDocumentToolName,
			createInput: WordDocumentParams{Action: "create", OutputPath: "blocked.docx", Blocks: []docx.Block{{Type: "paragraph", Text: "nope"}}},
			inspectIn:   WordDocumentParams{Action: "inspect", Path: "seed.docx"},
			blockedPath: filepath.Join(workingDir, "blocked.docx"),
		},
		{
			name:        "excel",
			tool:        NewExcelWorkbookTool(denyPerms, workingDir),
			toolName:    ExcelWorkbookToolName,
			createInput: ExcelWorkbookParams{Action: "create", OutputPath: "blocked.xlsx", Sheets: []xlsx.SheetSpec{{Name: "S1"}}},
			inspectIn:   ExcelWorkbookParams{Action: "inspect", Path: "seed.xlsx"},
			blockedPath: filepath.Join(workingDir, "blocked.xlsx"),
		},
		{
			name:        "powerpoint",
			tool:        NewPowerPointPresentationTool(denyPerms, workingDir),
			toolName:    PowerPointPresentationToolName,
			createInput: PowerPointPresentationParams{Action: "create", OutputPath: "blocked.pptx", Slides: []pptx.SlideSpec{{Title: "nope"}}},
			inspectIn:   PowerPointPresentationParams{Action: "inspect", Path: "seed.pptx"},
			blockedPath: filepath.Join(workingDir, "blocked.pptx"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			beforeDeny := len(denyPerms.requests)
			resp, err := runDocTool(t, tc.tool, tc.toolName, tc.createInput)
			require.NoError(t, err)
			require.True(t, resp.IsError)
			require.True(t, resp.StopTurn)
			require.Contains(t, resp.Content, "User denied permission")
			require.Equal(t, beforeDeny+1, len(denyPerms.requests))
			require.Equal(t, "write", denyPerms.requests[len(denyPerms.requests)-1].Action)
			_, statErr := os.Stat(tc.blockedPath)
			require.True(t, os.IsNotExist(statErr), "denied write must not create file %s", tc.blockedPath)

			beforeInspect := len(denyPerms.requests)
			inspectResp, err := runDocTool(t, tc.tool, tc.toolName, tc.inspectIn)
			require.NoError(t, err)
			require.False(t, inspectResp.IsError, inspectResp.Content)
			require.Equal(t, beforeInspect, len(denyPerms.requests), "inspect must not trigger permission request")
		})
	}
}

func TestDocumentValidationAndMultiFileSplit(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	perms := &docPermissionRecorder{allow: true}
	pdfTool := NewPDFDocumentTool(perms, workingDir)

	resp, err := runDocTool(t, pdfTool, PDFDocumentToolName, PDFDocumentParams{
		Action: "unknown_op",
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Empty(t, perms.requests)

	resp, err = runDocTool(t, pdfTool, PDFDocumentToolName, PDFDocumentParams{
		Action:     "create",
		OutputPath: "multi.pdf",
		Blocks: []pdf.Block{
			{Type: "paragraph", Text: "Page 1"},
			{Type: "page_break"},
			{Type: "paragraph", Text: "Page 2"},
			{Type: "page_break"},
			{Type: "paragraph", Text: "Page 3"},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)

	resp, err = runDocTool(t, pdfTool, PDFDocumentToolName, PDFDocumentParams{
		Action:    "split",
		InputPath: "multi.pdf",
		Span:      2,
		OutputDir: "chunks",
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	splitOut := parseDocJSON(t, resp.Content)
	require.Equal(t, filepath.Join(workingDir, "chunks"), splitOut["path"])
	require.Greater(t, splitOut["size_bytes"].(float64), float64(0))
	require.Len(t, splitOut["files"].([]any), 2)

	nilPermPDF := NewPDFDocumentTool(nil, workingDir)
	rawInspect, err := json.Marshal(PDFDocumentParams{Action: "inspect", Path: "multi.pdf"})
	require.NoError(t, err)
	inspectResp, err := nilPermPDF.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-no-session",
		Name:  PDFDocumentToolName,
		Input: string(rawInspect),
	})
	require.NoError(t, err)
	require.False(t, inspectResp.IsError, inspectResp.Content)
}
