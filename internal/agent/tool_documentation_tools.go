package agent

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

type readToolDocumentationParams struct {
	ToolName string `json:"tool_name" description:"Exact name of the tool whose full documentation to read"`
	Offset   int    `json:"offset,omitempty" description:"Optional byte offset to continue reading a long document"`
}

type readToolResultParams struct {
	ArtifactID string `json:"artifact_id" description:"Identifier of the stored tool result to read"`
	Offset     int    `json:"offset,omitempty" description:"Optional byte offset to continue reading"`
	Limit      int    `json:"limit,omitempty" description:"Optional maximum number of bytes to return"`
}

func newReadToolDocumentationTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(readToolDocumentationName,
		"Read the complete documentation and full input schema of one tool before using it. Every other tool must be documented this way once before its first execution.",
		func(ctx context.Context, params readToolDocumentationParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			documentation := toolDocumentationFromContext(ctx)
			if len(documentation.Documents) == 0 {
				return fantasy.NewTextErrorResponse("Tool documentation is unavailable in this request."), nil
			}
			document, ok := documentation.Documents[params.ToolName]
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("No tool named %q is available in this request.", params.ToolName)), nil
			}
			offset := params.Offset
			if offset < 0 || offset > len(document.Text) {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Offset %d is outside the documentation of %q, which is %d bytes.", offset, document.Name, len(document.Text))), nil
			}
			if offset < len(document.Text) && !utf8.RuneStart(document.Text[offset]) {
				return fantasy.NewTextErrorResponse("Offset is not on a UTF-8 character boundary."), nil
			}
			end := min(offset+toolDocumentationPageBytes, len(document.Text))
			for end > offset && end < len(document.Text) && !utf8.RuneStart(document.Text[end]) {
				end--
			}
			page := document.Text[offset:end]
			var builder strings.Builder
			fmt.Fprintf(&builder, "Documentation for %s, revision %s, bytes %d-%d of %d.\n\n%s", document.Name, document.Revision, offset, end, len(document.Text), page)
			if end < len(document.Text) {
				fmt.Fprintf(&builder, "\n\nContinue with %s tool_name=%q offset=%d.", readToolDocumentationName, document.Name, end)
			} else {
				fmt.Fprintf(&builder, "\n\nComplete documentation for %q received. It can now be executed.", document.Name)
			}
			response := fantasy.NewTextResponse(builder.String())
			response.Outcome = fantasy.ToolOutcomeSuccess
			response.Documentation = &fantasy.ToolDocumentationReceipt{
				Version:    1,
				Scope:      documentation.Scope,
				ToolName:   document.Name,
				Identity:   document.Identity,
				Revision:   document.Revision,
				Offset:     offset,
				End:        end,
				Total:      len(document.Text),
				PageSHA256: pageHash(page),
			}
			return response, nil
		})
}

func newReadToolResultTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(readToolResultName,
		"Read a stored tool result that was shortened in the conversation. Use the artifact_id reported by the shortened result to read the complete output.",
		func(ctx context.Context, params readToolResultParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			artifacts := toolArtifactsFromContext(ctx)
			if artifacts.Store == nil {
				return fantasy.NewTextErrorResponse("Stored tool results are unavailable in this request."), nil
			}
			page, next, total, err := artifacts.Store.Read(ctx, artifacts.Scope, params.ArtifactID, params.Offset, params.Limit)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Could not read stored result %q: %v", params.ArtifactID, err)), nil
			}
			var builder strings.Builder
			fmt.Fprintf(&builder, "Stored result %s, bytes %d-%d of %d.\n\n%s", params.ArtifactID, params.Offset, next, total, string(page))
			if next < total {
				fmt.Fprintf(&builder, "\n\nContinue with %s artifact_id=%q offset=%d.", readToolResultName, params.ArtifactID, next)
			}
			response := fantasy.NewTextResponse(builder.String())
			response.Outcome = fantasy.ToolOutcomeSuccess
			return response, nil
		})
}
