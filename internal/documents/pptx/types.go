package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/SpherePrime/CLI/internal/documents/docutil"
)

const (
	nsP            = "http://schemas.openxmlformats.org/presentationml/2006/main"
	nsA            = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsR            = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsPkgRel       = "http://schemas.openxmlformats.org/package/2006/relationships"
	nsContentTypes = "http://schemas.openxmlformats.org/package/2006/content-types"
	nsCP           = "http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
	nsDC           = "http://purl.org/dc/elements/1.1/"
	nsDCTerms      = "http://purl.org/dc/terms/"
	nsXSI          = "http://www.w3.org/2001/XMLSchema-instance"
	nsTableURI     = "http://schemas.openxmlformats.org/drawingml/2006/table"

	relTypeOfficeDoc   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	relTypeCoreProps   = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties"
	relTypeSlideMaster = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster"
	relTypeSlideLayout = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout"
	relTypeSlide       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide"
	relTypeTheme       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme"
	relTypeNotesMaster = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster"
	relTypeNotesSlide  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide"
	relTypeImage       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"

	ctPresentation = "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"
	ctSlideMaster  = "application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"
	ctSlideLayout  = "application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"
	ctSlide        = "application/vnd.openxmlformats-officedocument.presentationml.slide+xml"
	ctNotesMaster  = "application/vnd.openxmlformats-officedocument.presentationml.notesMaster+xml"
	ctNotesSlide   = "application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml"
	ctTheme        = "application/vnd.openxmlformats-officedocument.theme+xml"
	ctCoreProps    = "application/vnd.openxmlformats-package.core-properties+xml"
	ctRels         = "application/vnd.openxmlformats-package.relationships+xml"

	emuPerInch         = 914400
	slide16x9WidthEMU  = 12192000
	slide16x9HeightEMU = 6858000
	slide4x3WidthEMU   = 9144000
	slide4x3HeightEMU  = 6858000
	notesWidthEMU      = 6858000
	notesHeightEMU     = 9144000

	defaultPrimaryColor    = "1B365D"
	defaultAccentColor     = "2B6CB0"
	defaultBackgroundColor = "FFFFFF"
	defaultTextColor       = "1A202C"
	defaultFontFamily      = "Calibri"
)

var (
	errInvalidPptx     = errors.New("invalid pptx archive: missing ppt/presentation.xml")
	placeholderPattern = regexp.MustCompile(`\{\{([^{}]+)\}\}`)
)

type Theme struct {
	PrimaryColor    string `json:"primary_color,omitempty"`
	AccentColor     string `json:"accent_color,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`
	TextColor       string `json:"text_color,omitempty"`
	FontFamily      string `json:"font_family,omitempty"`
}

type ColumnContent struct {
	Heading string   `json:"heading,omitempty"`
	Body    string   `json:"body,omitempty"`
	Bullets []string `json:"bullets,omitempty"`
}

type TableSpec struct {
	Headers    []string   `json:"headers,omitempty"`
	Rows       [][]string `json:"rows,omitempty"`
	ColWidths  []float64  `json:"col_widths,omitempty"`
	HeaderFill string     `json:"header_fill,omitempty"`
}

type MetricCard struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Delta string `json:"delta,omitempty"`
	Color string `json:"color,omitempty"`
}

type SlideSpec struct {
	Layout          string         `json:"layout,omitempty"`
	Title           string         `json:"title,omitempty"`
	Subtitle        string         `json:"subtitle,omitempty"`
	Body            string         `json:"body,omitempty"`
	Bullets         []string       `json:"bullets,omitempty"`
	Left            *ColumnContent `json:"left,omitempty"`
	Right           *ColumnContent `json:"right,omitempty"`
	Table           *TableSpec     `json:"table,omitempty"`
	Metrics         []MetricCard   `json:"metrics,omitempty"`
	ImagePath       string         `json:"image_path,omitempty"`
	ImageCaption    string         `json:"image_caption,omitempty"`
	Notes           string         `json:"notes,omitempty"`
	BackgroundColor string         `json:"background_color,omitempty"`
}

type CreateOptions struct {
	OutputPath      string      `json:"output_path"`
	WorkingDir      string      `json:"working_dir,omitempty"`
	Title           string      `json:"title,omitempty"`
	Author          string      `json:"author,omitempty"`
	Subject         string      `json:"subject,omitempty"`
	AspectRatio     string      `json:"aspect_ratio,omitempty"`
	PrimaryColor    string      `json:"primary_color,omitempty"`
	AccentColor     string      `json:"accent_color,omitempty"`
	BackgroundColor string      `json:"background_color,omitempty"`
	TextColor       string      `json:"text_color,omitempty"`
	FontFamily      string      `json:"font_family,omitempty"`
	SlideNumbers    bool        `json:"slide_numbers,omitempty"`
	Slides          []SlideSpec `json:"slides,omitempty"`
}

type NotesUpdate struct {
	Slide int    `json:"slide"`
	Notes string `json:"notes"`
}

