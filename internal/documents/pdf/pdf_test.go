package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 120, 60))
	for y := range 60 {
		for x := range 120 {
			img.Set(x, y, color.RGBA{R: 37, G: 99, B: 235, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
}

func writeTestJPEG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	for y := range 50 {
		for x := range 100 {
			img.Set(x, y, color.RGBA{R: 22, G: 163, B: 74, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test jpeg: %v", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
}

func TestCreateAndInspectCyrillic(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "chart.png")
	writeTestPNG(t, imgPath)

	outPath := filepath.Join(dir, "report.pdf")
	longParagraph := strings.Repeat("Система анализирует финансовые и операционные показатели предприятия в реальном времени без потери точности. ", 5)

	rows := make([][]string, 0, 18)
	for i := 1; i <= 18; i++ {
		rows = append(rows, []string{
			"Регион " + string(rune('А'+i-1)),
			"Выполнено",
			"125 000 ₽",
		})
	}

	res, err := Create(CreateOptions{
		OutputPath:  outPath,
		Title:       "Аналитический отчёт 2026",
		Subtitle:    "Квартальные показатели и выводы",
		Author:      "Иван Петров",
		PageSize:    "A4",
		Orientation: "portrait",
		Margins:     Margins{Top: 50, Right: 45, Bottom: 50, Left: 45},
		Header:      "Конфиденциально — Отдел аналитики",
		Footer:      "ООО Сфера",
		PageNumbers: true,
		Blocks: []Block{
			{Type: "heading", Level: 1, Text: "1. Обзор ключевых результатов"},
			{Type: "heading", Level: 2, Text: "1.1 Динамика выручки"},
			{Type: "heading", Level: 3, Text: "Детализация по сегментам"},
			{Type: "paragraph", Text: longParagraph, Align: "left"},
			{Type: "callout", Title: "Важное примечание", Tone: "info", Text: "Все показатели проверены внутренним аудитом и утверждены руководством."},
			{Type: "divider"},
			{Type: "bullet_list", Items: []string{
				"Рост выручки составил 24 процента год к году",
				"Снижение издержек на инфраструктуру на 15 процентов",
			}},
			{Type: "numbered_list", Items: []string{
				"Утвердить бюджет следующего квартала",
				"Запустить автоматизацию отчётности",
			}},
			{Type: "image", ImagePath: imgPath, Width: 180, Caption: "Рисунок 1 — Визуализация метрик"},
			{Type: "table", Headers: []string{"Регион", "Статус", "Итого"}, Rows: rows, ColWidths: []float64{2, 1.5, 1.5}, Align: "left"},
			{Type: "page_break"},
			{Type: "heading", Level: 1, Text: "2. Заключительный раздел"},
			{Type: "paragraph", Text: "Текст на принудительно новой странице после разрыва."},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if res.Path != outPath {
		t.Fatalf("Path = %q, want %q", res.Path, outPath)
	}
	if res.PageCount < 2 {
		t.Fatalf("PageCount = %d, want >= 2", res.PageCount)
	}
	if res.SizeBytes <= 0 {
		t.Fatalf("SizeBytes = %d, want > 0", res.SizeBytes)
	}

	info, err := Inspect(outPath, "")
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	if info.PageCount != res.PageCount {
		t.Fatalf("Inspect PageCount = %d, want %d", info.PageCount, res.PageCount)
	}
	if info.Metadata.Title != "Аналитический отчёт 2026" {
		t.Fatalf("Metadata.Title = %q, want %q", info.Metadata.Title, "Аналитический отчёт 2026")
	}
	if info.Metadata.Author != "Иван Петров" {
		t.Fatalf("Metadata.Author = %q, want %q", info.Metadata.Author, "Иван Петров")
	}
	if info.Metadata.Creator != "Prime" {
		t.Fatalf("Metadata.Creator = %q, want %q", info.Metadata.Creator, "Prime")
	}
	if info.Metadata.Producer != "Prime PDF Engine" {
		t.Fatalf("Metadata.Producer = %q, want %q", info.Metadata.Producer, "Prime PDF Engine")
	}
	if len(info.Pages) != res.PageCount {
		t.Fatalf("len(Pages) = %d, want %d", len(info.Pages), res.PageCount)
	}

	var allText strings.Builder
	for _, p := range info.Pages {
		allText.WriteString(p.Text)
		allText.WriteByte('\n')
	}
	joined := allText.String()

	required := []string{
		"Аналитический отчёт 2026",
		"Квартальные показатели и выводы",
		"Конфиденциально — Отдел аналитики",
		"ООО Сфера",
		"Обзор ключевых результатов",
		"Динамика выручки",
		"Детализация по сегментам",
		"операционные показатели предприятия",
		"Важное примечание",
		"Рост выручки составил 24 процента",
		"Утвердить бюджет следующего квартала",
		"Визуализация метрик",
		"Регион Статус Итого",
		"Выполнено",
		"Заключительный раздел",
		"принудительно новой странице",
	}
	for _, want := range required {
		if !strings.Contains(joined, want) {
			t.Errorf("extracted text missing %q; full text:\n%s", want, joined)
		}
	}

	pageOne, err := Inspect(outPath, "1")
	if err != nil {
		t.Fatalf("Inspect page 1 failed: %v", err)
	}
	if len(pageOne.Pages) != 1 || pageOne.Pages[0].Number != 1 {
		t.Fatalf("Inspect page 1 returned %+v", pageOne.Pages)
	}
}

func TestCreateLandscapeLetter(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "landscape.pdf")

	res, err := Create(CreateOptions{
		OutputPath:  outPath,
		Title:       "Альбомный документ",
		PageSize:    "Letter",
		Orientation: "landscape",
		Blocks: []Block{
			{Type: "paragraph", Text: "Проверка альбомной ориентации."},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if res.PageCount != 1 {
		t.Fatalf("PageCount = %d, want 1", res.PageCount)
	}

	info, err := Inspect(outPath, "")
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	if len(info.Pages) != 1 {
		t.Fatalf("len(Pages) = %d, want 1", len(info.Pages))
	}
	p := info.Pages[0]
	if p.Width <= p.Height {
		t.Fatalf("expected landscape dimensions (width > height), got %.1fx%.1f", p.Width, p.Height)
	}
}

func TestMergeSplitRotate(t *testing.T) {
	dir := t.TempDir()
	pdf1 := filepath.Join(dir, "first.pdf")
	pdf2 := filepath.Join(dir, "second.pdf")

	if _, err := Create(CreateOptions{
		OutputPath: pdf1,
		Title:      "Первый файл",
		Blocks: []Block{
			{Type: "heading", Level: 1, Text: "Страница первая из первого документа"},
		},
	}); err != nil {
		t.Fatalf("Create pdf1: %v", err)
	}

	if _, err := Create(CreateOptions{
		OutputPath: pdf2,
		Title:      "Второй файл",
		Blocks: []Block{
			{Type: "heading", Level: 1, Text: "Вторая часть отчёта"},
			{Type: "page_break"},
			{Type: "heading", Level: 1, Text: "Третья страница документа"},
		},
	}); err != nil {
		t.Fatalf("Create pdf2: %v", err)
	}

	mergedPath := filepath.Join(dir, "merged.pdf")
	merged, err := Merge([]string{pdf1, pdf2}, mergedPath)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}
	if merged.PageCount != 3 {
		t.Fatalf("Merge PageCount = %d, want 3", merged.PageCount)
	}

	inspectedMerged, err := Inspect(mergedPath, "")
	if err != nil {
		t.Fatalf("Inspect merged: %v", err)
	}
	if inspectedMerged.PageCount != 3 {
		t.Fatalf("Inspect merged PageCount = %d, want 3", inspectedMerged.PageCount)
	}
	if !strings.Contains(inspectedMerged.Pages[0].Text, "Страница первая") {
		t.Errorf("page 1 missing expected text: %q", inspectedMerged.Pages[0].Text)
	}
	if !strings.Contains(inspectedMerged.Pages[2].Text, "Третья страница") {
		t.Errorf("page 3 missing expected text: %q", inspectedMerged.Pages[2].Text)
	}

	trimmedPath := filepath.Join(dir, "page2.pdf")
	splitRangeRes, err := Split(mergedPath, "2", 0, trimmedPath, "")
	if err != nil {
		t.Fatalf("Split pageRange failed: %v", err)
	}
	if len(splitRangeRes.Files) != 1 || splitRangeRes.Files[0].PageCount != 1 {
		t.Fatalf("unexpected Split pageRange result: %+v", splitRangeRes)
	}
	inspectedTrimmed, err := Inspect(trimmedPath, "")
	if err != nil {
		t.Fatalf("Inspect trimmed: %v", err)
	}
	if !strings.Contains(inspectedTrimmed.Pages[0].Text, "Вторая часть отчёта") {
		t.Errorf("trimmed page missing expected text: %q", inspectedTrimmed.Pages[0].Text)
	}

	splitDir := filepath.Join(dir, "split_pages")
	splitSpanRes, err := Split(mergedPath, "", 1, "", splitDir)
	if err != nil {
		t.Fatalf("Split span failed: %v", err)
	}
	if len(splitSpanRes.Files) != 3 {
		t.Fatalf("Split span produced %d files, want 3", len(splitSpanRes.Files))
	}
	for _, f := range splitSpanRes.Files {
		if f.PageCount != 1 || f.SizeBytes <= 0 {
			t.Errorf("unexpected split file metadata: %+v", f)
		}
	}

	rotatedPath := filepath.Join(dir, "rotated.pdf")
	rotRes, err := Rotate(mergedPath, "1", 90, rotatedPath)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}
	if rotRes.PageCount != 3 {
		t.Fatalf("Rotate PageCount = %d, want 3", rotRes.PageCount)
	}
	inspectedRotated, err := Inspect(rotatedPath, "")
	if err != nil {
		t.Fatalf("Inspect rotated: %v", err)
	}
	if inspectedRotated.Pages[0].Rotate != 90 {
		t.Fatalf("page 1 Rotate = %d, want 90", inspectedRotated.Pages[0].Rotate)
	}

	if _, err := Rotate(rotatedPath, "1", 90, rotatedPath); err != nil {
		t.Fatalf("in-place Rotate failed: %v", err)
	}
	inspectedInPlace, err := Inspect(rotatedPath, "")
	if err != nil {
		t.Fatalf("Inspect after in-place Rotate failed: %v", err)
	}
	if inspectedInPlace.Pages[0].Rotate != 180 {
		t.Fatalf("page 1 Rotate after second 90 = %d, want 180", inspectedInPlace.Pages[0].Rotate)
	}

	inPlaceMergeRes, err := Merge([]string{pdf1, pdf1}, pdf1)
	if err != nil {
		t.Fatalf("in-place Merge failed: %v", err)
	}
	if inPlaceMergeRes.PageCount != 2 {
		t.Fatalf("in-place Merge PageCount = %d, want 2", inPlaceMergeRes.PageCount)
	}
}

func TestCreateJPEGAndWorkingDir(t *testing.T) {
	dir := t.TempDir()
	jpgRel := filepath.Join("assets", "photo.jpg")
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	writeTestJPEG(t, filepath.Join(dir, jpgRel))

	res, err := Create(CreateOptions{
		WorkingDir: dir,
		OutputPath: filepath.Join("out", "with_jpeg.pdf"),
		Title:      "Документ с JPEG",
		Blocks: []Block{
			{Type: "image", ImagePath: jpgRel, Height: 60, Align: "center", Caption: "Фотография объекта"},
			{Type: "callout", Tone: "warning", Title: "Предупреждение", Text: "Проверьте исходные данные."},
		},
	})
	if err != nil {
		t.Fatalf("Create with JPEG failed: %v", err)
	}
	info, err := Inspect(res.Path, "")
	if err != nil {
		t.Fatalf("Inspect JPEG pdf failed: %v", err)
	}
	if !strings.Contains(info.Pages[0].Text, "Фотография объекта") {
		t.Fatalf("missing caption in text: %q", info.Pages[0].Text)
	}
}

func buildMinimalStandardPDF(contentStream string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 6)

	writeObj := func(id int, body string) {
		offsets[id] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, body)
	}

	writeObj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObj(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObj(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>")
	writeObj(4, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	writeObj(5, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contentStream), contentStream))

	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 6\n0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)
	return buf.Bytes()
}

func TestInspectStandardPDFAndCMap(t *testing.T) {
	dir := t.TempDir()
	stdPath := filepath.Join(dir, "standard.pdf")
	stream := "BT /F1 12 Tf 72 720 Td (Hello \\(PDF\\)) Tj T* [(Alpha) -250 (Beta)] TJ (Next Line) ' ET"
	if err := os.WriteFile(stdPath, buildMinimalStandardPDF(stream), 0o644); err != nil {
		t.Fatalf("write standard pdf: %v", err)
	}

	info, err := Inspect(stdPath, "")
	if err != nil {
		t.Fatalf("Inspect standard pdf failed: %v", err)
	}
	if info.PageCount != 1 || len(info.Pages) != 1 {
		t.Fatalf("unexpected pages: %+v", info)
	}
	txt := info.Pages[0].Text
	for _, want := range []string{"Hello (PDF)", "Alpha Beta", "Next Line"} {
		if !strings.Contains(txt, want) {
			t.Errorf("standard pdf text missing %q; got %q", want, txt)
		}
	}

	dec := &fontDecoder{
		codeLen:    1,
		toUnicode1: make(map[byte]string),
		toUnicode2: make(map[uint16]string),
	}
	cmap := `
1 begincodespacerange
<0001> <00FF>
endcodespacerange
2 beginbfchar
<0001> <041F>
<0002> <0440>
endbfchar
1 beginbfrange
<0003> <0004> [<0438> <0432>]
endbfrange
1 beginbfrange
<0005> <0006> <0435>
endbfrange
`
	parseToUnicodeCMap([]byte(cmap), dec)
	got := dec.decode([]byte{0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04, 0x00, 0x05})
	if got != "Приве" {
		t.Fatalf("CMap decode = %q, want %q", got, "Приве")
	}
}

func TestCMapTwoByteUnmappedNoDesync(t *testing.T) {
	dec := &fontDecoder{codeLen: 1}
	cmap := `
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
5 beginbfchar
<0001> <041F>
<0002> <0440>
<0003> <0438>
<0004> <0432>
<0005> <0435>
endbfchar
`
	parseToUnicodeCMap([]byte(cmap), dec)

	got := dec.decode([]byte{
		0x01, 0x99,
		0x00, 0x01,
		0x00, 0x02,
		0x00, 0x03,
		0x00, 0x04,
		0x00, 0x05,
	})
	if got != "Приве" {
		t.Fatalf("2-byte CMap decode with unmapped glyph = %q, want %q", got, "Приве")
	}

	oneByteDec := &fontDecoder{codeLen: 1}
	oneByteCMap := `
1 begincodespacerange
<00> <FF>
endcodespacerange
2 beginbfchar
<01> <041F>
<02> <0440>
endbfchar
`
	parseToUnicodeCMap([]byte(oneByteCMap), oneByteDec)
	got1 := oneByteDec.decode([]byte{0x01, 0x02, '!'})
	if got1 != "Пр!" {
		t.Fatalf("1-byte CMap decode = %q, want %q", got1, "Пр!")
	}
}

func TestCMapMalformedBfrangeInverted(t *testing.T) {
	dec := &fontDecoder{codeLen: 1}
	cmap := `
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
2 beginbfrange
<0010> <0005> <0435>
<0020> <0015> [<0410> <0411>]
endbfrange
1 beginbfchar
<0001> <041F>
endbfchar
`
	parseToUnicodeCMap([]byte(cmap), dec)

	if len(dec.toUnicode1) != 0 {
		t.Fatalf("unexpected 1-byte entries: %v", dec.toUnicode1)
	}
	if len(dec.toUnicode2) != 1 {
		t.Fatalf("len(toUnicode2) = %d, want 1; map = %v", len(dec.toUnicode2), dec.toUnicode2)
	}
	if got := dec.decode([]byte{0x00, 0x01}); got != "П" {
		t.Fatalf("decode after malformed bfrange = %q, want %q", got, "П")
	}
}

func TestErrors(t *testing.T) {
	dir := t.TempDir()
	validPDF := filepath.Join(dir, "valid.pdf")
	if _, err := Create(CreateOptions{
		OutputPath: validPDF,
		Blocks:     []Block{{Type: "paragraph", Text: "Тест"}},
	}); err != nil {
		t.Fatalf("setup valid PDF: %v", err)
	}

	if _, err := Create(CreateOptions{}); err == nil {
		t.Error("expected error for empty OutputPath")
	}
	if _, err := Create(CreateOptions{
		OutputPath: filepath.Join(dir, "missing_img.pdf"),
		Blocks:     []Block{{Type: "image", ImagePath: filepath.Join(dir, "nope.png")}},
	}); err == nil {
		t.Error("expected error for missing image path")
	}
	if _, err := Inspect("", ""); err == nil {
		t.Error("expected error for empty Inspect path")
	}
	if _, err := Inspect(filepath.Join(dir, "missing.pdf"), ""); err == nil {
		t.Error("expected error for missing Inspect file")
	}
	if _, err := Merge(nil, filepath.Join(dir, "out.pdf")); err == nil {
		t.Error("expected error for empty Merge inputs")
	}
	if _, err := Merge([]string{filepath.Join(dir, "missing.pdf")}, filepath.Join(dir, "out.pdf")); err == nil {
		t.Error("expected error for missing Merge input")
	}
	if _, err := Merge([]string{validPDF}, ""); err == nil {
		t.Error("expected error for empty Merge outputPath")
	}
	if _, err := Split(filepath.Join(dir, "missing.pdf"), "1", 0, filepath.Join(dir, "out.pdf"), ""); err == nil {
		t.Error("expected error for missing Split input")
	}
	if _, err := Split(validPDF, "", 0, "", dir); err == nil {
		t.Error("expected error for invalid span=0 without pageRange")
	}
	if _, err := Split(validPDF, "", -2, "", dir); err == nil {
		t.Error("expected error for negative span")
	}
	if _, err := Split(validPDF, "1", 0, "", ""); err == nil {
		t.Error("expected error for empty Split outputPath and outputDir")
	}
	if _, err := Rotate(filepath.Join(dir, "missing.pdf"), "", 90, filepath.Join(dir, "rot.pdf")); err == nil {
		t.Error("expected error for missing Rotate input")
	}
	if _, err := Rotate(validPDF, "", 45, filepath.Join(dir, "rot.pdf")); err == nil {
		t.Error("expected error for invalid Rotate angle=45")
	}
	if _, err := Rotate(validPDF, "", 90, ""); err == nil {
		t.Error("expected error for empty Rotate outputPath")
	}
}
