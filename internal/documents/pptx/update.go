package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
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

type drawingTextSegment struct {
	start int
	end   int
	text  string
	rPr   []byte
}

type drawingParaFrame struct {
	groups   [][]drawingTextSegment
	curGroup []drawingTextSegment
}

func (f *drawingParaFrame) flushGroup() {
	if len(f.curGroup) > 0 {
		f.groups = append(f.groups, f.curGroup)
		f.curGroup = nil
	}
}

type textMatch struct {
	start int
	end   int
	repl  string
}

var (
	relIDNumberPattern    = regexp.MustCompile(`^rId(\d+)$`)
	selfClosingSldIdLstRe = regexp.MustCompile(`<([A-Za-z0-9_]+:)?sldIdLst\s*/>`)
	typesOpenTagRe        = regexp.MustCompile(`<Types\b[^>]*>`)
)

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

	if _, ok := index["ppt/presentation.xml"]; !ok {
		return nil, errInvalidPptx
	}

	totalReplacements := 0
	if len(opts.Replacements) > 0 {
		var targetNames []string
		for _, e := range entries {
			isSlide := strings.HasPrefix(e.name, "ppt/slides/slide") && strings.HasSuffix(e.name, ".xml")
			isNotes := strings.HasPrefix(e.name, "ppt/notesSlides/notesSlide") && strings.HasSuffix(e.name, ".xml")
			if isSlide || isNotes {
				targetNames = append(targetNames, e.name)
			}
		}
		sort.Strings(targetNames)
		for _, name := range targetNames {
			i := index[name]
			updated, count, err := replaceInDrawingXML(entries[i].data, opts.Replacements)
			if err != nil {
				return nil, fmt.Errorf("replace text in %s: %w", name, err)
			}
			if count > 0 {
				entries[i].data = updated
				totalReplacements += count
			}
		}
	}

	if len(opts.NotesUpdates) > 0 {
		if err := applyNotesUpdates(&entries, index, opts.NotesUpdates); err != nil {
			return nil, fmt.Errorf("update speaker notes: %w", err)
		}
	}

	if len(opts.AppendSlides) > 0 {
		if err := appendSlidesToArchive(&entries, index, opts); err != nil {
			return nil, fmt.Errorf("append slides: %w", err)
		}
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
		return nil, fmt.Errorf("write updated pptx: %w", err)
	}

	fi, err := os.Stat(outPath)
	if err != nil {
		return nil, fmt.Errorf("stat updated pptx: %w", err)
	}

	slideCount := countSlidesInArchive(entries, index)
	return &Result{
		Path:                outPath,
		SizeBytes:           fi.Size(),
		SlideCount:          slideCount,
		ReplacementsApplied: totalReplacements,
	}, nil
}

func readAllZipEntries(pathStr string) ([]zipEntry, map[string]int, error) {
	zr, err := zip.OpenReader(pathStr)
	if err != nil {
		return nil, nil, fmt.Errorf("open pptx zip: %w", err)
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

func upsertEntry(entries *[]zipEntry, index map[string]int, name string, data []byte) {
	norm := strings.TrimPrefix(name, "/")
	if idx, ok := index[norm]; ok {
		(*entries)[idx].data = data
		return
	}
	index[norm] = len(*entries)
	*entries = append(*entries, zipEntry{
		name:   norm,
		method: zip.Deflate,
		data:   data,
	})
}

func getOrderedSlidePathsFromArchive(entries []zipEntry, index map[string]int) ([]string, presentationInfo, error) {
	presIdx, ok := index["ppt/presentation.xml"]
	if !ok {
		return nil, presentationInfo{}, errInvalidPptx
	}
	presInfo, err := parsePresentationXML(entries[presIdx].data)
	if err != nil {
		return nil, presentationInfo{}, err
	}
	var presRels []relDef
	if rIdx, ok := index["ppt/_rels/presentation.xml.rels"]; ok {
		presRels = parseRelsXML(entries[rIdx].data)
	}
	paths := resolveOrderedSlidePaths(presInfo.slideRelIDs, presRels, func(name string) bool {
		_, exists := index[name]
		return exists
	}, func() []string {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.name)
		}
		return names
	})
	return paths, presInfo, nil
}

