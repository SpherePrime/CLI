# Built-in Document Tools, Skills, Chat Attachments & File Cards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add built-in pure-Go tools (`pdf_document`, `word_document`, `excel_workbook`, `powerpoint_presentation`) for creating, inspecting, and editing PDF/DOCX/XLSX/PPTX files, four hidden built-in skills, full document attachment flow in chat (including drag-and-drop editing of user-attached files), and interactive file cards/chips with Open & Reveal actions in Prime Desktop chat.

**Architecture:** The backend adds modular pure-Go format engines under `internal/documents/` (using Go stdlib OOXML ZIP/XML for DOCX/PPTX and vendored pure-Go packages under `vendordeps/` for PDF and XLSX, plus embedded UTF-8 Cyrillic TrueType fonts), exposed via four tools in `internal/agent/tools/` and four hidden builtin skills in `internal/skills/builtin/`. Attached documents in chat are preserved with real extensions in `<dataDir>/attachments/<sessionID>/` and surfaced to the model as `<attached_document path="...">` instead of raw binary garbage. The Tauri host enriches `inspectPath` with `size`/`extension`, adds `openPath`, fixes office MIME detection in `attachments.rs`, and the React chat renderer renders file cards/chips in `ChatPathEntry.tsx` and `MessageAttachment.tsx`.

**Tech Stack:** Go 1.27 (`archive/zip`, `encoding/xml`, `vendordeps/` pure-Go `signintech/gopdf`, `pdfcpu`, `xuri/excelize/v2`), Rust/Tauri (`desktop/src-tauri/src/`), React 19 + TypeScript + Vite (`desktop/src/`), Go `testing`, `cargo test`, Vitest.

**Spec:** `docs/superpowers/specs/2026-11-25-builtin-documents-and-chat-cards-design.md`

## Global Constraints
- Zero external runtime dependencies (must work without Python, Node.js, or LibreOffice).
- Full UTF-8 & Cyrillic text support in all four formats (`.pdf`, `.docx`, `.xlsx`, `.pptx`).
- No AI/model attribution anywhere in code, docs, or commits.
- Keep code modular across focused files; do not add unrequested comments in code.
- Work only on branch `main`.
- All new UI labels must be in Russian with English translations in `desktop/src/shared/translations.ts`.

## Review Focus
1. Creating and inspecting Russian/Cyrillic text in PDF, DOCX, XLSX, and PPTX produces valid, non-corrupted files (no mojibake or missing glyph boxes).
2. Editing an existing `.docx` or `.pptx` with `update` / `fill_template` preserves original styles, images, themes, and replaces `{{placeholders}}` even when Word/PowerPoint splits placeholder text across multiple XML `<w:t>` / `<a:t>` runs.
3. Creating or updating `.xlsx` stores live formulas (`=SUM(B2:B5)`) rather than hardcoded totals and preserves existing sheets/styles on `update`.
4. Dropping or attaching a `.docx` / `.xlsx` / `.pptx` / `.pdf` file in Prime Desktop chat neither dumps binary ZIP bytes into the prompt nor sends unsupported Office MIME `FilePart`s to LLM APIs, and provides a real readable `path` on disk that `word_document` / `excel_workbook` / `powerpoint_presentation` / `pdf_document` can inspect and edit.
5. Outputting a local file path (e.g. `C:\...\report.pdf` or `./out/report.docx`) in an assistant message renders a file card (block) or file chip (inline) with extension icon, size, working **Открыть** (`openPath`) and **В папке** (`reveal`) buttons, while rejecting `..` path traversal.

---

### Task 1: Shared Document Foundation, Embedded Cyrillic Fonts & Pure-Go Dependencies

**Files:**
- Create: `internal/documents/docutil/fonts.go`
- Create: `internal/documents/docutil/paths.go`
- Create: `internal/documents/docutil/atomic.go`
- Create: `internal/documents/docutil/fonts/` (Regular, Bold, Italic, Mono OFL TTF fonts with verified Cyrillic glyphs)
- Create: `internal/documents/docutil/docutil_test.go`
- Vendor: `vendordeps/signintech/gopdf` (+ `phpdave11/gofpdi`), `vendordeps/pdfcpu/pdfcpu` (+ pure-Go transitive deps), `vendordeps/xuri/excelize/v2` (+ pure-Go transitive deps), with `go.mod` renamed to `go.dev.mod` and internal imports rewritten to `github.com/SpherePrime/CLI/vendordeps/...`

