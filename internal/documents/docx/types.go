package docx

import (
	"bytes"
	"encoding/xml"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/SpherePrime/CLI/internal/documents/docutil"
)

const (
	nsWordML       = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsRel          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsPkgRel       = "http://schemas.openxmlformats.org/package/2006/relationships"
	nsWP           = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsDrawingML    = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsPic          = "http://schemas.openxmlformats.org/drawingml/2006/picture"
	nsContentTypes = "http://schemas.openxmlformats.org/package/2006/content-types"

	relTypeOfficeDoc = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	relTypeCoreProps = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties"
	relTypeStyles    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
	relTypeNumbering = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering"
	relTypeHeader    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header"
	relTypeFooter    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer"
	relTypeImage     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"

	a4WidthTwips       = 11906
	a4HeightTwips      = 16838
	pageMarginTwips    = 1440
	contentWidthTwips  = a4WidthTwips - 2*pageMarginTwips
	contentWidthPoints = float64(contentWidthTwips) / 20.0
	emuPerPoint        = 12700.0
)

var placeholderPattern = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

type Run struct {
	Text       string  `json:"text"`
	Bold       bool    `json:"bold,omitempty"`
	Italic     bool    `json:"italic,omitempty"`
	Underline  bool    `json:"underline,omitempty"`
	Color      string  `json:"color,omitempty"`
	FontSize   float64 `json:"font_size,omitempty"`
	FontFamily string  `json:"font_family,omitempty"`
}

type Block struct {
	Type       string     `json:"type"`
	Text       string     `json:"text,omitempty"`
	Level      int        `json:"level,omitempty"`
	Runs       []Run      `json:"runs,omitempty"`
	Align      string     `json:"align,omitempty"`
	Items      []string   `json:"items,omitempty"`
	Headers    []string   `json:"headers,omitempty"`
	Rows       [][]string `json:"rows,omitempty"`
	ColWidths  []float64  `json:"col_widths,omitempty"`
	HeaderFill string     `json:"header_fill,omitempty"`
	Title      string     `json:"title,omitempty"`
	Tone       string     `json:"tone,omitempty"`
	ImagePath  string     `json:"image_path,omitempty"`
	Width      float64    `json:"width,omitempty"`
	Height     float64    `json:"height,omitempty"`
	Caption    string     `json:"caption,omitempty"`
}

type CreateOptions struct {
	OutputPath  string  `json:"output_path"`
	WorkingDir  string  `json:"working_dir,omitempty"`
	Title       string  `json:"title,omitempty"`
	Subtitle    string  `json:"subtitle,omitempty"`
	Author      string  `json:"author,omitempty"`
	Subject     string  `json:"subject,omitempty"`
	Header      string  `json:"header,omitempty"`
	Footer      string  `json:"footer,omitempty"`
	PageNumbers bool    `json:"page_numbers,omitempty"`
	Blocks      []Block `json:"blocks,omitempty"`
}

type TableUpdate struct {
	TableIndex int    `json:"table_index"`
	Row        int    `json:"row"`
	Col        int    `json:"col"`
	Value      string `json:"value"`
}

type UpdateOptions struct {
	InputPath    string            `json:"input_path"`
	OutputPath   string            `json:"output_path,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	Replacements map[string]string `json:"replacements,omitempty"`
	AppendBlocks []Block           `json:"append_blocks,omitempty"`
	TableUpdates []TableUpdate     `json:"table_updates,omitempty"`
}

type Result struct {
	Path                string `json:"path"`
	SizeBytes           int64  `json:"size_bytes"`
	ParagraphCount      int    `json:"paragraph_count,omitempty"`
	TableCount          int    `json:"table_count,omitempty"`
	ReplacementsApplied int    `json:"replacements_applied,omitempty"`
}

type Metadata struct {
	Title       string `json:"title,omitempty"`
	Subtitle    string `json:"subtitle,omitempty"`
	Author      string `json:"author,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Description string `json:"description,omitempty"`
	Created     string `json:"created,omitempty"`
	Modified    string `json:"modified,omitempty"`
}

type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

type TableData struct {
	Rows [][]string `json:"rows"`
}

type Comment struct {
	ID     string `json:"id,omitempty"`
	Author string `json:"author,omitempty"`
	Date   string `json:"date,omitempty"`
	Text   string `json:"text"`
}

type InspectResult struct {
	Path         string      `json:"path"`
	SizeBytes    int64       `json:"size_bytes"`
	Metadata     Metadata    `json:"metadata"`
	Headings     []Heading   `json:"headings,omitempty"`
	Paragraphs   []string    `json:"paragraphs,omitempty"`
	Tables       []TableData `json:"tables,omitempty"`
	Headers      []string    `json:"headers,omitempty"`
	Footers      []string    `json:"footers,omitempty"`
	Comments     []Comment   `json:"comments,omitempty"`
	Placeholders []string    `json:"placeholders,omitempty"`
}

func resolvePath(workingDir, rawPath string) (string, error) {
	wd := strings.TrimSpace(workingDir)
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			wd = ""
		}
	}
	return docutil.ResolvePath(wd, rawPath)
}

func normalizeHexColor(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return ""
	}
	for i := range 6 {
		c := s[i]
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isHex {
			return ""
		}
	}
	return strings.ToUpper(s)
}

func isDarkHex(hexColor string) bool {
	if len(hexColor) != 6 {
		return true
	}
	r, err1 := strconv.ParseUint(hexColor[0:2], 16, 8)
	g, err2 := strconv.ParseUint(hexColor[2:4], 16, 8)
	b, err3 := strconv.ParseUint(hexColor[4:6], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return true
	}
	luma := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	return luma < 150
}

func mapAlignment(align string) string {
	switch strings.ToLower(strings.TrimSpace(align)) {
	case "left":
		return "left"
	case "center":
		return "center"
	case "right":
		return "right"
	case "justify", "both":
		return "both"
	default:
		return ""
	}
}

func sanitizeXMLString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == 0x09 || r == 0x0A || r == 0x0D ||
			(r >= 0x20 && r <= 0xD7FF) ||
			(r >= 0xE000 && r <= 0xFFFD) ||
			(r >= 0x10000 && r <= 0x10FFFF) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func escapeXML(s string) string {
	clean := sanitizeXMLString(s)
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(clean))
	return buf.String()
}
