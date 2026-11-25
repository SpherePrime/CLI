package pptx

import (
	"archive/zip"
	"bytes"
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

type relDef struct {
	id      string
	relType string
	target  string
}

type mediaAsset struct {
	zipPath   string
	relTarget string
	relID     string
	ext       string
	data      []byte
	widthPx   int
	heightPx  int
}

type builtSlide struct {
	xml       []byte
	rels      []relDef
	notesXML  []byte
	notesRels []relDef
	media     []mediaAsset
}

type slideRenderer struct {
	workingDir   string
	theme        Theme
	widthEMU     int64
	heightEMU    int64
	slideNumbers bool
	nextImageIdx int
}

type textParagraph struct {
	text       string
	fontSizePt float64
	bold       bool
	italic     bool
	color      string
	align      string
	bullet     bool
	spaceBefPt float64
	spaceAftPt float64
}

func Create(opts CreateOptions) (*Result, error) {
	outPath, err := resolvePath(opts.WorkingDir, opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve output path: %w", err)
	}

	theme := resolveTheme(opts.PrimaryColor, opts.AccentColor, opts.BackgroundColor, opts.TextColor, opts.FontFamily)
	wEMU, hEMU, aspect := resolveSlideDimensions(opts.AspectRatio)

	slides := opts.Slides
	if len(slides) == 0 {
		if title := strings.TrimSpace(opts.Title); title != "" {
			slides = []SlideSpec{{Layout: "title", Title: title, Subtitle: strings.TrimSpace(opts.Subject)}}
		} else {
			slides = []SlideSpec{{Layout: "blank"}}
		}
	}

	renderer := &slideRenderer{
		workingDir:   opts.WorkingDir,
		theme:        theme,
		widthEMU:     wEMU,
		heightEMU:    hEMU,
		slideNumbers: opts.SlideNumbers,
		nextImageIdx: 1,
	}

	builtSlides := make([]*builtSlide, 0, len(slides))
	hasNotes := false
	hasPNG := false
	hasJPEG := false

	for idx, spec := range slides {
		slideNum := idx + 1
		bs, err := renderer.renderSlide(spec, slideNum, "../slideLayouts/slideLayout1.xml", "../notesMasters/notesMaster1.xml", slideNum, fmt.Sprintf("../slides/slide%d.xml", slideNum))
		if err != nil {
			return nil, fmt.Errorf("render slide %d: %w", slideNum, err)
		}
		if len(bs.notesXML) > 0 {
			hasNotes = true
		}
		for _, m := range bs.media {
			if m.ext == "png" {
				hasPNG = true
			} else if m.ext == "jpeg" {
				hasJPEG = true
			}
		}
		builtSlides = append(builtSlides, bs)
	}

	presRels := []relDef{
		{id: "rId1", relType: relTypeSlideMaster, target: "slideMasters/slideMaster1.xml"},
		{id: "rId2", relType: relTypeTheme, target: "theme/theme1.xml"},
	}
	nextPresRel := 3
	notesMasterRelID := ""
	if hasNotes {
		notesMasterRelID = fmt.Sprintf("rId%d", nextPresRel)
		nextPresRel++
		presRels = append(presRels, relDef{
			id:      notesMasterRelID,
			relType: relTypeNotesMaster,
			target:  "notesMasters/notesMaster1.xml",
		})
	}

	type slideRef struct {
		id    int
		relID string
	}
	slideRefs := make([]slideRef, 0, len(builtSlides))
	for i := range builtSlides {
		relID := fmt.Sprintf("rId%d", nextPresRel)
		nextPresRel++
		presRels = append(presRels, relDef{
			id:      relID,
			relType: relTypeSlide,
			target:  fmt.Sprintf("slides/slide%d.xml", i+1),
		})
		slideRefs = append(slideRefs, slideRef{id: 256 + i, relID: relID})
	}

	var presXML bytes.Buffer
	presXML.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&presXML, `<p:presentation xmlns:a="%s" xmlns:r="%s" xmlns:p="%s">`, nsA, nsR, nsP)
	presXML.WriteString(`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>`)
	if notesMasterRelID != "" {
		fmt.Fprintf(&presXML, `<p:notesMasterIdLst><p:notesMasterId r:id="%s"/></p:notesMasterIdLst>`, notesMasterRelID)
	}
	presXML.WriteString(`<p:sldIdLst>`)
	for _, ref := range slideRefs {
		fmt.Fprintf(&presXML, `<p:sldId id="%d" r:id="%s"/>`, ref.id, ref.relID)
	}
	presXML.WriteString(`</p:sldIdLst>`)
	if aspect == "4:3" {
		fmt.Fprintf(&presXML, `<p:sldSz cx="%d" cy="%d" type="screen4x3"/>`, wEMU, hEMU)
	} else {
		fmt.Fprintf(&presXML, `<p:sldSz cx="%d" cy="%d"/>`, wEMU, hEMU)
	}
	fmt.Fprintf(&presXML, `<p:notesSz cx="%d" cy="%d"/>`, notesWidthEMU, notesHeightEMU)
	presXML.WriteString(`<p:defaultTextStyle><a:defPPr><a:defRPr lang="en-US"/></a:defPPr></p:defaultTextStyle>`)
	presXML.WriteString(`</p:presentation>`)

	slidePartNames := make([]string, 0, len(builtSlides))
	notesPartNames := make([]string, 0)
	for i, bs := range builtSlides {
		slidePartNames = append(slidePartNames, fmt.Sprintf("/ppt/slides/slide%d.xml", i+1))
		if len(bs.notesXML) > 0 {
			notesPartNames = append(notesPartNames, fmt.Sprintf("/ppt/notesSlides/notesSlide%d.xml", i+1))
		}
	}

	contentTypesXML := buildContentTypesXML(slidePartNames, notesPartNames, hasNotes, hasPNG, hasJPEG)
	rootRelsXML := buildRelsXML([]relDef{
		{id: "rId1", relType: relTypeOfficeDoc, target: "ppt/presentation.xml"},
		{id: "rId2", relType: relTypeCoreProps, target: "docProps/core.xml"},
	})
	corePropsXML := buildCorePropsXML(opts)
	presRelsXML := buildRelsXML(presRels)
	themeXML := buildThemeXML(theme)
	slideMasterXML := buildSlideMasterXML()
	slideMasterRelsXML := buildRelsXML([]relDef{
		{id: "rId1", relType: relTypeSlideLayout, target: "../slideLayouts/slideLayout1.xml"},
		{id: "rId2", relType: relTypeTheme, target: "../theme/theme1.xml"},
	})
	slideLayoutXML := buildSlideLayoutXML()
	slideLayoutRelsXML := buildRelsXML([]relDef{
		{id: "rId1", relType: relTypeSlideMaster, target: "../slideMasters/slideMaster1.xml"},
	})

	err = docutil.WriteZipAtomic(outPath, func(zw *zip.Writer) error {
		entries := []struct {
			name string
			data []byte
		}{
			{"[Content_Types].xml", contentTypesXML},
			{"_rels/.rels", rootRelsXML},
			{"docProps/core.xml", corePropsXML},
			{"ppt/presentation.xml", presXML.Bytes()},
			{"ppt/_rels/presentation.xml.rels", presRelsXML},
			{"ppt/theme/theme1.xml", themeXML},
			{"ppt/slideMasters/slideMaster1.xml", slideMasterXML},
			{"ppt/slideMasters/_rels/slideMaster1.xml.rels", slideMasterRelsXML},
			{"ppt/slideLayouts/slideLayout1.xml", slideLayoutXML},
			{"ppt/slideLayouts/_rels/slideLayout1.xml.rels", slideLayoutRelsXML},
		}
		if hasNotes {
			entries = append(entries,
				struct {
					name string
					data []byte
				}{"ppt/notesMasters/notesMaster1.xml", buildNotesMasterXML()},
				struct {
					name string
					data []byte
				}{"ppt/notesMasters/_rels/notesMaster1.xml.rels", buildRelsXML([]relDef{
					{id: "rId1", relType: relTypeTheme, target: "../theme/theme1.xml"},
				})},
			)
		}
		for i, bs := range builtSlides {
			num := i + 1
			entries = append(entries,
				struct {
					name string
					data []byte
				}{fmt.Sprintf("ppt/slides/slide%d.xml", num), bs.xml},
				struct {
					name string
					data []byte
				}{fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", num), buildRelsXML(bs.rels)},
			)
			if len(bs.notesXML) > 0 {
				entries = append(entries,
					struct {
						name string
						data []byte
					}{fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", num), bs.notesXML},
					struct {
						name string
						data []byte
					}{fmt.Sprintf("ppt/notesSlides/_rels/notesSlide%d.xml.rels", num), buildRelsXML(bs.notesRels)},
				)
			}
			for _, m := range bs.media {
				entries = append(entries, struct {
					name string
					data []byte
				}{m.zipPath, m.data})
			}
		}

		for _, e := range entries {
			w, err := zw.Create(e.name)
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
		return nil, fmt.Errorf("write pptx zip: %w", err)
	}

	fi, err := os.Stat(outPath)
	if err != nil {
		return nil, fmt.Errorf("stat created pptx: %w", err)
	}

	return &Result{
		Path:       outPath,
		SizeBytes:  fi.Size(),
		SlideCount: len(builtSlides),
	}, nil
}

func inferSlideLayout(spec SlideSpec) string {
	layout := strings.ToLower(strings.TrimSpace(spec.Layout))
	switch layout {
	case "title", "title_content", "two_column", "table", "metrics", "image_text", "blank":
		return layout
	case "":
		switch {
		case spec.Table != nil:
			return "table"
		case len(spec.Metrics) > 0:
			return "metrics"
		case strings.TrimSpace(spec.ImagePath) != "":
			return "image_text"
		case spec.Left != nil || spec.Right != nil:
			return "two_column"
		case strings.TrimSpace(spec.Subtitle) != "" && len(spec.Bullets) == 0 && strings.TrimSpace(spec.Body) == "":
			return "title"
		default:
			return "title_content"
		}
	default:
		return "title_content"
	}
}

func (r *slideRenderer) renderSlide(spec SlideSpec, slideNum int, layoutRelTarget, notesMasterRelTarget string, notesSlideNum int, slideTargetFromNotes string) (*builtSlide, error) {
	layout := inferSlideLayout(spec)
	nextShapeID := 2
	allocID := func() int {
		id := nextShapeID
		nextShapeID++
		return id
	}

	rels := []relDef{
		{id: "rId1", relType: relTypeSlideLayout, target: layoutRelTarget},
	}
	nextRel := 2
	allocRel := func() string {
		id := fmt.Sprintf("rId%d", nextRel)
		nextRel++
		return id
	}

	var notesXML []byte
	var notesRels []relDef
	if strings.TrimSpace(spec.Notes) != "" {
		notesRelID := allocRel()
		rels = append(rels, relDef{
			id:      notesRelID,
			relType: relTypeNotesSlide,
			target:  fmt.Sprintf("../notesSlides/notesSlide%d.xml", notesSlideNum),
		})
		notesXML = buildNotesSlideXML(spec.Notes, r.theme.FontFamily)
		notesRels = []relDef{
			{id: "rId1", relType: relTypeNotesMaster, target: notesMasterRelTarget},
			{id: "rId2", relType: relTypeSlide, target: slideTargetFromNotes},
		}
	}

	bgHex := normalizeHexColor(spec.BackgroundColor)
	if bgHex == "" {
		if layout == "title" {
			bgHex = r.theme.PrimaryColor
		} else {
			bgHex = r.theme.BackgroundColor
		}
	}

	var shapes strings.Builder
	var media []mediaAsset

	marginX := int64(685800)
	contentW := r.widthEMU - 2*marginX

	addHeaderChrome := func() int64 {
		shapes.WriteString(renderRectShape(allocID(), "TopAccentBar", 0, 0, r.widthEMU, 82296, r.theme.AccentColor, "", false))
		contentTop := int64(340000)
		if title := strings.TrimSpace(spec.Title); title != "" {
			titleColor := r.theme.PrimaryColor
			if isDarkHex(bgHex) {
				titleColor = "FFFFFF"
			}
			shapes.WriteString(renderTextBox(allocID(), "Title", marginX, 260000, contentW, 660000, "", "", false, r.theme.FontFamily, []textParagraph{
				{text: title, fontSizePt: 28, bold: true, color: titleColor, align: "l"},
			}))
			shapes.WriteString(renderRectShape(allocID(), "TitleDivider", marginX, 940000, contentW, 19050, "E2E8F0", "", false))
			contentTop = 1120000
		}
		if sub := strings.TrimSpace(spec.Subtitle); sub != "" {
			subColor := mutedColorForBg(bgHex)
			shapes.WriteString(renderTextBox(allocID(), "Subtitle", marginX, contentTop, contentW, 420000, "", "", false, r.theme.FontFamily, []textParagraph{
				{text: sub, fontSizePt: 15, color: subColor, align: "l"},
			}))
			contentTop += 460000
		}
		return contentTop
	}

	textColor := r.theme.TextColor
	if isDarkHex(bgHex) {
		textColor = "FFFFFF"
	}

	switch layout {
	case "title":
		accentBarW := int64(137160)
		shapes.WriteString(renderRectShape(allocID(), "AccentStripe", marginX, 2000000, accentBarW, 2500000, r.theme.AccentColor, "", false))
		titleX := marginX + accentBarW + 274320
		titleW := contentW - accentBarW - 274320
		titleColor := "FFFFFF"
		subColor := "E2E8F0"
		if !isDarkHex(bgHex) {
			titleColor = r.theme.PrimaryColor
			subColor = r.theme.TextColor
		}
		if title := strings.TrimSpace(spec.Title); title != "" {
			shapes.WriteString(renderTextBox(allocID(), "MainTitle", titleX, 2040000, titleW, 1400000, "", "", false, r.theme.FontFamily, []textParagraph{
				{text: title, fontSizePt: 42, bold: true, color: titleColor, align: "l"},
			}))
		}
		if sub := strings.TrimSpace(spec.Subtitle); sub != "" {
			shapes.WriteString(renderTextBox(allocID(), "MainSubtitle", titleX, 3480000, titleW, 900000, "", "", false, r.theme.FontFamily, []textParagraph{
				{text: sub, fontSizePt: 20, color: subColor, align: "l"},
			}))
		}
		if body := strings.TrimSpace(spec.Body); body != "" {
			shapes.WriteString(renderTextBox(allocID(), "TitleBody", titleX, 4560000, titleW, 720000, "", "", false, r.theme.FontFamily, []textParagraph{
				{text: body, fontSizePt: 14, color: subColor, align: "l"},
			}))
		}

	case "title_content":
		top := addHeaderChrome()
		bodyH := r.heightEMU - top - 560000
		paras := buildBodyAndBulletParagraphs(spec.Body, spec.Bullets, textColor, 18)
		if len(paras) > 0 {
			shapes.WriteString(renderTextBox(allocID(), "Content", marginX, top, contentW, bodyH, "", "", false, r.theme.FontFamily, paras))
		}

	case "two_column":
		top := addHeaderChrome()
		gap := int64(365760)
		colW := (contentW - gap) / 2
		colH := r.heightEMU - top - 560000
		cardFill := cardFillForBg(bgHex)
		cardBorder := "E2E8F0"
		if isDarkHex(bgHex) {
			cardBorder = "334155"
		}
		renderCol := func(name string, x int64, col *ColumnContent) {
			if col == nil {
				return
			}
			var paras []textParagraph
			if h := strings.TrimSpace(col.Heading); h != "" {
				hColor := r.theme.PrimaryColor
				if isDarkHex(bgHex) {
					hColor = "FFFFFF"
				}
				paras = append(paras, textParagraph{text: h, fontSizePt: 18, bold: true, color: hColor, spaceAftPt: 8})
			}
			paras = append(paras, buildBodyAndBulletParagraphs(col.Body, col.Bullets, textColor, 15)...)
			shapes.WriteString(renderTextBox(allocID(), name, x, top, colW, colH, cardFill, cardBorder, true, r.theme.FontFamily, paras))
		}
		renderCol("LeftColumn", marginX, spec.Left)
		renderCol("RightColumn", marginX+colW+gap, spec.Right)

	case "table":
		top := addHeaderChrome()
		if spec.Table != nil && (len(spec.Table.Headers) > 0 || len(spec.Table.Rows) > 0) {
			availH := r.heightEMU - top - 560000
			shapes.WriteString(renderTableFrame(allocID(), marginX, top, contentW, availH, spec.Table, r.theme))
		}

	case "metrics":
		top := addHeaderChrome()
		n := len(spec.Metrics)
		if n > 0 {
			cols := min(n, 4)
			rowsCount := (n + cols - 1) / cols
			gapX := int64(274320)
			gapY := int64(274320)
			availH := r.heightEMU - top - 560000
			cardH := (availH - int64(rowsCount-1)*gapY) / int64(rowsCount)
			cardH = min(cardH, int64(2600000))
			cardFill := cardFillForBg(bgHex)
			cardBorder := "E2E8F0"
			if isDarkHex(bgHex) {
				cardBorder = "334155"
			}
			for i, card := range spec.Metrics {
				rowIdx := i / cols
				colIdx := i % cols
				rowItems := min(cols, n-rowIdx*cols)
				cardW := (contentW - int64(rowItems-1)*gapX) / int64(rowItems)
				x := marginX + int64(colIdx)*(cardW+gapX)
				y := top + int64(rowIdx)*(cardH+gapY)

				accent := normalizeHexColor(card.Color)
				if accent == "" {
					accent = r.theme.AccentColor
				}
				valColor := normalizeHexColor(card.Color)
				if valColor == "" {
					if isDarkHex(bgHex) {
						valColor = "FFFFFF"
					} else {
						valColor = r.theme.PrimaryColor
					}
				}
				var paras []textParagraph
				if lbl := strings.TrimSpace(card.Label); lbl != "" {
					paras = append(paras, textParagraph{text: lbl, fontSizePt: 14, bold: true, color: mutedColorForBg(bgHex), spaceAftPt: 10})
				}
				if val := strings.TrimSpace(card.Value); val != "" {
					paras = append(paras, textParagraph{text: val, fontSizePt: 32, bold: true, color: valColor, spaceAftPt: 8})
				}
				if delta := strings.TrimSpace(card.Delta); delta != "" {
					paras = append(paras, textParagraph{text: delta, fontSizePt: 13, bold: true, color: accent})
				}
				shapes.WriteString(renderTextBox(allocID(), fmt.Sprintf("MetricCard%d", i+1), x, y, cardW, cardH, cardFill, cardBorder, true, r.theme.FontFamily, paras))
				shapes.WriteString(renderRectShape(allocID(), fmt.Sprintf("MetricBar%d", i+1), x, y, cardW, 76200, accent, "", false))
			}
		}

	case "image_text":
		top := addHeaderChrome()
		availH := r.heightEMU - top - 560000
		hasText := strings.TrimSpace(spec.Body) != "" || len(spec.Bullets) > 0
		gap := int64(365760)
		imgX := marginX
		imgW := contentW
		if hasText {
			textW := (contentW - gap) / 2
			paras := buildBodyAndBulletParagraphs(spec.Body, spec.Bullets, textColor, 16)
			shapes.WriteString(renderTextBox(allocID(), "ImageTextBody", marginX, top, textW, availH, "", "", false, r.theme.FontFamily, paras))
			imgX = marginX + textW + gap
			imgW = contentW - textW - gap
		}
		if strings.TrimSpace(spec.ImagePath) != "" {
			asset, err := r.loadImage(spec.ImagePath, allocRel())
			if err != nil {
				return nil, err
			}
			rels = append(rels, relDef{
				id:      asset.relID,
				relType: relTypeImage,
				target:  asset.relTarget,
			})
			media = append(media, asset)

			captionH := int64(0)
			if strings.TrimSpace(spec.ImageCaption) != "" {
				captionH = 380000
			}
			boxH := availH - captionH
			fitX, fitY, fitW, fitH := fitAspectBox(imgX, top, imgW, boxH, asset.widthPx, asset.heightPx)
			shapes.WriteString(renderPictureShape(allocID(), fmt.Sprintf("Picture%d", slideNum), asset.relID, fitX, fitY, fitW, fitH))
			if capText := strings.TrimSpace(spec.ImageCaption); capText != "" {
				shapes.WriteString(renderTextBox(allocID(), "ImageCaption", imgX, fitY+fitH+60000, imgW, captionH, "", "", false, r.theme.FontFamily, []textParagraph{
					{text: capText, fontSizePt: 11, italic: true, color: mutedColorForBg(bgHex), align: "ctr"},
				}))
			}
		}

	case "blank":
		if title := strings.TrimSpace(spec.Title); title != "" {
			shapes.WriteString(renderTextBox(allocID(), "Title", marginX, 340000, contentW, 640000, "", "", false, r.theme.FontFamily, []textParagraph{
				{text: title, fontSizePt: 26, bold: true, color: textColor},
			}))
		}
		if paras := buildBodyAndBulletParagraphs(spec.Body, spec.Bullets, textColor, 16); len(paras) > 0 {
			shapes.WriteString(renderTextBox(allocID(), "Body", marginX, 1100000, contentW, r.heightEMU-1660000, "", "", false, r.theme.FontFamily, paras))
		}
	}

	if r.slideNumbers && layout != "title" {
		numColor := mutedColorForBg(bgHex)
		shapes.WriteString(renderTextBox(allocID(), "SlideNumber", r.widthEMU-marginX-700000, r.heightEMU-420000, 700000, 280000, "", "", false, r.theme.FontFamily, []textParagraph{
			{text: fmt.Sprintf("%d", slideNum), fontSizePt: 10, color: numColor, align: "r"},
		}))
	}

	var slideBuf bytes.Buffer
	slideBuf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&slideBuf, `<p:sld xmlns:a="%s" xmlns:r="%s" xmlns:p="%s">`, nsA, nsR, nsP)
	slideBuf.WriteString(`<p:cSld>`)
	fmt.Fprintf(&slideBuf, `<p:bg><p:bgPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>`, bgHex)
	slideBuf.WriteString(`<p:spTree>`)
	slideBuf.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`)
	slideBuf.WriteString(`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)
	slideBuf.WriteString(shapes.String())
	slideBuf.WriteString(`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)

	return &builtSlide{
		xml:       slideBuf.Bytes(),
		rels:      rels,
		notesXML:  notesXML,
		notesRels: notesRels,
		media:     media,
	}, nil
}

func (r *slideRenderer) loadImage(rawPath, relID string) (mediaAsset, error) {
	resolved, err := resolvePath(r.workingDir, rawPath)
	if err != nil {
		return mediaAsset{}, fmt.Errorf("resolve image path: %w", err)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return mediaAsset{}, fmt.Errorf("read image %s: %w", rawPath, err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return mediaAsset{}, fmt.Errorf("decode image %s: %w", rawPath, err)
	}
	ext := strings.ToLower(format)
	if ext != "png" && ext != "jpeg" {
		return mediaAsset{}, fmt.Errorf("unsupported image format %q (only png and jpeg supported)", format)
	}
	idx := r.nextImageIdx
	r.nextImageIdx++
	fileName := fmt.Sprintf("image%d.%s", idx, ext)
	return mediaAsset{
		zipPath:   "ppt/media/" + fileName,
		relTarget: "../media/" + fileName,
		relID:     relID,
		ext:       ext,
		data:      data,
		widthPx:   max(cfg.Width, 1),
		heightPx:  max(cfg.Height, 1),
	}, nil
}

func fitAspectBox(boxX, boxY, boxW, boxH int64, imgW, imgH int) (int64, int64, int64, int64) {
	if boxW <= 0 || boxH <= 0 || imgW <= 0 || imgH <= 0 {
		return boxX, boxY, max(boxW, 1), max(boxH, 1)
	}
	scaleX := float64(boxW) / float64(imgW)
	scaleY := float64(boxH) / float64(imgH)
	scale := math.Min(scaleX, scaleY)
	w := max(int64(math.Round(float64(imgW)*scale)), 1)
	h := max(int64(math.Round(float64(imgH)*scale)), 1)
	x := boxX + (boxW-w)/2
	y := boxY + (boxH-h)/2
	return x, y, w, h
}

func mutedColorForBg(bgHex string) string {
	if isDarkHex(bgHex) {
		return "CBD5E1"
	}
	return "4A5568"
}

func cardFillForBg(bgHex string) string {
	if isDarkHex(bgHex) {
		return "1E293B"
	}
	return "F7FAFC"
}

func buildBodyAndBulletParagraphs(body string, bullets []string, color string, bulletPt float64) []textParagraph {
	var paras []textParagraph
	if trimmed := strings.TrimSpace(body); trimmed != "" {
		for _, line := range strings.Split(trimmed, "\n") {
			if l := strings.TrimSpace(line); l != "" {
				paras = append(paras, textParagraph{
					text:       l,
					fontSizePt: max(bulletPt-1, 12),
					color:      color,
					spaceAftPt: 8,
				})
			}
		}
	}
	for _, b := range bullets {
		if l := strings.TrimSpace(b); l != "" {
			paras = append(paras, textParagraph{
				text:       l,
				fontSizePt: bulletPt,
				color:      color,
				bullet:     true,
				spaceBefPt: 4,
				spaceAftPt: 6,
			})
		}
	}
	return paras
}

func renderRectShape(id int, name string, x, y, cx, cy int64, fillHex, borderHex string, rounded bool) string {
	prst := "rect"
	if rounded {
		prst = "roundRect"
	}
	var b strings.Builder
	b.WriteString(`<p:sp><p:nvSpPr>`)
	fmt.Fprintf(&b, `<p:cNvPr id="%d" name="%s"/>`, id, escapeXML(name))
	b.WriteString(`<p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr>`)
	fmt.Fprintf(&b, `<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, x, y, cx, cy)
	fmt.Fprintf(&b, `<a:prstGeom prst="%s"><a:avLst/></a:prstGeom>`, prst)
	if fillHex != "" {
		fmt.Fprintf(&b, `<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, fillHex)
	} else {
		b.WriteString(`<a:noFill/>`)
	}
	if borderHex != "" {
		fmt.Fprintf(&b, `<a:ln w="12700"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:ln>`, borderHex)
	} else {
		b.WriteString(`<a:ln><a:noFill/></a:ln>`)
	}
	b.WriteString(`</p:spPr></p:sp>`)
	return b.String()
}

func renderTextBox(id int, name string, x, y, cx, cy int64, fillHex, borderHex string, rounded bool, fontFamily string, paras []textParagraph) string {
	prst := "rect"
	if rounded {
		prst = "roundRect"
	}
	var b strings.Builder
	b.WriteString(`<p:sp><p:nvSpPr>`)
	fmt.Fprintf(&b, `<p:cNvPr id="%d" name="%s"/>`, id, escapeXML(name))
	b.WriteString(`<p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr>`)
	fmt.Fprintf(&b, `<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, x, y, cx, cy)
	fmt.Fprintf(&b, `<a:prstGeom prst="%s"><a:avLst/></a:prstGeom>`, prst)
	if fillHex != "" {
		fmt.Fprintf(&b, `<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, fillHex)
	} else {
		b.WriteString(`<a:noFill/>`)
	}
	if borderHex != "" {
		fmt.Fprintf(&b, `<a:ln w="12700"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:ln>`, borderHex)
	} else {
		b.WriteString(`<a:ln><a:noFill/></a:ln>`)
	}
	b.WriteString(`</p:spPr>`)
	pad := int64(91440)
	if fillHex != "" {
		pad = 182880
	}
	fmt.Fprintf(&b, `<p:txBody><a:bodyPr wrap="square" lIns="%d" tIns="%d" rIns="%d" bIns="%d" anchor="t"/><a:lstStyle/>`, pad, pad, pad, pad)
	if len(paras) == 0 {
		b.WriteString(`<a:p><a:endParaRPr lang="en-US"/></a:p>`)
	} else {
		for _, p := range paras {
			b.WriteString(renderDrawingParagraph(p, fontFamily))
		}
	}
	b.WriteString(`</p:txBody></p:sp>`)
	return b.String()
}

func renderDrawingParagraph(p textParagraph, fontFamily string) string {
	var b strings.Builder
	b.WriteString(`<a:p>`)
	align := p.align
	if align == "" {
		align = "l"
	}
	if p.bullet {
		fmt.Fprintf(&b, `<a:pPr algn="%s" marL="285750" indent="-285750">`, align)
	} else {
		fmt.Fprintf(&b, `<a:pPr algn="%s">`, align)
	}
	if p.spaceBefPt > 0 {
		fmt.Fprintf(&b, `<a:spcBef><a:spcPts val="%d"/></a:spcBef>`, int(math.Round(p.spaceBefPt*100)))
	}
	if p.spaceAftPt > 0 {
		fmt.Fprintf(&b, `<a:spcAft><a:spcPts val="%d"/></a:spcAft>`, int(math.Round(p.spaceAftPt*100)))
	}
	if p.bullet {
		b.WriteString(`<a:buChar char="•"/>`)
	} else {
		b.WriteString(`<a:buNone/>`)
	}
	b.WriteString(`</a:pPr>`)

	sz := int(math.Round(max(p.fontSizePt, 8) * 100))
	boldFlag := 0
	if p.bold {
		boldFlag = 1
	}
	italicFlag := 0
	if p.italic {
		italicFlag = 1
	}
	color := normalizeHexColor(p.color)
	if color == "" {
		color = defaultTextColor
	}
	font := escapeXML(fontFamily)
	lines := strings.Split(p.text, "\n")
	for i, line := range lines {
		if i > 0 {
			b.WriteString(`<a:br/>`)
		}
		fmt.Fprintf(&b, `<a:r><a:rPr lang="en-US" sz="%d" b="%d" i="%d" dirty="0">`, sz, boldFlag, italicFlag)
		fmt.Fprintf(&b, `<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, color)
		fmt.Fprintf(&b, `<a:latin typeface="%s"/><a:ea typeface="%s"/><a:cs typeface="%s"/>`, font, font, font)
		b.WriteString(`</a:rPr><a:t>`)
		b.WriteString(escapeXML(line))
		b.WriteString(`</a:t></a:r>`)
	}
	fmt.Fprintf(&b, `<a:endParaRPr lang="en-US" sz="%d"/>`, sz)
	b.WriteString(`</a:p>`)
	return b.String()
}

func renderTableFrame(id int, x, y, cx, cy int64, tbl *TableSpec, theme Theme) string {
	colCount := len(tbl.Headers)
	for _, row := range tbl.Rows {
		colCount = max(colCount, len(row))
	}
	if colCount == 0 {
		return ""
	}

	colWidths := make([]int64, colCount)
	var sumWeights float64
	if len(tbl.ColWidths) == colCount {
		for _, w := range tbl.ColWidths {
			if w > 0 {
				sumWeights += w
			}
		}
	}
	var used int64
	for i := range colCount {
		if i == colCount-1 {
			colWidths[i] = cx - used
		} else if sumWeights > 0 && tbl.ColWidths[i] > 0 {
			colWidths[i] = int64(math.Round(float64(cx) * (tbl.ColWidths[i] / sumWeights)))
			used += colWidths[i]
		} else {
			colWidths[i] = cx / int64(colCount)
			used += colWidths[i]
		}
	}

	totalRows := len(tbl.Rows)
	if len(tbl.Headers) > 0 {
		totalRows++
	}
	rowH := min(cy/int64(max(totalRows, 1)), int64(548640))
	rowH = max(rowH, int64(340000))
	tableH := rowH * int64(totalRows)

	headerFill := normalizeHexColor(tbl.HeaderFill)
	if headerFill == "" {
		headerFill = theme.PrimaryColor
	}
	headerTextColor := "FFFFFF"
	if !isDarkHex(headerFill) {
		headerTextColor = theme.TextColor
	}

	var b strings.Builder
	b.WriteString(`<p:graphicFrame><p:nvGraphicFramePr>`)
	fmt.Fprintf(&b, `<p:cNvPr id="%d" name="Table"/>`, id)
	b.WriteString(`<p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr>`)
	fmt.Fprintf(&b, `<p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`, x, y, cx, tableH)
	fmt.Fprintf(&b, `<a:graphic><a:graphicData uri="%s"><a:tbl>`, nsTableURI)
	b.WriteString(`<a:tblPr firstRow="1" bandRow="1"/><a:tblGrid>`)
	for _, w := range colWidths {
		fmt.Fprintf(&b, `<a:gridCol w="%d"/>`, w)
	}
	b.WriteString(`</a:tblGrid>`)

	renderCell := func(val, fillHex, txtColor string, bold bool, fontPt float64) {
		b.WriteString(`<a:tc><a:txBody><a:bodyPr/><a:lstStyle/>`)
		b.WriteString(renderDrawingParagraph(textParagraph{
			text:       val,
			fontSizePt: fontPt,
			bold:       bold,
			color:      txtColor,
			align:      "l",
		}, theme.FontFamily))
		b.WriteString(`</a:txBody><a:tcPr marL="91440" marR="91440" marT="68580" marB="68580" anchor="ctr">`)
		b.WriteString(`<a:lnL w="6350"><a:solidFill><a:srgbClr val="E2E8F0"/></a:solidFill></a:lnL>`)
		b.WriteString(`<a:lnR w="6350"><a:solidFill><a:srgbClr val="E2E8F0"/></a:solidFill></a:lnR>`)
		b.WriteString(`<a:lnT w="6350"><a:solidFill><a:srgbClr val="E2E8F0"/></a:solidFill></a:lnT>`)
		b.WriteString(`<a:lnB w="6350"><a:solidFill><a:srgbClr val="E2E8F0"/></a:solidFill></a:lnB>`)
		fmt.Fprintf(&b, `<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, fillHex)
		b.WriteString(`</a:tcPr></a:tc>`)
	}

	if len(tbl.Headers) > 0 {
		fmt.Fprintf(&b, `<a:tr h="%d">`, rowH)
		for i := range colCount {
			val := ""
			if i < len(tbl.Headers) {
				val = tbl.Headers[i]
			}
			renderCell(val, headerFill, headerTextColor, true, 13)
		}
		b.WriteString(`</a:tr>`)
	}

	for rIdx, row := range tbl.Rows {
		fillHex := "FFFFFF"
		if rIdx%2 == 1 {
			fillHex = "F7FAFC"
		}
		fmt.Fprintf(&b, `<a:tr h="%d">`, rowH)
		for cIdx := range colCount {
			val := ""
			if cIdx < len(row) {
				val = row[cIdx]
			}
			renderCell(val, fillHex, theme.TextColor, false, 12)
		}
		b.WriteString(`</a:tr>`)
	}

	b.WriteString(`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`)
	return b.String()
}

func renderPictureShape(id int, name, relID string, x, y, cx, cy int64) string {
	var b strings.Builder
	b.WriteString(`<p:pic><p:nvPicPr>`)
	fmt.Fprintf(&b, `<p:cNvPr id="%d" name="%s"/>`, id, escapeXML(name))
	b.WriteString(`<p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr>`)
	fmt.Fprintf(&b, `<p:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>`, relID)
	b.WriteString(`<p:spPr>`)
	fmt.Fprintf(&b, `<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, x, y, cx, cy)
	b.WriteString(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`)
	return b.String()
}

func buildNotesSlideXML(notes, fontFamily string) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&b, `<p:notes xmlns:a="%s" xmlns:r="%s" xmlns:p="%s">`, nsA, nsR, nsP)
	b.WriteString(`<p:cSld><p:spTree>`)
	b.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`)
	b.WriteString(`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)
	b.WriteString(`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Slide Image Placeholder 1"/><p:cNvSpPr><a:spLocks noGrp="1" noRot="1" noChangeAspect="1"/></p:cNvSpPr><p:nvPr><p:ph type="sldImg"/></p:nvPr></p:nvSpPr><p:spPr/></p:sp>`)
	b.WriteString(`<p:sp><p:nvSpPr><p:cNvPr id="3" name="Notes Placeholder 2"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr/>`)
	b.WriteString(buildNotesTxBody(notes, fontFamily))
	b.WriteString(`</p:sp></p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:notes>`)
	return b.Bytes()
}

func buildNotesTxBody(notes, fontFamily string) string {
	if fontFamily == "" {
		fontFamily = defaultFontFamily
	}
	var b strings.Builder
	b.WriteString(`<p:txBody><a:bodyPr/><a:lstStyle/>`)
	lines := strings.Split(strings.ReplaceAll(notes, "\r\n", "\n"), "\n")
	for _, line := range lines {
		b.WriteString(renderDrawingParagraph(textParagraph{
			text:       line,
			fontSizePt: 12,
			color:      defaultTextColor,
		}, fontFamily))
	}
	b.WriteString(`</p:txBody>`)
	return b.String()
}

func buildNotesMasterXML() []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&b, `<p:notesMaster xmlns:a="%s" xmlns:r="%s" xmlns:p="%s">`, nsA, nsR, nsP)
	b.WriteString(`<p:cSld><p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree>`)
	b.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`)
	b.WriteString(`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)
	b.WriteString(`</p:spTree></p:cSld>`)
	b.WriteString(`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>`)
	b.WriteString(`<p:notesStyle><a:lvl1pPr marL="0" algn="l"><a:defRPr sz="1200"/></a:lvl1pPr></p:notesStyle>`)
	b.WriteString(`</p:notesMaster>`)
	return b.Bytes()
}

func buildSlideMasterXML() []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&b, `<p:sldMaster xmlns:a="%s" xmlns:r="%s" xmlns:p="%s">`, nsA, nsR, nsP)
	b.WriteString(`<p:cSld><p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree>`)
	b.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`)
	b.WriteString(`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)
	b.WriteString(`</p:spTree></p:cSld>`)
	b.WriteString(`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>`)
	b.WriteString(`<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>`)
	b.WriteString(`<p:txStyles><p:titleStyle/><p:bodyStyle/><p:otherStyle/></p:txStyles>`)
	b.WriteString(`</p:sldMaster>`)
	return b.Bytes()
}