**Interfaces:**
- Produces:
  - `docutil.RegularFontBytes() []byte`, `docutil.BoldFontBytes() []byte`, `docutil.ItalicFontBytes() []byte`, `docutil.MonoFontBytes() []byte`
  - `docutil.ResolvePath(workingDir, rawPath string) (string, error)`
  - `docutil.WriteFileAtomic(path string, data []byte) error`
  - `docutil.WriteZipAtomic(path string, writeFn func(zw *zip.Writer) error) error`

- [ ] **Step 1: Verify Cyrillic coverage on candidate OFL fonts**

Inspect `internal/skills/builtin/prime-designer/assets/fonts/` (`Lora-*`, `JetBrainsMono-*`, or download DejaVu/Noto Sans OFL if sans Cyrillic bold/regular is missing) using a small Go test against `vendordeps/golang/freetype/truetype` checking runes `'А'..'Я'`, `'а'..'я'`, `'Ё'`, `'ё'`.

- [ ] **Step 2: Write failing tests in `internal/documents/docutil/docutil_test.go`**

```go
package docutil

import (
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/golang/freetype/truetype"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestEmbeddedFontsSupportCyrillic(t *testing.T) {
	for _, data := range [][]byte{RegularFontBytes(), BoldFontBytes(), ItalicFontBytes(), MonoFontBytes()} {
		font, err := truetype.Parse(data)
		require.NoError(t, err)
		for _, r := range []rune{'A', 'z', 'А', 'Я', 'а', 'я', 'Ё', 'ё', '0', '9'} {
			require.NotZero(t, font.Index(r), "missing glyph for %q", r)
		}
	}
}

func TestResolvePathAndAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	resolved, err := ResolvePath(dir, "reports/out.pdf")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "reports", "out.pdf"), resolved)

	_, err = ResolvePath(dir, "")
	require.Error(t, err)

	require.NoError(t, WriteFileAtomic(resolved, []byte("hello")))
	require.FileExists(t, resolved)
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/documents/docutil/...`
Expected: FAIL (package does not exist yet).

- [ ] **Step 4: Implement `docutil` and vendor pure-Go PDF/XLSX packages into `vendordeps/`**

Implement `fonts.go`, `paths.go`, `atomic.go`. Download and vendor `github.com/signintech/gopdf`, `github.com/pdfcpu/pdfcpu`, and `github.com/xuri/excelize/v2` (and their minimal pure-Go dependencies) into `vendordeps/` matching existing repo convention (`go.dev.mod` + rewritten import paths to `github.com/SpherePrime/CLI/vendordeps/...`, `LICENSE` files preserved).

- [ ] **Step 5: Run tests to verify `docutil` and vendored packages compile and pass**

Run: `go test -count=1 ./internal/documents/docutil/... ./vendordeps/signintech/gopdf/... ./vendordeps/xuri/excelize/v2/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/documents/docutil vendordeps/
git commit -m "feat(documents): add docutil foundation, Cyrillic fonts, and pure-Go document deps"
```

---

### Task 2: PDF Engine (`internal/documents/pdf`)

**Files:**
- Create: `internal/documents/pdf/types.go`
- Create: `internal/documents/pdf/create.go`
- Create: `internal/documents/pdf/inspect.go`
- Create: `internal/documents/pdf/manipulate.go`
- Test: `internal/documents/pdf/pdf_test.go`

**Interfaces:**
- Consumes: `internal/documents/docutil`, `vendordeps/signintech/gopdf`, `vendordeps/pdfcpu/pdfcpu`
- Produces:
  - `Create(opts CreateOptions) (*Result, error)` — blocks: `heading`, `paragraph`, `bullet_list`, `numbered_list`, `table`, `callout`, `image`, `divider`, `page_break`; `A4`/`Letter`, `portrait`/`landscape`, headers/footers, page numbers, automatic word wrapping & page breaks, embedded Cyrillic TTF fonts.
  - `Inspect(path string, pageRange string) (*InspectResult, error)` — page count, metadata, extracted text per page.
  - `Merge(inputs []string, outputPath string) (*Result, error)`
  - `Split(inputPath string, pageRange string, span int, outputPath string, outputDir string) (*SplitResult, error)`
  - `Rotate(inputPath string, pageRange string, angle int, outputPath string) (*Result, error)`

