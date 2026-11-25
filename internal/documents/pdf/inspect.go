package pdf

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/SpherePrime/CLI/vendordeps/pdfcpu/pdfcpu/pkg/api"
	"github.com/SpherePrime/CLI/vendordeps/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/SpherePrime/CLI/vendordeps/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

type fontDecoder struct {
	codeLen   int
	twoByte   bool
	toUnicode map[uint32]string
	encoding  string
}

func Inspect(path string, pageRange string) (*InspectResult, error) {
	resolved, err := resolvePath("", path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	stat, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("stat pdf %s: %w", resolved, err)
	}

	f, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("open pdf %s: %w", resolved, err)
	}
	defer f.Close()

	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	bgCtx := context.Background()

	pdfCtx, err := api.ReadAndValidate(bgCtx, f, conf)
	if err != nil {
		return nil, fmt.Errorf("read pdf %s: %w", resolved, err)
	}

	selected, err := api.PagesForSelection(pdfCtx.PageCount, parsePageSelection(pageRange), true)
	if err != nil {
		return nil, fmt.Errorf("parse page_range %q: %w", pageRange, err)
	}

	pageNums := make([]int, 0, len(selected))
	for p, ok := range selected {
		if ok {
			pageNums = append(pageNums, p)
		}
	}
	sort.Ints(pageNums)

	pages := make([]PageInfo, 0, len(pageNums))
	for _, pageNr := range pageNums {
		pageDict, _, inhPAttrs, err := pdfCtx.PageDict(bgCtx, pageNr, true)
		if err != nil {
			return nil, fmt.Errorf("read page %d: %w", pageNr, err)
		}

		width, height := pageDimensions(inhPAttrs)
		rot := 0
		if inhPAttrs != nil {
			rot = ((inhPAttrs.Rotate % 360) + 360) % 360
		}

		resDict := pageResources(pdfCtx, pageDict, inhPAttrs)
		fonts := loadPageFonts(pdfCtx, resDict)
		content, err := pdfCtx.PageContent(pageDict, pageNr)
		if err != nil && err != model.ErrNoContent {
			return nil, fmt.Errorf("extract page %d content: %w", pageNr, err)
		}

		text := extractStreamText(pdfCtx, content, resDict, fonts, 0)
		pages = append(pages, PageInfo{
			Number: pageNr,
			Width:  width,
			Height: height,
			Rotate: rot,
			Text:   text,
		})
	}

	return &InspectResult{
		Path:      resolved,
		PageCount: pdfCtx.PageCount,
		SizeBytes: stat.Size(),
		Metadata:  extractMetadata(pdfCtx),
		Pages:     pages,
	}, nil
}

func extractMetadata(pdfCtx *model.Context) Metadata {
	meta := Metadata{
		Title:    strings.TrimSpace(pdfCtx.Title),
		Author:   strings.TrimSpace(pdfCtx.Author),
		Subject:  strings.TrimSpace(pdfCtx.Subject),
		Creator:  strings.TrimSpace(pdfCtx.Creator),
		Producer: strings.TrimSpace(pdfCtx.Producer),
	}
	if pdfCtx.Info == nil {
		return meta
	}
	d, err := pdfCtx.DereferenceDict(*pdfCtx.Info)
	if err != nil || d == nil {
		return meta
	}
	readField := func(key, current string) string {
		if current != "" {
			return current
		}
		obj, ok := d.Find(key)
		if !ok || obj == nil {
			return ""
		}
		val, err := pdfCtx.DereferenceText(obj)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(val)
	}
	meta.Title = readField("Title", meta.Title)
	meta.Author = readField("Author", meta.Author)
	meta.Subject = readField("Subject", meta.Subject)
	meta.Creator = readField("Creator", meta.Creator)
	meta.Producer = readField("Producer", meta.Producer)
	return meta
}

func pageDimensions(attrs *model.InheritedPageAttrs) (float64, float64) {
	if attrs == nil {
		return 0, 0
	}
	rect := attrs.CropBox
	if rect == nil {
		rect = attrs.MediaBox
	}
	if rect == nil {
		return 0, 0
	}
	return rect.Width(), rect.Height()
}

