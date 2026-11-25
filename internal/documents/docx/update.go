package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/SpherePrime/CLI/internal/documents/docutil"
)

type zipEntry struct {
	name   string
	method uint16
	data   []byte
}

type bytePatch struct {
	start int
	end   int
	repl  []byte
}

func FillTemplate(inputPath, outputPath string, values map[string]string) (*Result, error) {
	if strings.TrimSpace(inputPath) == "" {
		return nil, errors.New("input path is required")
	}
	if strings.TrimSpace(outputPath) == "" {
		return nil, errors.New("output path is required")
	}
	replacements := make(map[string]string, len(values))
	for k, v := range values {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		if strings.HasPrefix(key, "{{") && strings.HasSuffix(key, "}}") {
			replacements[key] = v
		} else {
			replacements["{{"+key+"}}"] = v
		}
	}
	return Update(UpdateOptions{
		InputPath:    inputPath,
		OutputPath:   outputPath,
		Replacements: replacements,
	})
}

func Update(opts UpdateOptions) (*Result, error) {
	inPath, err := resolvePath(opts.WorkingDir, opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve input path: %w", err)
	}
	rawOut := strings.TrimSpace(opts.OutputPath)
	if rawOut == "" {
		rawOut = opts.InputPath
	}
	outPath, err := resolvePath(opts.WorkingDir, rawOut)
	if err != nil {
		return nil, fmt.Errorf("resolve output path: %w", err)
	}

	entries, index, err := readAllZipEntries(inPath)
	if err != nil {
		return nil, err
	}

	docIdx, ok := index["word/document.xml"]
	if !ok {
		return nil, errInvalidDocx
	}

	totalReplacements := 0
	if len(opts.Replacements) > 0 {
		targetNames := []string{"word/document.xml"}
		for _, e := range entries {
			if (strings.HasPrefix(e.name, "word/header") || strings.HasPrefix(e.name, "word/footer")) && strings.HasSuffix(e.name, ".xml") {
				targetNames = append(targetNames, e.name)
			}
		}
		for _, name := range targetNames {
			i := index[name]
			updated, count, err := replaceInWordXML(entries[i].data, opts.Replacements)
			if err != nil {
				return nil, fmt.Errorf("replace text in %s: %w", name, err)
			}
			entries[i].data = updated
			totalReplacements += count
		}
	}

	if len(opts.TableUpdates) > 0 {
		updated, err := applyTableUpdates(entries[docIdx].data, opts.TableUpdates)
		if err != nil {
			return nil, fmt.Errorf("apply table updates: %w", err)
		}
		entries[docIdx].data = updated
	}

	if len(opts.AppendBlocks) > 0 {
		if err := appendBlocksToArchive(&entries, index, opts.WorkingDir, opts.AppendBlocks); err != nil {
			return nil, fmt.Errorf("append blocks: %w", err)
		}
		docIdx = index["word/document.xml"]
	}

	err = docutil.WriteZipAtomic(outPath, func(zw *zip.Writer) error {
		for _, e := range entries {
			hdr := &zip.FileHeader{
				Name:   e.name,
				Method: e.method,
			}
			if hdr.Method == 0 {
				hdr.Method = zip.Deflate
			}
			w, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			if _, err := w.Write(e.data); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("write updated docx: %w", err)
	}

	fi, err := os.Stat(outPath)
	if err != nil {
		return nil, fmt.Errorf("stat updated docx: %w", err)
	}

	_, paragraphs, tables, _ := parseDocumentXML(entries[docIdx].data, nil)
	return &Result{
		Path:                outPath,
		SizeBytes:           fi.Size(),
		ParagraphCount:      len(paragraphs),
		TableCount:          len(tables),
		ReplacementsApplied: totalReplacements,
	}, nil
}

func readAllZipEntries(path string) ([]zipEntry, map[string]int, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open docx zip: %w", err)
	}
	defer zr.Close()

	entries := make([]zipEntry, 0, len(zr.File))
	index := make(map[string]int, len(zr.File))
	for _, f := range zr.File {
		data, err := readZipFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("read zip entry %s: %w", f.Name, err)
		}
		normName := strings.TrimPrefix(f.Name, "/")
		index[normName] = len(entries)
		entries = append(entries, zipEntry{
			name:   normName,
			method: f.Method,
			data:   data,
		})
	}
	return entries, index, nil
}

type textSegment struct {
	start int
	end   int
	text  string
}