- [ ] **Step 1: Write failing tests in `internal/documents/pdf/pdf_test.go`**

Test `Create` with Cyrillic headings/paragraphs/tables/lists/multi-page pagination -> `Inspect` verifies page count, metadata, and extracted Cyrillic text -> `Merge` two PDFs -> `Split` into pages -> `Rotate` 90 degrees -> invalid path / non-existent file errors.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/documents/pdf/...`
Expected: FAIL.

- [ ] **Step 3: Implement `types.go`, `create.go`, `inspect.go`, `manipulate.go`**

Implement clean layout cursor with automatic line wrapping (measuring UTF-8 rune widths via `gopdf.MeasureTextWidth`), page-break overflow handling, styled tables with repeating header rows, `pdfcpu` merge/split/rotate via atomic temp files, and pure-Go PDF content-stream + CMap/ToUnicode text extraction in `inspect.go`.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test -count=1 ./internal/documents/pdf/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/documents/pdf/
git commit -m "feat(documents): add pure-Go PDF engine with Cyrillic support"
```

---

### Task 3: Word DOCX Engine (`internal/documents/docx`)

**Files:**
- Create: `internal/documents/docx/types.go`
- Create: `internal/documents/docx/create.go`
- Create: `internal/documents/docx/inspect.go`
- Create: `internal/documents/docx/update.go`
- Test: `internal/documents/docx/docx_test.go`

**Interfaces:**
- Consumes: `internal/documents/docutil`, stdlib `archive/zip`, `encoding/xml`
- Produces:
  - `Create(opts CreateOptions) (*Result, error)` — headings H1–H3, styled runs (bold, italic, underline, color, font size), alignment, bullet/numbered lists, styled tables (header shading, borders, column widths), embedded PNG/JPEG images, headers/footers, page numbers.
  - `Inspect(path string) (*InspectResult, error)` — metadata, headings, paragraphs, tables, headers/footers, comments, detected `{{placeholders}}`.
  - `Update(opts UpdateOptions) (*Result, error)` — opens existing `.docx`, appends blocks, updates matching paragraph/table text, or replaces text while copying untouched ZIP entries verbatim.
  - `FillTemplate(inputPath, outputPath string, values map[string]string) (*Result, error)` — replaces `{{key}}` across body, tables, headers, and footers, normalizing split XML `<w:r><w:t>` runs so Word-split placeholders are never missed.

- [ ] **Step 1: Write failing tests in `internal/documents/docx/docx_test.go`**

Test `Create` with Cyrillic headings/tables/lists/images -> `Inspect` verifies structure -> build a `.docx` where `{{client_name}}` is split across two `<w:r><w:t>` runs (`{{client_` + `name}}`) and verify `FillTemplate` replaces it cleanly while preserving an existing custom ZIP entry (`word/styles.xml` / media) -> test `Update` appending a new section and table.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/documents/docx/...`
Expected: FAIL.

- [ ] **Step 3: Implement `types.go`, `create.go`, `inspect.go`, `update.go`**

Generate standards-compliant `[Content_Types].xml`, `_rels/.rels`, `word/document.xml`, `word/_rels/document.xml.rels`, `word/styles.xml`, `word/numbering.xml`, `word/header1.xml`, `word/footer1.xml`, `docProps/core.xml`. In `update.go`, parse XML paragraphs cleanly so split placeholder runs coalesce before replacement while keeping run formatting of the opening run.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test -count=1 ./internal/documents/docx/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/documents/docx/
git commit -m "feat(documents): add pure-Go Word DOCX engine with template and update support"
```

---

### Task 4: Excel XLSX Engine (`internal/documents/xlsx`)

**Files:**
- Create: `internal/documents/xlsx/types.go`
- Create: `internal/documents/xlsx/create.go`
- Create: `internal/documents/xlsx/inspect.go`
- Create: `internal/documents/xlsx/update.go`
- Test: `internal/documents/xlsx/xlsx_test.go`