func pageResources(pdfCtx *model.Context, pageDict types.Dict, attrs *model.InheritedPageAttrs) types.Dict {
	if attrs != nil && len(attrs.Resources) > 0 {
		return attrs.Resources
	}
	if pageDict == nil {
		return nil
	}
	obj, ok := pageDict.Find("Resources")
	if !ok || obj == nil {
		return nil
	}
	d, err := pdfCtx.DereferenceDict(obj)
	if err != nil {
		return nil
	}
	return d
}

func loadPageFonts(pdfCtx *model.Context, resDict types.Dict) map[string]*fontDecoder {
	fonts := make(map[string]*fontDecoder)
	if resDict == nil {
		return fonts
	}
	fontObj, ok := resDict.Find("Font")
	if !ok || fontObj == nil {
		return fonts
	}
	fontDict, err := pdfCtx.DereferenceDict(fontObj)
	if err != nil || fontDict == nil {
		return fonts
	}

	for name, entry := range fontDict {
		fd, err := pdfCtx.DereferenceDict(entry)
		if err != nil || fd == nil {
			continue
		}
		fonts[name] = buildFontDecoder(pdfCtx, fd)
	}
	return fonts
}

func buildFontDecoder(pdfCtx *model.Context, fd types.Dict) *fontDecoder {
	dec := &fontDecoder{
		codeLen:   1,
		toUnicode: make(map[uint32]string),
	}

	if subtypeObj, ok := fd.Find("Subtype"); ok && subtypeObj != nil {
		if n, ok := subtypeObj.(types.Name); ok && n.Value() == "Type0" {
			dec.twoByte = true
			dec.codeLen = 2
		}
	}

	if encObj, ok := fd.Find("Encoding"); ok && encObj != nil {
		switch v := encObj.(type) {
		case types.Name:
			dec.encoding = v.Value()
			if v.Value() == "Identity-H" || v.Value() == "Identity-V" {
				dec.twoByte = true
				dec.codeLen = 2
			}
		default:
			if ed, err := pdfCtx.DereferenceDict(encObj); err == nil && ed != nil {
				if base, ok := ed.Find("BaseEncoding"); ok {
					if n, ok := base.(types.Name); ok {
						dec.encoding = n.Value()
					}
				}
			}
		}
	}

	if tuObj, ok := fd.Find("ToUnicode"); ok && tuObj != nil {
		if sd, _, err := pdfCtx.DereferenceStreamDict(tuObj); err == nil && sd != nil {
			_ = pdfCtx.DecodeStream(sd)
			data := sd.Content
			if len(data) == 0 {
				data = sd.Raw
			}
			parseToUnicodeCMap(data, dec)
		}
	}

	return dec
}

