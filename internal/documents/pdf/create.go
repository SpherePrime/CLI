package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/documents/docutil"
	"github.com/SpherePrime/CLI/vendordeps/signintech/gopdf"
)

const (
	fontRegular = "regular"
	fontBold    = "bold"
	fontItalic  = "italic"
	fontMono    = "mono"

	defaultMarginPT = 50.0
	bodyFontSize    = 10.5
	bodyLineHeight  = 15.0
)

type rgbColor struct {
	r, g, b uint8
}

type layoutWriter struct {
	pdf          *gopdf.GoPdf
	opts         CreateOptions
	pageW        float64
	pageH        float64
	margins      Margins
	contentLeft  float64
	contentWidth float64
	contentTop   float64
	bottomLimit  float64
	curY         float64
}

func Create(opts CreateOptions) (*Result, error) {
	outPath, err := resolvePath(opts.WorkingDir, opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("invalid output_path: %w", err)
	}

	pageRect := resolvePageRect(opts.PageSize, opts.Orientation)
	margins := normalizeMargins(opts.Margins)

	if margins.Left+margins.Right >= pageRect.W-40 {
		return nil, errors.New("horizontal margins leave no space for content")
	}
	if margins.Top+margins.Bottom >= pageRect.H-60 {
		return nil, errors.New("vertical margins leave no space for content")
	}

	gp := &gopdf.GoPdf{}
	gp.Start(gopdf.Config{
		PageSize: pageRect,
		Unit:     gopdf.UnitPT,
	})
	gp.SetMargins(margins.Left, margins.Top, margins.Right, margins.Bottom)

	if err := loadEmbeddedFonts(gp); err != nil {
		return nil, err
	}

	gp.SetInfo(gopdf.PdfInfo{
		Title:        opts.Title,
		Author:       opts.Author,
		Subject:      opts.Subject,
		Creator:      "Prime",
		Producer:     "Prime PDF Engine",
		CreationDate: time.Now(),
	})

	contentTop := margins.Top
	if strings.TrimSpace(opts.Header) != "" && contentTop < 48 {
		contentTop = 48
	}
	bottomLimit := pageRect.H - margins.Bottom
	if (strings.TrimSpace(opts.Footer) != "" || opts.PageNumbers) && bottomLimit > pageRect.H-46 {
		bottomLimit = pageRect.H - 46
	}

	w := &layoutWriter{
		pdf:          gp,
		opts:         opts,
		pageW:        pageRect.W,
		pageH:        pageRect.H,
		margins:      margins,
		contentLeft:  margins.Left,
		contentWidth: pageRect.W - margins.Left - margins.Right,
		contentTop:   contentTop,
		bottomLimit:  bottomLimit,
	}

	if err := w.newPage(); err != nil {
		return nil, err
	}
	if err := w.renderDocumentHeader(); err != nil {
		return nil, err
	}
	for i, blk := range opts.Blocks {
		if err := w.renderBlock(blk); err != nil {
			return nil, fmt.Errorf("block %d (%s): %w", i, blk.Type, err)
		}
	}

	data, err := gp.GetBytesPdfReturnErr()
	if err != nil {
		return nil, fmt.Errorf("compile pdf: %w", err)
	}
	if err := docutil.WriteFileAtomic(outPath, data); err != nil {
		return nil, fmt.Errorf("write pdf: %w", err)
	}

	return &Result{
		Path:      outPath,
		PageCount: gp.GetNumberOfPages(),
		SizeBytes: int64(len(data)),
	}, nil
}

func resolvePageRect(pageSize, orientation string) gopdf.Rect {
	var base gopdf.Rect
	switch strings.ToLower(strings.TrimSpace(pageSize)) {
	case "letter":
		base = *gopdf.PageSizeLetter
	default:
		base = *gopdf.PageSizeA4
	}
	if strings.EqualFold(strings.TrimSpace(orientation), "landscape") {
		base.W, base.H = base.H, base.W
	}
	return base
}