**Interfaces:**
- Consumes: `internal/documents/docutil`, `vendordeps/xuri/excelize/v2`
- Produces:
  - `Create(opts CreateOptions) (*Result, error)` — multiple sheets, typed cells (string/number/bool/date), live formulas (`=SUM(...)`), number formats, styles (header bold/fill/borders/wrap/alignment), column widths + auto-fit, freeze panes, auto-filters, merged cells, native charts (`col`, `bar`, `line`, `pie`).
  - `Inspect(path string, sheet string, cellRange string) (*InspectResult, error)` — sheet names, dimensions, cell values & formulas, formula error detection (`#REF!`, `#DIV/0!`, etc.).
  - `Update(opts UpdateOptions) (*Result, error)` — opens existing `.xlsx` and applies cell edits, formulas, row appends, style patches, sheet add/rename/delete, column widths, and writes atomically.

- [ ] **Step 1: Write failing tests in `internal/documents/xlsx/xlsx_test.go`**

Test `Create` with Cyrillic headers, numbers, `=SUM(B2:B4)` formula, freeze panes, and chart -> `Inspect` verifies sheet list, formula string preserved (`=SUM(B2:B4)`), and Cyrillic text -> `Update` adds a new sheet and updates cell `B5` without erasing sheet 1.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/documents/xlsx/...`
Expected: FAIL.

- [ ] **Step 3: Implement `types.go`, `create.go`, `inspect.go`, `update.go`**

Wrap `excelize.NewFile` and `excelize.OpenFile` with clean structured input structs, auto-width calculation using UTF-8 rune length, and atomic temp-file save (`WriteToBuffer` -> `docutil.WriteFileAtomic`).

- [ ] **Step 4: Run tests to verify pass**

Run: `go test -count=1 ./internal/documents/xlsx/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/documents/xlsx/
git commit -m "feat(documents): add pure-Go Excel XLSX engine with formulas and updates"
```

---

### Task 5: PowerPoint PPTX Engine (`internal/documents/pptx`)

**Files:**
- Create: `internal/documents/pptx/types.go`
- Create: `internal/documents/pptx/create.go`
- Create: `internal/documents/pptx/inspect.go`
- Create: `internal/documents/pptx/update.go`
- Test: `internal/documents/pptx/pptx_test.go`

**Interfaces:**
- Consumes: `internal/documents/docutil`, stdlib `archive/zip`, `encoding/xml`
- Produces:
  - `Create(opts CreateOptions) (*Result, error)` — 16:9 or 4:3 slides, theme palette, layouts (`title`, `title_content`, `two_column`, `table`, `metrics`, `image_text`, `blank`), bullet lists, styled tables, PNG/JPEG images, slide numbers, speaker notes.
  - `Inspect(path string) (*InspectResult, error)` — slide count, dimensions, per-slide text, tables, speaker notes, detected `{{placeholders}}`.
  - `Update(opts UpdateOptions) (*Result, error)` — updates slide text/notes, replaces text, or appends new slides while preserving existing masters/themes/layouts/relationships.
  - `FillTemplate(inputPath, outputPath string, values map[string]string) (*Result, error)` — replaces `{{key}}` across slide XMLs and notes XMLs (coalescing split `<a:r><a:t>` runs).

- [ ] **Step 1: Write failing tests in `internal/documents/pptx/pptx_test.go`**

Test `Create` with Cyrillic title slide, bullet slide, table slide, metrics slide, and speaker notes -> `Inspect` verifies slide count, Cyrillic text, table rows, and speaker notes -> `FillTemplate` replaces split `{{quarter}}` placeholder and `Update` appends a slide while preserving slide 1 master/theme.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/documents/pptx/...`
Expected: FAIL.

- [ ] **Step 3: Implement `types.go`, `create.go`, `inspect.go`, `update.go`**

Generate valid OOXML PresentationML (`[Content_Types].xml`, `_rels/.rels`, `ppt/presentation.xml`, `ppt/slideMasters/slideMaster1.xml`, `ppt/slideLayouts/slideLayout1.xml`, `ppt/theme/theme1.xml`, `ppt/slides/slideN.xml`, `ppt/notesSlides/notesSlideN.xml`, media).