type paraFrame struct {
	groups     [][]textSegment
	curGroup   []textSegment
	delDepth   int
	instrDepth int
}

func (f *paraFrame) flushGroup() {
	if len(f.curGroup) > 0 {
		f.groups = append(f.groups, f.curGroup)
		f.curGroup = nil
	}
}

func findTagStart(raw []byte, hint int) int {
	if hint < 0 {
		hint = 0
	}
	for i := hint; i < len(raw); i++ {
		if raw[i] == '<' {
			return i
		}
	}
	return hint
}

func replaceInWordXML(xmlBytes []byte, replacements map[string]string) ([]byte, int, error) {
	keys := make([]string, 0, len(replacements))
	for k := range replacements {
		if k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return xmlBytes, 0, nil
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})

	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	var stack []paraFrame
	var allGroups [][]textSegment

	for {
		before := int(dec.InputOffset())
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				if len(stack) > 0 {
					stack[len(stack)-1].flushGroup()
				}
				stack = append(stack, paraFrame{})
			case "del":
				if len(stack) > 0 {
					stack[len(stack)-1].delDepth++
				}
			case "instrText":
				if len(stack) > 0 {
					stack[len(stack)-1].instrDepth++
				}
			case "br", "cr", "tab", "fldChar", "drawing", "pict", "tbl":
				if len(stack) > 0 {
					stack[len(stack)-1].flushGroup()
				}
			case "t":
				if len(stack) > 0 {
					top := &stack[len(stack)-1]
					if top.delDepth == 0 && top.instrDepth == 0 {
						tStart := findTagStart(xmlBytes, before)
						var textBuf strings.Builder
						tDepth := 1
						for tDepth > 0 {
							inner, ierr := dec.RawToken()
							if ierr != nil {
								return nil, 0, ierr
							}
							switch it := inner.(type) {
							case xml.StartElement:
								tDepth++
							case xml.EndElement:
								tDepth--
							case xml.CharData:
								textBuf.Write(it)
							}
						}
						tEnd := int(dec.InputOffset())
						top.curGroup = append(top.curGroup, textSegment{
							start: tStart,
							end:   tEnd,
							text:  textBuf.String(),
						})
					}
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if len(stack) > 0 {
					top := &stack[len(stack)-1]
					top.flushGroup()
					allGroups = append(allGroups, top.groups...)
					stack = stack[:len(stack)-1]
				}
			case "del":
				if len(stack) > 0 && stack[len(stack)-1].delDepth > 0 {
					stack[len(stack)-1].delDepth--
				}
			case "instrText":
				if len(stack) > 0 && stack[len(stack)-1].instrDepth > 0 {
					stack[len(stack)-1].instrDepth--
				}
			}
		}
	}

	var patches []bytePatch
	totalApplied := 0

	for _, segs := range allGroups {
		var fullBuf strings.Builder
		segStarts := make([]int, len(segs))
		for i, seg := range segs {
			segStarts[i] = fullBuf.Len()
			fullBuf.WriteString(seg.text)
		}
		fullText := fullBuf.String()
		if fullText == "" {
			continue
		}

		matches := findNonOverlappingMatches(fullText, keys, replacements)
		if len(matches) == 0 {
			continue
		}
		totalApplied += len(matches)

		for i, seg := range segs {
			segStart := segStarts[i]
			segEnd := segStart + len(seg.text)
			var segBuf strings.Builder
			cursor := segStart
			for _, m := range matches {
				if m.end <= segStart {
					continue
				}
				if m.start >= segEnd {
					break
				}
				if m.start > cursor {
					segBuf.WriteString(fullText[cursor:m.start])
				}
				if m.start >= segStart && m.start < segEnd {
					segBuf.WriteString(m.repl)
				}
				cursor = min(m.end, segEnd)
			}
			if cursor < segEnd {
				segBuf.WriteString(fullText[cursor:segEnd])
			}
			newText := segBuf.String()
			if newText != seg.text {
				patches = append(patches, bytePatch{
					start: seg.start,
					end:   seg.end,
					repl:  []byte(formatWTReplacement(newText)),
				})
			}
		}
	}

	if len(patches) == 0 {
		return xmlBytes, 0, nil
	}
	return applyPatches(xmlBytes, patches), totalApplied, nil
}

type textMatch struct {
	start int
	end   int
	repl  string
}

