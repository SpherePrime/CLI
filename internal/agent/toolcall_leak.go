package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

// toolCallLeakModel wraps a [fantasy.LanguageModel] so tool calls that a
// provider streams as plain assistant text are turned back into real tool
// calls.
//
// Some OpenAI-compatible endpoints serving DeepSeek-family models send the
// model's tool-call markup as ordinary content instead of structured
// tool_calls. The markup then reaches the chat verbatim and the turn ends as
// if the model had finished, so the task stops half-done. This wrapper keeps
// the markup out of the visible text and re-emits it as a tool-call turn so
// the agent keeps working.
type toolCallLeakModel struct {
	fantasy.LanguageModel
}

// newToolCallLeakModel returns m wrapped so leaked tool-call markup in the
// streamed text becomes tool calls. A nil model is returned unchanged.
func newToolCallLeakModel(m fantasy.LanguageModel) fantasy.LanguageModel {
	if m == nil {
		return m
	}
	return toolCallLeakModel{LanguageModel: m}
}

// Stream implements [fantasy.LanguageModel].
func (m toolCallLeakModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	inner, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		return nil, err
	}
	knownTools := make(map[string]struct{}, len(call.Tools))
	for _, tool := range call.Tools {
		knownTools[normalizeToolName(tool.GetName())] = struct{}{}
	}
	return func(yield func(fantasy.StreamPart) bool) {
		filter := &toolCallLeakFilter{yield: yield, knownTools: knownTools}
		inner(filter.push)
		filter.flush()
	}, nil
}

// recoveredToolCall is a tool call parsed out of leaked assistant text.
type recoveredToolCall struct {
	name  string
	input string
}

// toolCallLeakFilter rewrites a stream so tool-call markup that arrives as
// text is held back, parsed, and re-emitted as tool-call stream parts.
//
// Text is emitted as soon as it cannot be the start of a marker, so normal
// answers still stream token by token; only a suspected block is buffered
// until its end is known.
type toolCallLeakFilter struct {
	yield      func(fantasy.StreamPart) bool
	knownTools map[string]struct{}

	buf         strings.Builder
	textID      string
	atLineStart bool
	fenced      bool
	capturing   bool
	closeTag    string
	recovered   []recoveredToolCall
	nextCallID  int
	stopped     bool
}

func (f *toolCallLeakFilter) push(part fantasy.StreamPart) bool {
	if f.stopped {
		return false
	}
	switch part.Type {
	case fantasy.StreamPartTypeTextStart:
		f.textID = part.ID
		f.atLineStart = true
		return f.emit(part)
	case fantasy.StreamPartTypeTextDelta:
		f.buf.WriteString(part.Delta)
		return f.drain(false)
	case fantasy.StreamPartTypeTextEnd:
		if !f.drain(true) {
			return false
		}
		f.textID = ""
		return f.emit(part)
	case fantasy.StreamPartTypeFinish:
		if !f.drain(true) {
			return false
		}
		recovered := len(f.recovered) > 0
		if !f.emitRecovered() {
			return false
		}
		if recovered && leakFinishReasonRewritable(part.FinishReason) {
			part.FinishReason = fantasy.FinishReasonToolCalls
		}
		return f.emit(part)
	default:
		return f.emit(part)
	}
}

// flush resolves anything still buffered once the provider stream has ended.
func (f *toolCallLeakFilter) flush() {
	if f.stopped {
		return
	}
	f.drain(true)
	f.emitRecovered()
}

func (f *toolCallLeakFilter) emit(part fantasy.StreamPart) bool {
	if !f.yield(part) {
		f.stopped = true
		return false
	}
	return true
}

// drain emits every buffered byte that is safe to emit. A false return means
// the consumer stopped reading.
func (f *toolCallLeakFilter) drain(final bool) bool {
	for {
		text := f.buf.String()
		if f.capturing {
			if end, ok := leakBlockEnd(text, f.closeTag); ok {
				block, rest := text[:end], text[end:]
				f.buf.Reset()
				f.buf.WriteString(rest)
				f.capturing = false
				if !f.parse(block) && !f.emitText(block) {
					return false
				}
				continue
			}
			if !final {
				return true
			}
			f.buf.Reset()
			f.capturing = false
			if !f.parse(text) && !f.emitText(text) {
				return false
			}
			continue
		}
		if start, tag, ok := leakBlockStart(text, f.fenced, f.atLineStart); ok {
			head := text[:start]
			f.buf.Reset()
			f.buf.WriteString(text[start:])
			f.closeTag = tag
			if !f.emitText(head) {
				return false
			}
			f.capturing = true
			continue
		}
		keep := 0
		if !final {
			keep = leakMarkerTail(text)
		}
		cut := len(text) - keep
		emit, rest := text[:cut], text[cut:]
		f.buf.Reset()
		f.buf.WriteString(rest)
		return f.emitText(emit)
	}
}

func (f *toolCallLeakFilter) emitText(text string) bool {
	if text == "" {
		return true
	}
	f.fenced = fenceStateAfter(text, f.fenced)
	f.atLineStart = strings.HasSuffix(text, "\n")
	return f.emit(fantasy.StreamPart{
		Type:  fantasy.StreamPartTypeTextDelta,
		ID:    f.textID,
		Delta: text,
	})
}

// parse turns a captured block into tool calls. It reports whether the block
// really was a leaked tool call; when it was not, the caller shows the text.
func (f *toolCallLeakFilter) parse(block string) bool {
	calls := parseLeakedToolCalls(block)
	if len(calls) == 0 {
		return false
	}
	if len(f.knownTools) > 0 {
		known := false
		for _, call := range calls {
			if _, ok := f.knownTools[normalizeToolName(call.name)]; ok {
				known = true
				break
			}
		}
		if !known {
			return false
		}
	}
	f.recovered = append(f.recovered, calls...)
	return true
}

// emitRecovered replays the parsed calls as a normal tool-call turn.
func (f *toolCallLeakFilter) emitRecovered() bool {
	for _, call := range f.recovered {
		f.nextCallID++
		id := fmt.Sprintf("leaked-call-%d", f.nextCallID)
		parts := []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeToolInputStart, ID: id, ToolCallName: call.name},
			{Type: fantasy.StreamPartTypeToolInputDelta, ID: id, Delta: call.input},
			{Type: fantasy.StreamPartTypeToolInputEnd, ID: id},
			{Type: fantasy.StreamPartTypeToolCall, ID: id, ToolCallName: call.name, ToolCallInput: call.input},
		}
		for _, part := range parts {
			if !f.emit(part) {
				return false
			}
		}
	}
	f.recovered = nil
	return true
}

// leakFinishReasonRewritable reports whether a finish reason may be turned
// into a tool-call turn. Reasons that can cut output mid-call keep their
// meaning: their arguments may be truncated, so they must never be dispatched.
func leakFinishReasonRewritable(reason fantasy.FinishReason) bool {
	switch reason {
	case fantasy.FinishReasonToolCalls,
		fantasy.FinishReasonLength,
		fantasy.FinishReasonContentFilter,
		fantasy.FinishReasonError:
		return false
	}
	return true
}