- [ ] **Step 4: Run tests to verify pass**

Run: `go test -count=1 ./internal/documents/pptx/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/documents/pptx/
git commit -m "feat(documents): add pure-Go PowerPoint PPTX engine with template and update support"
```

---

### Task 6: Agent Tools, Permissions, Config, Prompts & Tool Titles

**Files:**
- Create: `internal/agent/tools/pdf_document.go` & `pdf_document.md`
- Create: `internal/agent/tools/word_document.go` & `word_document.md`
- Create: `internal/agent/tools/excel_workbook.go` & `excel_workbook.md`
- Create: `internal/agent/tools/powerpoint_presentation.go` & `powerpoint_presentation.md`
- Create: `internal/agent/tools/documents_test.go`
- Modify: `internal/agent/coordinator.go` (register tools in `buildTools()`)
- Modify: `internal/config/config.go` (`allToolNames()`)
- Modify: `internal/config/load_test.go` (update expected tool list test if needed)
- Modify: `internal/agent/prompt/desktop.go` & `internal/agent/prompt/cli.go` (rule to always cite exact created/updated file paths in final response)
- Modify: `desktop/src/renderer/src/features/chat/toolTitle.ts`, `toolOutputParser.ts`, `desktop/src/shared/translations.ts`

**Interfaces:**
- Consumes: `internal/documents/{pdf,docx,xlsx,pptx}`, `internal/permission`
- Produces:
  - Tool names: `pdf_document`, `word_document`, `excel_workbook`, `powerpoint_presentation`
  - Every mutating action requests `write` permission via `permissions.Request(...)` and returns JSON/formatted text with `path`, `size_bytes`, summary, and explicit instruction to include `path` in the assistant's final reply.

- [ ] **Step 1: Write failing tests in `internal/agent/tools/documents_test.go`**

Test tool invocation, permission prompt gating (deny blocks write, allow writes file), read-only `inspect` without mutating, path validation, and output containing resolved absolute path.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/tools/... -run TestDocumentTools`
Expected: FAIL.

- [ ] **Step 3: Implement the 4 tool files + descriptions, register in `coordinator.go` and `config.go`, update prompts and `toolTitle.ts` / `toolOutputParser.ts`**

Register `NewPDFDocumentTool`, `NewWordDocumentTool`, `NewExcelWorkbookTool`, `NewPowerPointPresentationTool` in `coordinator.go`, add names to `allToolNames()` in `config.go`, add localized titles in `toolTitle.ts` (`PDF-документ`, `Word-документ`, `Excel-таблица`, `Презентация`) and classify them under `'write'` / `'read'` in `toolOutputParser.ts`. Regenerate `schema.json` if required by `prime schema` tests.

- [ ] **Step 4: Run Go & Desktop unit tests to verify pass**

Run: `go test -count=1 ./internal/agent/tools/... ./internal/config/... ./internal/agent/prompt/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/tools/ internal/agent/coordinator.go internal/agent/prompt/ internal/config/ desktop/src/renderer/src/features/chat/toolTitle.ts desktop/src/renderer/src/features/chat/toolOutputParser.ts desktop/src/shared/translations.ts
git commit -m "feat(tools): register pdf_document, word_document, excel_workbook, and powerpoint_presentation"
```

---

### Task 7: Four Built-in Hidden Skills (`prime-pdf`, `prime-docx`, `prime-xlsx`, `prime-pptx`)

**Files:**
- Create: `internal/skills/builtin/prime-pdf/SKILL.md`
- Create: `internal/skills/builtin/prime-docx/SKILL.md`
- Create: `internal/skills/builtin/prime-xlsx/SKILL.md`
- Create: `internal/skills/builtin/prime-pptx/SKILL.md`
- Modify: `desktop/src/renderer/src/features/settings/skillsFilter.ts`
- Modify: `internal/skills/skills_test.go`
- Modify: `desktop/tests/unit/skills-settings.test.tsx`

**Interfaces:**
- Consumes: `pdf_document`, `word_document`, `excel_workbook`, `powerpoint_presentation`
- Produces: four builtin skills loaded at `prime://skills/prime-{pdf,docx,xlsx,pptx}/SKILL.md` and excluded from Settings UI via `BUILTIN_PRIME_SKILLS`.