func parseToUnicodeCMap(data []byte, dec *fontDecoder) {
	tokens := tokenizeCMap(data)
	for i := 0; i < len(tokens); i++ {
		switch tokens[i] {
		case "begincodespacerange":
			for i++; i < len(tokens) && tokens[i] != "endcodespacerange"; i++ {
				if strings.HasPrefix(tokens[i], "<") && i+1 < len(tokens) && strings.HasPrefix(tokens[i+1], "<") {
					lo := decodeHexToken(tokens[i])
					if len(lo) >= 2 {
						dec.codeLen = 2
						dec.twoByte = true
					}
					i++
				}
			}
		case "beginbfchar":
			for i++; i < len(tokens) && tokens[i] != "endbfchar"; i++ {
				if !strings.HasPrefix(tokens[i], "<") || i+1 >= len(tokens) {
					continue
				}
				srcBytes := decodeHexToken(tokens[i])
				dstBytes := decodeHexToken(tokens[i+1])
				i++
				if len(srcBytes) == 0 || len(dstBytes) == 0 {
					continue
				}
				if len(srcBytes) >= 2 {
					dec.codeLen = 2
					dec.twoByte = true
				}
				code := bytesToCode(srcBytes)
				dec.toUnicode[code] = decodeUTF16BE(dstBytes)
			}
		case "beginbfrange":
			for i++; i < len(tokens) && tokens[i] != "endbfrange"; i++ {
				if !strings.HasPrefix(tokens[i], "<") || i+2 >= len(tokens) {
					continue
				}
				loBytes := decodeHexToken(tokens[i])
				hiBytes := decodeHexToken(tokens[i+1])
				if len(loBytes) == 0 || len(hiBytes) == 0 {
					i += 2
					continue
				}
				if len(loBytes) >= 2 {
					dec.codeLen = 2
					dec.twoByte = true
				}
				lo := bytesToCode(loBytes)
				hi := bytesToCode(hiBytes)
				i += 2
				if tokens[i] == "[" {
					cur := lo
					for i++; i < len(tokens) && tokens[i] != "]"; i++ {
						if strings.HasPrefix(tokens[i], "<") && cur <= hi {
							dec.toUnicode[cur] = decodeUTF16BE(decodeHexToken(tokens[i]))
							cur++
						}
					}
				} else if strings.HasPrefix(tokens[i], "<") {
					dst := decodeHexToken(tokens[i])
					if len(dst) == 0 {
						continue
					}
					span := hi - lo
					if span > 65536 {
						span = 65536
					}
					for offset := uint32(0); offset <= span; offset++ {
						dec.toUnicode[lo+offset] = decodeUTF16BEIncrement(dst, offset)
					}
				}
			}
		}
	}
}

func tokenizeCMap(data []byte) []string {
	var tokens []string
	n := len(data)
	for i := 0; i < n; {
		b := data[i]
		if isPDFSpace(b) {
			i++
			continue
		}
		if b == '%' {
			for i < n && data[i] != '\n' && data[i] != '\r' {
				i++
			}
			continue
		}
		if b == '<' {
			if i+1 < n && data[i+1] == '<' {
				tokens = append(tokens, "<<")
				i += 2
				continue
			}
			start := i
			i++
			for i < n && data[i] != '>' {
				i++
			}
			if i < n {
				i++
			}
			tokens = append(tokens, string(data[start:i]))
			continue
		}
		if b == '>' && i+1 < n && data[i+1] == '>' {
			tokens = append(tokens, ">>")
			i += 2
			continue
		}
		if b == '[' || b == ']' {
			tokens = append(tokens, string(b))
			i++
			continue
		}
		start := i
		for i < n && !isPDFSpace(data[i]) && !isPDFDelim(data[i]) {
			i++
		}
		if i == start {
			i++
			continue
		}
		tokens = append(tokens, string(data[start:i]))
	}
	return tokens
}

func decodeHexToken(tok string) []byte {
	s := strings.TrimPrefix(tok, "<")
	s = strings.TrimSuffix(s, ">")
	var cleaned strings.Builder
	for i := 0; i < len(s); i++ {
		if !isPDFSpace(s[i]) {
			cleaned.WriteByte(s[i])
		}
	}
	hs := cleaned.String()
	if len(hs)%2 == 1 {
		hs += "0"
	}
	out, err := hex.DecodeString(hs)
	if err != nil {
		return nil
	}
	return out
}

func bytesToCode(b []byte) uint32 {
	var v uint32
	for _, c := range b {
		v = (v << 8) | uint32(c)
	}
	return v
}

func decodeUTF16BE(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b) == 1 {
		return string(rune(b[0]))
	}
	if len(b)%2 == 1 {
		b = append(b, 0)
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = binary.BigEndian.Uint16(b[2*i : 2*i+2])
	}
	return string(utf16.Decode(u16))
}

func decodeUTF16BEIncrement(base []byte, offset uint32) string {
	if offset == 0 {
		return decodeUTF16BE(base)
	}
	cp := append([]byte(nil), base...)
	if len(cp) >= 2 {
		last := uint32(binary.BigEndian.Uint16(cp[len(cp)-2:])) + offset
		binary.BigEndian.PutUint16(cp[len(cp)-2:], uint16(last))
	} else if len(cp) == 1 {
		cp[0] = byte(uint32(cp[0]) + offset)
	}
	return decodeUTF16BE(cp)
}

