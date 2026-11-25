package pdf

import (
	"os"
	"strings"

	"github.com/SpherePrime/CLI/internal/documents/docutil"
)

type Margins struct {
	Top    float64 `json:"top,omitempty"`
	Right  float64 `json:"right,omitempty"`
	Bottom float64 `json:"bottom,omitempty"`
	Left   float64 `json:"left,omitempty"`
}

type Block struct {
	Type      string     `json:"type"`
	Text      string     `json:"text,omitempty"`
	Level     int        `json:"level,omitempty"`
	Items     []string   `json:"items,omitempty"`
	Headers   []string   `json:"headers,omitempty"`
	Rows      [][]string `json:"rows,omitempty"`
	ColWidths []float64  `json:"col_widths,omitempty"`
	Align     string     `json:"align,omitempty"`
	Title     string     `json:"title,omitempty"`
	Tone      string     `json:"tone,omitempty"`
	ImagePath string     `json:"image_path,omitempty"`
	Width     float64    `json:"width,omitempty"`
	Height    float64    `json:"height,omitempty"`
	Caption   string     `json:"caption,omitempty"`
}

type CreateOptions struct {
	OutputPath  string  `json:"output_path"`
	WorkingDir  string  `json:"working_dir,omitempty"`
	Title       string  `json:"title,omitempty"`
	Subtitle    string  `json:"subtitle,omitempty"`
	Author      string  `json:"author,omitempty"`
	Subject     string  `json:"subject,omitempty"`
	PageSize    string  `json:"page_size,omitempty"`
	Orientation string  `json:"orientation,omitempty"`
	Margins     Margins `json:"margins,omitempty"`
	Header      string  `json:"header,omitempty"`
	Footer      string  `json:"footer,omitempty"`
	PageNumbers bool    `json:"page_numbers,omitempty"`
	Blocks      []Block `json:"blocks,omitempty"`
}

type Result struct {
	Path      string `json:"path"`
	PageCount int    `json:"page_count"`
	SizeBytes int64  `json:"size_bytes"`
}

type SplitResult struct {
	Files []Result `json:"files"`
}

type Metadata struct {
	Title    string `json:"title,omitempty"`
	Author   string `json:"author,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Creator  string `json:"creator,omitempty"`
	Producer string `json:"producer,omitempty"`
}

type PageInfo struct {
	Number int     `json:"number"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Rotate int     `json:"rotate"`
	Text   string  `json:"text"`
}

type InspectResult struct {
	Path      string     `json:"path"`
	PageCount int        `json:"page_count"`
	SizeBytes int64      `json:"size_bytes"`
	Metadata  Metadata   `json:"metadata"`
	Pages     []PageInfo `json:"pages"`
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

func parsePageSelection(pageRange string) []string {
	trimmed := strings.TrimSpace(pageRange)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if tok := strings.TrimSpace(p); tok != "" {
			out = append(out, tok)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
