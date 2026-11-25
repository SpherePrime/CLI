package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

func Inspect(path string) (*InspectResult, error) {
	resolved, err := resolvePath("", path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	fi, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("stat docx: %w", err)
	}
	zr, err := zip.OpenReader(resolved)
	if err != nil {
		return nil, fmt.Errorf("open docx zip: %w", err)
	}
	defer zr.Close()

	entries := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		entries[strings.TrimPrefix(f.Name, "/")] = f
	}

	docFile, ok := entries["word/document.xml"]
	if !ok {
		return nil, errInvalidDocx
	}

	docBytes, err := readZipFile(docFile)
	if err != nil {
		return nil, fmt.Errorf("read word/document.xml: %w", err)
	}

	res := &InspectResult{
		Path:      resolved,
		SizeBytes: fi.Size(),
	}

	if coreFile, ok := entries["docProps/core.xml"]; ok {
		if coreBytes, err := readZipFile(coreFile); err == nil {
			res.Metadata = parseCoreProperties(coreBytes)
		}
	}

	styleHeadings := map[string]int{}
	if stylesFile, ok := entries["word/styles.xml"]; ok {
		if stylesBytes, err := readZipFile(stylesFile); err == nil {
			styleHeadings = parseStyleHeadings(stylesBytes)
		}
	}

	headings, paragraphs, tables, err := parseDocumentXML(docBytes, styleHeadings)
	if err != nil {
		return nil, fmt.Errorf("parse word/document.xml: %w", err)
	}
	res.Headings = headings
	res.Paragraphs = paragraphs
	res.Tables = tables

	var headerNames, footerNames []string
	for name := range entries {
		if strings.HasPrefix(name, "word/header") && strings.HasSuffix(name, ".xml") {
			headerNames = append(headerNames, name)
		} else if strings.HasPrefix(name, "word/footer") && strings.HasSuffix(name, ".xml") {
			footerNames = append(footerNames, name)
		}
	}
	sort.Strings(headerNames)
	sort.Strings(footerNames)

	for _, name := range headerNames {
		data, err := readZipFile(entries[name])
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		texts, err := parsePartTexts(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		res.Headers = append(res.Headers, texts...)
	}

	for _, name := range footerNames {
		data, err := readZipFile(entries[name])
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		texts, err := parsePartTexts(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		res.Footers = append(res.Footers, texts...)
	}

	if commentsFile, ok := entries["word/comments.xml"]; ok {
		data, err := readZipFile(commentsFile)
		if err != nil {
			return nil, fmt.Errorf("read word/comments.xml: %w", err)
		}
		comments, err := parseCommentsXML(data)
		if err != nil {
			return nil, fmt.Errorf("parse word/comments.xml: %w", err)
		}
		res.Comments = comments
	}

	res.Placeholders = collectPlaceholders(res)
	return res, nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func parseCoreProperties(data []byte) Metadata {
	var meta Metadata
	dec := xml.NewDecoder(bytes.NewReader(data))
	var current string
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			current = strings.ToLower(t.Name.Local)
		case xml.EndElement:
			current = ""
		case xml.CharData:
			val := strings.TrimSpace(string(t))
			if val == "" {
				continue
			}
			switch current {
			case "title":
				meta.Title += val
			case "creator":
				meta.Author += val
			case "subject":
				meta.Subject += val
			case "description":
				meta.Description += val
			case "created":
				meta.Created += val
			case "modified":
				meta.Modified += val
			}
		}
	}
	if meta.Subtitle == "" && meta.Description != "" {
		meta.Subtitle = meta.Description
	}
	return meta
}

func parseStyleHeadings(data []byte) map[string]int {
	result := make(map[string]int)
	dec := xml.NewDecoder(bytes.NewReader(data))
	var currentStyleID string
	var currentLevel int
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "style":
				currentStyleID = attrVal(t.Attr, "styleId")
				currentLevel = headingLevelFromName(currentStyleID)
			case "name":
				if currentStyleID != "" && currentLevel == 0 {
					currentLevel = headingLevelFromName(attrVal(t.Attr, "val"))
				}
			case "outlineLvl":
				if currentStyleID != "" {
					if v, err := strconv.Atoi(attrVal(t.Attr, "val")); err == nil && v >= 0 && v < 9 {
						currentLevel = v + 1
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "style" {
				if currentStyleID != "" && currentLevel > 0 {
					result[currentStyleID] = currentLevel
				}
				currentStyleID = ""
				currentLevel = 0
			}
		}
	}
	return result
}

func headingLevelFromName(name string) int {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, prefix := range []string{"heading ", "heading", "заголовок ", "заголовок"} {
		if rest, ok := strings.CutPrefix(lower, prefix); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil && n >= 1 && n <= 9 {
				return n
			}
		}
	}
	return 0
}

func attrVal(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func parseDocumentXML(data []byte, styleHeadings map[string]int) ([]Heading, []string, []TableData, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var headings []Heading
	var paragraphs []string
	var tables []TableData

	inBody := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			if ee, ok := tok.(xml.EndElement); ok && ee.Name.Local == "body" {
				break
			}
			continue
		}
		if !inBody {
			if se.Name.Local == "body" {
				inBody = true
			}
			continue
		}

		switch se.Name.Local {
		case "p":
			p, err := readParagraphElement(dec, styleHeadings)
			if err != nil {
				return nil, nil, nil, err
			}
			if strings.TrimSpace(p.text) != "" {
				paragraphs = append(paragraphs, p.text)
				if p.headingLevel > 0 {
					headings = append(headings, Heading{Level: p.headingLevel, Text: p.text})
				}
			}
		case "tbl":
			tbl, err := readTableElement(dec, styleHeadings)
			if err != nil {
				return nil, nil, nil, err
			}
			tables = append(tables, tbl)
		case "sdt", "sdtContent":
			continue
		default:
			if err := dec.Skip(); err != nil {
				return nil, nil, nil, err
			}
		}
	}

	return headings, paragraphs, tables, nil
}