func findNonOverlappingMatches(text string, sortedKeys []string, replacements map[string]string) []textMatch {
	var matches []textMatch
	pos := 0
	for pos < len(text) {
		bestIdx := -1
		bestKey := ""
		for _, k := range sortedKeys {
			idx := strings.Index(text[pos:], k)
			if idx < 0 {
				continue
			}
			absIdx := pos + idx
			if bestIdx < 0 || absIdx < bestIdx || (absIdx == bestIdx && len(k) > len(bestKey)) {
				bestIdx = absIdx
				bestKey = k
			}
		}
		if bestIdx < 0 {
			break
		}
		matches = append(matches, textMatch{
			start: bestIdx,
			end:   bestIdx + len(bestKey),
			repl:  replacements[bestKey],
		})
		pos = bestIdx + len(bestKey)
	}
	return matches
}

func formatWTReplacement(text string) string {
	parts := strings.Split(text, "\n")
	if len(parts) == 1 {
		return `<w:t xml:space="preserve">` + escapeXML(text) + `</w:t>`
	}
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString(`<w:br/>`)
		}
		b.WriteString(`<w:t xml:space="preserve">`)
		b.WriteString(escapeXML(p))
		b.WriteString(`</w:t>`)
	}
	return b.String()
}

func applyPatches(raw []byte, patches []bytePatch) []byte {
	sort.Slice(patches, func(i, j int) bool {
		return patches[i].start > patches[j].start
	})
	out := make([]byte, len(raw))
	copy(out, raw)
	for _, p := range patches {
		var buf bytes.Buffer
		buf.Grow(len(out) - (p.end - p.start) + len(p.repl))
		buf.Write(out[:p.start])
		buf.Write(p.repl)
		buf.Write(out[p.end:])
		out = buf.Bytes()
	}
	return out
}

type cellSpan struct {
	start int
	end   int
}

func applyTableUpdates(docXML []byte, updates []TableUpdate) ([]byte, error) {
	tables, err := locateTableCells(docXML)
	if err != nil {
		return nil, err
	}

	type cellKey struct {
		table int
		row   int
		col   int
	}
	merged := make(map[cellKey]string, len(updates))
	order := make([]cellKey, 0, len(updates))
	for _, u := range updates {
		if u.TableIndex < 0 || u.TableIndex >= len(tables) {
			return nil, fmt.Errorf("table_index %d out of range (found %d tables)", u.TableIndex, len(tables))
		}
		rows := tables[u.TableIndex]
		if u.Row < 0 || u.Row >= len(rows) {
			return nil, fmt.Errorf("table %d row %d out of range (found %d rows)", u.TableIndex, u.Row, len(rows))
		}
		cols := rows[u.Row]
		if u.Col < 0 || u.Col >= len(cols) {
			return nil, fmt.Errorf("table %d row %d col %d out of range (found %d cols)", u.TableIndex, u.Row, u.Col, len(cols))
		}
		k := cellKey{table: u.TableIndex, row: u.Row, col: u.Col}
		if _, exists := merged[k]; !exists {
			order = append(order, k)
		}
		merged[k] = u.Value
	}

	patches := make([]bytePatch, 0, len(order))
	for _, k := range order {
		span := tables[k.table][k.row][k.col]
		updatedCell, err := replaceCellText(docXML[span.start:span.end], merged[k])
		if err != nil {
			return nil, err
		}
		patches = append(patches, bytePatch{
			start: span.start,
			end:   span.end,
			repl:  updatedCell,
		})
	}
	return applyPatches(docXML, patches), nil
}

func locateTableCells(docXML []byte) ([][][]cellSpan, error) {
	dec := xml.NewDecoder(bytes.NewReader(docXML))
	var tables [][][]cellSpan

	depth := 0
	bodyDepth := -1
	tblDepth := -1
	trDepth := -1
	tcDepth := -1
	tcStart := 0

	for {
		before := int(dec.InputOffset())
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch t.Name.Local {
			case "body":
				if bodyDepth < 0 {
					bodyDepth = depth
				}
			case "tbl":
				if bodyDepth > 0 && tblDepth < 0 {
					tblDepth = depth
					tables = append(tables, nil)
				}
			case "tr":
				if tblDepth > 0 && depth == tblDepth+1 {
					trDepth = depth
					ti := len(tables) - 1
					tables[ti] = append(tables[ti], nil)
				}
			case "tc":
				if trDepth > 0 && depth == trDepth+1 {
					tcDepth = depth
					tcStart = findTagStart(docXML, before)
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "tc":
				if tcDepth == depth {
					ti := len(tables) - 1
					ri := len(tables[ti]) - 1
					tables[ti][ri] = append(tables[ti][ri], cellSpan{
						start: tcStart,
						end:   int(dec.InputOffset()),
					})
					tcDepth = -1
				}
			case "tr":
				if trDepth == depth {
					trDepth = -1
				}
			case "tbl":
				if tblDepth == depth {
					tblDepth = -1
				}
			case "body":
				if bodyDepth == depth {
					bodyDepth = -1
				}
			}
			depth--
		}
	}
	return tables, nil
}

