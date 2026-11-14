package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	projectionInlineSuccessBytes = 8192
	projectionInlineFailureBytes = 65536
	projectionEdgeBytes          = 16384
)

type toolArtifactsContext struct {
	Store toolArtifactStore
	Scope string
}

type toolArtifactsContextKey struct{}

func withToolArtifacts(ctx context.Context, store toolArtifactStore, scope string) context.Context {
	return context.WithValue(ctx, toolArtifactsContextKey{}, toolArtifactsContext{Store: store, Scope: scope})
}

func toolArtifactsFromContext(ctx context.Context) toolArtifactsContext {
	value, _ := ctx.Value(toolArtifactsContextKey{}).(toolArtifactsContext)
	return value
}

func (a *sessionAgent) toolArtifactStore() (toolArtifactStore, error) {
	a.artifactStoreOnce.Do(func() {
		a.artifactStore, a.artifactStoreErr = newToolArtifactStore(a.dataDir)
	})
	return a.artifactStore, a.artifactStoreErr
}

func (a *sessionAgent) toolScope(sessionID string) string {
	return a.dataDir + "|" + sessionID
}

func projectToolMessages(ctx context.Context, scope string, store toolArtifactStore, messages []fantasy.Message) []fantasy.Message {
	calls := agedCalls(messages)
	projected := ageToolHistory(ctx, scope, store, cloneFantasyMessages(messages), defaultHistoryAgingPolicy())
	for i := range projected {
		for j, part := range projected[i].Content {
			result, ok := part.(fantasy.ToolResultPart)
			if !ok {
				continue
			}
			call := calls[result.ToolCallID]
			if inlineDocumentedTool(call.Name) || isProtectedSkillView(call.Name, call.Input) {
				continue
			}
			name := call.Name
			switch output := result.Output.(type) {
			case fantasy.ToolResultOutputContentText:
				compacted := compactToolOutput(name, output.Text)
				rewritten, changed := projectToolResultText(ctx, scope, store, result.ToolCallID, result.Outcome, compacted)
				if !changed {
					rewritten, changed = compacted, compacted != output.Text
				}
				if !changed {
					continue
				}
				result.Output = fantasy.ToolResultOutputContentText{Text: rewritten}
			case fantasy.ToolResultOutputContentError:
				if output.Error == nil {
					continue
				}
				compacted := compactToolOutput(name, output.Error.Error())
				rewritten, changed := projectToolResultText(ctx, scope, store, result.ToolCallID, fantasy.ToolOutcomeFailure, compacted)
				if !changed {
					rewritten, changed = compacted, compacted != output.Error.Error()
				}
				if !changed {
					continue
				}
				result.Output = fantasy.ToolResultOutputContentError{Error: errors.New(rewritten)}
			default:
				continue
			}
			projected[i].Content[j] = result
		}
	}
	return projected
}

func projectToolResultText(ctx context.Context, scope string, store toolArtifactStore, callID string, outcome fantasy.ToolOutcome, text string) (string, bool) {
	failed := outcome == fantasy.ToolOutcomeFailure || outcome == fantasy.ToolOutcomeUnknown || outcome == ""
	limit := projectionInlineSuccessBytes
	if failed {
		limit = projectionInlineFailureBytes
	}
	if len(text) <= limit || store == nil {
		return text, false
	}
	label := "success"
	if failed {
		label = "failure"
	}
	id, err := store.Put(ctx, scope, callID, outcome, []byte(text))
	if err != nil {
		return text, false
	}
	if len(text) <= 2*projectionEdgeBytes {
		return fmt.Sprintf("%s result shortened: %d bytes stored as %s. Call %s with artifact_id=%q to read it.", label, len(text), id, readToolResultName, id), true
	}
	head := utf8SafePrefix(text, projectionEdgeBytes)
	tail := utf8SafeSuffix(text, projectionEdgeBytes)
	var builder strings.Builder
	fmt.Fprintf(&builder, "%s result shortened: %d bytes stored as %s.\n\nFirst %d bytes:\n%s\n\nLast %d bytes:\n%s\n\nCall %s with artifact_id=%q to read the complete output.",
		label, len(text), id, len(head), head, len(tail), tail, readToolResultName, id)
	return builder.String(), true
}

func utf8SafePrefix(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	end := limit
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end]
}

func utf8SafeSuffix(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	start := len(text) - limit
	for start < len(text) && !utf8.RuneStart(text[start]) {
		start++
	}
	return text[start:]
}