func buildSlideLayoutXML() []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&b, `<p:sldLayout xmlns:a="%s" xmlns:r="%s" xmlns:p="%s" type="blank" preserve="1">`, nsA, nsR, nsP)
	b.WriteString(`<p:cSld name="Blank"><p:spTree>`)
	b.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`)
	b.WriteString(`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)
	b.WriteString(`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`)
	return b.Bytes()
}

func buildThemeXML(theme Theme) []byte {
	font := escapeXML(theme.FontFamily)
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&b, `<a:theme xmlns:a="%s" name="Presentation Theme"><a:themeElements>`, nsA)
	b.WriteString(`<a:clrScheme name="CustomPalette">`)
	fmt.Fprintf(&b, `<a:dk1><a:srgbClr val="%s"/></a:dk1>`, theme.TextColor)
	fmt.Fprintf(&b, `<a:lt1><a:srgbClr val="%s"/></a:lt1>`, theme.BackgroundColor)
	fmt.Fprintf(&b, `<a:dk2><a:srgbClr val="%s"/></a:dk2>`, theme.PrimaryColor)
	b.WriteString(`<a:lt2><a:srgbClr val="F7FAFC"/></a:lt2>`)
	fmt.Fprintf(&b, `<a:accent1><a:srgbClr val="%s"/></a:accent1>`, theme.PrimaryColor)
	fmt.Fprintf(&b, `<a:accent2><a:srgbClr val="%s"/></a:accent2>`, theme.AccentColor)
	b.WriteString(`<a:accent3><a:srgbClr val="38A169"/></a:accent3>`)
	b.WriteString(`<a:accent4><a:srgbClr val="D69E2E"/></a:accent4>`)
	b.WriteString(`<a:accent5><a:srgbClr val="E53E3E"/></a:accent5>`)
	b.WriteString(`<a:accent6><a:srgbClr val="805AD5"/></a:accent6>`)
	fmt.Fprintf(&b, `<a:hlink><a:srgbClr val="%s"/></a:hlink>`, theme.AccentColor)
	b.WriteString(`<a:folHlink><a:srgbClr val="553C9A"/></a:folHlink>`)
	b.WriteString(`</a:clrScheme>`)
	fmt.Fprintf(&b, `<a:fontScheme name="CustomFonts"><a:majorFont><a:latin typeface="%s"/><a:ea typeface="%s"/><a:cs typeface="%s"/></a:majorFont><a:minorFont><a:latin typeface="%s"/><a:ea typeface="%s"/><a:cs typeface="%s"/></a:minorFont></a:fontScheme>`, font, font, font, font, font, font)
	b.WriteString(`<a:fmtScheme name="Standard"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst><a:lnStyleLst><a:ln w="9525"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="19050"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="25400"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme>`)
	b.WriteString(`</a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/></a:theme>`)
	return b.Bytes()
}