func replaceCellText(cellXML []byte, value string) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(cellXML))
	var tSpans []cellSpan
	firstPEnd := -1
	firstPStart := -1
	firstPSelfClosing := false
	depth := 0

	for {
		before := int(dec.InputOffset())
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "p" && firstPStart < 0 && depth == 2 {
				firstPStart = findTagStart(cellXML, before)
				afterStart := int(dec.InputOffset())
				if afterStart >= 2 && cellXML[afterStart-2] == '/' {
					firstPSelfClosing = true
					firstPEnd = afterStart
				}
			}
			if t.Name.Local == "t" {
				tStart := findTagStart(cellXML, before)
				tDepth := 1
				for tDepth > 0 {
					inner, ierr := dec.RawToken()
					if ierr != nil {
						return nil, ierr
					}
					switch inner.(type) {
					case xml.StartElement:
						tDepth++
					case xml.EndElement:
						tDepth--
					}
				}
				tEnd := int(dec.InputOffset())
				tSpans = append(tSpans, cellSpan{start: tStart, end: tEnd})
				depth--
			}
		case xml.EndElement:
			if t.Name.Local == "p" && firstPStart >= 0 && firstPEnd < 0 && depth == 2 {
				firstPEnd = findTagStart(cellXML, before)
			}
			depth--
		}
	}

	if len(tSpans) > 0 {
		patches := make([]bytePatch, 0, len(tSpans))
		for i, sp := range tSpans {
			if i == 0 {
				patches = append(patches, bytePatch{
					start: sp.start,
					end:   sp.end,
					repl:  []byte(formatWTReplacement(value)),
				})
			} else {
				patches = append(patches, bytePatch{
					start: sp.start,
					end:   sp.end,
					repl:  []byte(`<w:t></w:t>`),
				})
			}
		}
		return applyPatches(cellXML, patches), nil
	}

	runXML := `<w:r>` + formatWTReplacement(value) + `</w:r>`
	if firstPStart >= 0 && firstPEnd > firstPStart {
		if firstPSelfClosing {
			return applyPatches(cellXML, []bytePatch{{
				start: firstPStart,
				end:   firstPEnd,
				repl:  []byte(`<w:p>` + runXML + `</w:p>`),
			}}), nil
		}
		return applyPatches(cellXML, []bytePatch{{
			start: firstPEnd,
			end:   firstPEnd,
			repl:  []byte(runXML),
		}}), nil
	}

	closeIdx := bytes.LastIndex(cellXML, []byte("</w:tc>"))
	if closeIdx < 0 {
		return nil, errors.New("malformed table cell: missing </w:tc>")
	}
	return applyPatches(cellXML, []bytePatch{{
		start: closeIdx,
		end:   closeIdx,
		repl:  []byte(`<w:p>` + runXML + `</w:p>`),
	}}), nil
}

var (
	reRelID       = regexp.MustCompile(`Id="rId(\d+)"`)
	reMediaIdx    = regexp.MustCompile(`^word/media/image(\d+)\.`)
	reDocPrID     = regexp.MustCompile(`docPr[^>]*id="(\d+)"`)
	reNumID       = regexp.MustCompile(`numId="(\d+)"`)
	reAbstractID  = regexp.MustCompile(`abstractNumId="(\d+)"`)
)

func maxRegexInt(re *regexp.Regexp, data []byte) int {
	best := 0
	for _, m := range re.FindAllSubmatch(data, -1) {
		if len(m) >= 2 {
			if n, err := strconv.Atoi(string(m[1])); err == nil && n > best {
				best = n
			}
		}
	}
	return best
}