type parsedParagraph struct {
	text         string
	headingLevel int
}

func readParagraphElement(dec *xml.Decoder, styleHeadings map[string]int) (parsedParagraph, error) {
	var out parsedParagraph
	var text strings.Builder
	depth := 1
	inPPr := false
	inDel := 0
	inInstr := 0
	inText := 0

	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return out, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch t.Name.Local {
			case "pPr":
				if depth == 2 {
					inPPr = true
				}
			case "pStyle":
				if inPPr {
					styleID := attrVal(t.Attr, "val")
					if lvl, ok := styleHeadings[styleID]; ok && out.headingLevel == 0 {
						out.headingLevel = lvl
					} else if lvl := headingLevelFromName(styleID); lvl > 0 && out.headingLevel == 0 {
						out.headingLevel = lvl
					}
				}
			case "outlineLvl":
				if inPPr {
					if v, err := strconv.Atoi(attrVal(t.Attr, "val")); err == nil && v >= 0 && v < 9 {
						out.headingLevel = v + 1
					}
				}
			case "del":
				inDel++
			case "instrText":
				inInstr++
			case "t":
				if inDel == 0 && inInstr == 0 {
					inText++
				}
			case "tab":
				if inDel == 0 && inInstr == 0 {
					text.WriteByte('\t')
				}
			case "br", "cr":
				if inDel == 0 && inInstr == 0 {
					text.WriteByte('\n')
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "pPr":
				if depth == 2 {
					inPPr = false
				}
			case "del":
				if inDel > 0 {
					inDel--
				}
			case "instrText":
				if inInstr > 0 {
					inInstr--
				}
			case "t":
				if inText > 0 {
					inText--
				}
			}
			depth--
		case xml.CharData:
			if inText > 0 && inDel == 0 && inInstr == 0 {
				text.Write(t)
			}
		}
	}

	out.text = text.String()
	return out, nil
}

func readTableElement(dec *xml.Decoder, styleHeadings map[string]int) (TableData, error) {
	var tbl TableData
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return tbl, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 && t.Name.Local == "tr" {
				row, err := readTableRowElement(dec, styleHeadings)
				if err != nil {
					return tbl, err
				}
				tbl.Rows = append(tbl.Rows, row)
				depth--
			}
		case xml.EndElement:
			depth--
		}
	}
	return tbl, nil
}

func readTableRowElement(dec *xml.Decoder, styleHeadings map[string]int) ([]string, error) {
	var row []string
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return row, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 && t.Name.Local == "tc" {
				cell, err := readTableCellElement(dec, styleHeadings)
				if err != nil {
					return row, err
				}
				row = append(row, cell)
				depth--
			}
		case xml.EndElement:
			depth--
		}
	}
	return row, nil
}

func readTableCellElement(dec *xml.Decoder, styleHeadings map[string]int) (string, error) {
	var lines []string
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "p" {
				p, err := readParagraphElement(dec, styleHeadings)
				if err != nil {
					return "", err
				}
				if trimmed := strings.TrimSpace(p.text); trimmed != "" {
					lines = append(lines, trimmed)
				}
				depth--
			}
		case xml.EndElement:
			depth--
		}
	}
	return strings.Join(lines, "\n"), nil
}

func parsePartTexts(data []byte) ([]string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var texts []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local == "p" {
			p, err := readParagraphElement(dec, nil)
			if err != nil {
				return nil, err
			}
			if trimmed := strings.TrimSpace(p.text); trimmed != "" {
				texts = append(texts, trimmed)
			}
		}
	}
	return texts, nil
}

func parseCommentsXML(data []byte) ([]Comment, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var comments []Comment
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "comment" {
			continue
		}
		c := Comment{
			ID:     attrVal(se.Attr, "id"),
			Author: attrVal(se.Attr, "author"),
			Date:   attrVal(se.Attr, "date"),
		}
		var lines []string
		depth := 1
		for depth > 0 {
			ctok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			switch ct := ctok.(type) {
			case xml.StartElement:
				depth++
				if ct.Name.Local == "p" {
					p, err := readParagraphElement(dec, nil)
					if err != nil {
						return nil, err
					}
					if trimmed := strings.TrimSpace(p.text); trimmed != "" {
						lines = append(lines, trimmed)
					}
					depth--
				}
			case xml.EndElement:
				depth--
			}
		}
		c.Text = strings.Join(lines, "\n")
		comments = append(comments, c)
	}
	return comments, nil
}

func collectPlaceholders(info *InspectResult) []string {
	seen := make(map[string]struct{})
	var out []string
	scan := func(s string) {
		matches := placeholderPattern.FindAllStringSubmatch(s, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			key := strings.TrimSpace(m[1])
			if key == "" {
				continue
			}
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				out = append(out, key)
			}
		}
	}
	for _, h := range info.Headings {
		scan(h.Text)
	}
	for _, p := range info.Paragraphs {
		scan(p)
	}
	for _, tbl := range info.Tables {
		for _, row := range tbl.Rows {
			for _, cell := range row {
				scan(cell)
			}
		}
	}
	for _, hdr := range info.Headers {
		scan(hdr)
	}
	for _, ftr := range info.Footers {
		scan(ftr)
	}
	for _, c := range info.Comments {
		scan(c.Text)
	}
	return out
}
