package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/documents/docutil"
	"github.com/SpherePrime/CLI/internal/documents/pdf"
	"github.com/SpherePrime/CLI/internal/fsext"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const PDFDocumentToolName = "pdf_document"

//go:embed pdf_document.md
var pdfDocumentDescription string

type PDFDocumentParams struct {
	Action      string       `json:"action" enum:"create,inspect,merge,split,rotate" description:"Operation to perform: create, inspect, merge, split, or rotate"`
	Path        string       `json:"path,omitempty" description:"Target file path (used by inspect, or fallback for input_path/output_path)"`
	InputPath   string       `json:"input_path,omitempty" description:"Input PDF file path for inspect, split, or rotate"`
	InputPaths  []string     `json:"input_paths,omitempty" description:"Ordered list of input PDF file paths for merge"`
	OutputPath  string       `json:"output_path,omitempty" description:"Output PDF file path for create, merge, split (with page_range), or rotate"`
	OutputDir   string       `json:"output_dir,omitempty" description:"Output directory for split"`
	PageRange   string       `json:"page_range,omitempty" description:"Page selection such as '1-3,5' for inspect, split, or rotate"`
	Span        int          `json:"span,omitempty" description:"Pages per output file for split when page_range is omitted"`
	Angle       int          `json:"angle,omitempty" description:"Clockwise rotation angle in degrees (multiple of 90) for rotate"`
	Title       string       `json:"title,omitempty" description:"Document title for create"`
	Subtitle    string       `json:"subtitle,omitempty" description:"Document subtitle for create"`
	Author      string       `json:"author,omitempty" description:"Document author metadata for create"`
	Subject     string       `json:"subject,omitempty" description:"Document subject metadata for create"`
	PageSize    string       `json:"page_size,omitempty" description:"Page size for create: A4 or Letter"`
	Orientation string       `json:"orientation,omitempty" description:"Page orientation for create: portrait or landscape"`
	Margins     *pdf.Margins `json:"margins,omitempty" description:"Page margins in points (top, right, bottom, left) for create"`
	Header      string       `json:"header,omitempty" description:"Running header text for create"`
	Footer      string       `json:"footer,omitempty" description:"Running footer text for create"`
	PageNumbers bool         `json:"page_numbers,omitempty" description:"Render page numbers in footer for create"`
	Blocks      []pdf.Block  `json:"blocks,omitempty" description:"Content blocks for create"`
}

type pdfWriteToolResult struct {
	Action    string       `json:"action"`
	Path      string       `json:"path"`
	SizeBytes int64        `json:"size_bytes"`
	PageCount int          `json:"page_count,omitempty"`
	Files     []pdf.Result `json:"files,omitempty"`
	Summary   string       `json:"summary"`
	Note      string       `json:"note"`
}