func appendBlocksToArchive(entries *[]zipEntry, index map[string]int, workingDir string, blocks []Block) error {
	docIdx := index["word/document.xml"]
	docXML := (*entries)[docIdx].data

	maxRel := 0
	if relIdx, ok := index["word/_rels/document.xml.rels"]; ok {
		maxRel = maxRegexInt(reRelID, (*entries)[relIdx].data)
	}
	maxImg := 0
	for _, e := range *entries {
		if m := reMediaIdx.FindStringSubmatch(e.name); len(m) == 2 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > maxImg {
				maxImg = n
			}
		}
	}
	maxDocPr := maxRegexInt(reDocPrID, docXML)

	maxNumID := 0
	maxAbstractID := 0
	hasNumbering := false
	if numIdx, ok := index["word/numbering.xml"]; ok {
		hasNumbering = true
		maxNumID = maxRegexInt(reNumID, (*entries)[numIdx].data)
		maxAbstractID = maxRegexInt(reAbstractID, (*entries)[numIdx].data)
	}

	bulletNumID := maxNumID + 1
	startNumberedID := bulletNumID + 1

	b := newDocBuilder(workingDir, maxRel+1, maxImg+1, maxDocPr+1, bulletNumID, startNumberedID)
	rendered, err := b.renderBlocks(blocks)
	if err != nil {
		return err
	}

	insertPos, err := findBodyInsertOffset(docXML)
	if err != nil {
		return err
	}
	(*entries)[docIdx].data = applyPatches(docXML, []bytePatch{{
		start: insertPos,
		end:   insertPos,
		repl:  []byte(rendered),
	}})

	var newRels []relEntry
	if b.usedBullets || len(b.numberedIDs) > 0 {
		if !hasNumbering {
			numRelID := b.allocRelID()
			newRels = append(newRels, relEntry{id: numRelID, relType: relTypeNumbering, target: "numbering.xml"})
			numXML := buildAppendedNumberingPart(bulletNumID, b.numberedIDs)
			addOrReplaceEntry(entries, index, "word/numbering.xml", numXML)
			ensureContentTypeOverride(entries, index, "/word/numbering.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml")
		} else {
			numIdx := index["word/numbering.xml"]
			(*entries)[numIdx].data = injectNumberingDefs((*entries)[numIdx].data, maxAbstractID+1, bulletNumID, b.usedBullets, b.numberedIDs)
		}
	}

	for _, m := range b.media {
		addOrReplaceEntry(entries, index, m.zipPath, m.data)
		newRels = append(newRels, relEntry{id: m.relID, relType: relTypeImage, target: m.target})
		if m.ext == "png" {
			ensureContentTypeDefault(entries, index, "png", "image/png")
		} else if m.ext == "jpeg" {
			ensureContentTypeDefault(entries, index, "jpeg", "image/jpeg")
		}
	}

	if len(newRels) > 0 {
		ensureRelationships(entries, index, newRels)
	}
	return nil
}

func findBodyInsertOffset(docXML []byte) (int, error) {
	dec := xml.NewDecoder(bytes.NewReader(docXML))
	depth := 0
	bodyDepth := -1
	sectPrStart := -1
	bodyCloseStart := -1

	for {
		before := int(dec.InputOffset())
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "body" && bodyDepth < 0 {
				bodyDepth = depth
			} else if bodyDepth > 0 && depth == bodyDepth+1 && t.Name.Local == "sectPr" {
				sectPrStart = findTagStart(docXML, before)
			}
		case xml.EndElement:
			if t.Name.Local == "body" && depth == bodyDepth {
				bodyCloseStart = findTagStart(docXML, before)
			}
			depth--
		}
	}

	if sectPrStart >= 0 {
		return sectPrStart, nil
	}
	if bodyCloseStart >= 0 {
		return bodyCloseStart, nil
	}
	return 0, errors.New("malformed word/document.xml: missing </w:body>")
}

func addOrReplaceEntry(entries *[]zipEntry, index map[string]int, name string, data []byte) {
	if idx, ok := index[name]; ok {
		(*entries)[idx].data = data
		return
	}
	index[name] = len(*entries)
	*entries = append(*entries, zipEntry{name: name, method: zip.Deflate, data: data})
}

func ensureRelationships(entries *[]zipEntry, index map[string]int, rels []relEntry) {
	relPath := "word/_rels/document.xml.rels"
	idx, ok := index[relPath]
	if !ok {
		addOrReplaceEntry(entries, index, relPath, buildDocRelsXML(rels))
		return
	}
	var snippet strings.Builder
	for _, r := range rels {
		snippet.WriteString(fmt.Sprintf(`<Relationship Id="%s" Type="%s" Target="%s"/>`, escapeXML(r.id), escapeXML(r.relType), escapeXML(r.target)))
	}
	data := (*entries)[idx].data
	closeIdx := bytes.LastIndex(data, []byte("</Relationships>"))
	if closeIdx < 0 {
		return
	}
	(*entries)[idx].data = applyPatches(data, []bytePatch{{
		start: closeIdx,
		end:   closeIdx,
		repl:  []byte(snippet.String()),
	}})
}

