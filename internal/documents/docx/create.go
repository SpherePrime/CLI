package docx

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/documents/docutil"
)

type mediaEntry struct {
	zipPath string
	target  string
	relID   string
	ext     string
	data    []byte
}

type relEntry struct {
	id      string
	relType string
	target  string
}

type docBuilder struct {
	workingDir     string
	nextRelID      int
	nextImageIdx   int
	nextDocPrID    int
	bulletNumID    int
	nextNumberedID int
	numberedIDs    []int
	usedBullets    bool
	media          []mediaEntry
	paragraphCount int
	tableCount     int
}

func newDocBuilder(workingDir string, startRelID, startImageIdx, startDocPrID, bulletNumID, startNumberedID int) *docBuilder {
	return &docBuilder{
		workingDir:     workingDir,
		nextRelID:      startRelID,
		nextImageIdx:   startImageIdx,
		nextDocPrID:    startDocPrID,
		bulletNumID:    bulletNumID,
		nextNumberedID: startNumberedID,
	}
}

func (b *docBuilder) allocRelID() string {
	id := fmt.Sprintf("rId%d", b.nextRelID)
	b.nextRelID++
	return id
}

func Create(opts CreateOptions) (*Result, error) {
	outPath, err := resolvePath(opts.WorkingDir, opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve output path: %w", err)
	}

	b := newDocBuilder(opts.WorkingDir, 3, 1, 1, 1, 2)

	var body strings.Builder
	if title := strings.TrimSpace(opts.Title); title != "" {
		body.WriteString(renderStyledParagraph("Title", "", []Run{{Text: title}}))
		b.paragraphCount++
	}
	if subtitle := strings.TrimSpace(opts.Subtitle); subtitle != "" {
		body.WriteString(renderStyledParagraph("Subtitle", "", []Run{{Text: subtitle}}))
		b.paragraphCount++
	}

	blocksXML, err := b.renderBlocks(opts.Blocks)
	if err != nil {
		return nil, err
	}
	body.WriteString(blocksXML)

	rels := []relEntry{
		{id: "rId1", relType: relTypeStyles, target: "styles.xml"},
		{id: "rId2", relType: relTypeNumbering, target: "numbering.xml"},
	}

	var headerRelID string
	var headerXML []byte
	if hdr := strings.TrimSpace(opts.Header); hdr != "" {
		headerRelID = b.allocRelID()
		rels = append(rels, relEntry{id: headerRelID, relType: relTypeHeader, target: "header1.xml"})
		headerXML = buildHeaderXML(hdr)
	}

	var footerRelID string
	var footerXML []byte
	if strings.TrimSpace(opts.Footer) != "" || opts.PageNumbers {
		footerRelID = b.allocRelID()
		rels = append(rels, relEntry{id: footerRelID, relType: relTypeFooter, target: "footer1.xml"})
		footerXML = buildFooterXML(strings.TrimSpace(opts.Footer), opts.PageNumbers)
	}

	hasPNG := false
	hasJPEG := false
	for _, m := range b.media {
		rels = append(rels, relEntry{id: m.relID, relType: relTypeImage, target: m.target})
		if m.ext == "png" {
			hasPNG = true
		} else if m.ext == "jpeg" {
			hasJPEG = true
		}
	}

	sectPr := buildSectPr(headerRelID, footerRelID)
	docXML := buildDocumentXML(body.String() + sectPr)
	contentTypesXML := buildContentTypesXML(len(headerXML) > 0, len(footerXML) > 0, hasPNG, hasJPEG)
	rootRelsXML := buildRootRelsXML()
	corePropsXML := buildCorePropsXML(opts)
	docRelsXML := buildDocRelsXML(rels)
	stylesXML := buildStylesXML()
	numberingXML := buildNumberingXML(b.numberedIDs)

	err = docutil.WriteZipAtomic(outPath, func(zw *zip.Writer) error {
		files := []struct {
			name string
			data []byte
		}{
			{"[Content_Types].xml", contentTypesXML},
			{"_rels/.rels", rootRelsXML},
			{"docProps/core.xml", corePropsXML},
			{"word/document.xml", docXML},
			{"word/_rels/document.xml.rels", docRelsXML},
			{"word/styles.xml", stylesXML},
			{"word/numbering.xml", numberingXML},
		}
		if len(headerXML) > 0 {
			files = append(files, struct {
				name string
				data []byte
			}{"word/header1.xml", headerXML})
		}
		if len(footerXML) > 0 {
			files = append(files, struct {
				name string
				data []byte
			}{"word/footer1.xml", footerXML})
		}
		for _, m := range b.media {
			files = append(files, struct {
				name string
				data []byte
			}{m.zipPath, m.data})
		}
		for _, entry := range files {
			w, err := zw.Create(entry.name)
			if err != nil {
				return err
			}
			if _, err := w.Write(entry.data); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("write docx: %w", err)
	}

	fi, err := os.Stat(outPath)
	if err != nil {
		return nil, fmt.Errorf("stat docx: %w", err)
	}
	return &Result{
		Path:           outPath,
		SizeBytes:      fi.Size(),
		ParagraphCount: b.paragraphCount,
		TableCount:     b.tableCount,
	}, nil
}

func (b *docBuilder) renderBlocks(blocks []Block) (string, error) {
	var out strings.Builder
	for i, blk := range blocks {
		kind := strings.ToLower(strings.TrimSpace(blk.Type))
		switch kind {
		case "heading":
			out.WriteString(b.renderHeading(blk))
		case "paragraph", "":
			out.WriteString(b.renderParagraph(blk))
		case "bullet_list":
			out.WriteString(b.renderBulletList(blk))
		case "numbered_list":
			out.WriteString(b.renderNumberedList(blk))
		case "table":
			out.WriteString(b.renderTable(blk))
		case "callout":
			out.WriteString(b.renderCallout(blk))
		case "image":
			imgXML, err := b.renderImage(blk)
			if err != nil {
				return "", fmt.Errorf("block[%d] image: %w", i, err)
			}
			out.WriteString(imgXML)
		case "divider":
			out.WriteString(renderDivider())
		case "page_break":
			out.WriteString(renderPageBreak())
		default:
			return "", fmt.Errorf("block[%d]: unsupported block type %q", i, blk.Type)
		}
	}
	return out.String(), nil
}

func (b *docBuilder) renderHeading(blk Block) string {
	lvl := blk.Level
	if lvl < 1 {
		lvl = 1
	}
	if lvl > 3 {
		lvl = 3
	}
	style := fmt.Sprintf("Heading%d", lvl)
	runs := blk.Runs
	if len(runs) == 0 {
		runs = []Run{{Text: blk.Text}}
	}
	b.paragraphCount++
	return renderStyledParagraph(style, blk.Align, runs)
}

func (b *docBuilder) renderParagraph(blk Block) string {
	runs := blk.Runs
	if len(runs) == 0 {
		runs = []Run{{Text: blk.Text}}
	}
	b.paragraphCount++
	return renderStyledParagraph("", blk.Align, runs)
}

func (b *docBuilder) renderBulletList(blk Block) string {
	if len(blk.Items) == 0 {
		return ""
	}
	b.usedBullets = true
	var out strings.Builder
	for _, item := range blk.Items {
		out.WriteString(`<w:p><w:pPr><w:pStyle w:val="ListParagraph"/><w:numPr><w:ilvl w:val="0"/><w:numId w:val="`)
		out.WriteString(fmt.Sprintf("%d", b.bulletNumID))
		out.WriteString(`"/></w:numPr></w:pPr>`)
		out.WriteString(renderRun(Run{Text: item}))
		out.WriteString(`</w:p>`)
		b.paragraphCount++
	}
	return out.String()
}

func (b *docBuilder) renderNumberedList(blk Block) string {
	if len(blk.Items) == 0 {
		return ""
	}
	numID := b.nextNumberedID
	b.nextNumberedID++
	b.numberedIDs = append(b.numberedIDs, numID)
	var out strings.Builder
	for _, item := range blk.Items {
		out.WriteString(`<w:p><w:pPr><w:pStyle w:val="ListParagraph"/><w:numPr><w:ilvl w:val="0"/><w:numId w:val="`)
		out.WriteString(fmt.Sprintf("%d", numID))
		out.WriteString(`"/></w:numPr></w:pPr>`)
		out.WriteString(renderRun(Run{Text: item}))
		out.WriteString(`</w:p>`)
		b.paragraphCount++
	}
	return out.String()
}

func (b *docBuilder) renderTable(blk Block) string {
	numCols := len(blk.Headers)
	for _, row := range blk.Rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}
	if numCols == 0 {
		return ""
	}
	b.tableCount++

	colWidths := resolveColWidthsTwips(numCols, blk.ColWidths, contentWidthTwips)
	headerFill := normalizeHexColor(blk.HeaderFill)
	if headerFill == "" {
		headerFill = "1E3A5F"
	}
	headerTextColor := "FFFFFF"
	if !isDarkHex(headerFill) {
		headerTextColor = "0F172A"
	}

	var out strings.Builder
	out.WriteString(`<w:tbl>`)
	out.WriteString(fmt.Sprintf(
		`<w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="%d" w:type="dxa"/><w:tblLayout w:type="fixed"/>`+
			`<w:tblBorders>`+
			`<w:top w:val="single" w:sz="4" w:space="0" w:color="CBD5E1"/>`+
			`<w:left w:val="single" w:sz="4" w:space="0" w:color="CBD5E1"/>`+
			`<w:bottom w:val="single" w:sz="4" w:space="0" w:color="CBD5E1"/>`+
			`<w:right w:val="single" w:sz="4" w:space="0" w:color="CBD5E1"/>`+
			`<w:insideH w:val="single" w:sz="4" w:space="0" w:color="CBD5E1"/>`+
			`<w:insideV w:val="single" w:sz="4" w:space="0" w:color="CBD5E1"/>`+
			`</w:tblBorders>`+
			`<w:tblCellMar><w:top w:w="100" w:type="dxa"/><w:left w:w="140" w:type="dxa"/><w:bottom w:w="100" w:type="dxa"/><w:right w:w="140" w:type="dxa"/></w:tblCellMar>`+
			`</w:tblPr>`,
		contentWidthTwips,
	))

	out.WriteString(`<w:tblGrid>`)
	for _, w := range colWidths {
		out.WriteString(fmt.Sprintf(`<w:gridCol w:w="%d"/>`, w))
	}
	out.WriteString(`</w:tblGrid>`)

	align := mapAlignment(blk.Align)
	if len(blk.Headers) > 0 {
		out.WriteString(`<w:tr><w:trPr><w:tblHeader/><w:cantSplit/></w:trPr>`)
		for c := range numCols {
			val := ""
			if c < len(blk.Headers) {
				val = blk.Headers[c]
			}
			out.WriteString(fmt.Sprintf(
				`<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/><w:shd w:val="clear" w:color="auto" w:fill="%s"/><w:vAlign w:val="center"/></w:tcPr>`,
				colWidths[c], headerFill,
			))
			out.WriteString(renderTableCellParagraph(val, align, Run{Bold: true, Color: headerTextColor, FontSize: 10.5}))
			out.WriteString(`</w:tc>`)
		}
		out.WriteString(`</w:tr>`)
	}

	for rIdx, row := range blk.Rows {
		out.WriteString(`<w:tr><w:trPr><w:cantSplit/></w:trPr>`)
		fill := ""
		if rIdx%2 == 1 {
			fill = "F8FAFC"
		}
		for c := range numCols {
			val := ""
			if c < len(row) {
				val = row[c]
			}
			out.WriteString(fmt.Sprintf(`<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>`, colWidths[c]))
			if fill != "" {
				out.WriteString(fmt.Sprintf(`<w:shd w:val="clear" w:color="auto" w:fill="%s"/>`, fill))
			}
			out.WriteString(`<w:vAlign w:val="center"/></w:tcPr>`)
			out.WriteString(renderTableCellParagraph(val, align, Run{FontSize: 10}))
			out.WriteString(`</w:tc>`)
		}
		out.WriteString(`</w:tr>`)
	}

	out.WriteString(`</w:tbl>`)
	out.WriteString(`<w:p><w:pPr><w:spacing w:before="0" w:after="120"/></w:pPr></w:p>`)
	return out.String()
}

func (b *docBuilder) renderCallout(blk Block) string {
	bg, accent, titleColor := calloutColors(blk.Tone)
	b.tableCount++

	var out strings.Builder
	out.WriteString(fmt.Sprintf(
		`<w:tbl><w:tblPr><w:tblW w:w="%d" w:type="dxa"/><w:tblLayout w:type="fixed"/>`+
			`<w:tblBorders>`+
			`<w:top w:val="none" w:sz="0" w:space="0" w:color="auto"/>`+
			`<w:left w:val="single" w:sz="24" w:space="0" w:color="%s"/>`+
			`<w:bottom w:val="none" w:sz="0" w:space="0" w:color="auto"/>`+
			`<w:right w:val="none" w:sz="0" w:space="0" w:color="auto"/>`+
			`<w:insideH w:val="none" w:sz="0" w:space="0" w:color="auto"/>`+
			`<w:insideV w:val="none" w:sz="0" w:space="0" w:color="auto"/>`+
			`</w:tblBorders>`+
			`<w:tblCellMar><w:top w:w="140" w:type="dxa"/><w:left w:w="220" w:type="dxa"/><w:bottom w:w="140" w:type="dxa"/><w:right w:w="180" w:type="dxa"/></w:tblCellMar>`+
			`</w:tblPr>`+
			`<w:tblGrid><w:gridCol w:w="%d"/></w:tblGrid>`+
			`<w:tr><w:trPr><w:cantSplit/></w:trPr>`+
			`<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/><w:shd w:val="clear" w:color="auto" w:fill="%s"/></w:tcPr>`,
		contentWidthTwips, accent, contentWidthTwips, contentWidthTwips, bg,
	))

	if title := strings.TrimSpace(blk.Title); title != "" {
		out.WriteString(`<w:p><w:pPr><w:spacing w:before="0" w:after="60"/></w:pPr>`)
		out.WriteString(renderRun(Run{Text: title, Bold: true, Color: titleColor, FontSize: 10.5}))
		out.WriteString(`</w:p>`)
	}
	runs := blk.Runs
	if len(runs) == 0 {
		runs = []Run{{Text: blk.Text, Color: "1F2937", FontSize: 10}}
	}
	out.WriteString(`<w:p><w:pPr><w:spacing w:before="0" w:after="0"/></w:pPr>`)
	for _, r := range runs {
		out.WriteString(renderRun(r))
	}
	out.WriteString(`</w:p></w:tc></w:tr></w:tbl>`)
	out.WriteString(`<w:p><w:pPr><w:spacing w:before="0" w:after="120"/></w:pPr></w:p>`)
	return out.String()
}

func (b *docBuilder) renderImage(blk Block) (string, error) {
	imgPath, err := resolvePath(b.workingDir, blk.ImagePath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(imgPath)
	if err != nil {
		return "", err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode image %s: %w", imgPath, err)
	}
	if format != "png" && format != "jpeg" {
		return "", fmt.Errorf("unsupported image format %q (expected png or jpeg)", format)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return "", fmt.Errorf("invalid image dimensions %dx%d", cfg.Width, cfg.Height)
	}

	aspect := float64(cfg.Height) / float64(cfg.Width)
	wPt := blk.Width
	hPt := blk.Height
	switch {
	case wPt > 0 && hPt > 0:
	case wPt > 0:
		hPt = wPt * aspect
	case hPt > 0:
		wPt = hPt / aspect
	default:
		wPt = float64(cfg.Width) * 0.75
		hPt = wPt * aspect
	}
	if wPt > contentWidthPoints {
		scale := contentWidthPoints / wPt
		wPt = contentWidthPoints
		hPt *= scale
	}

	cx := int64(math.Round(wPt * emuPerPoint))
	cy := int64(math.Round(hPt * emuPerPoint))
	if cx < 1 {
		cx = 1
	}
	if cy < 1 {
		cy = 1
	}

	imgIdx := b.nextImageIdx
	b.nextImageIdx++
	relID := b.allocRelID()
	docPrID := b.nextDocPrID
	b.nextDocPrID++

	fileName := fmt.Sprintf("image%d.%s", imgIdx, format)
	b.media = append(b.media, mediaEntry{
		zipPath: "word/media/" + fileName,
		target:  "media/" + fileName,
		relID:   relID,
		ext:     format,
		data:    data,
	})

	align := mapAlignment(blk.Align)
	var out strings.Builder
	out.WriteString(`<w:p>`)
	if align != "" {
		out.WriteString(fmt.Sprintf(`<w:pPr><w:jc w:val="%s"/></w:pPr>`, align))
	}
	out.WriteString(fmt.Sprintf(
		`<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:effectExtent l="0" t="0" r="0" b="0"/>`+
			`<wp:docPr id="%d" name="Picture %d"/>`+
			`<wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr>`+
			`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">`+
			`<pic:pic>`+
			`<pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr>`+
			`<pic:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>`+
			`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>`+
			`</pic:pic></a:graphicData></a:graphic>`+
			`</wp:inline></w:drawing></w:r></w:p>`,
		cx, cy, docPrID, imgIdx, docPrID, escapeXML(fileName), relID, cx, cy,
	))
	b.paragraphCount++

	if caption := strings.TrimSpace(blk.Caption); caption != "" {
		capAlign := align
		if capAlign == "" {
			capAlign = "center"
		}
		out.WriteString(renderStyledParagraph("", capAlign, []Run{{Text: caption, Italic: true, Color: "64748B", FontSize: 9.5}}))
		b.paragraphCount++
	}
	return out.String(), nil
}

func renderDivider() string {
	return `<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="CBD5E1"/></w:pBdr><w:spacing w:before="120" w:after="120"/></w:pPr></w:p>`
}

func renderPageBreak() string {
	return `<w:p><w:r><w:br w:type="page"/></w:r></w:p>`
}

func renderStyledParagraph(styleID, align string, runs []Run) string {
	var out strings.Builder
	out.WriteString(`<w:p>`)
	jc := mapAlignment(align)
	if styleID != "" || jc != "" {
		out.WriteString(`<w:pPr>`)
		if styleID != "" {
			out.WriteString(fmt.Sprintf(`<w:pStyle w:val="%s"/>`, escapeXML(styleID)))
		}
		if jc != "" {
			out.WriteString(fmt.Sprintf(`<w:jc w:val="%s"/>`, jc))
		}
		out.WriteString(`</w:pPr>`)
	}
	for _, r := range runs {
		out.WriteString(renderRun(r))
	}
	out.WriteString(`</w:p>`)
	return out.String()
}

func renderTableCellParagraph(text, align string, style Run) string {
	var out strings.Builder
	out.WriteString(`<w:p><w:pPr><w:spacing w:before="0" w:after="0"/>`)
	if align != "" {
		out.WriteString(fmt.Sprintf(`<w:jc w:val="%s"/>`, align))
	}
	out.WriteString(`</w:pPr>`)
	style.Text = text
	out.WriteString(renderRun(style))
	out.WriteString(`</w:p>`)
	return out.String()
}

func renderRun(r Run) string {
	var out strings.Builder
	out.WriteString(`<w:r>`)
	rPr := buildRunProperties(r)
	if rPr != "" {
		out.WriteString(rPr)
	}
	parts := strings.Split(r.Text, "\n")
	for i, part := range parts {
		if i > 0 {
			out.WriteString(`<w:br/>`)
		}
		out.WriteString(`<w:t xml:space="preserve">`)
		out.WriteString(escapeXML(part))
		out.WriteString(`</w:t>`)
	}
	out.WriteString(`</w:r>`)
	return out.String()
}

func buildRunProperties(r Run) string {
	var p strings.Builder
	if fam := strings.TrimSpace(r.FontFamily); fam != "" {
		esc := escapeXML(fam)
		p.WriteString(fmt.Sprintf(`<w:rFonts w:ascii="%s" w:hAnsi="%s" w:cs="%s"/>`, esc, esc, esc))
	}
	if r.Bold {
		p.WriteString(`<w:b/>`)
	}
	if r.Italic {
		p.WriteString(`<w:i/>`)
	}
	if col := normalizeHexColor(r.Color); col != "" {
		p.WriteString(fmt.Sprintf(`<w:color w:val="%s"/>`, col))
	}
	if r.FontSize > 0 {
		halfPts := int(math.Round(r.FontSize * 2))
		if halfPts > 0 {
			p.WriteString(fmt.Sprintf(`<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, halfPts, halfPts))
		}
	}
	if r.Underline {
		p.WriteString(`<w:u w:val="single"/>`)
	}
	if p.Len() == 0 {
		return ""
	}
	return `<w:rPr>` + p.String() + `</w:rPr>`
}

func resolveColWidthsTwips(numCols int, specified []float64, total int) []int {
	widths := make([]int, numCols)
	if numCols <= 0 {
		return widths
	}
	sum := 0.0
	valid := len(specified) == numCols
	if valid {
		for _, v := range specified {
			if v <= 0 {
				valid = false
				break
			}
			sum += v
		}
	}
	if !valid || sum <= 0 {
		base := total / numCols
		rem := total - base*numCols
		for i := range numCols {
			widths[i] = base
			if i == numCols-1 {
				widths[i] += rem
			}
		}
		return widths
	}
	used := 0
	for i := range numCols {
		if i == numCols-1 {
			widths[i] = max(total-used, 1)
		} else {
			w := max(int(math.Round((specified[i]/sum)*float64(total))), 1)
			widths[i] = w
			used += w
		}
	}
	return widths
}

func calloutColors(tone string) (bg, accent, title string) {
	switch strings.ToLower(strings.TrimSpace(tone)) {
	case "warning", "warn":
		return "FFFBEB", "D97706", "92400E"
	case "success", "ok":
		return "F0FDF4", "16A34A", "166534"
	case "danger", "error":
		return "FEF2F2", "DC2626", "991B1B"
	default:
		return "EFF6FF", "2563EB", "1E40AF"
	}
}

func buildSectPr(headerRelID, footerRelID string) string {
	var out strings.Builder
	out.WriteString(`<w:sectPr>`)
	if headerRelID != "" {
		out.WriteString(fmt.Sprintf(`<w:headerReference w:type="default" r:id="%s"/>`, headerRelID))
	}
	if footerRelID != "" {
		out.WriteString(fmt.Sprintf(`<w:footerReference w:type="default" r:id="%s"/>`, footerRelID))
	}
	out.WriteString(fmt.Sprintf(
		`<w:pgSz w:w="%d" w:h="%d"/>`+
			`<w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="720" w:footer="720" w:gutter="0"/>`,
		a4WidthTwips, a4HeightTwips, pageMarginTwips, pageMarginTwips, pageMarginTwips, pageMarginTwips,
	))
	out.WriteString(`</w:sectPr>`)
	return out.String()
}

func buildDocumentXML(bodyContent string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="` + nsWordML + `" xmlns:r="` + nsRel + `" xmlns:wp="` + nsWP + `" xmlns:a="` + nsDrawingML + `" xmlns:pic="` + nsPic + `">` +
		`<w:body>` + bodyContent + `</w:body></w:document>`)
}

func buildHeaderXML(text string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:hdr xmlns:w="` + nsWordML + `" xmlns:r="` + nsRel + `">` +
		`<w:p><w:pPr><w:pStyle w:val="Header"/><w:jc w:val="right"/><w:pBdr><w:bottom w:val="single" w:sz="4" w:space="4" w:color="E2E8F0"/></w:pBdr></w:pPr>` +
		renderRun(Run{Text: text, Color: "64748B", FontSize: 9}) +
		`</w:p></w:hdr>`)
}

func buildFooterXML(text string, pageNumbers bool) []byte {
	var content strings.Builder
	content.WriteString(`<w:p><w:pPr><w:pStyle w:val="Footer"/><w:pBdr><w:top w:val="single" w:sz="4" w:space="4" w:color="E2E8F0"/></w:pBdr>`)
	if text != "" && pageNumbers {
		content.WriteString(fmt.Sprintf(`<w:tabs><w:tab w:val="right" w:pos="%d"/></w:tabs>`, contentWidthTwips))
	} else if pageNumbers {
		content.WriteString(`<w:jc w:val="right"/>`)
	}
	content.WriteString(`</w:pPr>`)

	if text != "" {
		content.WriteString(renderRun(Run{Text: text, Color: "64748B", FontSize: 9}))
	}
	if pageNumbers {
		if text != "" {
			content.WriteString(`<w:r><w:tab/></w:r>`)
		}
		content.WriteString(
			`<w:r><w:rPr><w:color w:val="64748B"/><w:sz w:val="18"/><w:szCs w:val="18"/></w:rPr><w:fldChar w:fldCharType="begin"/></w:r>` +
				`<w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r>` +
				`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
				`<w:r><w:rPr><w:color w:val="64748B"/><w:sz w:val="18"/><w:szCs w:val="18"/></w:rPr><w:t>1</w:t></w:r>` +
				`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
				`<w:r><w:rPr><w:color w:val="64748B"/><w:sz w:val="18"/><w:szCs w:val="18"/></w:rPr><w:t xml:space="preserve"> / </w:t></w:r>` +
				`<w:r><w:rPr><w:color w:val="64748B"/><w:sz w:val="18"/><w:szCs w:val="18"/></w:rPr><w:fldChar w:fldCharType="begin"/></w:r>` +
				`<w:r><w:instrText xml:space="preserve"> NUMPAGES </w:instrText></w:r>` +
				`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
				`<w:r><w:rPr><w:color w:val="64748B"/><w:sz w:val="18"/><w:szCs w:val="18"/></w:rPr><w:t>1</w:t></w:r>` +
				`<w:r><w:fldChar w:fldCharType="end"/></w:r>`,
		)
	}
	content.WriteString(`</w:p>`)
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:ftr xmlns:w="` + nsWordML + `" xmlns:r="` + nsRel + `">` +
		content.String() + `</w:ftr>`)
}

func buildContentTypesXML(hasHeader, hasFooter, hasPNG, hasJPEG bool) []byte {
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	out.WriteString(`<Types xmlns="` + nsContentTypes + `">`)
	out.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	out.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	if hasPNG {
		out.WriteString(`<Default Extension="png" ContentType="image/png"/>`)
	}
	if hasJPEG {
		out.WriteString(`<Default Extension="jpeg" ContentType="image/jpeg"/>`)
	}
	out.WriteString(`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>`)
	out.WriteString(`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>`)
	out.WriteString(`<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>`)
	out.WriteString(`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`)
	if hasHeader {
		out.WriteString(`<Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>`)
	}
	if hasFooter {
		out.WriteString(`<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>`)
	}
	out.WriteString(`</Types>`)
	return []byte(out.String())
}

func buildRootRelsXML() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="` + nsPkgRel + `">` +
		`<Relationship Id="rId1" Type="` + relTypeOfficeDoc + `" Target="word/document.xml"/>` +
		`<Relationship Id="rId2" Type="` + relTypeCoreProps + `" Target="docProps/core.xml"/>` +
		`</Relationships>`)
}

func buildDocRelsXML(rels []relEntry) []byte {
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	out.WriteString(`<Relationships xmlns="` + nsPkgRel + `">`)
	for _, r := range rels {
		out.WriteString(fmt.Sprintf(`<Relationship Id="%s" Type="%s" Target="%s"/>`, escapeXML(r.id), escapeXML(r.relType), escapeXML(r.target)))
	}
	out.WriteString(`</Relationships>`)
	return []byte(out.String())
}

func buildCorePropsXML(opts CreateOptions) []byte {
	now := time.Now().UTC().Format(time.RFC3339)
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	out.WriteString(`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">`)
	if v := strings.TrimSpace(opts.Title); v != "" {
		out.WriteString(`<dc:title>` + escapeXML(v) + `</dc:title>`)
	}
	if v := strings.TrimSpace(opts.Subject); v != "" {
		out.WriteString(`<dc:subject>` + escapeXML(v) + `</dc:subject>`)
	}
	if v := strings.TrimSpace(opts.Author); v != "" {
		out.WriteString(`<dc:creator>` + escapeXML(v) + `</dc:creator>`)
	}
	if v := strings.TrimSpace(opts.Subtitle); v != "" {
		out.WriteString(`<dc:description>` + escapeXML(v) + `</dc:description>`)
	}
	out.WriteString(`<dcterms:created xsi:type="dcterms:W3CDTF">` + now + `</dcterms:created>`)
	out.WriteString(`<dcterms:modified xsi:type="dcterms:W3CDTF">` + now + `</dcterms:modified>`)
	out.WriteString(`</cp:coreProperties>`)
	return []byte(out.String())
}

func buildStylesXML() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:styles xmlns:w="` + nsWordML + `">` +
		`<w:docDefaults>` +
		`<w:rPrDefault><w:rPr>` +
		`<w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:cs="Calibri"/>` +
		`<w:color w:val="1F2937"/><w:sz w:val="22"/><w:szCs w:val="22"/>` +
		`</w:rPr></w:rPrDefault>` +
		`<w:pPrDefault><w:pPr><w:spacing w:after="120" w:line="276" w:lineRule="auto"/></w:pPr></w:pPrDefault>` +
		`</w:docDefaults>` +
		`<w:style w:type="paragraph" w:default="1" w:styleId="Normal">` +
		`<w:name w:val="Normal"/><w:qFormat/>` +
		`</w:style>` +
		`<w:style w:type="paragraph" w:styleId="Title">` +
		`<w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>` +
		`<w:pPr><w:spacing w:before="0" w:after="100"/></w:pPr>` +
		`<w:rPr><w:b/><w:color w:val="0F172A"/><w:sz w:val="48"/><w:szCs w:val="48"/></w:rPr>` +
		`</w:style>` +
		`<w:style w:type="paragraph" w:styleId="Subtitle">` +
		`<w:name w:val="Subtitle"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>` +
		`<w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="8" w:color="CBD5E1"/></w:pBdr><w:spacing w:before="0" w:after="240"/></w:pPr>` +
		`<w:rPr><w:color w:val="475569"/><w:sz w:val="26"/><w:szCs w:val="26"/></w:rPr>` +
		`</w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading1">` +
		`<w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>` +
		`<w:pPr><w:keepNext/><w:spacing w:before="320" w:after="120"/><w:outlineLvl w:val="0"/></w:pPr>` +
		`<w:rPr><w:b/><w:color w:val="0F172A"/><w:sz w:val="34"/><w:szCs w:val="34"/></w:rPr>` +
		`</w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading2">` +
		`<w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>` +
		`<w:pPr><w:keepNext/><w:spacing w:before="260" w:after="100"/><w:outlineLvl w:val="1"/></w:pPr>` +
		`<w:rPr><w:b/><w:color w:val="1E3A5F"/><w:sz w:val="28"/><w:szCs w:val="28"/></w:rPr>` +
		`</w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading3">` +
		`<w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>` +
		`<w:pPr><w:keepNext/><w:spacing w:before="200" w:after="80"/><w:outlineLvl w:val="2"/></w:pPr>` +
		`<w:rPr><w:b/><w:color w:val="334155"/><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr>` +
		`</w:style>` +
		`<w:style w:type="paragraph" w:styleId="ListParagraph">` +
		`<w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:qFormat/>` +
		`<w:pPr><w:spacing w:after="60"/><w:ind w:left="720" w:hanging="360"/></w:pPr>` +
		`</w:style>` +
		`<w:style w:type="paragraph" w:styleId="Header">` +
		`<w:name w:val="header"/><w:basedOn w:val="Normal"/>` +
		`<w:pPr><w:spacing w:after="0"/></w:pPr>` +
		`<w:rPr><w:color w:val="64748B"/><w:sz w:val="18"/><w:szCs w:val="18"/></w:rPr>` +
		`</w:style>` +
		`<w:style w:type="paragraph" w:styleId="Footer">` +
		`<w:name w:val="footer"/><w:basedOn w:val="Normal"/>` +
		`<w:pPr><w:spacing w:after="0"/></w:pPr>` +
		`<w:rPr><w:color w:val="64748B"/><w:sz w:val="18"/><w:szCs w:val="18"/></w:rPr>` +
		`</w:style>` +
		`<w:style w:type="table" w:styleId="TableGrid">` +
		`<w:name w:val="Table Grid"/>` +
		`</w:style>` +
		`</w:styles>`)
}

func buildNumberingXML(numberedIDs []int) []byte {
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	out.WriteString(`<w:numbering xmlns:w="` + nsWordML + `">`)
	out.WriteString(`<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>`)
	out.WriteString(`<w:abstractNum w:abstractNumId="1"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>`)
	out.WriteString(`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`)
	if len(numberedIDs) == 0 {
		out.WriteString(`<w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>`)
	} else {
		for _, id := range numberedIDs {
			out.WriteString(fmt.Sprintf(
				`<w:num w:numId="%d"><w:abstractNumId w:val="1"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/></w:lvlOverride></w:num>`,
				id,
			))
		}
	}
	out.WriteString(`</w:numbering>`)
	return []byte(out.String())
}

var errInvalidDocx = errors.New("not a valid docx archive: missing word/document.xml")