func (d *fontDecoder) decode(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if d == nil {
		return decodeWinAnsi(raw)
	}
	if len(d.toUnicode) > 0 {
		var sb strings.Builder
		step := d.codeLen
		if step <= 0 {
			step = 1
		}
		for i := 0; i < len(raw); {
			if step == 2 && i+1 < len(raw) {
				code := uint32(raw[i])<<8 | uint32(raw[i+1])
				if s, ok := d.toUnicode[code]; ok {
					sb.WriteString(s)
					i += 2
					continue
				}
			}
			if s, ok := d.toUnicode[uint32(raw[i])]; ok {
				sb.WriteString(s)
				i++
				continue
			}
			if step == 2 && i+1 < len(raw) {
				i += 2
			} else {
				sb.WriteRune(winAnsiRune(raw[i]))
				i++
			}
		}
		return sb.String()
	}
	if d.twoByte {
		if len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF {
			return decodeUTF16BE(raw[2:])
		}
		return decodeUTF16BE(raw)
	}
	return decodeWinAnsi(raw)
}

func decodeWinAnsi(raw []byte) string {
	if len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF {
		return decodeUTF16BE(raw[2:])
	}
	var sb strings.Builder
	for _, b := range raw {
		sb.WriteRune(winAnsiRune(b))
	}
	return sb.String()
}

var winAnsiHigh = [32]rune{
	0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F,
	0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178,
}

func winAnsiRune(b byte) rune {
	if b >= 0x80 && b <= 0x9F {
		return winAnsiHigh[b-0x80]
	}
	return rune(b)
}

type csTokenKind int

const (
	tokWord csTokenKind = iota
	tokName
	tokString
	tokArray
)

type csToken struct {
	kind  csTokenKind
	text  string
	raw   []byte
	items []csToken
}

func extractStreamText(pdfCtx *model.Context, content []byte, resDict types.Dict, fonts map[string]*fontDecoder, depth int) string {
	if len(content) == 0 || depth > 4 {
		return ""
	}
	tokens := parseContentTokens(content)

	var out strings.Builder
	var stack []csToken
	var curFont *fontDecoder
	inText := false
	curX, curY := 0.0, 0.0
	lastX, lastY := 0.0, 0.0
	hasLastPos := false

	appendNewline := func() {
		s := out.String()
		if len(s) > 0 && !strings.HasSuffix(s, "\n") {
			out.WriteByte('\n')
		}
	}

	appendSpace := func() {
		s := out.String()
		if len(s) > 0 && !strings.HasSuffix(s, " ") && !strings.HasSuffix(s, "\n") && !strings.HasSuffix(s, "\t") {
			out.WriteByte(' ')
		}
	}

	emitPositionBreak := func(newX, newY float64) {
		if hasLastPos {
			dy := math.Abs(newY - lastY)
			if dy > 2.0 {
				appendNewline()
			} else if math.Abs(newX-lastX) > 3.0 {
				appendSpace()
			}
		}
		curX, curY = newX, newY
		lastX, lastY = newX, newY
		hasLastPos = true
	}

	for _, tok := range tokens {
		if tok.kind != tokWord {
			stack = append(stack, tok)
			continue
		}
		if _, err := strconv.ParseFloat(tok.text, 64); err == nil {
			stack = append(stack, tok)
			continue
		}

		switch tok.text {
		case "BT":
			inText = true
			curX, curY = 0, 0
		case "ET":
			inText = false
		case "Tf":
			if len(stack) >= 2 {
				fontTok := stack[len(stack)-2]
				if fontTok.kind == tokName {
					curFont = fonts[fontTok.text]
				}
			}
		case "Td", "TD":
			if inText && len(stack) >= 2 {
				dx, ok1 := tokenFloat(stack[len(stack)-2])
				dy, ok2 := tokenFloat(stack[len(stack)-1])
				if ok1 && ok2 {
					emitPositionBreak(curX+dx, curY+dy)
				}
			}
		case "Tm":
			if inText && len(stack) >= 6 {
				tx, ok1 := tokenFloat(stack[len(stack)-2])
				ty, ok2 := tokenFloat(stack[len(stack)-1])
				if ok1 && ok2 {
					emitPositionBreak(tx, ty)
				}
			}
		case "T*":
			if inText {
				appendNewline()
			}
		case "Tj":
			if len(stack) >= 1 && stack[len(stack)-1].kind == tokString {
				out.WriteString(curFont.decode(stack[len(stack)-1].raw))
			}
		case "'", "\"":
			appendNewline()
			if len(stack) >= 1 && stack[len(stack)-1].kind == tokString {
				out.WriteString(curFont.decode(stack[len(stack)-1].raw))
			}
		case "TJ":
			if len(stack) >= 1 && stack[len(stack)-1].kind == tokArray {
				for _, item := range stack[len(stack)-1].items {
					if item.kind == tokString {
						out.WriteString(curFont.decode(item.raw))
					} else if item.kind == tokWord {
						if adj, err := strconv.ParseFloat(item.text, 64); err == nil && adj < -180 {
							appendSpace()
						}
					}
				}
			}
		case "Do":
			if len(stack) >= 1 && stack[len(stack)-1].kind == tokName && resDict != nil {
				formText := extractFormXObjectText(pdfCtx, resDict, stack[len(stack)-1].text, fonts, depth+1)
				if formText != "" {
					appendNewline()
					out.WriteString(formText)
				}
			}
		}
		stack = stack[:0]
	}

	return normalizeExtractedText(out.String())
}

