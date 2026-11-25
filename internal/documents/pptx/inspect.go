package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

type presentationInfo struct {
	widthEMU    int64
	heightEMU   int64
	slideRelIDs []string
	maxSlideID  int
}

func Inspect(pathStr string) (*InspectResult, error) {
	resolved, err := resolvePath("", pathStr)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	fi, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("stat pptx: %w", err)
	}
	zr, err := zip.OpenReader(resolved)
	if err != nil {
		return nil, fmt.Errorf("open pptx zip: %w", err)
	}
	defer zr.Close()

	entries := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		entries[strings.TrimPrefix(f.Name, "/")] = f
	}

	presFile, ok := entries["ppt/presentation.xml"]
	if !ok {
		return nil, errInvalidPptx
	}
	presBytes, err := readZipFile(presFile)
	if err != nil {
		return nil, fmt.Errorf("read ppt/presentation.xml: %w", err)
	}
	presInfo, err := parsePresentationXML(presBytes)
	if err != nil {
		return nil, fmt.Errorf("parse ppt/presentation.xml: %w", err)
	}

	res := &InspectResult{
		Path:       resolved,
		SizeBytes:  fi.Size(),
		Dimensions: buildSlideDimensions(presInfo.widthEMU, presInfo.heightEMU),
	}

	if coreFile, ok := entries["docProps/core.xml"]; ok {
		if coreBytes, err := readZipFile(coreFile); err == nil {
			res.Metadata = parseCoreProperties(coreBytes)
		}
	}

	var presRels []relDef
	if relsFile, ok := entries["ppt/_rels/presentation.xml.rels"]; ok {
		if relsBytes, err := readZipFile(relsFile); err == nil {
			presRels = parseRelsXML(relsBytes)
		}
	}

	slidePaths := resolveOrderedSlidePaths(presInfo.slideRelIDs, presRels, func(name string) bool {
		_, exists := entries[name]
		return exists
	}, func() []string {
		names := make([]string, 0, len(entries))
		for k := range entries {
			names = append(names, k)
		}
		return names
	})

	slides := make([]SlideInfo, 0, len(slidePaths))
	for idx, sp := range slidePaths {
		sf, ok := entries[sp]
		if !ok {
			continue
		}
		sBytes, err := readZipFile(sf)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", sp, err)
		}
		info, err := parseSlideXML(sBytes)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", sp, err)
		}
		info.Number = idx + 1

		sRelsPath := relsPathForPart(sp)
		if sRelsFile, ok := entries[sRelsPath]; ok {
			if sRelsBytes, err := readZipFile(sRelsFile); err == nil {
				for _, r := range parseRelsXML(sRelsBytes) {
					if r.relType == relTypeNotesSlide {
						notesPath := resolveRelTarget(sp, r.target)
						if nf, exists := entries[notesPath]; exists {
							if nBytes, err := readZipFile(nf); err == nil {
								if notes, err := parseNotesSlideXML(nBytes); err == nil {
									info.Notes = notes
								}
							}
						}
						break
					}
				}
			}
		}

		slides = append(slides, info)
	}

	res.Slides = slides
	res.SlideCount = len(slides)
	res.Placeholders = collectPlaceholders(slides)
	return res, nil
}

func relsPathForPart(partPath string) string {
	clean := strings.TrimPrefix(partPath, "/")
	dir := path.Dir(clean)
	base := path.Base(clean)
	if dir == "." || dir == "" {
		return "_rels/" + base + ".rels"
	}
	return dir + "/_rels/" + base + ".rels"
}

func parsePresentationXML(data []byte) (presentationInfo, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var info presentationInfo
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return info, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "sldSz":
			if v, err := strconv.ParseInt(attrVal(se.Attr, "cx"), 10, 64); err == nil {
				info.widthEMU = v
			}
			if v, err := strconv.ParseInt(attrVal(se.Attr, "cy"), 10, 64); err == nil {
				info.heightEMU = v
			}
		case "sldId":
			if idStr := nonNSAttrVal(se.Attr, "id"); idStr != "" {
				if v, err := strconv.Atoi(idStr); err == nil && v > info.maxSlideID {
					info.maxSlideID = v
				}
			}
			if rID := relIDAttrVal(se.Attr); rID != "" {
				info.slideRelIDs = append(info.slideRelIDs, rID)
			}
		}
	}
	return info, nil
}