func countSlidesInArchive(entries []zipEntry, index map[string]int) int {
	paths, _, err := getOrderedSlidePathsFromArchive(entries, index)
	if err != nil {
		return 0
	}
	return len(paths)
}

func isDrawingMLTag(name xml.Name, defaultNS string) bool {
	if name.Space == "a" {
		return true
	}
	return name.Space == "" && defaultNS == nsA
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

func skipRawSubtree(dec *xml.Decoder) error {
	depth := 1
	for depth > 0 {
		tok, err := dec.RawToken()
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

func replaceInDrawingXML(xmlBytes []byte, replacements map[string]string) ([]byte, int, error) {
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
	var paraStack []drawingParaFrame
	var allGroups [][]drawingTextSegment
	nsStack := []string{""}

	inRun := false
	var curRPr []byte

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
			curNS := nsStack[len(nsStack)-1]
			for _, a := range t.Attr {
				if a.Name.Space == "" && a.Name.Local == "xmlns" {
					curNS = a.Value
				}
			}
			nsStack = append(nsStack, curNS)

			if !isDrawingMLTag(t.Name, curNS) {
				continue
			}

			switch t.Name.Local {
			case "p":
				if len(paraStack) > 0 {
					paraStack[len(paraStack)-1].flushGroup()
				}
				paraStack = append(paraStack, drawingParaFrame{})
			case "br", "tab":
				if len(paraStack) > 0 {
					paraStack[len(paraStack)-1].flushGroup()
				}
			case "fld":
				if len(paraStack) > 0 {
					paraStack[len(paraStack)-1].flushGroup()
				}
				if err := skipRawSubtree(dec); err != nil {
					return nil, 0, err
				}
				nsStack = nsStack[:len(nsStack)-1]
			case "r":
				if len(paraStack) > 0 {
					inRun = true
					curRPr = nil
				}
			case "rPr":
				if inRun {
					rPrStart := findTagStart(xmlBytes, before)
					if err := skipRawSubtree(dec); err != nil {
						return nil, 0, err
					}
					rPrEnd := int(dec.InputOffset())
					curRPr = xmlBytes[rPrStart:rPrEnd]
					nsStack = nsStack[:len(nsStack)-1]
				}
			case "t":
				if len(paraStack) > 0 && inRun {
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
					nsStack = nsStack[:len(nsStack)-1]
					top := &paraStack[len(paraStack)-1]
					var rPrCopy []byte
					if len(curRPr) > 0 {
						rPrCopy = append([]byte(nil), curRPr...)
					}
					top.curGroup = append(top.curGroup, drawingTextSegment{
						start: tStart,
						end:   tEnd,
						text:  textBuf.String(),
						rPr:   rPrCopy,
					})
				}
			}

		case xml.EndElement:
			curNS := ""
			if len(nsStack) > 0 {
				curNS = nsStack[len(nsStack)-1]
				if len(nsStack) > 1 {
					nsStack = nsStack[:len(nsStack)-1]
				}
			}
			if isDrawingMLTag(t.Name, curNS) {
				switch t.Name.Local {
				case "r":
					inRun = false
					curRPr = nil
				case "p":
					if len(paraStack) > 0 {
						top := &paraStack[len(paraStack)-1]
						top.flushGroup()
						allGroups = append(allGroups, top.groups...)
						paraStack = paraStack[:len(paraStack)-1]
					}
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
					repl:  formatDrawingTReplacement(newText, seg.rPr),
				})
			}
		}
	}

	if len(patches) == 0 {
		return xmlBytes, 0, nil
	}
	return applyPatches(xmlBytes, patches), totalApplied, nil
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

func formatDrawingTReplacement(text string, rPr []byte) []byte {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.Split(normalized, "\n")
	if len(parts) == 1 {
		return []byte(`<a:t>` + escapeXML(normalized) + `</a:t>`)
	}
	var b bytes.Buffer
	for i, part := range parts {
		if i > 0 {
			b.WriteString(`</a:r><a:br/><a:r>`)
			if len(rPr) > 0 {
				b.Write(rPr)
			}
		}
		b.WriteString(`<a:t>`)
		b.WriteString(escapeXML(part))
		b.WriteString(`</a:t>`)
	}
	return b.Bytes()
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

func applyNotesUpdates(entries *[]zipEntry, index map[string]int, updates []NotesUpdate) error {
	slidePaths, _, err := getOrderedSlidePathsFromArchive(*entries, index)
	if err != nil {
		return err
	}

	nextNotesNum := maxPartNumber(*entries, "ppt/notesSlides/notesSlide", ".xml") + 1
	var newOverrides []struct {
		partName    string
		contentType string
	}

	for _, u := range updates {
		if u.Slide < 1 || u.Slide > len(slidePaths) {
			return fmt.Errorf("slide %d out of range (presentation has %d slides)", u.Slide, len(slidePaths))
		}
		slidePath := slidePaths[u.Slide-1]
		slideRelsPath := relsPathForPart(slidePath)

		var existingNotesPath string
		if rIdx, ok := index[slideRelsPath]; ok {
			for _, r := range parseRelsXML((*entries)[rIdx].data) {
				if r.relType == relTypeNotesSlide {
					candidate := resolveRelTarget(slidePath, r.target)
					if _, exists := index[candidate]; exists {
						existingNotesPath = candidate
					}
					break
				}
			}
		}

		if existingNotesPath != "" {
			nIdx := index[existingNotesPath]
			(*entries)[nIdx].data = updateNotesBodyXML((*entries)[nIdx].data, u.Notes)
			continue
		}

		notesMasterPath, createdMaster, err := ensureNotesMaster(entries, index)
		if err != nil {
			return err
		}
		if createdMaster {
			newOverrides = append(newOverrides, struct {
				partName    string
				contentType string
			}{"/" + notesMasterPath, ctNotesMaster})
		}

		notesNum := nextNotesNum
		nextNotesNum++
		notesPart := fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", notesNum)
		notesRelsPart := relsPathForPart(notesPart)

		notesMasterRelTarget := relativeTargetFromDir("ppt/notesSlides", notesMasterPath)
		slideRelTarget := relativeTargetFromDir("ppt/notesSlides", slidePath)

		upsertEntry(entries, index, notesPart, buildNotesSlideXML(u.Notes, defaultFontFamily))
		upsertEntry(entries, index, notesRelsPart, buildRelsXML([]relDef{
			{id: "rId1", relType: relTypeNotesMaster, target: notesMasterRelTarget},
			{id: "rId2", relType: relTypeSlide, target: slideRelTarget},
		}))

		notesTargetFromSlide := relativeTargetFromDir(path.Dir(slidePath), notesPart)
		if err := addRelationshipToPart(entries, index, slideRelsPath, relTypeNotesSlide, notesTargetFromSlide); err != nil {
			return err
		}

		newOverrides = append(newOverrides, struct {
			partName    string
			contentType string
		}{"/" + notesPart, ctNotesSlide})
	}

	if len(newOverrides) > 0 {
		if ctIdx, ok := index["[Content_Types].xml"]; ok {
			(*entries)[ctIdx].data = ensureContentTypes((*entries)[ctIdx].data, nil, newOverrides)
		}
	}
	return nil
}

func updateNotesBodyXML(xmlBytes []byte, notes string) []byte {
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	inShape := false
	shapeDepth := 0
	isBodyShape := false
	txStart := -1
	txEnd := -1
	shapeCloseStart := -1

	for {
		before := int(dec.InputOffset())
		tok, err := dec.RawToken()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if !inShape && t.Name.Local == "sp" && (t.Name.Space == "p" || t.Name.Space == "") {
				inShape = true
				shapeDepth = 1
				isBodyShape = false
				txStart = -1
				txEnd = -1
				shapeCloseStart = -1
				continue
			}
			if inShape {
				shapeDepth++
				if t.Name.Local == "ph" && (t.Name.Space == "p" || t.Name.Space == "") {
					tp := attrVal(t.Attr, "type")
					idx := attrVal(t.Attr, "idx")
					if tp == "body" || (tp == "" && idx == "1") {
						isBodyShape = true
					}
				}
				if t.Name.Local == "txBody" && (t.Name.Space == "p" || t.Name.Space == "") && txStart < 0 {
					txStart = findTagStart(xmlBytes, before)
					if err := skipRawSubtree(dec); err != nil {
						return buildNotesSlideXML(notes, defaultFontFamily)
					}
					txEnd = int(dec.InputOffset())
					shapeDepth--
				}
			}
		case xml.EndElement:
			if inShape {
				shapeDepth--
				if shapeDepth == 0 {
					shapeCloseStart = findTagStart(xmlBytes, before)
					if isBodyShape {
						replacement := []byte(buildNotesTxBody(notes, defaultFontFamily))
						if txStart >= 0 && txEnd > txStart {
							return applyPatches(xmlBytes, []bytePatch{{start: txStart, end: txEnd, repl: replacement}})
						}
						if shapeCloseStart >= 0 {
							return applyPatches(xmlBytes, []bytePatch{{start: shapeCloseStart, end: shapeCloseStart, repl: replacement}})
						}
					}
					inShape = false
				}
			}
		}
	}
	return buildNotesSlideXML(notes, defaultFontFamily)
}

func appendSlidesToArchive(entries *[]zipEntry, index map[string]int, opts UpdateOptions) error {
	existingSlides, presInfo, err := getOrderedSlidePathsFromArchive(*entries, index)
	if err != nil {
		return err
	}

	wEMU := presInfo.widthEMU
	hEMU := presInfo.heightEMU
	if wEMU <= 0 || hEMU <= 0 {
		wEMU = slide16x9WidthEMU
		hEMU = slide16x9HeightEMU
	}

	theme := resolveTheme(opts.PrimaryColor, opts.AccentColor, opts.BackgroundColor, opts.TextColor, "")
	layoutRelTarget := detectSlideLayoutTarget(*entries, index, existingSlides)

	nextSlideNum := maxPartNumber(*entries, "ppt/slides/slide", ".xml") + 1
	nextNotesNum := maxPartNumber(*entries, "ppt/notesSlides/notesSlide", ".xml") + 1
	nextImageIdx := maxMediaNumber(*entries) + 1
	nextSldID := max(presInfo.maxSlideID+1, 256)

	renderer := &slideRenderer{
		workingDir:   opts.WorkingDir,
		theme:        theme,
		widthEMU:     wEMU,
		heightEMU:    hEMU,
		slideNumbers: opts.SlideNumbers,
		nextImageIdx: nextImageIdx,
	}

	var newOverrides []struct {
		partName    string
		contentType string
	}
	var newDefaults []struct {
		ext         string
		contentType string
	}
	var newSlideTags strings.Builder

	totalExisting := len(existingSlides)
	for i, spec := range opts.AppendSlides {
		slideNum := nextSlideNum
		nextSlideNum++
		displayNum := totalExisting + i + 1

		notesNum := 0
		notesMasterRelTarget := "../notesMasters/notesMaster1.xml"
		if strings.TrimSpace(spec.Notes) != "" {
			notesMasterPath, createdMaster, err := ensureNotesMaster(entries, index)
			if err != nil {
				return err
			}
			if createdMaster {
				newOverrides = append(newOverrides, struct {
					partName    string
					contentType string
				}{"/" + notesMasterPath, ctNotesMaster})
			}
			notesMasterRelTarget = relativeTargetFromDir("ppt/notesSlides", notesMasterPath)
			notesNum = nextNotesNum
			nextNotesNum++
		}

		slidePart := fmt.Sprintf("ppt/slides/slide%d.xml", slideNum)
		bs, err := renderer.renderSlide(spec, displayNum, layoutRelTarget, notesMasterRelTarget, notesNum, fmt.Sprintf("../slides/slide%d.xml", slideNum))
		if err != nil {
			return fmt.Errorf("render appended slide %d: %w", i+1, err)
		}

		upsertEntry(entries, index, slidePart, bs.xml)
		upsertEntry(entries, index, relsPathForPart(slidePart), buildRelsXML(bs.rels))
		newOverrides = append(newOverrides, struct {
			partName    string
			contentType string
		}{"/" + slidePart, ctSlide})

		if len(bs.notesXML) > 0 {
			notesPart := fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", notesNum)
			upsertEntry(entries, index, notesPart, bs.notesXML)
			upsertEntry(entries, index, relsPathForPart(notesPart), buildRelsXML(bs.notesRels))
			newOverrides = append(newOverrides, struct {
				partName    string
				contentType string
			}{"/" + notesPart, ctNotesSlide})
		}

		for _, m := range bs.media {
			upsertEntry(entries, index, m.zipPath, m.data)
			if m.ext == "png" {
				newDefaults = append(newDefaults, struct {
					ext         string
					contentType string
				}{"png", "image/png"})
			} else if m.ext == "jpeg" {
				newDefaults = append(newDefaults, struct {
					ext         string
					contentType string
				}{"jpeg", "image/jpeg"})
			}
		}

		relID, err := addRelationshipAndReturnID(entries, index, "ppt/_rels/presentation.xml.rels", relTypeSlide, fmt.Sprintf("slides/slide%d.xml", slideNum))
		if err != nil {
			return err
		}
		fmt.Fprintf(&newSlideTags, `<p:sldId id="%d" r:id="%s"/>`, nextSldID, relID)
		nextSldID++
	}

	presIdx := index["ppt/presentation.xml"]
	(*entries)[presIdx].data = insertSlideIDsIntoPresentation((*entries)[presIdx].data, newSlideTags.String())

	if ctIdx, ok := index["[Content_Types].xml"]; ok {
		(*entries)[ctIdx].data = ensureContentTypes((*entries)[ctIdx].data, newDefaults, newOverrides)
	}
	return nil
}

func detectSlideLayoutTarget(entries []zipEntry, index map[string]int, existingSlides []string) string {
	for _, e := range entries {
		if strings.HasPrefix(e.name, "ppt/slideLayouts/") && strings.HasSuffix(e.name, ".xml") {
			if bytes.Contains(e.data, []byte(`type="blank"`)) {
				return relativeTargetFromDir("ppt/slides", e.name)
			}
		}
	}
	for _, sp := range existingSlides {
		if rIdx, ok := index[relsPathForPart(sp)]; ok {
			for _, r := range parseRelsXML(entries[rIdx].data) {
				if r.relType == relTypeSlideLayout && r.target != "" {
					return r.target
				}
			}
		}
	}
	for _, e := range entries {
		if strings.HasPrefix(e.name, "ppt/slideLayouts/") && strings.HasSuffix(e.name, ".xml") {
			return relativeTargetFromDir("ppt/slides", e.name)
		}
	}
	return "../slideLayouts/slideLayout1.xml"
}

func ensureNotesMaster(entries *[]zipEntry, index map[string]int) (string, bool, error) {
	for _, e := range *entries {
		if strings.HasPrefix(e.name, "ppt/notesMasters/") && strings.HasSuffix(e.name, ".xml") {
			return e.name, false, nil
		}
	}
	notesMasterPath := "ppt/notesMasters/notesMaster1.xml"
	themeTarget := "../theme/theme1.xml"
	for _, e := range *entries {
		if strings.HasPrefix(e.name, "ppt/theme/") && strings.HasSuffix(e.name, ".xml") {
			themeTarget = relativeTargetFromDir("ppt/notesMasters", e.name)
			break
		}
	}
	upsertEntry(entries, index, notesMasterPath, buildNotesMasterXML())
	upsertEntry(entries, index, relsPathForPart(notesMasterPath), buildRelsXML([]relDef{
		{id: "rId1", relType: relTypeTheme, target: themeTarget},
	}))

	relID, err := addRelationshipAndReturnID(entries, index, "ppt/_rels/presentation.xml.rels", relTypeNotesMaster, "notesMasters/notesMaster1.xml")
	if err != nil {
		return "", false, err
	}

	presIdx := index["ppt/presentation.xml"]
	(*entries)[presIdx].data = insertNotesMasterIntoPresentation((*entries)[presIdx].data, relID)
	return notesMasterPath, true, nil
}

func addRelationshipToPart(entries *[]zipEntry, index map[string]int, relsPath, relType, target string) error {
	_, err := addRelationshipAndReturnID(entries, index, relsPath, relType, target)
	return err
}

func addRelationshipAndReturnID(entries *[]zipEntry, index map[string]int, relsPath, relType, target string) (string, error) {
	if rIdx, ok := index[relsPath]; ok {
		existing := parseRelsXML((*entries)[rIdx].data)
		maxID := 0
		for _, r := range existing {
			if m := relIDNumberPattern.FindStringSubmatch(r.id); len(m) == 2 {
				if n, err := strconv.Atoi(m[1]); err == nil && n > maxID {
					maxID = n
				}
			}
		}
		newID := fmt.Sprintf("rId%d", maxID+1)
		tag := fmt.Sprintf(`<Relationship Id="%s" Type="%s" Target="%s"/>`, escapeXML(newID), escapeXML(relType), escapeXML(target))
		raw := (*entries)[rIdx].data
		closeTag := []byte("</Relationships>")
		if pos := bytes.LastIndex(raw, closeTag); pos >= 0 {
			updated := make([]byte, 0, len(raw)+len(tag))
			updated = append(updated, raw[:pos]...)
			updated = append(updated, tag...)
			updated = append(updated, raw[pos:]...)
			(*entries)[rIdx].data = updated
			return newID, nil
		}
		existing = append(existing, relDef{id: newID, relType: relType, target: target})
		(*entries)[rIdx].data = buildRelsXML(existing)
		return newID, nil
	}

	newID := "rId1"
	upsertEntry(entries, index, relsPath, buildRelsXML([]relDef{
		{id: newID, relType: relType, target: target},
	}))
	return newID, nil
}

func insertSlideIDsIntoPresentation(presXML []byte, sldIDTags string) []byte {
	if sldIDTags == "" {
		return presXML
	}
	for _, closeTag := range []string{"</p:sldIdLst>", "</sldIdLst>"} {
		if pos := bytes.LastIndex(presXML, []byte(closeTag)); pos >= 0 {
			out := make([]byte, 0, len(presXML)+len(sldIDTags))
			out = append(out, presXML[:pos]...)
			out = append(out, sldIDTags...)
			out = append(out, presXML[pos:]...)
			return out
		}
	}
	if loc := selfClosingSldIdLstRe.FindIndex(presXML); loc != nil {
		replacement := "<p:sldIdLst>" + sldIDTags + "</p:sldIdLst>"
		return applyPatches(presXML, []bytePatch{{start: loc[0], end: loc[1], repl: []byte(replacement)}})
	}
	block := "<p:sldIdLst>" + sldIDTags + "</p:sldIdLst>"
	for _, anchor := range []string{"<p:sldSz", "<sldSz", "</p:presentation>", "</presentation>"} {
		if pos := bytes.Index(presXML, []byte(anchor)); pos >= 0 {
			out := make([]byte, 0, len(presXML)+len(block))
			out = append(out, presXML[:pos]...)
			out = append(out, block...)
			out = append(out, presXML[pos:]...)
			return out
		}
	}
	return presXML
}

func insertNotesMasterIntoPresentation(presXML []byte, relID string) []byte {
	if bytes.Contains(presXML, []byte("notesMasterIdLst")) {
		return presXML
	}
	block := fmt.Sprintf(`<p:notesMasterIdLst><p:notesMasterId r:id="%s"/></p:notesMasterIdLst>`, escapeXML(relID))
	for _, afterTag := range []string{"</p:sldMasterIdLst>", "</sldMasterIdLst>"} {
		if pos := bytes.Index(presXML, []byte(afterTag)); pos >= 0 {
			insertAt := pos + len(afterTag)
			out := make([]byte, 0, len(presXML)+len(block))
			out = append(out, presXML[:insertAt]...)
			out = append(out, block...)
			out = append(out, presXML[insertAt:]...)
			return out
		}
	}
	for _, beforeTag := range []string{"<p:sldIdLst", "<sldIdLst", "<p:sldSz", "<sldSz"} {
		if pos := bytes.Index(presXML, []byte(beforeTag)); pos >= 0 {
			out := make([]byte, 0, len(presXML)+len(block))
			out = append(out, presXML[:pos]...)
			out = append(out, block...)
			out = append(out, presXML[pos:]...)
			return out
		}
	}
	return presXML
}

func ensureContentTypes(ctXML []byte, defaults []struct {
	ext         string
	contentType string
}, overrides []struct {
	partName    string
	contentType string
}) []byte {
	existingExt := make(map[string]struct{})
	existingParts := make(map[string]struct{})
	dec := xml.NewDecoder(bytes.NewReader(ctXML))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "Default":
			if ext := strings.ToLower(attrVal(se.Attr, "Extension")); ext != "" {
				existingExt[ext] = struct{}{}
			}
		case "Override":
			if pn := attrVal(se.Attr, "PartName"); pn != "" {
				existingParts[pn] = struct{}{}
			}
		}
	}

	var defaultTags strings.Builder
	for _, d := range defaults {
		ext := strings.ToLower(strings.TrimSpace(d.ext))
		if ext == "" {
			continue
		}
		if _, ok := existingExt[ext]; ok {
			continue
		}
		existingExt[ext] = struct{}{}
		fmt.Fprintf(&defaultTags, `<Default Extension="%s" ContentType="%s"/>`, escapeXML(ext), escapeXML(d.contentType))
	}

	var overrideTags strings.Builder
	for _, o := range overrides {
		pn := strings.TrimSpace(o.partName)
		if pn == "" {
			continue
		}
		if !strings.HasPrefix(pn, "/") {
			pn = "/" + pn
		}
		if _, ok := existingParts[pn]; ok {
			continue
		}
		existingParts[pn] = struct{}{}
		fmt.Fprintf(&overrideTags, `<Override PartName="%s" ContentType="%s"/>`, escapeXML(pn), escapeXML(o.contentType))
	}

	out := ctXML
	if defaultTags.Len() > 0 {
		if loc := typesOpenTagRe.FindIndex(out); loc != nil {
			insertAt := loc[1]
			next := make([]byte, 0, len(out)+defaultTags.Len())
			next = append(next, out[:insertAt]...)
			next = append(next, defaultTags.String()...)
			next = append(next, out[insertAt:]...)
			out = next
		}
	}
	if overrideTags.Len() > 0 {
		closeTag := []byte("</Types>")
		if pos := bytes.LastIndex(out, closeTag); pos >= 0 {
			next := make([]byte, 0, len(out)+overrideTags.Len())
			next = append(next, out[:pos]...)
			next = append(next, overrideTags.String()...)
			next = append(next, out[pos:]...)
			out = next
		}
	}
	return out
}

