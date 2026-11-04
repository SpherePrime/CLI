package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const toolDocumentationPageBytes = 32768

type toolDocument struct {
	Name     string
	Identity string
	Revision string
	Text     string
	Complete bool
}

type toolDocumentationContext struct {
	Scope     string
	Documents map[string]toolDocument
}

type toolDocumentationContextKey struct{}

func withToolDocumentation(ctx context.Context, scope string, documents map[string]toolDocument) context.Context {
	return context.WithValue(ctx, toolDocumentationContextKey{}, toolDocumentationContext{Scope: scope, Documents: documents})
}

func toolDocumentationFromContext(ctx context.Context) toolDocumentationContext {
	value, _ := ctx.Value(toolDocumentationContextKey{}).(toolDocumentationContext)
	return value
}

func buildToolDocuments(scope string, tools []fantasy.AgentTool) (map[string]toolDocument, error) {
	documents := make(map[string]toolDocument, len(tools))
	for _, tool := range tools {
		info := tool.Info()
		if inlineDocumentedTool(info.Name) {
			continue
		}
		schema, err := canonicalToolSchema(info)
		if err != nil {
			return nil, err
		}
		encoded, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return nil, err
		}
		_, complete, err := compactToolDefinition(tool)
		if err != nil {
			return nil, err
		}
		text := fmt.Sprintf("Tool: %s\n\nDescription:\n%s\n\nInput schema (JSON Schema):\n%s\n", info.Name, info.Description, encoded)
		documents[info.Name] = toolDocument{
			Name:     info.Name,
			Identity: info.Name,
			Revision: documentRevision(scope, text),
			Text:     text,
			Complete: complete,
		}
	}
	return documents, nil
}

func documentRevision(scope, text string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + text))
	return hex.EncodeToString(sum[:16])
}

func pageHash(page string) string {
	sum := sha256.Sum256([]byte(page))
	return hex.EncodeToString(sum[:])
}

type documentPage struct {
	start int
	end   int
}

func toolCallNames(messages []fantasy.Message) map[string]string {
	names := map[string]string{}
	for _, message := range messages {
		for _, part := range message.Content {
			call, ok := part.(fantasy.ToolCallPart)
			if !ok {
				continue
			}
			names[call.ToolCallID] = call.ToolName
		}
	}
	return names
}

func documentedTools(scope string, documents map[string]toolDocument, messages []fantasy.Message) map[string]string {
	callNames := toolCallNames(messages)
	pages := map[string][]documentPage{}
	for _, message := range messages {
		for _, part := range message.Content {
			result, ok := part.(fantasy.ToolResultPart)
			if !ok || result.Documentation == nil {
				continue
			}
			if callNames[result.ToolCallID] != readToolDocumentationName {
				continue
			}
			receipt := result.Documentation
			document, ok := documents[receipt.ToolName]
			if !ok || receipt.Scope != scope || receipt.Revision != document.Revision {
				continue
			}
			if receipt.Offset < 0 || receipt.End <= receipt.Offset || receipt.End > receipt.Total || receipt.Total != len(document.Text) {
				continue
			}
			if receipt.PageSHA256 != pageHash(document.Text[receipt.Offset:receipt.End]) {
				continue
			}
			pages[receipt.ToolName] = append(pages[receipt.ToolName], documentPage{start: receipt.Offset, end: receipt.End})
		}
	}
	covered := map[string]string{}
	for name, document := range documents {
		if document.Complete || coversDocument(pages[name], len(document.Text)) {
			covered[name] = document.Revision
		}
	}
	return covered
}

func coversDocument(pages []documentPage, total int) bool {
	if total == 0 {
		return true
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].start < pages[j].start })
	covered := 0
	for _, page := range pages {
		if page.start > covered {
			return false
		}
		if page.end > covered {
			covered = page.end
		}
	}
	return covered >= total
}

func documentationGate(documents map[string]toolDocument, covered map[string]string) func(context.Context, fantasy.ToolCall) error {
	return func(_ context.Context, call fantasy.ToolCall) error {
		if inlineDocumentedTool(call.Name) {
			return nil
		}
		document, ok := documents[call.Name]
		if !ok {
			return fmt.Errorf("tool %q is not available in this request", call.Name)
		}
		if covered[call.Name] == document.Revision {
			return nil
		}
		return fmt.Errorf("tool_documentation_required: read the complete documentation of %q first by calling %s with tool_name=%q, then call %q in a later step", call.Name, readToolDocumentationName, call.Name, call.Name)
	}
}