func buildCorePropsXML(opts CreateOptions) []byte {
	now := time.Now().UTC().Format(time.RFC3339)
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&b, `<cp:coreProperties xmlns:cp="%s" xmlns:dc="%s" xmlns:dcterms="%s" xmlns:xsi="%s">`, nsCP, nsDC, nsDCTerms, nsXSI)
	if v := strings.TrimSpace(opts.Title); v != "" {
		fmt.Fprintf(&b, `<dc:title>%s</dc:title>`, escapeXML(v))
	}
	if v := strings.TrimSpace(opts.Author); v != "" {
		fmt.Fprintf(&b, `<dc:creator>%s</dc:creator>`, escapeXML(v))
	}
	if v := strings.TrimSpace(opts.Subject); v != "" {
		fmt.Fprintf(&b, `<dc:subject>%s</dc:subject>`, escapeXML(v))
	}
	fmt.Fprintf(&b, `<dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created>`, now)
	fmt.Fprintf(&b, `<dcterms:modified xsi:type="dcterms:W3CDTF">%s</dcterms:modified>`, now)
	b.WriteString(`</cp:coreProperties>`)
	return b.Bytes()
}

func buildRelsXML(rels []relDef) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&b, `<Relationships xmlns="%s">`, nsPkgRel)
	for _, r := range rels {
		fmt.Fprintf(&b, `<Relationship Id="%s" Type="%s" Target="%s"/>`, escapeXML(r.id), escapeXML(r.relType), escapeXML(r.target))
	}
	b.WriteString(`</Relationships>`)
	return b.Bytes()
}