func maxPartNumber(entries []zipEntry, prefix, suffix string) int {
	maxNum := 0
	for _, e := range entries {
		if strings.HasPrefix(e.name, prefix) && strings.HasSuffix(e.name, suffix) {
			mid := strings.TrimSuffix(strings.TrimPrefix(e.name, prefix), suffix)
			if n, err := strconv.Atoi(mid); err == nil && n > maxNum {
				maxNum = n
			}
		}
	}
	return maxNum
}

func maxMediaNumber(entries []zipEntry) int {
	maxNum := 0
	for _, e := range entries {
		if strings.HasPrefix(e.name, "ppt/media/image") {
			if n := extractTrailingNumber(e.name); n > maxNum {
				maxNum = n
			}
		}
	}
	return maxNum
}

func relativeTargetFromDir(fromDir, targetPath string) string {
	fromClean := strings.Trim(path.Clean(fromDir), "/")
	toClean := strings.Trim(path.Clean(targetPath), "/")
	fromParts := strings.Split(fromClean, "/")
	toParts := strings.Split(toClean, "/")

	common := 0
	for common < len(fromParts) && common < len(toParts) && fromParts[common] == toParts[common] {
		common++
	}
	var rel []string
	for i := common; i < len(fromParts); i++ {
		rel = append(rel, "..")
	}
	rel = append(rel, toParts[common:]...)
	return strings.Join(rel, "/")
}
