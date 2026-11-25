package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/documents/docutil"
	"github.com/SpherePrime/CLI/internal/documents/pptx"
	"github.com/SpherePrime/CLI/internal/fsext"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const PowerPointPresentationToolName = "powerpoint_presentation"

//go:embed powerpoint_presentation.md
var powerPointPresentationDescription string

type PowerPointPresentationParams struct {
	Action          string             `json:"action" enum:"create,inspect,update,fill_template" description:"Operation to perform: create, inspect, update, or fill_template"`
	Path            string             `json:"path,omitempty" description:"Target file path (used by inspect, or fallback for input_path/output_path)"`
	InputPath       string             `json:"input_path,omitempty" description:"Input PPTX file path for inspect, update, or fill_template"`
	OutputPath      string             `json:"output_path,omitempty" description:"Output PPTX file path for create, update (defaults to input_path in-place), or fill_template"`
	Title           string             `json:"title,omitempty" description:"Presentation title metadata for create"`
	Author          string             `json:"author,omitempty" description:"Presentation author metadata for create"`
	Subject         string             `json:"subject,omitempty" description:"Presentation subject metadata for create"`
	AspectRatio     string             `json:"aspect_ratio,omitempty" description:"Slide aspect ratio for create: 16:9 or 4:3"`
	PrimaryColor    string             `json:"primary_color,omitempty" description:"Primary theme hex color"`
	AccentColor     string             `json:"accent_color,omitempty" description:"Accent theme hex color"`
	BackgroundColor string             `json:"background_color,omitempty" description:"Slide background hex color"`
	TextColor       string             `json:"text_color,omitempty" description:"Primary text hex color"`
	FontFamily      string             `json:"font_family,omitempty" description:"Presentation font family for create"`
	SlideNumbers    bool               `json:"slide_numbers,omitempty" description:"Render slide numbers on slides"`
	Slides          []pptx.SlideSpec   `json:"slides,omitempty" description:"Slides to create"`
	Replacements    map[string]string  `json:"replacements,omitempty" description:"Exact text replacements for update"`
	Values          map[string]string  `json:"values,omitempty" description:"Placeholder values for fill_template"`
	NotesUpdates    []pptx.NotesUpdate `json:"notes_updates,omitempty" description:"Speaker notes updates by 1-based slide number for update"`
	AppendSlides    []pptx.SlideSpec   `json:"append_slides,omitempty" description:"Slides to append during update"`
}

type pptxWriteToolResult struct {
	Action              string `json:"action"`
	Path                string `json:"path"`
	SizeBytes           int64  `json:"size_bytes"`
	SlideCount          int    `json:"slide_count"`
	ReplacementsApplied int    `json:"replacements_applied,omitempty"`
	Summary             string `json:"summary"`
	Note                string `json:"note"`
}

func NewPowerPointPresentationTool(permissions permission.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		PowerPointPresentationToolName,
		powerPointPresentationDescription,
		func(ctx context.Context, params PowerPointPresentationParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
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
					PowerPointPresentationToolName, action, outPath, "",
					fmt.Sprintf("Create PowerPoint presentation %s", outPath),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				res, err := pptx.Create(pptx.CreateOptions{
					OutputPath:      outPath,
					WorkingDir:      workingDir,
					Title:           params.Title,
					Author:          params.Author,
					Subject:         params.Subject,
					AspectRatio:     params.AspectRatio,
					PrimaryColor:    params.PrimaryColor,
					AccentColor:     params.AccentColor,
					BackgroundColor: params.BackgroundColor,
					TextColor:       params.TextColor,
					FontFamily:      params.FontFamily,
					SlideNumbers:    params.SlideNumbers,
					Slides:          params.Slides,
				})
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to create PowerPoint presentation: " + err.Error()), nil
				}
				fsext.InvalidateIndex(res.Path)
				return documentJSONResponse(pptxWriteToolResult{
					Action:     action,
					Path:       res.Path,
					SizeBytes:  res.SizeBytes,
					SlideCount: res.SlideCount,
					Summary:    fmt.Sprintf("Created PowerPoint presentation (%d slide(s), %d bytes)", res.SlideCount, res.SizeBytes),
					Note:       documentCitePathNote,
				})

			case "inspect":
				rawIn := firstTrimmed(params.Path, params.InputPath)
				inPath, err := docutil.ResolvePath(workingDir, rawIn)
				if err != nil {
					return fantasy.NewTextErrorResponse("path is required: " + err.Error()), nil
				}
				res, err := pptx.Inspect(inPath)
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to inspect PowerPoint presentation: " + err.Error()), nil
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
					PowerPointPresentationToolName, action, outPath, inPath,
					fmt.Sprintf("Update PowerPoint presentation %s -> %s", inPath, outPath),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				res, err := pptx.Update(pptx.UpdateOptions{
					InputPath:       inPath,
					OutputPath:      outPath,
					WorkingDir:      workingDir,
					Replacements:    params.Replacements,
					NotesUpdates:    params.NotesUpdates,
					AppendSlides:    params.AppendSlides,
					SlideNumbers:    params.SlideNumbers,
					PrimaryColor:    params.PrimaryColor,
					AccentColor:     params.AccentColor,
					BackgroundColor: params.BackgroundColor,
					TextColor:       params.TextColor,
				})
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to update PowerPoint presentation: " + err.Error()), nil
				}
				fsext.InvalidateIndex(res.Path)
				return documentJSONResponse(pptxWriteToolResult{
					Action:              action,
					Path:                res.Path,
					SizeBytes:           res.SizeBytes,
					SlideCount:          res.SlideCount,
					ReplacementsApplied: res.ReplacementsApplied,
					Summary:             fmt.Sprintf("Updated PowerPoint presentation (%d slide(s), %d replacement(s), %d bytes)", res.SlideCount, res.ReplacementsApplied, res.SizeBytes),
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
					PowerPointPresentationToolName, action, outPath, inPath,
					fmt.Sprintf("Fill PowerPoint template %s -> %s", inPath, outPath),
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
				res, err := pptx.FillTemplate(inPath, outPath, values)
				if err != nil {
					return fantasy.NewTextErrorResponse("failed to fill PowerPoint template: " + err.Error()), nil
				}
				fsext.InvalidateIndex(res.Path)
				return documentJSONResponse(pptxWriteToolResult{
					Action:              action,
					Path:                res.Path,
					SizeBytes:           res.SizeBytes,
					SlideCount:          res.SlideCount,
					ReplacementsApplied: res.ReplacementsApplied,
					Summary:             fmt.Sprintf("Filled PowerPoint template (%d slide(s), %d replacement(s), %d bytes)", res.SlideCount, res.ReplacementsApplied, res.SizeBytes),
					Note:                documentCitePathNote,
				})

			default:
				return fantasy.NewTextErrorResponse("action must be one of: create, inspect, update, fill_template"), nil
			}
		},
	)
}
