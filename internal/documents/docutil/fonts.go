package docutil

import (
	"bytes"
	_ "embed"
)

var (
	//go:embed fonts/Lora-Regular.ttf
	regularFont []byte
	//go:embed fonts/Lora-Bold.ttf
	boldFont []byte
	//go:embed fonts/Lora-Italic.ttf
	italicFont []byte
	//go:embed fonts/JetBrainsMono-Regular.ttf
	monoFont []byte
)

func RegularFontBytes() []byte {
	return bytes.Clone(regularFont)
}

func BoldFontBytes() []byte {
	return bytes.Clone(boldFont)
}

func ItalicFontBytes() []byte {
	return bytes.Clone(italicFont)
}

func MonoFontBytes() []byte {
	return bytes.Clone(monoFont)
}