func ensureContentTypeDefault(entries *[]zipEntry, index map[string]int, ext, contentType string) {
	idx, ok := index["[Content_Types].xml"]
	if !ok {
		return
	}
	data := (*entries)[idx].data
	if bytes.Contains(bytes.ToLower(data), []byte(fmt.Sprintf(`extension="%s"`, strings.ToLower(ext)))) {
		return
	}
	closeIdx := bytes.LastIndex(data, []byte("</Types>"))
	if closeIdx < 0 {
		return
	}
	snippet := fmt.Sprintf(`<Default Extension="%s" ContentType="%s"/>`, escapeXML(ext), escapeXML(contentType))
	(*entries)[idx].data = applyPatches(data, []bytePatch{{
		start: closeIdx,
		end:   closeIdx,
		repl:  []byte(snippet),
	}})
}

func ensureContentTypeOverride(entries *[]zipEntry, index map[string]int, partName, contentType string) {
	idx, ok := index["[Content_Types].xml"]
	if !ok {
		return
	}
	data := (*entries)[idx].data
	if bytes.Contains(data, []byte(fmt.Sprintf(`PartName="%s"`, partName))) {
		return
	}
	closeIdx := bytes.LastIndex(data, []byte("</Types>"))
	if closeIdx < 0 {
		return
	}
	snippet := fmt.Sprintf(`<Override PartName="%s" ContentType="%s"/>`, escapeXML(partName), escapeXML(contentType))
	(*entries)[idx].data = applyPatches(data, []bytePatch{{
		start: closeIdx,
		end:   closeIdx,
		repl:  []byte(snippet),
	}})
}

func buildAppendedNumberingPart(bulletNumID int, numberedIDs []int) []byte {
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	out.WriteString(`<w:numbering xmlns:w="` + nsWordML + `">`)
	out.WriteString(`<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>`)
	out.WriteString(`<w:abstractNum w:abstractNumId="1"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>`)
	out.WriteString(fmt.Sprintf(`<w:num w:numId="%d"><w:abstractNumId w:val="0"/></w:num>`, bulletNumID))
	for _, id := range numberedIDs {
		out.WriteString(fmt.Sprintf(`<w:num w:numId="%d"><w:abstractNumId w:val="1"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/></w:lvlOverride></w:num>`, id))
	}
	out.WriteString(`</w:numbering>`)
	return []byte(out.String())
}

func injectNumberingDefs(existing []byte, startAbstractID, bulletNumID int, usedBullets bool, numberedIDs []int) []byte {
	var absDefs strings.Builder
	var numDefs strings.Builder
	curAbs := startAbstractID
	if usedBullets {
		absID := curAbs
		curAbs++
		absDefs.WriteString(fmt.Sprintf(`<w:abstractNum w:abstractNumId="%d"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>`, absID))
		numDefs.WriteString(fmt.Sprintf(`<w:num w:numId="%d"><w:abstractNumId w:val="%d"/></w:num>`, bulletNumID, absID))
	}
	if len(numberedIDs) > 0 {
		absID := curAbs
		absDefs.WriteString(fmt.Sprintf(`<w:abstractNum w:abstractNumId="%d"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%%1."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>`, absID))
		for _, id := range numberedIDs {
			numDefs.WriteString(fmt.Sprintf(`<w:num w:numId="%d"><w:abstractNumId w:val="%d"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/></w:lvlOverride></w:num>`, id, absID))
		}
	}

	out := existing
	if absDefs.Len() > 0 {
		firstNum := bytes.Index(out, []byte("<w:num "))
		if firstNum >= 0 {
			out = applyPatches(out, []bytePatch{{start: firstNum, end: firstNum, repl: []byte(absDefs.String())}})
		} else if closeIdx := bytes.LastIndex(out, []byte("</w:numbering>")); closeIdx >= 0 {
			out = applyPatches(out, []bytePatch{{start: closeIdx, end: closeIdx, repl: []byte(absDefs.String())}})
		}
	}
	if numDefs.Len() > 0 {
		if closeIdx := bytes.LastIndex(out, []byte("</w:numbering>")); closeIdx >= 0 {
			out = applyPatches(out, []bytePatch{{start: closeIdx, end: closeIdx, repl: []byte(numDefs.String())}})
		}
	}
	return out
}