func nonNSAttrVal(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Space == "" && a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func relIDAttrVal(attrs []xml.Attr) string {
	for _, a := range attrs {
		if a.Name.Local == "id" && (a.Name.Space == nsR || a.Name.Space == "r") {
			return a.Value
		}
	}
	return ""
}

func parseRelsXML(data []byte) []relDef {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var rels []relDef
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Relationship" {
			continue
		}
		rels = append(rels, relDef{
			id:      attrVal(se.Attr, "Id"),
			relType: attrVal(se.Attr, "Type"),
			target:  attrVal(se.Attr, "Target"),
		})
	}
	return rels
}

func resolveOrderedSlidePaths(slideRelIDs []string, presRels []relDef, existsFn func(string) bool, allNamesFn func() []string) []string {
	relByID := make(map[string]relDef, len(presRels))
	for _, r := range presRels {
		relByID[r.id] = r
	}
	var out []string
	seen := make(map[string]struct{})
	for _, rID := range slideRelIDs {
		r, ok := relByID[rID]
		if !ok {
			continue
		}
		p := resolveRelTarget("ppt/presentation.xml", r.target)
		if existsFn(p) {
			if _, dup := seen[p]; !dup {
				seen[p] = struct{}{}
				out = append(out, p)
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	var fallback []string
	for _, name := range allNamesFn() {
		if strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml") {
			fallback = append(fallback, name)
		}
	}
	sort.Slice(fallback, func(i, j int) bool {
		ni := extractTrailingNumber(fallback[i])
		nj := extractTrailingNumber(fallback[j])
		if ni != nj {
			return ni < nj
		}
		return fallback[i] < fallback[j]
	})
	return fallback
}

func extractTrailingNumber(partPath string) int {
	base := path.Base(partPath)
	base = strings.TrimSuffix(base, path.Ext(base))
	i := len(base)
	for i > 0 && base[i-1] >= '0' && base[i-1] <= '9' {
		i--
	}
	if i == len(base) {
		return 0
	}
	v, _ := strconv.Atoi(base[i:])
	return v
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
			case "created":
				meta.Created += val
			case "modified":
				meta.Modified += val
			}
		}
	}
	return meta
}

func parseSlideXML(data []byte) (SlideInfo, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var info SlideInfo
	var explicitTitle string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return info, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch {
		case se.Name.Local == "sp" && (se.Name.Space == nsP || se.Name.Space == ""):
			phType, texts, err := readShapeElement(dec)
			if err != nil {
				return info, err
			}
			if len(texts) > 0 {
				if (phType == "title" || phType == "ctrTitle") && explicitTitle == "" {
					explicitTitle = texts[0]
				}
				info.Texts = append(info.Texts, texts...)
			}
		case se.Name.Local == "tbl" && (se.Name.Space == nsA || se.Name.Space == ""):
			tbl, err := readDrawingTableElement(dec)
			if err != nil {
				return info, err
			}
			if len(tbl.Rows) > 0 {
				info.Tables = append(info.Tables, tbl)
			}
		}
	}
	if explicitTitle != "" {
		info.Title = explicitTitle
	} else if len(info.Texts) > 0 {
		info.Title = info.Texts[0]
	}
	return info, nil
}