- [ ] **Step 1: Add failing assertions in `internal/skills/skills_test.go` and `desktop/tests/unit/skills-settings.test.tsx`**

Verify `DiscoverBuiltin()` contains `prime-pdf`, `prime-docx`, `prime-xlsx`, `prime-pptx` and `filterVisibleSettingsSkills` hides all four.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/skills/...` and `npm run test:unit -- tests/unit/skills-settings.test.tsx` (in `desktop/`).
Expected: FAIL.

- [ ] **Step 3: Create the four `SKILL.md` files and update `skillsFilter.ts`**

Each `SKILL.md` defines triggers, design/formatting best practices (live formulas in XLSX, concise slide density in PPTX, `inspect` verification after `create`/`update`, handling attached user documents via `input_path`, and mandatory final-reply file path output). Add all 4 names to `BUILTIN_PRIME_SKILLS` in `skillsFilter.ts`.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test -count=1 ./internal/skills/...` and `npm run test:unit -- tests/unit/skills-settings.test.tsx` (in `desktop/`).
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/skills/ desktop/src/renderer/src/features/settings/skillsFilter.ts desktop/tests/unit/skills-settings.test.tsx
git commit -m "feat(skills): add hidden builtin skills for pdf, docx, xlsx, and pptx"
```

---

### Task 8: Chat Document Attachments & Tauri Host `openPath` / `inspectPath`

**Files:**
- Modify: `desktop/src-tauri/src/attachments.rs` (MIME detection for `.pdf`, `.docx`, `.xlsx`, `.pptx` + `import_files_bytes` for non-image drag-and-drop uploads)
- Modify: `desktop/src-tauri/src/media.rs` (`size` and `extension` in `inspect_local_path` for `kind: "file"`)
- Modify: `desktop/src-tauri/src/commands.rs` (`openPath` and `importAttachments` commands)
- Modify: `desktop/src/shared/hostMethods.json`, `desktop/src/shared/contracts.ts`, `desktop/src/renderer/src/host/installPreviewHost.ts`
- Modify: `internal/backend/agent.go`, `internal/message/content.go`, `internal/agent/agent.go`, `internal/agent/agent_test.go` (persist attached non-text/non-image documents in `<dataDir>/attachments/<sessionID>/<safe_name>` and pass `<attached_document name="..." mime="..." path="...">` to the prompt without dumping raw ZIP binary bytes or sending unsupported Office MIME `FilePart`s)
- Modify: `desktop/src/renderer/src/features/chat/Composer.tsx`, `desktop/src/renderer/src/features/chat/MessageAttachment.tsx`, `desktop/src/shared/translations.ts`

**Interfaces:**
- Consumes: `desktop/src-tauri/src/attachments.rs`, `internal/backend/agent.go`, `internal/message/content.go`
- Produces:
  - `DesktopApi.openPath(workspaceId: string, path: string): Promise<void>`
  - `DesktopApi.importAttachments(files: AttachmentUpload[]): Promise<Attachment[]>`
  - `InspectedPath.size?: number` and `InspectedPath.extension?: string`
  - Persistent readable `<attached_document path="...">` passed to model when user attaches or drops `.docx` / `.xlsx` / `.pptx` / `.pdf` in chat.

- [ ] **Step 1: Write failing tests in Rust (`attachments.rs`, `media.rs`) and Go (`internal/message/content_test.go`, `internal/agent/agent_test.go`)**

Test `.docx`/`.xlsx`/`.pptx`/`.pdf` MIME detection in `attachments.rs`, `size`/`extension` in `media.rs` `inspect_local_path`, and `PromptWithTextAttachments` + `prepareSessionPrompt` skipping raw binary Office bytes while emitting `<attached_document path="...">` with a valid readable file on disk.

- [ ] **Step 2: Run tests to verify failure**

Run: `cargo test --manifest-path desktop/src-tauri/Cargo.toml` and `go test ./internal/message/... ./internal/agent/...`
Expected: FAIL.

- [ ] **Step 3: Implement Rust host updates, Go attachment persistence/prompt formatting, and React Composer drag-and-drop + MessageAttachment document badge**

Update `attachments.rs` (inspect ZIP magic / filename extension for `.docx`/`.xlsx`/`.pptx`), `media.rs`, `commands.rs` (`openPath`, `importAttachments`), `contracts.ts`, `hostMethods.json`, `installPreviewHost.ts`, `internal/backend/agent.go`, `internal/message/content.go`, `internal/agent/agent.go`, `Composer.tsx` (handle `onDragOver`/`onDrop` for files and non-image empty prompt fallback `"Посмотри на вложение."`), and `MessageAttachment.tsx` (show clean document attachment pill with filename & icon).

- [ ] **Step 4: Run Rust, Go, and Vitest unit tests**

Run: `cargo test --manifest-path desktop/src-tauri/Cargo.toml`, `go test -count=1 ./internal/message/... ./internal/backend/... ./internal/agent/...`, `npm run test:unit` (in `desktop/`).
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add desktop/src-tauri/src/ desktop/src/ internal/backend/ internal/message/ internal/agent/
git commit -m "feat(attachments): support document attachments, openPath, and file metadata in inspectPath"
```

