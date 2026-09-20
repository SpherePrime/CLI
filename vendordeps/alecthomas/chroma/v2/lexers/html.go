package lexers

import (
	"github.com/dwertyfa288/CLI/vendordeps/alecthomas/chroma/v2"
)

// HTML lexer.
var HTML = chroma.MustNewXMLLexer(embedded, "embedded/html.xml")