func readShapeElement(dec *xml.Decoder) (string, []string, error) {
	depth := 1
	phType := ""
	var texts []string
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return phType, texts, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch {
			case t.Name.Local == "ph" && (t.Name.Space == nsP || t.Name.Space == ""):
				if tp := attrVal(t.Attr, "type"); tp != "" {
					phType = tp
				}
			case t.Name.Local == "p" && (t.Name.Space == nsA || t.Name.Space == ""):
				pText, err := readDrawingParagraph(dec, false)
				if err != nil {
					return phType, texts, err
				}
				depth--
				if trimmed := strings.TrimSpace(pText); trimmed != "" {
					texts = append(texts, trimmed)
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	return phType, texts, nil
}

func readDrawingParagraph(dec *xml.Decoder, skipSlideNumField bool) (string, error) {
	depth := 1
	var buf strings.Builder
	inText := 0
	skipFieldDepth := 0
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if skipFieldDepth > 0 {
				continue
			}
			if t.Name.Space != nsA && t.Name.Space != "" {
				continue
			}
			switch t.Name.Local {
			case "fld":
				if skipSlideNumField && strings.EqualFold(attrVal(t.Attr, "type"), "slidenum") {
					skipFieldDepth = depth
				}
			case "t":
				inText++
			case "br":
				buf.WriteByte('\n')
			case "tab":
				buf.WriteByte('\t')
			}
		case xml.EndElement:
			if skipFieldDepth > 0 && depth == skipFieldDepth {
				skipFieldDepth = 0
			} else if t.Name.Local == "t" && (t.Name.Space == nsA || t.Name.Space == "") && inText > 0 {
				inText--
			}
			depth--
		case xml.CharData:
			if inText > 0 && skipFieldDepth == 0 {
				buf.Write(t)
			}
		}
	}
	return buf.String(), nil
}

func readDrawingTableElement(dec *xml.Decoder) (TableData, error) {
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
			if depth == 2 && t.Name.Local == "tr" && (t.Name.Space == nsA || t.Name.Space == "") {
				row, err := readDrawingTableRow(dec)
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

func readDrawingTableRow(dec *xml.Decoder) ([]string, error) {
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
			if depth == 2 && t.Name.Local == "tc" && (t.Name.Space == nsA || t.Name.Space == "") {
				cell, err := readDrawingTableCell(dec)
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

func readDrawingTableCell(dec *xml.Decoder) (string, error) {
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
			if t.Name.Local == "p" && (t.Name.Space == nsA || t.Name.Space == "") {
				pText, err := readDrawingParagraph(dec, false)
				if err != nil {
					return "", err
				}
				depth--
				if trimmed := strings.TrimSpace(pText); trimmed != "" {
					lines = append(lines, trimmed)
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	return strings.Join(lines, "\n"), nil
}

func parseNotesSlideXML(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var bodyLines []string
	var fallbackLines []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local == "sp" && (se.Name.Space == nsP || se.Name.Space == "") {
			phType, lines, err := readNotesShapeElement(dec)
			if err != nil {
				return "", err
			}
			switch phType {
			case "sldNum", "hdr", "ftr", "dt", "sldImg":
				continue
			case "body":
				bodyLines = append(bodyLines, lines...)
			default:
				fallbackLines = append(fallbackLines, lines...)
			}
		}
	}
	if len(bodyLines) > 0 {
		return strings.Join(bodyLines, "\n"), nil
	}
	return strings.Join(fallbackLines, "\n"), nil
}

func readNotesShapeElement(dec *xml.Decoder) (string, []string, error) {
	depth := 1
	phType := ""
	var lines []string
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return phType, lines, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch {
			case t.Name.Local == "ph" && (t.Name.Space == nsP || t.Name.Space == ""):
				if tp := attrVal(t.Attr, "type"); tp != "" {
					phType = tp
				}
			case t.Name.Local == "p" && (t.Name.Space == nsA || t.Name.Space == ""):
				pText, err := readDrawingParagraph(dec, true)
				if err != nil {
					return phType, lines, err
				}
				depth--
				if trimmed := strings.TrimSpace(pText); trimmed != "" {
					lines = append(lines, trimmed)
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	return phType, lines, nil
}

func collectPlaceholders(slides []SlideInfo) []string {
	seen := make(map[string]struct{})
	var out []string
	scan := func(s string) {
		for _, m := range placeholderPattern.FindAllStringSubmatch(s, -1) {
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
	for _, s := range slides {
		for _, txt := range s.Texts {
			scan(txt)
		}
		for _, tbl := range s.Tables {
			for _, row := range tbl.Rows {
				for _, cell := range row {
					scan(cell)
				}
			}
		}
		if s.Notes != "" {
			scan(s.Notes)
		}
	}
	return out
}