---

### Task 9: Chat Document Cards & Inline Chips UI + Visual Verification

**Files:**
- Modify: `desktop/src/renderer/src/features/chat/chatPaths.ts`
- Modify: `desktop/src/renderer/src/features/chat/ChatPathEntry.tsx`
- Modify: `desktop/src/renderer/src/features/chat/chatPaths.css`
- Modify: `desktop/src/shared/translations.ts`
- Modify: `desktop/tests/unit/chat-paths.test.tsx`

**Interfaces:**
- Consumes: `InspectedPath` (`kind: 'file'`, `name`, `path`, `size`, `extension`), `window.prime.openPath`, `window.prime.reveal`
- Produces: styled file card (`chat-path-file-card`) for standalone paths and clickable chip (`chat-path-chip is-file`) + adjacent folder button for inline paths, with extension-specific icon & color badge (`PDF`, `DOCX`, `XLSX`, `PPTX`, generic file) and human-readable file size.

- [ ] **Step 1: Write failing tests in `desktop/tests/unit/chat-paths.test.tsx`**

Test `isLocalPathCandidate` for relative/bare document filenames (`report.pdf`, `./out/plan.docx`, `budget/q4.xlsx`, `deck.pptx`) and test `MarkdownText` rendering block `chat-path-file-card` and inline `chat-path-chip is-file` for `.pdf`, `.docx`, `.xlsx`, `.pptx` with size (`128 КБ`), extension badge, **Открыть**, and **В папке** actions.

- [ ] **Step 2: Run test to verify failure**

Run: `npm run test:unit -- tests/unit/chat-paths.test.tsx` (in `desktop/`).
Expected: FAIL (`kind === 'file'` still returns plain fallback).

- [ ] **Step 3: Implement document card & chip rendering in `chatPaths.ts`, `ChatPathEntry.tsx`, `chatPaths.css`, and translations**

Follow existing `chat-path-folder-card` / `chat-path-chip` visual grammar and theme tokens (`--surface`, `--raised`, `--border`, `--text`, `--muted`, semantic muted file-type badge colors without neon glow). Support keyboard focus, long-filename truncation (`min-width: 0`, `text-overflow: ellipsis`), narrow container wrapping, and click handlers calling `openPath` and `reveal`.

- [ ] **Step 4: Run unit tests, typecheck, and visual check in built-in browser**

Run: `npm run test:unit` and `npm run typecheck` (or `tsc --noEmit`) in `desktop/`. Start the Vite preview dev server (`npm run dev` in `desktop/`) and verify rendered PDF/DOCX/XLSX/PPTX block cards and inline chips in the built-in browser via `browser_run` across desktop and narrow widths (light & dark themes, close-up screenshots inspected).

- [ ] **Step 5: Final full verification & commit**

Run `go test -count=1 ./internal/documents/... ./internal/agent/... ./internal/skills/... ./internal/config/...`, `cargo test --manifest-path desktop/src-tauri/Cargo.toml`, `npm run test:unit` in `desktop/`. Check `git log -1 --format=%B` for zero AI attribution and push to `origin/main`.

```bash
git add desktop/
git commit -m "feat(chat): render interactive document cards and chips for local file paths"
git push origin main
```