func normalizeMargins(m Margins) Margins {
	if m.Top <= 0 {
		m.Top = defaultMarginPT
	}
	if m.Right <= 0 {
		m.Right = defaultMarginPT
	}
	if m.Bottom <= 0 {
		m.Bottom = defaultMarginPT
	}
	if m.Left <= 0 {
		m.Left = defaultMarginPT
	}
	return m
}

func loadEmbeddedFonts(gp *gopdf.GoPdf) error {
	fonts := []struct {
		name string
		data []byte
	}{
		{fontRegular, docutil.RegularFontBytes()},
		{fontBold, docutil.BoldFontBytes()},
		{fontItalic, docutil.ItalicFontBytes()},
		{fontMono, docutil.MonoFontBytes()},
	}
	for _, f := range fonts {
		if err := gp.AddTTFFontData(f.name, f.data); err != nil {
			return fmt.Errorf("load font %s: %w", f.name, err)
		}
	}
	return nil
}

func (w *layoutWriter) newPage() error {
	w.pdf.AddPage()
	w.curY = w.contentTop

	if hdr := strings.TrimSpace(w.opts.Header); hdr != "" {
		if err := w.pdf.SetFont(fontRegular, "", 8.5); err != nil {
			return err
		}
		w.pdf.SetTextColor(100, 116, 139)
		w.pdf.SetXY(w.contentLeft, 20)
		if err := w.pdf.CellWithOption(&gopdf.Rect{W: w.contentWidth, H: 12}, hdr, gopdf.CellOption{
			Align: gopdf.Left | gopdf.Top,
			Float: gopdf.Right,
		}); err != nil {
			return err
		}
		w.pdf.SetStrokeColor(226, 232, 240)
		w.pdf.SetLineWidth(0.5)
		w.pdf.Line(w.contentLeft, 35, w.contentLeft+w.contentWidth, 35)
	}

	footerText := strings.TrimSpace(w.opts.Footer)
	if footerText != "" || w.opts.PageNumbers {
		footerY := w.pageH - 28
		w.pdf.SetStrokeColor(226, 232, 240)
		w.pdf.SetLineWidth(0.5)
		w.pdf.Line(w.contentLeft, footerY-6, w.contentLeft+w.contentWidth, footerY-6)

		if err := w.pdf.SetFont(fontRegular, "", 8.5); err != nil {
			return err
		}
		w.pdf.SetTextColor(100, 116, 139)
		if footerText != "" {
			w.pdf.SetXY(w.contentLeft, footerY)
			if err := w.pdf.CellWithOption(&gopdf.Rect{W: w.contentWidth * 0.75, H: 12}, footerText, gopdf.CellOption{
				Align: gopdf.Left | gopdf.Top,
				Float: gopdf.Right,
			}); err != nil {
				return err
			}
		}
		if w.opts.PageNumbers {
			pageLabel := strconv.Itoa(w.pdf.GetNumberOfPages())
			w.pdf.SetXY(w.contentLeft, footerY)
			if err := w.pdf.CellWithOption(&gopdf.Rect{W: w.contentWidth, H: 12}, pageLabel, gopdf.CellOption{
				Align: gopdf.Right | gopdf.Top,
				Float: gopdf.Right,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *layoutWriter) ensureSpace(need float64) error {
	if w.curY+need > w.bottomLimit && w.curY > w.contentTop+1 {
		return w.newPage()
	}
	return nil
}

func (w *layoutWriter) renderDocumentHeader() error {
	title := strings.TrimSpace(w.opts.Title)
	subtitle := strings.TrimSpace(w.opts.Subtitle)
	if title == "" && subtitle == "" {
		return nil
	}
	if title != "" {
		if err := w.pdf.SetFont(fontBold, "", 21); err != nil {
			return err
		}
		w.pdf.SetTextColor(15, 23, 42)
		lines, err := wrapText(w.pdf, title, w.contentWidth)
		if err != nil {
			return err
		}
		for _, line := range lines {
			if err := w.drawLine(w.contentLeft, w.contentWidth, 27, line, gopdf.Left); err != nil {
				return err
			}
		}
		w.curY += 2
	}
	if subtitle != "" {
		if err := w.pdf.SetFont(fontItalic, "", 12); err != nil {
			return err
		}
		w.pdf.SetTextColor(71, 85, 105)
		lines, err := wrapText(w.pdf, subtitle, w.contentWidth)
		if err != nil {
			return err
		}
		for _, line := range lines {
			if err := w.drawLine(w.contentLeft, w.contentWidth, 17, line, gopdf.Left); err != nil {
				return err
			}
		}
	}
	w.curY += 6
	w.pdf.SetStrokeColor(37, 99, 235)
	w.pdf.SetLineWidth(1.5)
	w.pdf.Line(w.contentLeft, w.curY, w.contentLeft+w.contentWidth, w.curY)
	w.curY += 14
	return nil
}

func (w *layoutWriter) renderBlock(blk Block) error {
	switch strings.ToLower(strings.TrimSpace(blk.Type)) {
	case "heading":
		return w.renderHeading(blk)
	case "paragraph":
		return w.renderParagraph(blk)
	case "bullet_list":
		return w.renderList(blk.Items, false)
	case "numbered_list":
		return w.renderList(blk.Items, true)
	case "table":
		return w.renderTable(blk)
	case "callout":
		return w.renderCallout(blk)
	case "image":
		return w.renderImage(blk)
	case "divider":
		return w.renderDivider()
	case "page_break":
		return w.newPage()
	default:
		return fmt.Errorf("unsupported block type %q", blk.Type)
	}
}

func (w *layoutWriter) renderHeading(blk Block) error {
	text := strings.TrimSpace(blk.Text)
	if text == "" {
		return nil
	}
	level := blk.Level
	if level < 1 {
		level = 1
	} else if level > 3 {
		level = 3
	}

	var size, lineH, spaceBefore, spaceAfter float64
	var col rgbColor
	switch level {
	case 1:
		size, lineH, spaceBefore, spaceAfter = 16.5, 22, 12, 5
		col = rgbColor{15, 23, 42}
	case 2:
		size, lineH, spaceBefore, spaceAfter = 13.5, 18.5, 10, 4
		col = rgbColor{30, 41, 59}
	default:
		size, lineH, spaceBefore, spaceAfter = 11.5, 16, 8, 3
		col = rgbColor{51, 65, 85}
	}

	if w.curY > w.contentTop+1 {
		w.curY += spaceBefore
	}
	if err := w.pdf.SetFont(fontBold, "", size); err != nil {
		return err
	}
	lines, err := wrapText(w.pdf, text, w.contentWidth)
	if err != nil {
		return err
	}
	if err := w.ensureSpace(float64(len(lines))*lineH + bodyLineHeight); err != nil {
		return err
	}
	if err := w.pdf.SetFont(fontBold, "", size); err != nil {
		return err
	}
	w.pdf.SetTextColor(col.r, col.g, col.b)
	align := parseAlign(blk.Align)
	for _, line := range lines {
		if err := w.drawLine(w.contentLeft, w.contentWidth, lineH, line, align); err != nil {
			return err
		}
	}
	w.curY += spaceAfter
	return nil
}

func (w *layoutWriter) renderParagraph(blk Block) error {
	if blk.Text == "" {
		w.curY += bodyLineHeight * 0.6
		return nil
	}
	if err := w.pdf.SetFont(fontRegular, "", bodyFontSize); err != nil {
		return err
	}
	lines, err := wrapText(w.pdf, blk.Text, w.contentWidth)
	if err != nil {
		return err
	}
	align := parseAlign(blk.Align)
	for i, line := range lines {
		if err := w.ensureSpace(bodyLineHeight); err != nil {
			return err
		}
		if err := w.pdf.SetFont(fontRegular, "", bodyFontSize); err != nil {
			return err
		}
		w.pdf.SetTextColor(30, 41, 59)
		lineAlign := align
		if lineAlign == gopdf.Justify && i == len(lines)-1 {
			lineAlign = gopdf.Left
		}
		if err := w.drawLine(w.contentLeft, w.contentWidth, bodyLineHeight, line, lineAlign); err != nil {
			return err
		}
	}
	w.curY += 6
	return nil
}

func (w *layoutWriter) renderList(items []string, numbered bool) error {
	if len(items) == 0 {
		return nil
	}
	indent := 18.0
	textWidth := w.contentWidth - indent
	if textWidth < 40 {
		textWidth = 40
	}

	for idx, item := range items {
		marker := "•"
		if numbered {
			marker = strconv.Itoa(idx+1) + "."
		}
		if err := w.pdf.SetFont(fontRegular, "", bodyFontSize); err != nil {
			return err
		}
		lines, err := wrapText(w.pdf, item, textWidth)
		if err != nil {
			return err
		}
		if len(lines) == 0 {
			lines = []string{""}
		}
		for li, line := range lines {
			if err := w.ensureSpace(bodyLineHeight); err != nil {
				return err
			}
			if li == 0 {
				if err := w.pdf.SetFont(fontBold, "", bodyFontSize); err != nil {
					return err
				}
				w.pdf.SetTextColor(37, 99, 235)
				w.pdf.SetXY(w.contentLeft+2, w.curY)
				if err := w.pdf.CellWithOption(&gopdf.Rect{W: indent - 2, H: bodyLineHeight}, marker, gopdf.CellOption{
					Align: gopdf.Left | gopdf.Top,
					Float: gopdf.Right,
				}); err != nil {
					return err
				}
			}
			if err := w.pdf.SetFont(fontRegular, "", bodyFontSize); err != nil {
				return err
			}
			w.pdf.SetTextColor(30, 41, 59)
			if err := w.drawLine(w.contentLeft+indent, textWidth, bodyLineHeight, line, gopdf.Left); err != nil {
				return err
			}
		}
		w.curY += 2
	}
	w.curY += 5
	return nil
}

func (w *layoutWriter) renderTable(blk Block) error {
	numCols := len(blk.Headers)
	for _, row := range blk.Rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}
	if numCols == 0 {
		return nil
	}

	colWidths := resolveColWidths(numCols, blk.ColWidths, w.contentWidth)
	colAligns := resolveColAligns(numCols, blk.Align)
	cellPadX := 6.0
	cellPadY := 4.5
	tableFontSize := 9.5
	tableLineH := 13.5

	measureRow := func(cells []string, fontName string) ([][]string, float64, error) {
		if err := w.pdf.SetFont(fontName, "", tableFontSize); err != nil {
			return nil, 0, err
		}
		wrapped := make([][]string, numCols)
		maxLines := 1
		for c := 0; c < numCols; c++ {
			val := ""
			if c < len(cells) {
				val = cells[c]
			}
			avail := colWidths[c] - 2*cellPadX
			if avail < 10 {
				avail = 10
			}
			lines, err := wrapText(w.pdf, val, avail)
			if err != nil {
				return nil, 0, err
			}
			wrapped[c] = lines
			if len(lines) > maxLines {
				maxLines = len(lines)
			}
		}
		return wrapped, float64(maxLines)*tableLineH + 2*cellPadY, nil
	}

	drawRow := func(wrapped [][]string, rowH float64, fontName string, bg *rgbColor, fg rgbColor) error {
		x := w.contentLeft
		y := w.curY
		for c := 0; c < numCols; c++ {
			cw := colWidths[c]
			w.pdf.SetStrokeColor(203, 213, 225)
			w.pdf.SetLineWidth(0.5)
			if bg != nil {
				w.pdf.SetFillColor(bg.r, bg.g, bg.b)
				if err := w.pdf.Rectangle(x, y, x+cw, y+rowH, "DF", 0, 0); err != nil {
					return err
				}
			} else {
				if err := w.pdf.Rectangle(x, y, x+cw, y+rowH, "D", 0, 0); err != nil {
					return err
				}
			}
			if err := w.pdf.SetFont(fontName, "", tableFontSize); err != nil {
				return err
			}
			w.pdf.SetTextColor(fg.r, fg.g, fg.b)
			ty := y + cellPadY
			for _, line := range wrapped[c] {
				if line != "" {
					w.pdf.SetXY(x+cellPadX, ty)
					if err := w.pdf.CellWithOption(&gopdf.Rect{W: cw - 2*cellPadX, H: tableLineH}, line, gopdf.CellOption{
						Align: colAligns[c] | gopdf.Top,
						Float: gopdf.Right,
					}); err != nil {
						return err
					}
				}
				ty += tableLineH
			}
			x += cw
		}
		w.curY += rowH
		return nil
	}

	var headerWrapped [][]string
	var headerH float64
	hasHeader := len(blk.Headers) > 0
	if hasHeader {
		var err error
		headerWrapped, headerH, err = measureRow(blk.Headers, fontBold)
		if err != nil {
			return err
		}
	}

	headerBG := &rgbColor{30, 41, 59}
	headerFG := rgbColor{255, 255, 255}
	zebraBG := &rgbColor{248, 250, 252}
	bodyFG := rgbColor{30, 41, 59}

	if hasHeader {
		firstNeed := headerH
		if len(blk.Rows) > 0 {
			_, firstRowH, err := measureRow(blk.Rows[0], fontRegular)
			if err != nil {
				return err
			}
			firstNeed += firstRowH
		}
		if err := w.ensureSpace(firstNeed); err != nil {
			return err
		}
		if err := drawRow(headerWrapped, headerH, fontBold, headerBG, headerFG); err != nil {
			return err
		}
	}

	for rIdx, row := range blk.Rows {
		wrapped, rowH, err := measureRow(row, fontRegular)
		if err != nil {
			return err
		}
		if w.curY+rowH > w.bottomLimit && w.curY > w.contentTop+1 {
			if err := w.newPage(); err != nil {
				return err
			}
			if hasHeader {
				if err := drawRow(headerWrapped, headerH, fontBold, headerBG, headerFG); err != nil {
					return err
				}
			}
		}
		var bg *rgbColor
		if rIdx%2 == 1 {
			bg = zebraBG
		}
		if err := drawRow(wrapped, rowH, fontRegular, bg, bodyFG); err != nil {
			return err
		}
	}

	w.curY += 10
	return nil
}

func (w *layoutWriter) renderCallout(blk Block) error {
	title := strings.TrimSpace(blk.Title)
	text := strings.TrimSpace(blk.Text)
	if title == "" && text == "" {
		return nil
	}

	padLeft := 14.0
	padRight := 10.0
	padY := 8.0
	innerW := w.contentWidth - padLeft - padRight

	var titleLines []string
	var textLines []string
	var err error
	if title != "" {
		if err = w.pdf.SetFont(fontBold, "", 10.5); err != nil {
			return err
		}
		titleLines, err = wrapText(w.pdf, title, innerW)
		if err != nil {
			return err
		}
	}
	if text != "" {
		if err = w.pdf.SetFont(fontRegular, "", 10.0); err != nil {
			return err
		}
		textLines, err = wrapText(w.pdf, text, innerW)
		if err != nil {
			return err
		}
	}

	boxH := 2 * padY
	if len(titleLines) > 0 {
		boxH += float64(len(titleLines))*15.0 + 2
	}
	if len(textLines) > 0 {
		boxH += float64(len(textLines)) * 14.5
	}

	if err := w.ensureSpace(boxH + 6); err != nil {
		return err
	}

	bg, accent, titleCol := calloutPalette(blk.Tone)
	topY := w.curY
	w.pdf.SetFillColor(bg.r, bg.g, bg.b)
	if err := w.pdf.Rectangle(w.contentLeft, topY, w.contentLeft+w.contentWidth, topY+boxH, "F", 0, 0); err != nil {
		return err
	}
	w.pdf.SetFillColor(accent.r, accent.g, accent.b)
	if err := w.pdf.Rectangle(w.contentLeft, topY, w.contentLeft+3.5, topY+boxH, "F", 0, 0); err != nil {
		return err
	}

	w.curY = topY + padY
	if len(titleLines) > 0 {
		if err := w.pdf.SetFont(fontBold, "", 10.5); err != nil {
			return err
		}
		w.pdf.SetTextColor(titleCol.r, titleCol.g, titleCol.b)
		for _, line := range titleLines {
			if err := w.drawLine(w.contentLeft+padLeft, innerW, 15.0, line, gopdf.Left); err != nil {
				return err
			}
		}
		w.curY += 2
	}
	if len(textLines) > 0 {
		if err := w.pdf.SetFont(fontRegular, "", 10.0); err != nil {
			return err
		}
		w.pdf.SetTextColor(30, 41, 59)
		for _, line := range textLines {
			if err := w.drawLine(w.contentLeft+padLeft, innerW, 14.5, line, gopdf.Left); err != nil {
				return err
			}
		}
	}

	w.curY = topY + boxH + 10
	return nil
}

func calloutPalette(tone string) (bg, accent, title rgbColor) {
	switch strings.ToLower(strings.TrimSpace(tone)) {
	case "warning":
		return rgbColor{255, 251, 235}, rgbColor{217, 119, 6}, rgbColor{146, 64, 14}
	case "success":
		return rgbColor{240, 253, 244}, rgbColor{22, 163, 74}, rgbColor{22, 101, 52}
	case "error", "danger":
		return rgbColor{254, 242, 242}, rgbColor{220, 38, 38}, rgbColor{153, 27, 27}
	default:
		return rgbColor{239, 246, 255}, rgbColor{37, 99, 235}, rgbColor{30, 64, 175}
	}
}

func (w *layoutWriter) renderImage(blk Block) error {
	imgPath, err := resolvePath(w.opts.WorkingDir, blk.ImagePath)
	if err != nil {
		return fmt.Errorf("invalid image_path: %w", err)
	}
	raw, err := os.ReadFile(imgPath)
	if err != nil {
		return fmt.Errorf("read image %s: %w", imgPath, err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("decode image %s: %w", imgPath, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("invalid image dimensions %dx%d", cfg.Width, cfg.Height)
	}

	aspect := float64(cfg.Height) / float64(cfg.Width)
	drawW := blk.Width
	drawH := blk.Height
	switch {
	case drawW > 0 && drawH > 0:
	case drawW > 0:
		drawH = drawW * aspect
	case drawH > 0:
		drawW = drawH / aspect
	default:
		drawW = float64(cfg.Width) * 0.75
		if drawW > w.contentWidth {
			drawW = w.contentWidth
		}
		drawH = drawW * aspect
	}

	maxH := w.bottomLimit - w.contentTop - 30
	if drawW > w.contentWidth {
		scale := w.contentWidth / drawW
		drawW *= scale
		drawH *= scale
	}
	if drawH > maxH && maxH > 40 {
		scale := maxH / drawH
		drawW *= scale
		drawH *= scale
	}

	caption := strings.TrimSpace(blk.Caption)
	var captionLines []string
	captionH := 0.0
	if caption != "" {
		if err := w.pdf.SetFont(fontItalic, "", 9.0); err != nil {
			return err
		}
		captionLines, err = wrapText(w.pdf, caption, w.contentWidth)
		if err != nil {
			return err
		}
		captionH = float64(len(captionLines))*13.0 + 4
	}

	if err := w.ensureSpace(drawH + captionH + 8); err != nil {
		return err
	}

	holder, err := gopdf.ImageHolderByBytes(raw)
	if err != nil {
		return fmt.Errorf("prepare image %s: %w", imgPath, err)
	}

	x := w.contentLeft
	switch parseAlign(blk.Align) {
	case gopdf.Center:
		x = w.contentLeft + (w.contentWidth-drawW)/2
	case gopdf.Right:
		x = w.contentLeft + w.contentWidth - drawW
	}

	if err := w.pdf.ImageByHolder(holder, x, w.curY, &gopdf.Rect{W: drawW, H: drawH}); err != nil {
		return fmt.Errorf("embed image %s: %w", imgPath, err)
	}
	w.curY += drawH + 4

	if len(captionLines) > 0 {
		if err := w.pdf.SetFont(fontItalic, "", 9.0); err != nil {
			return err
		}
		w.pdf.SetTextColor(100, 116, 139)
		capAlign := parseAlign(blk.Align)
		for _, line := range captionLines {
			if err := w.drawLine(w.contentLeft, w.contentWidth, 13.0, line, capAlign); err != nil {
				return err
			}
		}
	}
	w.curY += 8
	return nil
}

func (w *layoutWriter) renderDivider() error {
	if err := w.ensureSpace(14); err != nil {
		return err
	}
	w.curY += 5
	w.pdf.SetStrokeColor(203, 213, 225)
	w.pdf.SetLineWidth(0.75)
	w.pdf.Line(w.contentLeft, w.curY, w.contentLeft+w.contentWidth, w.curY)
	w.curY += 9
	return nil
}

func (w *layoutWriter) drawLine(x, width, lineH float64, line string, align int) error {
	if line != "" {
		w.pdf.SetXY(x, w.curY)
		if err := w.pdf.CellWithOption(&gopdf.Rect{W: width, H: lineH}, line, gopdf.CellOption{
			Align: align | gopdf.Top,
			Float: gopdf.Right,
		}); err != nil {
			return err
		}
	}
	w.curY += lineH
	return nil
}

func parseAlign(raw string) int {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "center":
		return gopdf.Center
	case "right":
		return gopdf.Right
	case "justify":
		return gopdf.Justify
	default:
		return gopdf.Left
	}
}

func resolveColWidths(numCols int, specified []float64, totalWidth float64) []float64 {
	widths := make([]float64, numCols)
	valid := len(specified) == numCols
	sum := 0.0
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
		each := totalWidth / float64(numCols)
		for i := range widths {
			widths[i] = each
		}
		return widths
	}
	for i, v := range specified {
		widths[i] = (v / sum) * totalWidth
	}
	return widths
}

func resolveColAligns(numCols int, raw string) []int {
	aligns := make([]int, numCols)
	parts := strings.Split(raw, ",")
	for i := 0; i < numCols; i++ {
		tok := ""
		if i < len(parts) {
			tok = parts[i]
		} else if len(parts) == 1 {
			tok = parts[0]
		}
		aligns[i] = parseAlign(tok)
	}
	return aligns
}

func wrapText(gp *gopdf.GoPdf, text string, maxWidth float64) ([]string, error) {
	if text == "" {
		return []string{""}, nil
	}
	if maxWidth <= 0 {
		return []string{text}, nil
	}

	rawLines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var out []string
	for _, raw := range rawLines {
		if strings.TrimSpace(raw) == "" {
			out = append(out, "")
			continue
		}
		words := strings.Fields(raw)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		current := ""
		for _, word := range words {
			candidate := word
			if current != "" {
				candidate = current + " " + word
			}
			cw, err := gp.MeasureTextWidth(candidate)
			if err != nil {
				return nil, err
			}
			if cw <= maxWidth {
				current = candidate
				continue
			}
			if current != "" {
				out = append(out, current)
				current = ""
			}
			ww, err := gp.MeasureTextWidth(word)
			if err != nil {
				return nil, err
			}
			if ww <= maxWidth {
				current = word
				continue
			}
			chunks, err := splitLongWord(gp, word, maxWidth)
			if err != nil {
				return nil, err
			}
			if len(chunks) > 1 {
				out = append(out, chunks[:len(chunks)-1]...)
			}
			if len(chunks) > 0 {
				current = chunks[len(chunks)-1]
			}
		}
		if current != "" {
			out = append(out, current)
		}
	}
	return out, nil
}

func splitLongWord(gp *gopdf.GoPdf, word string, maxWidth float64) ([]string, error) {
	var chunks []string
	var buf []rune
	for _, r := range word {
		next := append(buf, r)
		w, err := gp.MeasureTextWidth(string(next))
		if err != nil {
			return nil, err
		}
		if w > maxWidth && len(buf) > 0 {
			chunks = append(chunks, string(buf))
			buf = []rune{r}
		} else {
			buf = next
		}
	}
	if len(buf) > 0 {
		chunks = append(chunks, string(buf))
	}
	return chunks, nil
}
