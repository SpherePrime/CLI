package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/documents/docx"
	"github.com/SpherePrime/CLI/internal/documents/docutil"
	"github.com/SpherePrime/CLI/internal/fsext"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const WordDocumentToolName = "word_document"

//go:embed word_document.md
var wordDocumentDescription string

type WordDocumentParams struct {
	Action       string             `json:"action" enum:"create,inspect,update,fill_template" description:"Operation to perform: create, inspect, update, or fill_template"`
	Path         string             `json:"path,omitempty" description:"Target file path (used by inspect, or fallback for input_path/output_path)"`
	InputPath    string             `json:"input_path,omitempty" description:"Input DOCX file path for inspect, update, or fill_template"`
	OutputPath   string             `json:"output_path,omitempty" description:"Output DOCX file path for create, update (defaults to input_path in-place), or fill_template"`
	Title        string             `json:"title,omitempty" description:"Document title for create"`
	Subtitle     string             `json:"subtitle,omitempty" description:"Document subtitle for create"`
	Author       string             `json:"author,omitempty" description:"Document author metadata for create"`
	Subject      string             `json:"subject,omitempty" description:"Document subject metadata for create"`
	Header       string             `json:"header,omitempty" description:"Running header text for create"`
	Footer       string             `json:"footer,omitempty" description:"Running footer text for create"`
	PageNumbers  bool               `json:"page_numbers,omitempty" description:"Render page numbers in footer for create"`
	Blocks       []docx.Block       `json:"blocks,omitempty" description:"Content blocks for create"`
	Replacements map[string]string  `json:"replacements,omitempty" description:"Exact text replacements for update"`
	Values       map[string]string  `json:"values,omitempty" description:"Placeholder values for fill_template"`
	AppendBlocks []docx.Block       `json:"append_blocks,omitempty" description:"Blocks to append during update"`
	TableUpdates []docx.TableUpdate `json:"table_updates,omitempty" description:"Zero-based table cell updates during update"`
}

type wordWriteToolResult struct {
	Action              string `json:"action"`
	Path                string `json:"path"`
	SizeBytes           int64  `json:"size_bytes"`
	ParagraphCount      int    `json:"paragraph_count,omitempty"`
	TableCount          int    `json:"table_count,omitempty"`
	ReplacementsApplied int    `json:"replacements_applied,omitempty"`
	Summary             string `json:"summary"`
	Note                string `json:"note"`
}

func NewWordDocumentTool(permissions permission.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		WordDocumentToolName,
		wordDocumentDescription,
		func(ctx context.Context, params WordDocumentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
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
					WordDocumentToolName, action, outPath, "",
					fmt.Sprintf("Create Word document %s", outPath),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				res, err := docx.Create(docx.CreateOptions{
					OutputPath:  outPath,
					WorkingDir:  workingDir,
					Title:       params.Title,
					Subtitle:    params.Subtitle,
					Author:      params.Author,
					Subject:     params.Subject,
					Header:      params.Header,
					Footer:      params.Footer,
					PageNumbers: params.PageNumbers,
					Blocks:      params.Blocks,
				})
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to create Word document: " + err.Error()), nil
				}
				fsext.InvalidateIndex(res.Path)
				return documentJSONResponse(wordWriteToolResult{
					Action:         action,
					Path:           res.Path,
					SizeBytes:      res.SizeBytes,
					ParagraphCount: res.ParagraphCount,
					TableCount:     res.TableCount,
					Summary:        fmt.Sprintf("Created Word document (%d paragraph(s), %d table(s), %d bytes)", res.ParagraphCount, res.TableCount, res.SizeBytes),
					Note:           documentCitePathNote,
				})

			case "inspect":
				rawIn := firstTrimmed(params.Path, params.InputPath)
				inPath, err := docutil.ResolvePath(workingDir, rawIn)
				if err != nil {
					return fantasy.NewTextErrorResponse("path is required: " + err.Error()), nil
				}
				res, err := docx.Inspect(inPath)
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to inspect Word document: " + err.Error()), nil
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
					WordDocumentToolName, action, outPath, inPath,
					fmt.Sprintf("Update Word document %s -> %s", inPath, outPath),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				res, err := docx.Update(docx.UpdateOptions{
					InputPath:    inPath,
					OutputPath:   outPath,
					WorkingDir:   workingDir,
					Replacements: params.Replacements,
					AppendBlocks: params.AppendBlocks,
					TableUpdates: params.TableUpdates,
				})
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to update Word document: " + err.Error()), nil
				}
				fsext.InvalidateIndex(res.Path)
				return documentJSONResponse(wordWriteToolResult{
					Action:              action,
					Path:                res.Path,
					SizeBytes:           res.SizeBytes,
					ParagraphCount:      res.ParagraphCount,
					TableCount:          res.TableCount,
					ReplacementsApplied: res.ReplacementsApplied,
					Summary:             fmt.Sprintf("Updated Word document (%d replacement(s), %d bytes)", res.ReplacementsApplied, res.SizeBytes),
					Note:                documentCitePathNote,
				})

			case "fill_template":
				rawIn := firstTrimmed(params.InputPath, params.Path)
				inPath, err := docutil.ResolvePath(workingDir, rawIn)
				if err != nil {
					return fantasy.NewTextErrorResponse("input_path is required: " + err.Error()), nil
				}
				outPath, err := docutil.ResolvePath(workingDir, params.OutputPath)
				if err != nil {
					return fantasy.NewTextErrorResponse("output_path is required for fill_template: " + err.Error()), nil
				}
				values := params.Values
				if len(values) == 0 {
					values = params.Replacements
				}
				allowed, err := requestDocumentWritePermission(
					ctx, permissions, call, workingDir,
					WordDocumentToolName, action, outPath, inPath,
					fmt.Sprintf("Fill Word template %s -> %s", inPath, outPath),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				res, err := docx.FillTemplate(inPath, outPath, values)
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to fill Word template: " + err.Error()), nil
				}
				fsext.InvalidateIndex(res.Path)
				return documentJSONResponse(wordWriteToolResult{
					Action:              action,
					Path:                res.Path,
					SizeBytes:           res.SizeBytes,
					ParagraphCount:      res.ParagraphCount,
					TableCount:          res.TableCount,
					ReplacementsApplied: res.ReplacementsApplied,
					Summary:             fmt.Sprintf("Filled Word template (%d replacement(s), %d bytes)", res.ReplacementsApplied, res.SizeBytes),
					Note:                documentCitePathNote,
				})

			default:
				return fantasy.NewTextErrorResponse("action must be one of: create, inspect, update, fill_template"), nil
			}
		},
	)
}