type UpdateOptions struct {
	InputPath       string            `json:"input_path"`
	OutputPath      string            `json:"output_path,omitempty"`
	WorkingDir      string            `json:"working_dir,omitempty"`
	Replacements    map[string]string `json:"replacements,omitempty"`
	NotesUpdates    []NotesUpdate     `json:"notes_updates,omitempty"`
	AppendSlides    []SlideSpec       `json:"append_slides,omitempty"`
	SlideNumbers    bool              `json:"slide_numbers,omitempty"`
	PrimaryColor    string            `json:"primary_color,omitempty"`
	AccentColor     string            `json:"accent_color,omitempty"`
	BackgroundColor string            `json:"background_color,omitempty"`
	TextColor       string            `json:"text_color,omitempty"`
}

type Result struct {
	Path                string `json:"path"`
	SizeBytes           int64  `json:"size_bytes"`
	SlideCount          int    `json:"slide_count"`
	ReplacementsApplied int    `json:"replacements_applied,omitempty"`
}

type Metadata struct {
	Title    string `json:"title,omitempty"`
	Author   string `json:"author,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Created  string `json:"created,omitempty"`
	Modified string `json:"modified,omitempty"`
}

type SlideDimensions struct {
	WidthEMU     int64   `json:"width_emu"`
	HeightEMU    int64   `json:"height_emu"`
	WidthInches  float64 `json:"width_inches"`
	HeightInches float64 `json:"height_inches"`
	AspectRatio  string  `json:"aspect_ratio"`
}

type TableData struct {
	Rows [][]string `json:"rows"`
}

type SlideInfo struct {
	Number int         `json:"number"`
	Title  string      `json:"title,omitempty"`
	Texts  []string    `json:"texts,omitempty"`
	Tables []TableData `json:"tables,omitempty"`
	Notes  string      `json:"notes,omitempty"`
}

type InspectResult struct {
	Path         string          `json:"path"`
	SizeBytes    int64           `json:"size_bytes"`
	SlideCount   int             `json:"slide_count"`
	Dimensions   SlideDimensions `json:"dimensions"`
	Metadata     Metadata        `json:"metadata"`
	Slides       []SlideInfo     `json:"slides,omitempty"`
	Placeholders []string        `json:"placeholders,omitempty"`
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

func resolveTheme(primary, accent, bg, text, font string) Theme {
	t := Theme{
		PrimaryColor:    normalizeHexColor(primary),
		AccentColor:     normalizeHexColor(accent),
		BackgroundColor: normalizeHexColor(bg),
		TextColor:       normalizeHexColor(text),
		FontFamily:      strings.TrimSpace(font),
	}
	if t.PrimaryColor == "" {
		t.PrimaryColor = defaultPrimaryColor
	}
	if t.AccentColor == "" {
		t.AccentColor = defaultAccentColor
	}
	if t.BackgroundColor == "" {
		t.BackgroundColor = defaultBackgroundColor
	}
	if t.TextColor == "" {
		t.TextColor = defaultTextColor
	}
	if t.FontFamily == "" {
		t.FontFamily = defaultFontFamily
	}
	return t
}

func resolveSlideDimensions(aspect string) (int64, int64, string) {
	switch strings.TrimSpace(strings.ToLower(aspect)) {
	case "4:3", "4x3", "standard":
		return slide4x3WidthEMU, slide4x3HeightEMU, "4:3"
	default:
		return slide16x9WidthEMU, slide16x9HeightEMU, "16:9"
	}
}

func buildSlideDimensions(wEMU, hEMU int64) SlideDimensions {
	if wEMU <= 0 || hEMU <= 0 {
		wEMU = slide16x9WidthEMU
		hEMU = slide16x9HeightEMU
	}
	wIn := math.Round((float64(wEMU)/float64(emuPerInch))*1000) / 1000
	hIn := math.Round((float64(hEMU)/float64(emuPerInch))*1000) / 1000
	ratio := float64(wEMU) / float64(hEMU)
	aspect := "custom"
	if math.Abs(ratio-(16.0/9.0)) < 0.03 {
		aspect = "16:9"
	} else if math.Abs(ratio-(4.0/3.0)) < 0.03 {
		aspect = "4:3"
	}
	return SlideDimensions{
		WidthEMU:     wEMU,
		HeightEMU:    hEMU,
		WidthInches:  wIn,
		HeightInches: hIn,
		AspectRatio:  aspect,
	}
}

func escapeXML(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func attrVal(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func resolveRelTarget(partPath, target string) string {
	cleanedTarget := strings.ReplaceAll(strings.TrimSpace(target), "\\", "/")
	if strings.HasPrefix(cleanedTarget, "/") {
		return strings.TrimPrefix(path.Clean(cleanedTarget), "/")
	}
	baseDir := path.Dir(strings.TrimPrefix(partPath, "/"))
	return strings.TrimPrefix(path.Clean(path.Join(baseDir, cleanedTarget)), "/")
}
