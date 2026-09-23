package config

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/tidwall/pretty"
)

// Config files are updated in place with sjson, which emits compact JSON and
// only keeps the layout a file already had. That leaves freshly created
// sections as a single unreadable line. Formatting happens at the one write
// choke point so every config file on disk is consistently indented and hand
// edits survive the next write.

const configJSONIndent = "  "

// formatConfigJSON re-indents JSON content for a file about to be written.
// Non-JSON files and content that cannot be parsed are returned unchanged.
func formatConfigJSON(path string, data []byte) []byte {
	if !strings.EqualFold(filepath.Ext(path), ".json") {
		return data
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return data
	}

	formatted := pretty.PrettyOptions(trimmed, &pretty.Options{
		Indent: configJSONIndent,
	})
	if !json.Valid(formatted) {
		return data
	}

	formatted = trimTrailingNewlines(unescapeHTML(formatted))
	return append(formatted, '\n')
}

// trimTrailingNewlines removes line breaks at the end of content so the
// writer can add exactly one.
func trimTrailingNewlines(data []byte) []byte {
	return bytes.TrimRight(data, "\r\n")
}

// unescapeHTML restores the characters that JSON marshalling escapes by
// default. The escape sequences only appear inside string literals, so
// replacing them keeps the document valid and makes URLs and shell snippets
// readable again.
func unescapeHTML(data []byte) []byte {
	for _, pair := range [][2]string{
		{`\u003c`, "<"},
		{`\u003e`, ">"},
		{`\u0026`, "&"},
	} {
		data = bytes.ReplaceAll(data, []byte(pair[0]), []byte(pair[1]))
	}
	return data
}
