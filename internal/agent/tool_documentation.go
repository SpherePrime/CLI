package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const toolDocumentationPageBytes = 32768

type toolDocument struct {
	Name     string
	Identity string
	Revision string
	Text     string
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
		text := fmt.Sprintf("Tool: %s\n\nDescription:\n%s\n\nInput schema (JSON Schema):\n%s\n", info.Name, info.Description, encoded)
		documents[info.Name] = toolDocument{
			Name:     info.Name,
			Identity: info.Name,
			Revision: documentRevision(scope, text),
			Text:     text,
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
