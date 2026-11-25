package pptx

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
	"strconv"
	"strings"
	"testing"
)

func writeTestPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 27, G: 54, B: 93, A: 255})
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

func writeTestJPEG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 43, G: 108, B: 176, A: 255})
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

func readZipMap(t *testing.T, path string) map[string][]byte {
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

func assertValidXMLParts(t *testing.T, path string) {
	t.Helper()
	parts := readZipMap(t, path)
	for name, data := range parts {
		if !strings.HasSuffix(name, ".xml") && !strings.HasSuffix(name, ".rels") {
			continue
		}
		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("invalid xml in %s: %v", name, err)
			}
		}
	}
}

func TestCreateAndInspectAllLayoutsCyrillic(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "chart.png")
	writeTestPNG(t, pngPath, 320, 200)
	jpgPath := filepath.Join(dir, "photo.jpg")
	writeTestJPEG(t, jpgPath, 240, 180)

	outPath := filepath.Join(dir, "deck.pptx")
	res, err := Create(CreateOptions{
		OutputPath:      outPath,
		WorkingDir:      dir,
		Title:           "Годовой отчёт 2026",
		Author:          "Аналитический отдел",
		Subject:         "Финансовые итоги",
		AspectRatio:     "16:9",
		PrimaryColor:    "#1B365D",
		AccentColor:     "#2B6CB0",
		BackgroundColor: "#FFFFFF",
		TextColor:       "#1A202C",
		SlideNumbers:    true,
		Slides: []SlideSpec{
			{
				Layout:   "title",
				Title:    "Итоги года 2026",
				Subtitle: "Рост выручки и ключевые инициативы",
				Notes:    "Приветствовать участников и обозначить цели встречи.",
			},
			{
				Layout: "title_content",
				Title:  "Ключевые достижения",
				Bullets: []string{
					"Запуск новой платформы в России",
					"Увеличение клиентской базы на 35%",
					"Сокращение времени отклика вдвое",
				},
				Notes: "Подчеркнуть устойчивый рост по всем сегментам.",
			},
			{
				Layout: "two_column",
				Title:  "Сравнение направлений",
				Left: &ColumnContent{
					Heading: "Корпоративный сегмент",
					Body:    "Высокая маржинальность и долгие контракты.",
					Bullets: []string{"Выручка +42%", "Retention 96%"},
				},
				Right: &ColumnContent{
					Heading: "Малый бизнес",
					Body:    "Быстрое подключение через облако.",
					Bullets: []string{"Новых клиентов 12 400", "Средний чек +18%"},
				},
				Notes: "Ответить на вопросы по структуре продаж.",
			},
			{
				Layout: "table",
				Title:  "Финансовые показатели по кварталам",
				Table: &TableSpec{
					Headers: []string{"Квартал", "Выручка", "Прибыль"},
					Rows: [][]string{
						{"Q1", "120 млн ₽", "24 млн ₽"},
						{"Q2", "145 млн ₽", "31 млн ₽"},
						{"Q3", "168 млн ₽", "39 млн ₽"},
					},
				},
				Notes: "Данные приведены по МСФО.",
			},
			{
				Layout: "metrics",
				Title:  "Ключевые KPI",
				Metrics: []MetricCard{
					{Label: "Выручка", Value: "620 млн ₽", Delta: "+34% YoY"},
					{Label: "EBITDA", Value: "28%", Delta: "+4 п.п."},
					{Label: "NPS", Value: "74", Delta: "+9 пунктов"},
				},
				Notes: "Все целевые показатели перевыполнены.",
			},
			{
				Layout:       "image_text",
				Title:        "Динамика роста",
				Body:         "График подтверждает ускорение темпов во втором полугодии.",
				Bullets:      []string{"Пик продаж в ноябре", "Рост повторных заказов"},
				ImagePath:    "chart.png",
				ImageCaption: "Рисунок 1. Квартальная динамика",
				Notes:        "Показать график на большом экране.",
			},
			{
				Layout:    "image_text",
				Title:     "Команда проекта",
				Body:      "Распределённая команда разработки.",
				ImagePath: jpgPath,
			},
			{
				Layout: "blank",
				Title:  "Свободный слайд",
			},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if res.SlideCount != 8 {
		t.Fatalf("expected 8 slides, got %d", res.SlideCount)
	}
	if res.SizeBytes <= 0 {
		t.Fatalf("expected positive file size, got %d", res.SizeBytes)
	}

	assertValidXMLParts(t, outPath)

	parts := readZipMap(t, outPath)
	for _, required := range []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"docProps/core.xml",
		"ppt/presentation.xml",
		"ppt/_rels/presentation.xml.rels",
		"ppt/slideMasters/slideMaster1.xml",
		"ppt/slideLayouts/slideLayout1.xml",
		"ppt/theme/theme1.xml",
		"ppt/slides/slide1.xml",
		"ppt/notesSlides/notesSlide1.xml",
	} {
		if _, ok := parts[required]; !ok {
			t.Fatalf("missing required zip part %s", required)
		}
	}

	info, err := Inspect(outPath)
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	if info.SlideCount != 8 {
		t.Fatalf("Inspect SlideCount = %d, want 8", info.SlideCount)
	}
	if info.Dimensions.AspectRatio != "16:9" {
		t.Fatalf("AspectRatio = %q, want 16:9", info.Dimensions.AspectRatio)
	}
	if info.Dimensions.WidthEMU != slide16x9WidthEMU || info.Dimensions.HeightEMU != slide16x9HeightEMU {
		t.Fatalf("unexpected 16:9 dimensions: %+v", info.Dimensions)
	}
	if info.Metadata.Title != "Годовой отчёт 2026" || info.Metadata.Author != "Аналитический отдел" {
		t.Fatalf("unexpected metadata: %+v", info.Metadata)
	}

	slide1Joined := strings.Join(info.Slides[0].Texts, "\n")
	if !strings.Contains(slide1Joined, "Итоги года 2026") || !strings.Contains(slide1Joined, "Рост выручки и ключевые инициативы") {
		t.Fatalf("slide 1 missing cyrillic text: %v", info.Slides[0].Texts)
	}
	if !strings.Contains(info.Slides[0].Notes, "Приветствовать участников") {
		t.Fatalf("slide 1 notes mismatch: %q", info.Slides[0].Notes)
	}

	slide2Joined := strings.Join(info.Slides[1].Texts, "\n")
	if !strings.Contains(slide2Joined, "Запуск новой платформы в России") {
		t.Fatalf("slide 2 missing bullet: %v", info.Slides[1].Texts)
	}

	slide3Joined := strings.Join(info.Slides[2].Texts, "\n")
	if !strings.Contains(slide3Joined, "Корпоративный сегмент") || !strings.Contains(slide3Joined, "Малый бизнес") {
		t.Fatalf("slide 3 missing two-column text: %v", info.Slides[2].Texts)
	}

	if len(info.Slides[3].Tables) != 1 {
		t.Fatalf("slide 4 expected 1 table, got %d", len(info.Slides[3].Tables))
	}
	rows := info.Slides[3].Tables[0].Rows
	if len(rows) != 4 || rows[0][0] != "Квартал" || rows[1][1] != "120 млн ₽" || rows[3][2] != "39 млн ₽" {
		t.Fatalf("unexpected table rows: %#v", rows)
	}

	slide5Joined := strings.Join(info.Slides[4].Texts, "\n")
	if !strings.Contains(slide5Joined, "620 млн ₽") || !strings.Contains(slide5Joined, "+34% YoY") {
		t.Fatalf("slide 5 missing KPI metrics: %v", info.Slides[4].Texts)
	}

	slide6Joined := strings.Join(info.Slides[5].Texts, "\n")
	if !strings.Contains(slide6Joined, "Динамика роста") || !strings.Contains(slide6Joined, "Рисунок 1. Квартальная динамика") {
		t.Fatalf("slide 6 missing image_text content: %v", info.Slides[5].Texts)
	}
	if !strings.Contains(info.Slides[5].Notes, "Показать график") {
		t.Fatalf("slide 6 notes mismatch: %q", info.Slides[5].Notes)
	}

	out4x3 := filepath.Join(dir, "deck4x3.pptx")
	if _, err := Create(CreateOptions{
		OutputPath:  out4x3,
		AspectRatio: "4:3",
		Slides: []SlideSpec{
			{Layout: "title", Title: "Стандарт 4:3"},
		},
	}); err != nil {
		t.Fatalf("Create 4:3 failed: %v", err)
	}
	info4x3, err := Inspect(out4x3)
	if err != nil {
		t.Fatalf("Inspect 4:3 failed: %v", err)
	}
	if info4x3.Dimensions.AspectRatio != "4:3" || info4x3.Dimensions.WidthEMU != slide4x3WidthEMU {
		t.Fatalf("unexpected 4:3 dimensions: %+v", info4x3.Dimensions)
	}
}

func TestFillTemplateSplitRunsAndPreservation(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.pptx")
	if _, err := Create(CreateOptions{
		OutputPath: basePath,
		Title:      "Template",
		Slides: []SlideSpec{
			{
				Layout: "title_content",
				Title:  "Отчёт за {{quarter}}",
				Bullets: []string{
					"Команда: {{team}}",
					"Цепочка: {{first}} / {{second}}",
				},
				Notes: "Подсказка спикеру: {{speaker_hint}}",
			},
			{
				Layout: "table",
				Title:  "Таблица {{quarter}}",
				Table: &TableSpec{
					Headers: []string{"Показатель", "Значение"},
					Rows:    [][]string{{"Период", "{{quarter}}"}},
				},
			},
		},
	}); err != nil {
		t.Fatalf("Create base failed: %v", err)
	}

	origParts := readZipMap(t, basePath)
	slide1XML := string(origParts["ppt/slides/slide1.xml"])
	target := "{{quarter}}"
	if !strings.Contains(slide1XML, target) {
		t.Fatalf("expected %s in slide1.xml", target)
	}
	splitReplacement := "{{quar</a:t></a:r><a:r><a:rPr lang=\"ru-RU\" sz=\"2800\" b=\"0\"><a:solidFill><a:srgbClr val=\"FF0000\"/></a:solidFill></a:rPr><a:t>ter}}"
	slide1XML = strings.Replace(slide1XML, target, splitReplacement, 1)
	origParts["ppt/slides/slide1.xml"] = []byte(slide1XML)

	splitPath := filepath.Join(dir, "split-template.pptx")
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	for name, data := range origParts {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := os.WriteFile(splitPath, zipBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("write split template: %v", err)
	}

	preInspect, err := Inspect(splitPath)
	if err != nil {
		t.Fatalf("Inspect split template failed: %v", err)
	}
	for _, want := range []string{"quarter", "team", "speaker_hint", "first", "second"} {
		if !slices.Contains(preInspect.Placeholders, want) {
			t.Fatalf("expected placeholder %q in %v", want, preInspect.Placeholders)
		}
	}

	filledPath := filepath.Join(dir, "filled.pptx")
	res, err := FillTemplate(splitPath, filledPath, map[string]string{
		"quarter":      "Q4 2026",
		"team":         "Платформа",
		"speaker_hint": "Сделать паузу перед вопросами",
		"first":        "{{second}}",
		"second":       "готово",
	})
	if err != nil {
		t.Fatalf("FillTemplate failed: %v", err)
	}
	if res.ReplacementsApplied < 6 {
		t.Fatalf("expected at least 6 replacements applied, got %d", res.ReplacementsApplied)
	}

	assertValidXMLParts(t, filledPath)

	filledParts := readZipMap(t, filledPath)
	if !bytes.Equal(origParts["ppt/theme/theme1.xml"], filledParts["ppt/theme/theme1.xml"]) {
		t.Fatalf("ppt/theme/theme1.xml was modified unexpectedly")
	}
	if !bytes.Equal(origParts["ppt/slideMasters/slideMaster1.xml"], filledParts["ppt/slideMasters/slideMaster1.xml"]) {
		t.Fatalf("ppt/slideMasters/slideMaster1.xml was modified unexpectedly")
	}
	filledSlide1 := string(filledParts["ppt/slides/slide1.xml"])
	if !strings.Contains(filledSlide1, `b="1"`) {
		t.Fatalf("opening run rPr formatting was not preserved in slide1.xml")
	}
	if strings.Contains(filledSlide1, `xml:space="preserve"`) {
		t.Fatalf("DrawingML a:t must not emit xml:space=\"preserve\"")
	}

	postInspect, err := Inspect(filledPath)
	if err != nil {
		t.Fatalf("Inspect filled failed: %v", err)
	}
	s1Text := strings.Join(postInspect.Slides[0].Texts, "\n")
	if !strings.Contains(s1Text, "Отчёт за Q4 2026") {
		t.Fatalf("split placeholder not replaced on slide 1: %q", s1Text)
	}
	if !strings.Contains(s1Text, "Команда: Платформа") {
		t.Fatalf("team placeholder not replaced: %q", s1Text)
	}
	if !strings.Contains(s1Text, "Цепочка: {{second}} / готово") {
		t.Fatalf("single-pass replacement violated: %q", s1Text)
	}
	if !strings.Contains(postInspect.Slides[0].Notes, "Сделать паузу перед вопросами") {
		t.Fatalf("notes placeholder not replaced: %q", postInspect.Slides[0].Notes)
	}
	if postInspect.Slides[1].Tables[0].Rows[1][1] != "Q4 2026" {
		t.Fatalf("table cell placeholder not replaced: %#v", postInspect.Slides[1].Tables[0].Rows)
	}
}

func TestUpdateNotesReplaceAndAppendInPlace(t *testing.T) {
	dir := t.TempDir()
	deckPath := filepath.Join(dir, "editable.pptx")
	if _, err := Create(CreateOptions{
		OutputPath: deckPath,
		Title:      "Презентация",
		Slides: []SlideSpec{
			{
				Layout:   "title",
				Title:    "Черновик стратегии",
				Subtitle: "Версия 1",
				Notes:    "Старые заметки первого слайда.",
			},
			{
				Layout:  "title_content",
				Title:   "План работ",
				Bullets: []string{"Этап 1", "Этап 2"},
			},
		},
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	beforeParts := readZipMap(t, deckPath)

	res, err := Update(UpdateOptions{
		InputPath:  deckPath,
		OutputPath: deckPath,
		WorkingDir: dir,
		Replacements: map[string]string{
			"Черновик стратегии": "Финальная стратегия",
			"Версия 1":           "Версия 2",
		},
		NotesUpdates: []NotesUpdate{
			{Slide: 1, Notes: "Обновлённые заметки спикера.\nВторая строка заметок."},
			{Slide: 2, Notes: "Новые заметки для второго слайда."},
		},
		AppendSlides: []SlideSpec{
			{
				Layout: "metrics",
				Title:  "Целевые ориентиры",
				Metrics: []MetricCard{
					{Label: "Рост", Value: "2.5x", Delta: "к 2027 году"},
				},
				Notes: "Заметки третьего добавленного слайда.",
			},
		},
	})
	if err != nil {
		t.Fatalf("Update in-place failed: %v", err)
	}
	if res.SlideCount != 3 {
		t.Fatalf("expected 3 slides after append, got %d", res.SlideCount)
	}
	if res.ReplacementsApplied != 2 {
		t.Fatalf("expected 2 replacements applied, got %d", res.ReplacementsApplied)
	}

	assertValidXMLParts(t, deckPath)

	afterParts := readZipMap(t, deckPath)
	if !bytes.Equal(beforeParts["ppt/theme/theme1.xml"], afterParts["ppt/theme/theme1.xml"]) {
		t.Fatalf("theme1.xml changed during Update")
	}
	if !bytes.Equal(beforeParts["ppt/slideMasters/slideMaster1.xml"], afterParts["ppt/slideMasters/slideMaster1.xml"]) {
		t.Fatalf("slideMaster1.xml changed during Update")
	}
	if !bytes.Equal(beforeParts["ppt/slides/slide2.xml"], afterParts["ppt/slides/slide2.xml"]) {
		t.Fatalf("untouched slide2.xml changed during Update")
	}

	info, err := Inspect(deckPath)
	if err != nil {
		t.Fatalf("Inspect updated failed: %v", err)
	}
	if info.SlideCount != 3 {
		t.Fatalf("Inspect SlideCount = %d, want 3", info.SlideCount)
	}
	s1 := strings.Join(info.Slides[0].Texts, "\n")
	if !strings.Contains(s1, "Финальная стратегия") || !strings.Contains(s1, "Версия 2") {
		t.Fatalf("slide 1 replacements missing: %q", s1)
	}
	if !strings.Contains(info.Slides[0].Notes, "Обновлённые заметки спикера.") || !strings.Contains(info.Slides[0].Notes, "Вторая строка заметок.") {
		t.Fatalf("slide 1 notes not updated: %q", info.Slides[0].Notes)
	}
	if !strings.Contains(info.Slides[1].Notes, "Новые заметки для второго слайда.") {
		t.Fatalf("slide 2 new notes not added: %q", info.Slides[1].Notes)
	}
	s3 := strings.Join(info.Slides[2].Texts, "\n")
	if !strings.Contains(s3, "Целевые ориентиры") || !strings.Contains(s3, "2.5x") {
		t.Fatalf("appended slide 3 content missing: %q", s3)
	}
	if !strings.Contains(info.Slides[2].Notes, "Заметки третьего добавленного слайда.") {
		t.Fatalf("appended slide 3 notes missing: %q", info.Slides[2].Notes)
	}
}

func TestErrorHandling(t *testing.T) {
	dir := t.TempDir()

	if _, err := Create(CreateOptions{OutputPath: ""}); err == nil {
		t.Fatalf("expected error on empty output path")
	}
	if _, err := Inspect(""); err == nil {
		t.Fatalf("expected error on empty inspect path")
	}
	if _, err := Inspect(filepath.Join(dir, "missing.pptx")); err == nil {
		t.Fatalf("expected error on missing file")
	}
	if _, err := FillTemplate("", filepath.Join(dir, "out.pptx"), map[string]string{"a": "b"}); err == nil {
		t.Fatalf("expected error on empty FillTemplate input path")
	}
	if _, err := FillTemplate(filepath.Join(dir, "in.pptx"), "", map[string]string{"a": "b"}); err == nil {
		t.Fatalf("expected error on empty FillTemplate output path")
	}
	if _, err := Update(UpdateOptions{InputPath: ""}); err == nil {
		t.Fatalf("expected error on empty Update input path")
	}

	notZip := filepath.Join(dir, "corrupt.pptx")
	if err := os.WriteFile(notZip, []byte("not a zip archive"), 0o644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if _, err := Inspect(notZip); err == nil {
		t.Fatalf("expected error on invalid zip in Inspect")
	}
	if _, err := Update(UpdateOptions{InputPath: notZip, OutputPath: filepath.Join(dir, "out.pptx")}); err == nil {
		t.Fatalf("expected error on invalid zip in Update")
	}

	emptyZip := filepath.Join(dir, "not-pptx.zip")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("hello.txt")
	_, _ = w.Write([]byte("hi"))
	_ = zw.Close()
	if err := os.WriteFile(emptyZip, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write non-pptx zip: %v", err)
	}
	if _, err := Inspect(emptyZip); err == nil {
		t.Fatalf("expected error on zip without ppt/presentation.xml")
	}

	validPath := filepath.Join(dir, "valid.pptx")
	if _, err := Create(CreateOptions{
		OutputPath: validPath,
		Slides:     []SlideSpec{{Layout: "title", Title: "One"}},
	}); err != nil {
		t.Fatalf("Create valid failed: %v", err)
	}
	if _, err := Update(UpdateOptions{
		InputPath:    validPath,
		NotesUpdates: []NotesUpdate{{Slide: 5, Notes: "out of range"}},
	}); err == nil {
		t.Fatalf("expected error on out-of-range slide in NotesUpdates")
	}

	if _, err := Create(CreateOptions{
		OutputPath: filepath.Join(dir, "bad-img.pptx"),
		Slides: []SlideSpec{
			{Layout: "image_text", Title: "Bad", ImagePath: filepath.Join(dir, "nope.png")},
		},
	}); err == nil {
		t.Fatalf("expected error on missing image path")
	}
}

func TestUpdateAddNotesAndImageToMinimalDeckAndMultilineReplace(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "minimal.pptx")
	if _, err := Create(CreateOptions{
		OutputPath: basePath,
		Title:      "Minimal",
		Slides: []SlideSpec{
			{
				Layout:  "title_content",
				Title:   "Заголовок {{tag}}",
				Bullets: []string{"Детали: {{details}}"},
			},
		},
	}); err != nil {
		t.Fatalf("Create minimal failed: %v", err)
	}

	pngPath := filepath.Join(dir, "added.png")
	writeTestPNG(t, pngPath, 200, 100)

	outPath := filepath.Join(dir, "enriched.pptx")
	res, err := Update(UpdateOptions{
		InputPath:  basePath,
		OutputPath: outPath,
		WorkingDir: dir,
		Replacements: map[string]string{
			"{{tag}}":     `A & B <C> "D"`,
			"{{details}}": "Первая строка\nВторая строка",
		},
		NotesUpdates: []NotesUpdate{
			{Slide: 1, Notes: "Добавленные заметки к слайду без заметок"},
		},
		AppendSlides: []SlideSpec{
			{
				Layout:       "image_text",
				Title:        "Добавленный слайд с картинкой",
				Body:         "Пояснение к изображению",
				ImagePath:    "added.png",
				ImageCaption: "Подпись",
				Notes:        "Заметки нового слайда",
			},
		},
	})
	if err != nil {
		t.Fatalf("Update minimal deck failed: %v", err)
	}
	if res.SlideCount != 2 {
		t.Fatalf("expected 2 slides, got %d", res.SlideCount)
	}

	assertValidXMLParts(t, outPath)

	parts := readZipMap(t, outPath)
	ctStr := string(parts["[Content_Types].xml"])
	pngDefaultIdx := strings.Index(ctStr, `Extension="png"`)
	firstOverrideIdx := strings.Index(ctStr, `<Override `)
	if pngDefaultIdx < 0 || firstOverrideIdx < 0 || pngDefaultIdx > firstOverrideIdx {
		t.Fatalf("expected png Default before Override in [Content_Types].xml: %s", ctStr)
	}
	if _, ok := parts["ppt/notesMasters/notesMaster1.xml"]; !ok {
		t.Fatalf("expected notesMaster1.xml to be created")
	}

	info, err := Inspect(outPath)
	if err != nil {
		t.Fatalf("Inspect enriched failed: %v", err)
	}
	if info.SlideCount != 2 {
		t.Fatalf("Inspect SlideCount = %d, want 2", info.SlideCount)
	}
	s1 := strings.Join(info.Slides[0].Texts, "\n")
	if !strings.Contains(s1, `Заголовок A & B <C> "D"`) {
		t.Fatalf("escaped special chars replacement mismatch: %q", s1)
	}
	if !strings.Contains(s1, "Первая строка\nВторая строка") {
		t.Fatalf("multiline replacement mismatch: %q", s1)
	}
	if info.Slides[0].Notes != "Добавленные заметки к слайду без заметок" {
		t.Fatalf("slide 1 newly added notes mismatch: %q", info.Slides[0].Notes)
	}
	if info.Slides[1].Notes != "Заметки нового слайда" {
		t.Fatalf("slide 2 notes mismatch: %q", info.Slides[1].Notes)
	}
}

func TestRenderTableFrameColWidthsPositive(t *testing.T) {
	theme := resolveTheme("", "", "", "", "")
	const cx int64 = 9000000

	cases := []struct {
		name      string
		headers   []string
		colWidths []float64
	}{
		{
			name:      "mixed positive and zero 3 cols",
			headers:   []string{"A", "B", "C"},
			colWidths: []float64{3, 0, 1},
		},
		{
			name:      "positive and zero 2 cols",
			headers:   []string{"A", "B"},
			colWidths: []float64{1, 0},
		},
		{
			name:      "mixed positive and negative 3 cols",
			headers:   []string{"A", "B", "C"},
			colWidths: []float64{3, -1, 2},
		},
		{
			name:      "extreme skew 3 cols",
			headers:   []string{"A", "B", "C"},
			colWidths: []float64{1e9, 1, 1},
		},
		{
			name:      "valid custom weights 3 cols",
			headers:   []string{"A", "B", "C"},
			colWidths: []float64{2, 3, 5},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tbl := &TableSpec{
				Headers:   tc.headers,
				Rows:      [][]string{make([]string, len(tc.headers))},
				ColWidths: tc.colWidths,
			}
			frameXML := renderTableFrame(10, 0, 0, cx, 3000000, tbl, theme)
			wrapped := `<root xmlns:a="` + nsA + `" xmlns:p="` + nsP + `">` + frameXML + `</root>`

			dec := xml.NewDecoder(strings.NewReader(wrapped))
			var widths []int64
			for {
				tok, err := dec.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("parse table frame XML: %v", err)
				}
				se, ok := tok.(xml.StartElement)
				if !ok || se.Name.Local != "gridCol" {
					continue
				}
				for _, attr := range se.Attr {
					if attr.Name.Local == "w" {
						w, perr := strconv.ParseInt(attr.Value, 10, 64)
						if perr != nil {
							t.Fatalf("parse gridCol w=%q: %v", attr.Value, perr)
						}
						widths = append(widths, w)
					}
				}
			}

			if len(widths) != len(tc.headers) {
				t.Fatalf("expected %d gridCol elements, got %d (%v)", len(tc.headers), len(widths), widths)
			}
			var sum int64
			for i, w := range widths {
				if w <= 0 {
					t.Fatalf("gridCol[%d] = %d, want > 0 (all widths: %v)", i, w, widths)
				}
				sum += w
			}
			if sum != cx {
				t.Fatalf("gridCol sum = %d, want %d (widths: %v)", sum, cx, widths)
			}
		})
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "mixed-colwidths.pptx")
	if _, err := Create(CreateOptions{
		OutputPath: outPath,
		Slides: []SlideSpec{
			{
				Layout: "table",
				Title:  "Table",
				Table: &TableSpec{
					Headers:   []string{"Col1", "Col2", "Col3"},
					Rows:      [][]string{{"1", "2", "3"}},
					ColWidths: []float64{3, 0, 1},
				},
			},
		},
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	assertValidXMLParts(t, outPath)
}

func TestReplaceInDrawingXMLDefaultNamespace(t *testing.T) {
	t.Run("single line", func(t *testing.T) {
		input := []byte(`<p xmlns="` + nsA + `"><r><t>{{key}}</t></r></p>`)
		patched, count, err := replaceInDrawingXML(input, map[string]string{
			"{{key}}": "Updated value",
		})
		if err != nil {
			t.Fatalf("replaceInDrawingXML failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 replacement, got %d", count)
		}
		if strings.Contains(string(patched), "a:") {
			t.Fatalf("unexpected a: prefix in default-namespace XML: %s", string(patched))
		}

		dec := xml.NewDecoder(bytes.NewReader(patched))
		var texts []string
		inT := false
		for {
			tok, derr := dec.Token()
			if derr == io.EOF {
				break
			}
			if derr != nil {
				t.Fatalf("patched XML failed xml.Decoder.Token: %v\nxml: %s", derr, string(patched))
			}
			switch el := tok.(type) {
			case xml.StartElement:
				if el.Name.Space == nsA && el.Name.Local == "t" {
					inT = true
				}
			case xml.EndElement:
				if el.Name.Space == nsA && el.Name.Local == "t" {
					inT = false
				}
			case xml.CharData:
				if inT {
					texts = append(texts, string(el))
				}
			}
		}
		if got := strings.Join(texts, ""); got != "Updated value" {
			t.Fatalf("decoded text = %q, want %q", got, "Updated value")
		}
	})

	t.Run("multi line", func(t *testing.T) {
		input := []byte(`<p xmlns="` + nsA + `"><r><rPr b="1"/><t>{{key}}</t></r></p>`)
		patched, count, err := replaceInDrawingXML(input, map[string]string{
			"{{key}}": "Line 1\nLine 2",
		})
		if err != nil {
			t.Fatalf("replaceInDrawingXML failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 replacement, got %d", count)
		}
		if strings.Contains(string(patched), "a:") {
			t.Fatalf("unexpected a: prefix in default-namespace XML: %s", string(patched))
		}

		dec := xml.NewDecoder(bytes.NewReader(patched))
		var lines []string
		var brCount int
		var rPrCount int
		inT := false
		for {
			tok, derr := dec.Token()
			if derr == io.EOF {
				break
			}
			if derr != nil {
				t.Fatalf("patched XML failed xml.Decoder.Token: %v\nxml: %s", derr, string(patched))
			}
			switch el := tok.(type) {
			case xml.StartElement:
				if el.Name.Space == nsA && el.Name.Local == "t" {
					inT = true
				}
				if el.Name.Space == nsA && el.Name.Local == "br" {
					brCount++
				}
				if el.Name.Space == nsA && el.Name.Local == "rPr" {
					rPrCount++
				}
			case xml.EndElement:
				if el.Name.Space == nsA && el.Name.Local == "t" {
					inT = false
				}
			case xml.CharData:
				if inT {
					lines = append(lines, string(el))
				}
			}
		}
		if brCount != 1 {
			t.Fatalf("expected 1 <br/> in DrawingML namespace, got %d (xml: %s)", brCount, string(patched))
		}
		if rPrCount != 2 {
			t.Fatalf("expected 2 <rPr> elements preserving run formatting, got %d (xml: %s)", rPrCount, string(patched))
		}
		if !slices.Equal(lines, []string{"Line 1", "Line 2"}) {
			t.Fatalf("decoded lines = %#v, want [\"Line 1\", \"Line 2\"]", lines)
		}
	})

	t.Run("split run multi line", func(t *testing.T) {
		input := []byte(`<p xmlns="` + nsA + `"><r><t>{{ke</t></r><r><t>y}}</t></r></p>`)
		patched, count, err := replaceInDrawingXML(input, map[string]string{
			"{{key}}": "Line 1\nLine 2",
		})
		if err != nil {
			t.Fatalf("replaceInDrawingXML failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 replacement, got %d", count)
		}

		dec := xml.NewDecoder(bytes.NewReader(patched))
		var lines []string
		inT := false
		for {
			tok, derr := dec.Token()
			if derr == io.EOF {
				break
			}
			if derr != nil {
				t.Fatalf("patched XML failed xml.Decoder.Token: %v\nxml: %s", derr, string(patched))
			}
			switch el := tok.(type) {
			case xml.StartElement:
				if el.Name.Space == nsA && el.Name.Local == "t" {
					inT = true
				}
			case xml.EndElement:
				if el.Name.Space == nsA && el.Name.Local == "t" {
					inT = false
				}
			case xml.CharData:
				if inT {
					lines = append(lines, string(el))
				}
			}
		}
		if !slices.Equal(lines, []string{"Line 1", "Line 2"}) {
			t.Fatalf("decoded lines = %#v, want [\"Line 1\", \"Line 2\"]", lines)
		}
	})
}
