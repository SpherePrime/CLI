package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 30, G: 90, B: 180, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write png: %v", err)
	}
}

func writeTestJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 200, G: 80, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write jpeg: %v", err)
	}
}

func readZipEntries(t *testing.T, path string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip %s: %v", path, err)
	}
	defer zr.Close()
	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", f.Name, err)
		}
		out[f.Name] = data
	}
	return out
}

func writeZipEntries(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip %s: %v", path, err)
	}
	zw := zip.NewWriter(f)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create entry %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
}

func assertValidXML(t *testing.T, name string, data []byte) {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("invalid xml in %s: %v", name, err)
		}
	}
}

func TestCreateAndInspectCyrillicRichDocument(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "chart.png")
	writeTestPNG(t, imgPath, 160, 90)
	outPath := filepath.Join(dir, "report.docx")

	res, err := Create(CreateOptions{
		OutputPath:  outPath,
		WorkingDir:  dir,
		Title:       "Годовой отчёт 2026",
		Subtitle:    "Итоги операционной деятельности",
		Author:      "Иван Петров",
		Header:      "Конфиденциально — Отчёт 2026",
		Footer:      "ООО «СфераПрайм»",
		PageNumbers: true,
		Blocks: []Block{
			{Type: "heading", Level: 1, Text: "Раздел 1. Обзор"},
			{Type: "heading", Level: 2, Text: "Подраздел 1.1. Ключевые показатели"},
			{Type: "heading", Level: 3, Text: "Детализация по кварталам"},
			{
				Type:  "paragraph",
				Align: "justify",
				Runs: []Run{
					{Text: "Выручка выросла на ", FontSize: 11},
					{Text: "34%", Bold: true, Color: "#1D4ED8", FontSize: 12},
					{Text: " по сравнению с прошлым периодом (", Italic: true},
					{Text: "подтверждено аудитом", Underline: true},
					{Text: ")."},
				},
			},
			{
				Type:  "bullet_list",
				Items: []string{"Запуск нового модуля", "Рост клиентской базы"},
			},
			{
				Type:  "numbered_list",
				Items: []string{"Этап проектирования", "Этап внедрения"},
			},
			{
				Type:       "table",
				Headers:    []string{"Регион", "Выручка", "Статус"},
				Rows:       [][]string{{"Москва", "12,4 млн ₽", "Выполнено"}, {"Казань", "4,8 млн ₽", "В работе"}},
				ColWidths:  []float64{2, 1.5, 1.5},
				HeaderFill: "#1E3A5F",
			},
			{
				Type:  "callout",
				Title: "Важное примечание",
				Tone:  "info",
				Text:  "Все расчёты выполнены по стандарту МСФО.",
			},
			{
				Type:      "image",
				ImagePath: imgPath,
				Width:     240,
				Align:     "center",
				Caption:   "Рисунок 1 — Динамика роста",
			},
			{Type: "divider"},
			{Type: "page_break"},
			{Type: "paragraph", Align: "center", Text: "Заключительный раздел документа."},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if res.Path != outPath {
		t.Fatalf("expected path %s, got %s", outPath, res.Path)
	}
	if res.SizeBytes <= 0 {
		t.Fatalf("expected positive size, got %d", res.SizeBytes)
	}

	entries := readZipEntries(t, outPath)
	requiredParts := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"docProps/core.xml",
		"word/document.xml",
		"word/_rels/document.xml.rels",
		"word/styles.xml",
		"word/numbering.xml",
		"word/header1.xml",
		"word/footer1.xml",
		"word/media/image1.png",
	}
	for _, part := range requiredParts {
		data, ok := entries[part]
		if !ok || len(data) == 0 {
			t.Fatalf("missing required docx part %s", part)
		}
		if strings.HasSuffix(part, ".xml") || strings.HasSuffix(part, ".rels") {
			assertValidXML(t, part, data)
		}
	}

	info, err := Inspect(outPath)
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	if info.Metadata.Title != "Годовой отчёт 2026" {
		t.Fatalf("expected title %q, got %q", "Годовой отчёт 2026", info.Metadata.Title)
	}
	if info.Metadata.Author != "Иван Петров" {
		t.Fatalf("expected author %q, got %q", "Иван Петров", info.Metadata.Author)
	}

	expectedHeadings := []Heading{
		{Level: 1, Text: "Раздел 1. Обзор"},
		{Level: 2, Text: "Подраздел 1.1. Ключевые показатели"},
		{Level: 3, Text: "Детализация по кварталам"},
	}
	if len(info.Headings) != len(expectedHeadings) {
		t.Fatalf("expected %d headings, got %d: %+v", len(expectedHeadings), len(info.Headings), info.Headings)
	}
	for i, want := range expectedHeadings {
		if info.Headings[i] != want {
			t.Fatalf("heading[%d] = %+v, want %+v", i, info.Headings[i], want)
		}
	}

	allPara := strings.Join(info.Paragraphs, "\n")
	for _, snippet := range []string{
		"Годовой отчёт 2026",
		"Итоги операционной деятельности",
		"Выручка выросла на 34% по сравнению с прошлым периодом (подтверждено аудитом).",
		"Запуск нового модуля",
		"Этап внедрения",
		"Рисунок 1 — Динамика роста",
		"Заключительный раздел документа.",
	} {
		if !strings.Contains(allPara, snippet) {
			t.Fatalf("expected paragraph text %q not found in:\n%s", snippet, allPara)
		}
	}

	if len(info.Tables) < 1 {
		t.Fatalf("expected at least 1 table, got 0")
	}
	dataTable := info.Tables[0]
	if len(dataTable.Rows) != 3 {
		t.Fatalf("expected 3 rows in data table, got %d: %+v", len(dataTable.Rows), dataTable.Rows)
	}
	if dataTable.Rows[0][0] != "Регион" || dataTable.Rows[1][0] != "Москва" || dataTable.Rows[2][2] != "В работе" {
		t.Fatalf("unexpected table rows: %+v", dataTable.Rows)
	}

	if len(info.Headers) == 0 || !strings.Contains(strings.Join(info.Headers, " "), "Конфиденциально — Отчёт 2026") {
		t.Fatalf("expected header text, got %+v", info.Headers)
	}
	if len(info.Footers) == 0 || !strings.Contains(strings.Join(info.Footers, " "), "ООО «СфераПрайм»") {
		t.Fatalf("expected footer text, got %+v", info.Footers)
	}
}

func TestFillTemplateSplitRunsAndPreservesCustomParts(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.docx")
	filledPath := filepath.Join(dir, "filled.docx")

	_, err := Create(CreateOptions{
		OutputPath: templatePath,
		Header:     "Дата документа: {{doc_date}}",
		Blocks: []Block{
			{Type: "paragraph", Text: "PLACEHOLDER_SPLIT_MARKER"},
			{
				Type:    "table",
				Headers: []string{"Показатель", "Значение"},
				Rows:    [][]string{{"Сумма к оплате", "{{amount}}"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Create template failed: %v", err)
	}

	entries := readZipEntries(t, templatePath)
	docXML := string(entries["word/document.xml"])
	splitRunsXML := `<w:r><w:rPr><w:b/><w:color w:val="1D4ED8"/></w:rPr><w:t xml:space="preserve">Клиент: {{client_</w:t></w:r><w:proofErr w:type="spellStart"/><w:r><w:rPr><w:i/></w:rPr><w:t>name}} — договор</w:t></w:r>`
	oldRun := `<w:r><w:t xml:space="preserve">PLACEHOLDER_SPLIT_MARKER</w:t></w:r>`
	if !strings.Contains(docXML, oldRun) {
		oldRun = `<w:r><w:t>PLACEHOLDER_SPLIT_MARKER</w:t></w:r>`
	}
	if !strings.Contains(docXML, oldRun) {
		t.Fatalf("marker run not found in generated document.xml:\n%s", docXML)
	}
	entries["word/document.xml"] = []byte(strings.Replace(docXML, oldRun, splitRunsXML, 1))

	customTheme := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="CustomTheme"><a:themeElements/></a:theme>`)
	entries["word/theme/theme1.xml"] = customTheme
	writeZipEntries(t, templatePath, entries)

	before, err := Inspect(templatePath)
	if err != nil {
		t.Fatalf("Inspect template failed: %v", err)
	}
	for _, key := range []string{"client_name", "doc_date", "amount"} {
		if !slices.Contains(before.Placeholders, key) {
			t.Fatalf("expected placeholder %q in %v", key, before.Placeholders)
		}
	}

	res, err := FillTemplate(templatePath, filledPath, map[string]string{
		"client_name": "ООО «Северный Вектор»",
		"doc_date":    "26 ноября 2026",
		"amount":      "1 250 000 ₽",
	})
	if err != nil {
		t.Fatalf("FillTemplate failed: %v", err)
	}
	if res.ReplacementsApplied < 3 {
		t.Fatalf("expected at least 3 replacements applied, got %d", res.ReplacementsApplied)
	}

	after, err := Inspect(filledPath)
	if err != nil {
		t.Fatalf("Inspect filled failed: %v", err)
	}
	if len(after.Placeholders) != 0 {
		t.Fatalf("expected no remaining placeholders, got %v", after.Placeholders)
	}
	if !strings.Contains(strings.Join(after.Paragraphs, "\n"), "Клиент: ООО «Северный Вектор» — договор") {
		t.Fatalf("split placeholder not replaced cleanly in body: %+v", after.Paragraphs)
	}
	if !strings.Contains(strings.Join(after.Headers, "\n"), "Дата документа: 26 ноября 2026") {
		t.Fatalf("header placeholder not replaced: %+v", after.Headers)
	}
	if len(after.Tables) != 1 || after.Tables[0].Rows[1][1] != "1 250 000 ₽" {
		t.Fatalf("table placeholder not replaced: %+v", after.Tables)
	}

	outEntries := readZipEntries(t, filledPath)
	if !bytes.Equal(outEntries["word/theme/theme1.xml"], customTheme) {
		t.Fatalf("custom ZIP entry word/theme/theme1.xml was not preserved verbatim")
	}
	outDocXML := string(outEntries["word/document.xml"])
	assertValidXML(t, "word/document.xml", outEntries["word/document.xml"])
	if !strings.Contains(outDocXML, `<w:rPr><w:b/><w:color w:val="1D4ED8"/></w:rPr><w:t xml:space="preserve">Клиент: ООО «Северный Вектор»`) {
		t.Fatalf("opening run rPr formatting was not preserved on replaced split run:\n%s", outDocXML)
	}
}

func TestUpdateAppendReplaceTableCellAndInPlace(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "contract.docx")

	_, err := Create(CreateOptions{
		OutputPath: docPath,
		Title:      "Договор поставки",
		Blocks: []Block{
			{Type: "heading", Level: 1, Text: "Основные условия"},
			{Type: "paragraph", Text: "Статус документа: Черновик"},
			{
				Type:    "table",
				Headers: []string{"Этап", "Статус"},
				Rows:    [][]string{{"Поставка", "Ожидается"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	res, err := Update(UpdateOptions{
		InputPath:  docPath,
		OutputPath: docPath,
		WorkingDir: dir,
		Replacements: map[string]string{
			"Черновик": "Утверждено",
		},
		TableUpdates: []TableUpdate{
			{TableIndex: 0, Row: 1, Col: 1, Value: "Выполнено"},
		},
		AppendBlocks: []Block{
			{Type: "heading", Level: 2, Text: "Приложение А"},
			{Type: "paragraph", Text: "Дополнительная спецификация оборудования."},
			{
				Type:    "table",
				Headers: []string{"Позиция", "Кол-во"},
				Rows:    [][]string{{"Сервер", "4"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if res.Path != docPath || res.SizeBytes <= 0 {
		t.Fatalf("unexpected Update result: %+v", res)
	}

	info, err := Inspect(docPath)
	if err != nil {
		t.Fatalf("Inspect updated failed: %v", err)
	}
	if len(info.Headings) != 2 {
		t.Fatalf("expected 2 headings, got %+v", info.Headings)
	}
	if info.Headings[0].Text != "Основные условия" || info.Headings[1].Text != "Приложение А" {
		t.Fatalf("unexpected headings: %+v", info.Headings)
	}
	allPara := strings.Join(info.Paragraphs, "\n")
	if !strings.Contains(allPara, "Статус документа: Утверждено") {
		t.Fatalf("replacement not applied: %s", allPara)
	}
	if !strings.Contains(allPara, "Дополнительная спецификация оборудования.") {
		t.Fatalf("appended paragraph missing: %s", allPara)
	}
	if len(info.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(info.Tables))
	}
	if info.Tables[0].Rows[1][1] != "Выполнено" {
		t.Fatalf("table cell update not applied: %+v", info.Tables[0])
	}
	if info.Tables[1].Rows[1][0] != "Сервер" || info.Tables[1].Rows[1][1] != "4" {
		t.Fatalf("appended table unexpected: %+v", info.Tables[1])
	}
}

func TestInspectComments(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "commented.docx")
	_, err := Create(CreateOptions{
		OutputPath: docPath,
		Blocks:     []Block{{Type: "paragraph", Text: "Текст для проверки {{reviewer}}."}},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	entries := readZipEntries(t, docPath)
	entries["word/comments.xml"] = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:comments xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:comment w:id="0" w:author="Анна Смирнова" w:date="2026-11-26T10:00:00Z">
    <w:p><w:r><w:t>Уточнить реквизиты</w:t></w:r></w:p>
  </w:comment>
</w:comments>`)
	writeZipEntries(t, docPath, entries)

	info, err := Inspect(docPath)
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	if len(info.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %+v", info.Comments)
	}
	if info.Comments[0].Author != "Анна Смирнова" || info.Comments[0].Text != "Уточнить реквизиты" {
		t.Fatalf("unexpected comment: %+v", info.Comments[0])
	}
}

func TestErrorHandling(t *testing.T) {
	dir := t.TempDir()

	if _, err := Create(CreateOptions{}); err == nil {
		t.Fatal("expected error for empty Create OutputPath")
	}
	if _, err := Create(CreateOptions{
		OutputPath: filepath.Join(dir, "bad_img.docx"),
		Blocks:     []Block{{Type: "image", ImagePath: filepath.Join(dir, "missing.png")}},
	}); err == nil {
		t.Fatal("expected error for missing image")
	}

	if _, err := Inspect(""); err == nil {
		t.Fatal("expected error for empty Inspect path")
	}
	if _, err := Inspect(filepath.Join(dir, "nope.docx")); err == nil {
		t.Fatal("expected error for missing Inspect file")
	}

	corruptPath := filepath.Join(dir, "corrupt.docx")
	if err := os.WriteFile(corruptPath, []byte("not a zip archive"), 0o644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if _, err := Inspect(corruptPath); err == nil {
		t.Fatal("expected error for corrupt ZIP in Inspect")
	}

	emptyZipPath := filepath.Join(dir, "not_docx.docx")
	writeZipEntries(t, emptyZipPath, map[string][]byte{"readme.txt": []byte("hello")})
	if _, err := Inspect(emptyZipPath); err == nil {
		t.Fatal("expected error for ZIP missing word/document.xml")
	}

	if _, err := Update(UpdateOptions{}); err == nil {
		t.Fatal("expected error for empty Update InputPath")
	}
	if _, err := Update(UpdateOptions{InputPath: filepath.Join(dir, "nope.docx")}); err == nil {
		t.Fatal("expected error for missing Update file")
	}
	if _, err := Update(UpdateOptions{InputPath: corruptPath, OutputPath: filepath.Join(dir, "out.docx")}); err == nil {
		t.Fatal("expected error for corrupt ZIP in Update")
	}

	validPath := filepath.Join(dir, "valid.docx")
	if _, err := Create(CreateOptions{
		OutputPath: validPath,
		Blocks:     []Block{{Type: "paragraph", Text: "ok"}},
	}); err != nil {
		t.Fatalf("Create valid failed: %v", err)
	}
	if _, err := Update(UpdateOptions{
		InputPath:    validPath,
		OutputPath:   filepath.Join(dir, "out2.docx"),
		TableUpdates: []TableUpdate{{TableIndex: 0, Row: 0, Col: 0, Value: "x"}},
	}); err == nil {
		t.Fatal("expected error for out-of-range TableUpdate")
	}

	if _, err := FillTemplate("", filepath.Join(dir, "out.docx"), nil); err == nil {
		t.Fatal("expected error for empty FillTemplate inputPath")
	}
	if _, err := FillTemplate(validPath, "", nil); err == nil {
		t.Fatal("expected error for empty FillTemplate outputPath")
	}
	if _, err := FillTemplate(filepath.Join(dir, "nope.docx"), filepath.Join(dir, "out.docx"), map[string]string{"a": "b"}); err == nil {
		t.Fatal("expected error for missing FillTemplate input file")
	}
}

func TestUpdateAppendJPEGAndLists(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "first.png")
	jpgPath := filepath.Join(dir, "second.jpg")
	writeTestPNG(t, pngPath, 80, 60)
	writeTestJPEG(t, jpgPath, 120, 80)

	docPath := filepath.Join(dir, "media_lists.docx")
	_, err := Create(CreateOptions{
		OutputPath: docPath,
		WorkingDir: dir,
		Footer:     "Ответственный: {{owner}}",
		Blocks: []Block{
			{Type: "image", ImagePath: pngPath, Width: 100},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = Update(UpdateOptions{
		InputPath:  docPath,
		OutputPath: docPath,
		WorkingDir: dir,
		Replacements: map[string]string{
			"{{owner}}": "Елена Волкова",
		},
		AppendBlocks: []Block{
			{Type: "bullet_list", Items: []string{"Пункт А", "Пункт Б"}},
			{Type: "numbered_list", Items: []string{"Шаг 1", "Шаг 2"}},
			{Type: "image", ImagePath: jpgPath, Height: 90, Caption: "Фото 2"},
		},
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	entries := readZipEntries(t, docPath)
	for _, part := range []string{
		"[Content_Types].xml",
		"word/document.xml",
		"word/_rels/document.xml.rels",
		"word/numbering.xml",
		"word/footer1.xml",
		"word/media/image1.png",
		"word/media/image2.jpeg",
	} {
		data, ok := entries[part]
		if !ok || len(data) == 0 {
			t.Fatalf("expected part %s in updated docx", part)
		}
		if strings.HasSuffix(part, ".xml") || strings.HasSuffix(part, ".rels") {
			assertValidXML(t, part, data)
		}
	}

	info, err := Inspect(docPath)
	if err != nil {
		t.Fatalf("Inspect updated failed: %v", err)
	}
	if !strings.Contains(strings.Join(info.Footers, " "), "Ответственный: Елена Волкова") {
		t.Fatalf("footer replacement not applied: %+v", info.Footers)
	}
	allPara := strings.Join(info.Paragraphs, "\n")
	for _, snippet := range []string{"Пункт А", "Шаг 2", "Фото 2"} {
		if !strings.Contains(allPara, snippet) {
			t.Fatalf("expected appended text %q in:\n%s", snippet, allPara)
		}
	}
}