func extractFormXObjectText(pdfCtx *model.Context, resDict types.Dict, name string, parentFonts map[string]*fontDecoder, depth int) string {
	xoObj, ok := resDict.Find("XObject")
	if !ok || xoObj == nil {
		return ""
	}
	xoDict, err := pdfCtx.DereferenceDict(xoObj)
	if err != nil || xoDict == nil {
		return ""
	}
	entry, ok := xoDict.Find(name)
	if !ok || entry == nil {
		return ""
	}
	sd, _, err := pdfCtx.DereferenceStreamDict(entry)
	if err != nil || sd == nil {
		return ""
	}
	subtypeObj, ok := sd.Find("Subtype")
	if !ok {
		return ""
	}
	if subtype, ok := subtypeObj.(types.Name); !ok || subtype.Value() != "Form" {
		return ""
	}
	_ = pdfCtx.DecodeStream(sd)
	data := sd.Content
	if len(data) == 0 {
		data = sd.Raw
	}
	formRes := resDict
	formFonts := parentFonts
	if frObj, ok := sd.Find("Resources"); ok && frObj != nil {
		if d, err := pdfCtx.DereferenceDict(frObj); err == nil && d != nil {
			formRes = d
			merged := make(map[string]*fontDecoder, len(parentFonts))
			for k, v := range parentFonts {
				merged[k] = v
			}
			for k, v := range loadPageFonts(pdfCtx, d) {
				merged[k] = v
			}
			formFonts = merged
		}
	}
	return extractStreamText(pdfCtx, data, formRes, formFonts, depth)
}

func tokenFloat(tok csToken) (float64, bool) {
	if tok.kind != tokWord {
		return 0, false
	}
	v, err := strconv.ParseFloat(tok.text, 64)
	return v, err == nil
}

func parseContentTokens(data []byte) []csToken {
	p := &csParser{data: data}
	return p.parseAll(false)
}

type csParser struct {
	data []byte
	pos  int
}

