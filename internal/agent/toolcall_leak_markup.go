package agent

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Models that were trained on another agent framework sometimes write their
// tool calls into the answer text instead of using the provider's structured
// tool-call channel. The shapes seen in the wild are the DeepSeek DSML
// wrapper, a plain XML wrapper, and Claude-style antml tags; all of them wrap
// one or more <invoke name="..."> elements whose arguments are
// <parameter name="..."> children.
var (
	// leakDsmlTagRe strips the DSML decoration ("<" + fullwidth bar + "DSML"
	// + bar + optional space) from a tag, leaving a plain XML tag.
	leakDsmlTagRe = regexp.MustCompile(`<(/)?[｜|]*DSML[｜|]*\s*`)
	// leakAntmlRe strips the antml namespace prefix from a tag.
	leakAntmlRe = regexp.MustCompile(`(?i)antml:`)

	// leakOpenRe matches a block-opening tag at the start of a candidate.
	leakOpenRe = regexp.MustCompile(`(?i)^<[｜|]*(?:DSML[｜|]*\s*)?(?:antml:)?(tool_calls|function_calls|invoke)\b[^>]*>`)
	// leakCloseRe matches any block-closing tag.
	leakCloseRe = regexp.MustCompile(`(?i)</\s*[｜|]*(?:DSML[｜|]*\s*)?(?:antml:)?(tool_calls|function_calls|invoke)\s*>`)

	leakInvokeOpenRe  = regexp.MustCompile(`(?is)<invoke\s+name\s*=\s*"([^"]*)"\s*>`)
	leakInvokeCloseRe = regexp.MustCompile(`(?is)</\s*invoke\s*>`)
	leakParameterRe   = regexp.MustCompile(`(?is)<parameter\s+name\s*=\s*"([^"]*)"(?:\s+string\s*=\s*"(true|false)")?\s*>(.*?)</\s*parameter\s*>`)
)

// maxLeakMarkerLen bounds how much trailing text is held back while waiting
// for the rest of a possible marker. Every marker is far shorter than this.
const maxLeakMarkerLen = 64

// leakBlockStart finds the first leaked tool-call block that starts a line and
// is not inside a fenced code block, which is how a model that answers in
// prose writes a call. A quoted example inside a fence is left alone.
func leakBlockStart(text string, fenced, atLineStart bool) (int, string, bool) {
	inFence := fenced
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "```") {
			inFence = !inFence
			i += 3
			continue
		}
		lineStart := atLineStart
		if i > 0 {
			lineStart = text[i-1] == '\n'
		}
		if !inFence && lineStart {
			j := i
			for j < len(text) && (text[j] == ' ' || text[j] == '\t' || text[j] == '\r') {
				j++
			}
			if match := leakOpenRe.FindStringSubmatch(text[j:]); match != nil {
				return j, strings.ToLower(match[1]), true
			}
		}
		i++
	}
	return 0, "", false
}

// leakBlockEnd returns the offset just past the closing tag of the block that
// started with tag.
func leakBlockEnd(text, tag string) (int, bool) {
	for _, loc := range leakCloseRe.FindAllStringSubmatchIndex(text, -1) {
		if strings.EqualFold(text[loc[2]:loc[3]], tag) {
			return loc[1], true
		}
	}
	return 0, false
}

// leakMarkerTail returns the length of the trailing text that could still
// grow into a marker, so it is held back instead of being shown.
func leakMarkerTail(text string) int {
	index := strings.LastIndexByte(text, '<')
	if index < 0 {
		return 0
	}
	tail := text[index:]
	if len(tail) > maxLeakMarkerLen || strings.ContainsAny(tail, ">\n") {
		return 0
	}
	return len(tail)
}

// fenceStateAfter reports the code-fence state once text has been emitted.
func fenceStateAfter(text string, fenced bool) bool {
	for {
		index := strings.Index(text, "```")
		if index < 0 {
			return fenced
		}
		fenced = !fenced
		text = text[index+3:]
	}
}

// parseLeakedToolCalls extracts the tool calls encoded in a leaked block.
// Only complete invoke elements count, so a truncated block yields nothing
// rather than a half-read call.
func parseLeakedToolCalls(block string) []recoveredToolCall {
	rest := normalizeLeakMarkup(block)
	var calls []recoveredToolCall
	for {
		open := leakInvokeOpenRe.FindStringSubmatchIndex(rest)
		if open == nil {
			return calls
		}
		name := strings.TrimSpace(rest[open[2]:open[3]])
		body := rest[open[1]:]
		close := leakInvokeCloseRe.FindStringIndex(body)
		if close == nil {
			return calls
		}
		body = body[:close[0]]
		rest = rest[open[1]+close[1]:]
		if name == "" {
			continue
		}
		calls = append(calls, recoveredToolCall{name: name, input: parseLeakParameters(body)})
	}
}

// normalizeLeakMarkup removes the provider-specific decoration from the tags
// of a leaked block so the element names can be matched directly.
func normalizeLeakMarkup(block string) string {
	block = leakDsmlTagRe.ReplaceAllString(block, "<$1")
	return leakAntmlRe.ReplaceAllString(block, "")
}

// parseLeakParameters renders the parameter elements of an invoke as the JSON
// object the tool expects. A parameter marked string="false" carries raw JSON
// and is decoded; everything else is passed through as a string.
func parseLeakParameters(body string) string {
	parameters := make(map[string]any)
	for _, match := range leakParameterRe.FindAllStringSubmatch(body, -1) {
		name := strings.TrimSpace(match[1])
		if name == "" {
			continue
		}
		raw := match[3]
		if match[2] == "false" {
			var decoded any
			if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &decoded); err == nil {
				parameters[name] = decoded
				continue
			}
		}
		parameters[name] = raw
	}
	if len(parameters) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(parameters)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}