func NewPDFDocumentTool(permissions permission.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		PDFDocumentToolName,
		pdfDocumentDescription,
		func(ctx context.Context, params PDFDocumentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			action := strings.ToLower(strings.TrimSpace(params.Action))
			switch action {
			case "create":
				rawOut := firstTrimmed(params.OutputPath, params.Path)
				outPath, err := docutil.ResolvePath(workingDir, rawOut)
				if err != nil {
					return fantasy.NewTextErrorResponse("output_path is required: " + err.Error()), nil
				}
				allowed, err := requestDocumentWritePermission(
					ctx, permissions, call, workingDir,
					PDFDocumentToolName, action, outPath, "",
					fmt.Sprintf("Create PDF document %s", outPath),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				opts := pdf.CreateOptions{
					OutputPath:  outPath,
					WorkingDir:  workingDir,
					Title:       params.Title,
					Subtitle:    params.Subtitle,
					Author:      params.Author,
					Subject:     params.Subject,
					PageSize:    params.PageSize,
					Orientation: params.Orientation,
					Header:      params.Header,
					Footer:      params.Footer,
					PageNumbers: params.PageNumbers,
					Blocks:      params.Blocks,
				}
				if params.Margins != nil {
					opts.Margins = *params.Margins
				}
				res, err := pdf.Create(opts)
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to create PDF: " + err.Error()), nil
				}
				fsext.InvalidateIndex(res.Path)
				return documentJSONResponse(pdfWriteToolResult{
					Action:    action,
					Path:      res.Path,
					SizeBytes: res.SizeBytes,
					PageCount: res.PageCount,
					Summary:   fmt.Sprintf("Created PDF document (%d page(s), %d bytes)", res.PageCount, res.SizeBytes),
					Note:      documentCitePathNote,
				})

			case "inspect":
				rawIn := firstTrimmed(params.Path, params.InputPath)
				inPath, err := docutil.ResolvePath(workingDir, rawIn)
				if err != nil {
					return fantasy.NewTextErrorResponse("path is required: " + err.Error()), nil
				}
				res, err := pdf.Inspect(inPath, params.PageRange)
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to inspect PDF: " + err.Error()), nil
				}
				return documentJSONResponse(res)

			case "merge":
				if len(params.InputPaths) == 0 {
					return fantasy.NewTextErrorResponse("input_paths is required for merge"), nil
				}
				rawOut := firstTrimmed(params.OutputPath, params.Path)
				outPath, err := docutil.ResolvePath(workingDir, rawOut)
				if err != nil {
					return fantasy.NewTextErrorResponse("output_path is required: " + err.Error()), nil
				}
				resolvedInputs := make([]string, 0, len(params.InputPaths))
				for i, rawIn := range params.InputPaths {
					resolved, resolveErr := docutil.ResolvePath(workingDir, rawIn)
					if resolveErr != nil {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid input_paths[%d]: %s", i, resolveErr.Error())), nil
					}
					resolvedInputs = append(resolvedInputs, resolved)
				}
				allowed, err := requestDocumentWritePermission(
					ctx, permissions, call, workingDir,
					PDFDocumentToolName, action, outPath, strings.Join(resolvedInputs, ", "),
					fmt.Sprintf("Merge %d PDF document(s) into %s", len(resolvedInputs), outPath),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				res, err := pdf.Merge(resolvedInputs, outPath)
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to merge PDFs: " + err.Error()), nil
				}
				fsext.InvalidateIndex(res.Path)
				return documentJSONResponse(pdfWriteToolResult{
					Action:    action,
					Path:      res.Path,
					SizeBytes: res.SizeBytes,
					PageCount: res.PageCount,
					Summary:   fmt.Sprintf("Merged %d PDF document(s) (%d page(s), %d bytes)", len(resolvedInputs), res.PageCount, res.SizeBytes),
					Note:      documentCitePathNote,
				})

			case "split":
				rawIn := firstTrimmed(params.InputPath, params.Path)
				inPath, err := docutil.ResolvePath(workingDir, rawIn)
				if err != nil {
					return fantasy.NewTextErrorResponse("input_path is required: " + err.Error()), nil
				}
				var outPath, outDir string
				if strings.TrimSpace(params.OutputPath) != "" {
					outPath, err = docutil.ResolvePath(workingDir, params.OutputPath)
					if err != nil {
						return fantasy.NewTextErrorResponse("invalid output_path: " + err.Error()), nil
					}
				}
				if strings.TrimSpace(params.OutputDir) != "" {
					outDir, err = docutil.ResolvePath(workingDir, params.OutputDir)
					if err != nil {
						return fantasy.NewTextErrorResponse("invalid output_dir: " + err.Error()), nil
					}
				}
				var permTarget string
				if strings.TrimSpace(params.PageRange) != "" {
					if outPath == "" && outDir == "" {
						return fantasy.NewTextErrorResponse("output_path or output_dir is required for split"), nil
					}
					permTarget = firstTrimmed(outPath, outDir)
				} else {
					if params.Span <= 0 {
						return fantasy.NewTextErrorResponse("span must be >= 1 when page_range is empty"), nil
					}
					if outDir == "" {
						return fantasy.NewTextErrorResponse("output_dir is required when splitting by span"), nil
					}
					outPath = ""
					permTarget = outDir
				}
				allowed, err := requestDocumentWritePermission(
					ctx, permissions, call, workingDir,
					PDFDocumentToolName, action, permTarget, inPath,
					fmt.Sprintf("Split PDF document %s into %s", inPath, permTarget),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				res, err := pdf.Split(inPath, params.PageRange, params.Span, outPath, outDir)
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to split PDF: " + err.Error()), nil
				}
				var totalBytes int64
				for _, f := range res.Files {
					totalBytes += f.SizeBytes
					fsext.InvalidateIndex(f.Path)
				}
				primaryPath := permTarget
				pageCount := 0
				if len(res.Files) == 1 {
					primaryPath = res.Files[0].Path
					pageCount = res.Files[0].PageCount
				}
				return documentJSONResponse(pdfWriteToolResult{
					Action:    action,
					Path:      primaryPath,
					SizeBytes: totalBytes,
					PageCount: pageCount,
					Files:     res.Files,
					Summary:   fmt.Sprintf("Split PDF into %d file(s) (%d bytes total)", len(res.Files), totalBytes),
					Note:      documentCitePathNote,
				})

			case "rotate":
				rawIn := firstTrimmed(params.InputPath, params.Path)
				inPath, err := docutil.ResolvePath(workingDir, rawIn)
				if err != nil {
					return fantasy.NewTextErrorResponse("input_path is required: " + err.Error()), nil
				}
				if params.Angle%90 != 0 {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("angle must be a multiple of 90, got %d", params.Angle)), nil
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
					PDFDocumentToolName, action, outPath, inPath,
					fmt.Sprintf("Rotate PDF document %s by %d degrees to %s", inPath, params.Angle, outPath),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				res, err := pdf.Rotate(inPath, params.PageRange, params.Angle, outPath)
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to rotate PDF: " + err.Error()), nil
				}
				fsext.InvalidateIndex(res.Path)
				return documentJSONResponse(pdfWriteToolResult{
					Action:    action,
					Path:      res.Path,
					SizeBytes: res.SizeBytes,
					PageCount: res.PageCount,
					Summary:   fmt.Sprintf("Rotated PDF document (%d page(s), %d bytes)", res.PageCount, res.SizeBytes),
					Note:      documentCitePathNote,
				})

			default:
				return fantasy.NewTextErrorResponse("action must be one of: create, inspect, merge, split, rotate"), nil
			}
		},
	)
}