func (p *csParser) parseAll(stopAtBracket bool) []csToken {
	var tokens []csToken
	n := len(p.data)
	for p.pos < n {
		p.skipSpaceAndComments()
		if p.pos >= n {
			break
		}
		b := p.data[p.pos]
		if stopAtBracket && b == ']' {
			p.pos++
			return tokens
		}
		switch {
		case b == '(':
			tokens = append(tokens, csToken{kind: tokString, raw: p.readLiteralString()})
		case b == '<':
			if p.pos+1 < n && p.data[p.pos+1] == '<' {
				p.skipDict()
			} else {
				tokens = append(tokens, csToken{kind: tokString, raw: p.readHexString()})
			}
		case b == '[':
			p.pos++
			items := p.parseAll(true)
			tokens = append(tokens, csToken{kind: tokArray, items: items})
		case b == ']':
			p.pos++
		case b == '/':
			p.pos++
			start := p.pos
			for p.pos < n && !isPDFSpace(p.data[p.pos]) && !isPDFDelim(p.data[p.pos]) {
				p.pos++
			}
			tokens = append(tokens, csToken{kind: tokName, text: string(p.data[start:p.pos])})
		default:
			start := p.pos
			for p.pos < n && !isPDFSpace(p.data[p.pos]) && !isPDFDelim(p.data[p.pos]) {
				p.pos++
			}
			if p.pos == start {
				p.pos++
				continue
			}
			tokens = append(tokens, csToken{kind: tokWord, text: string(p.data[start:p.pos])})
		}
	}
	return tokens
}

func (p *csParser) skipSpaceAndComments() {
	n := len(p.data)
	for p.pos < n {
		if isPDFSpace(p.data[p.pos]) {
			p.pos++
			continue
		}
		if p.data[p.pos] == '%' {
			for p.pos < n && p.data[p.pos] != '\n' && p.data[p.pos] != '\r' {
				p.pos++
			}
			continue
		}
		break
	}
}

func (p *csParser) skipDict() {
	n := len(p.data)
	depth := 0
	for p.pos < n {
		if p.data[p.pos] == '(' {
			_ = p.readLiteralString()
			continue
		}
		if p.data[p.pos] == '<' && p.pos+1 < n && p.data[p.pos+1] == '<' {
			depth++
			p.pos += 2
			continue
		}
		if p.data[p.pos] == '>' && p.pos+1 < n && p.data[p.pos+1] == '>' {
			depth--
			p.pos += 2
			if depth <= 0 {
				return
			}
			continue
		}
		p.pos++
	}
}

func (p *csParser) readLiteralString() []byte {
	n := len(p.data)
	p.pos++
	var buf bytes.Buffer
	depth := 1
	for p.pos < n && depth > 0 {
		b := p.data[p.pos]
		p.pos++
		if b == '\\' {
			if p.pos >= n {
				break
			}
			esc := p.data[p.pos]
			p.pos++
			switch esc {
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 't':
				buf.WriteByte('\t')
			case 'b':
				buf.WriteByte('\b')
			case 'f':
				buf.WriteByte('\f')
			case '\r':
				if p.pos < n && p.data[p.pos] == '\n' {
					p.pos++
				}
			case '\n':
			default:
				if esc >= '0' && esc <= '7' {
					val := int(esc - '0')
					for k := 0; k < 2 && p.pos < n; k++ {
						d := p.data[p.pos]
						if d < '0' || d > '7' {
							break
						}
						val = val*8 + int(d-'0')
						p.pos++
					}
					buf.WriteByte(byte(val))
				} else {
					buf.WriteByte(esc)
				}
			}
			continue
		}
		if b == '(' {
			depth++
			buf.WriteByte(b)
			continue
		}
		if b == ')' {
			depth--
			if depth > 0 {
				buf.WriteByte(b)
			}
			continue
		}
		buf.WriteByte(b)
	}
	return buf.Bytes()
}

func (p *csParser) readHexString() []byte {
	n := len(p.data)
	p.pos++
	var digits []byte
	for p.pos < n {
		b := p.data[p.pos]
		p.pos++
		if b == '>' {
			break
		}
		if !isPDFSpace(b) {
			digits = append(digits, b)
		}
	}
	if len(digits)%2 == 1 {
		digits = append(digits, '0')
	}
	out := make([]byte, hex.DecodedLen(len(digits)))
	m, err := hex.Decode(out, digits)
	if err != nil {
		return nil
	}
	return out[:m]
}

func normalizeExtractedText(s string) string {
	lines := strings.Split(s, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}

func isPDFSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\f' || b == 0
}

func isPDFDelim(b byte) bool {
	return b == '(' || b == ')' || b == '<' || b == '>' || b == '[' || b == ']' || b == '{' || b == '}' || b == '/' || b == '%'
}