func buildContentTypesXML(slideParts, notesParts []string, hasNotesMaster, hasPNG, hasJPEG bool) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&b, `<Types xmlns="%s">`, nsContentTypes)
	fmt.Fprintf(&b, `<Default Extension="rels" ContentType="%s"/>`, ctRels)
	b.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	if hasPNG {
		b.WriteString(`<Default Extension="png" ContentType="image/png"/>`)
	}
	if hasJPEG {
		b.WriteString(`<Default Extension="jpeg" ContentType="image/jpeg"/>`)
		b.WriteString(`<Default Extension="jpg" ContentType="image/jpeg"/>`)
	}
	fmt.Fprintf(&b, `<Override PartName="/ppt/presentation.xml" ContentType="%s"/>`, ctPresentation)
	fmt.Fprintf(&b, `<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="%s"/>`, ctSlideMaster)
	fmt.Fprintf(&b, `<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="%s"/>`, ctSlideLayout)
	fmt.Fprintf(&b, `<Override PartName="/ppt/theme/theme1.xml" ContentType="%s"/>`, ctTheme)
	fmt.Fprintf(&b, `<Override PartName="/docProps/core.xml" ContentType="%s"/>`, ctCoreProps)
	if hasNotesMaster {
		fmt.Fprintf(&b, `<Override PartName="/ppt/notesMasters/notesMaster1.xml" ContentType="%s"/>`, ctNotesMaster)
	}
	for _, sp := range slideParts {
		fmt.Fprintf(&b, `<Override PartName="%s" ContentType="%s"/>`, escapeXML(sp), ctSlide)
	}
	for _, np := range notesParts {
		fmt.Fprintf(&b, `<Override PartName="%s" ContentType="%s"/>`, escapeXML(np), ctNotesSlide)
	}
	b.WriteString(`</Types>`)
	return b.Bytes()
}
