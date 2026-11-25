# Built-in Document Tools, Skills, and Chat File Cards

Date: 2026-11-25
Status: approved in chat

## Goal

Provide first-class built-in support for creating, inspecting, and editing
**PDF (`.pdf`)**, **Word (`.docx`)**, **Excel (`.xlsx`)**, and **PowerPoint (`.pptx`)**
documents directly inside Prime (`desktop-engine` and CLI) with zero external
runtime dependencies (no Python, Node.js, or LibreOffice required), paired with
built-in hidden system skills and rich file cards/chips in the Prime Desktop
chat whenever a file path is referenced.

## Architecture Overview

1. **Core document engines (`internal/documents/`):** modular Go packages per
   format (`pdf`, `docx`, `xlsx`, `pptx`) backed by pure-Go libraries vendored
   under `vendordeps/` and Go stdlib (`archive/zip`, `encoding/xml`), plus
   embedded UTF-8 TrueType fonts with Cyrillic coverage.
2. **Agent tools (`internal/agent/tools/`):** four dedicated tools registered in
   `internal/agent/coordinator.go` and listed in `allToolNames()` in
   `internal/config/config.go`:
   - `pdf_document`
   - `word_document`
   - `excel_workbook`
   - `powerpoint_presentation`
3. **Built-in hidden system skills (`internal/skills/builtin/`):** four system
   skills (`prime-pdf`, `prime-docx`, `prime-xlsx`, `prime-pptx`) hidden in the
   Settings UI (`skillsFilter.ts`) and exposed to the agent via
   `<available_skills>` for structure, layout, formula, and citation guidance.
4. **Chat file cards & native file opening (`desktop/`):**
   - Tauri host (`desktop/src-tauri/src/media.rs`, `commands.rs`): enrich
     `inspectPath` metadata (`size`, `extension`) for files and add an
     `openPath` command that opens workspace or absolute generated files with the
     OS default application.
   - Chat renderer (`ChatPathEntry.tsx`, `chatPaths.ts`, `chatPaths.css`): render
     recognized local file paths as styled document cards (block) or compact
     chips (inline) with extension-specific icons, color badges, file size,
     **Open**, and **Reveal in folder** actions.

---

## 1. Document Tools (`internal/agent/tools/` + `internal/documents/`)

All four tools share consistent safety and output conventions:
- **Path resolution & permissions:** resolve relative paths against
  `workingDir`, reject empty paths, and request `write` permission via
  `permission.Service` for mutating actions (`create`, `update`,
  `fill_template`, `merge`, `split`, `rotate`) before touching disk. Read-only
  `inspect` actions verify file existence without mutating.
- **Atomic writes:** write to a temporary file in the target directory and
  rename into place so partial files are never left behind on failure.
- **Explicit path reporting:** every mutating result returns the resolved
  absolute path (`path`), file size (`size_bytes`), format summary (pages /
  sheets / slides / paragraphs), and an explicit instruction telling the agent
  to include the exact file path in its final reply to the user.

### 1.1 `pdf_document` (`internal/documents/pdf` + `internal/agent/tools/pdf_document.go`)
- **Operations (`action`):**
  - `create`: builds a multi-page PDF from structured sections (`title`,
    `subtitle`, `author`, `page_size` `A4|Letter`, `orientation`
    `portrait|landscape`, `margins`, `header`, `footer`, `page_numbers`, and
    `blocks` supporting `heading` levels 1–3, `paragraph`, `bullet_list`,
    `numbered_list`, `table` with headers/column widths/alignment, `callout`,
    `image` PNG/JPEG, `divider`, `page_break`).
  - `inspect`: extracts page count, PDF metadata, and per-page text content
    (with optional `page_range`).
  - `merge`: combines multiple input PDFs into `output_path`.
  - `split`: extracts a `page_range` or splits every N pages into `output_dir`
    / `output_path`.
  - `rotate`: rotates selected pages (`90`, `180`, `270`) into `output_path`.
- **Implementation:** pure-Go PDF writer (`signintech/gopdf` or `maroto/v2` +
  `pdfcpu` in `vendordeps/`) with embedded UTF-8 TrueType fonts (Normal, Bold,
  Italic, Monospace) supporting full Cyrillic and Latin character sets.

### 1.2 `word_document` (`internal/documents/docx` + `internal/agent/tools/word_document.go`)
- **Operations (`action`):**
  - `create`: creates a valid OOXML `.docx` package with title/metadata, styles,
    headings (H1–H3), styled paragraphs/runs (bold, italic, underline, color,
    alignment), bullet/numbered lists, styled tables (header fill, borders,
    column widths), embedded PNG/JPEG images, headers/footers, and page numbers.
  - `inspect`: parses an existing `.docx` and returns headings, paragraphs,
    tables, headers/footers, comments, and detected `{{placeholders}}`.
  - `update`: opens an existing `.docx` and appends sections/tables/paragraphs,
    or replaces matching paragraphs/table cells while preserving unrelated OOXML
    parts (themes, styles, media, relationships, numbering).
  - `fill_template`: replaces `{{key}}` placeholders across body paragraphs,
    tables, headers, and footers (including split-run XML tokens) and saves to
    `output_path`.
- **Implementation:** modular pure-Go OOXML engine in `internal/documents/docx`
  using stdlib `archive/zip` and `encoding/xml` (with round-trip ZIP part
  preservation for `update` and `fill_template`).

### 1.3 `excel_workbook` (`internal/documents/xlsx` + `internal/agent/tools/excel_workbook.go`)
- **Operations (`action`):**
  - `create`: creates a multi-sheet `.xlsx` workbook with cell values (strings,
    numbers, booleans, dates), live Excel formulas (`=SUM(...)`, `=IF(...)`,
    etc.), number formats, fonts/fills/borders/alignment, column widths, row
    heights, freeze panes, auto-filters, merged cells, and native charts.
  - `inspect`: returns sheet names, dimensions, cell values, formulas, and any
    formula error tokens.
  - `update`: opens an existing `.xlsx` workbook and applies targeted operations
    (set cell/range values or formulas, apply styles, add/rename/delete sheets,
    append rows, resize columns) while preserving existing formatting and
    worksheets.
- **Implementation:** `github.com/xuri/excelize/v2` vendored under
  `vendordeps/`.

### 1.4 `powerpoint_presentation` (`internal/documents/pptx` + `internal/agent/tools/powerpoint_presentation.go`)
- **Operations (`action`):**
  - `create`: builds a 16:9 or 4:3 `.pptx` presentation with theme colors,
    layouts (`title`, `title_content`, `two_column`, `table`, `metrics`,
    `image_text`, `blank`), shapes, styled text boxes, bullet lists, tables,
    embedded PNG/JPEG images, slide numbers, and speaker notes.
  - `inspect`: reads slide count, slide dimensions, per-slide text runs,
    tables, speaker notes, and detected `{{placeholders}}`.
  - `update`: opens an existing `.pptx`, updates slide text/placeholders,
    speaker notes, or appends new slides while preserving existing slide
    masters, themes, media, and layouts.
  - `fill_template`: substitutes `{{key}}` placeholders across slides, tables,
    and notes in an existing deck and writes to `output_path`.
- **Implementation:** modular pure-Go OOXML presentation engine in
  `internal/documents/pptx` using stdlib `archive/zip` and `encoding/xml` (or
  `F31/go-pptx/v2` vendored if suited to round-trip patching).

---

## 2. Config & Agent Prompt Integration

- `internal/config/config.go`: add `pdf_document`, `word_document`,
  `excel_workbook`, `powerpoint_presentation` to `allToolNames()` and include them in the
  appropriate agent tool profiles (`coder`, `general`; read-only `inspect` usage
  or full exclusion in `resolveReadOnlyTools` consistent with `write`/`edit`).
- `internal/agent/coordinator.go`: register the four tools in `buildTools()`.
- `internal/agent/prompt/desktop.go` & `cli.go`: add a brief rule stating that
  whenever a document or file is created or modified for the user, the assistant
  must include its exact file path in the reply.
- Regenerate `schema.json` if tool names appear in schema definitions.

---

## 3. Built-in Hidden Skills (`internal/skills/builtin/`)

Add four built-in skills:
- `internal/skills/builtin/prime-pdf/SKILL.md`
- `internal/skills/builtin/prime-docx/SKILL.md`
- `internal/skills/builtin/prime-xlsx/SKILL.md`
- `internal/skills/builtin/prime-pptx/SKILL.md`

Each skill:
- Instructs the agent to prefer the corresponding built-in tool (`pdf_document`,
  `word_document`, `excel_workbook`, `powerpoint_presentation`) over ad-hoc shell scripts.
- Documents clean structure, typography, formula hygiene (live formulas over
  hardcoded totals in `.xlsx`), slide density limits, and post-edit `inspect`
  verification.
- Requires citing the created/updated file path on its own line or link in the
  final answer so the Desktop chat renders the file card.
- Is added to `BUILTIN_PRIME_SKILLS` in
  `desktop/src/renderer/src/features/settings/skillsFilter.ts` so all builtin
  skills remain hidden from **Settings -> Skills**.

---

## 4. Desktop Chat File Cards & Native Open (`desktop/`)

### 4.1 Tauri Host (`desktop/src-tauri/src/`)
- `media.rs` (`inspect_local_path`):
  - For regular files (`kind: "file"`), include `"size": metadata.len()` and
    `"extension": ext_lowercase` alongside `exists`, `kind`, `name`, `path`,
    `previewable`, `revealable`.
  - Add `resolve_open_target` (or reuse `resolve_reveal_target` validation) for
    opening files outside or inside workspace root safely while blocking `..`
    traversal.
- `commands.rs`:
  - Add `"openPath"` command in `desktop_call`: resolves the path via
    `media::resolve_reveal_target` (supports both workspace-relative and
    absolute generated paths) and opens it with the OS default handler via
    `open::that(&target)`.
  - Register `"openPath": true` in `desktop/src/shared/hostMethods.json`,
    `desktop/src/shared/contracts.ts` (`InspectedPath` + `DesktopApi.openPath`),
    and `desktop/src/renderer/src/host/installPreviewHost.ts`.

### 4.2 Chat UI (`desktop/src/renderer/src/features/chat/`)
- `chatPaths.ts`:
  - Extend `isLocalPathCandidate` so bare or relative document/file paths with
    common file extensions (`.pdf`, `.docx`, `.doc`, `.xlsx`, `.xls`, `.csv`,
    `.pptx`, `.ppt`, `.txt`, `.md`, `.rtf`, `.odt`, `.ods`, `.odp`, `.zip`,
    `.json`, `.html`) are also recognized alongside images/videos, without
    false-triggering on URLs or version strings.
- `ChatPathEntry.tsx` + `chatPaths.css`:
  - Replace the plain-text fallback for `inspected.kind === 'file'` with:
    - **Block card (`variant === 'block'`):**
      - Left: crisp file-type badge/icon (`PDF` red `FileText`, `DOCX`/`DOC`
        blue `FileText`, `XLSX`/`XLS`/`CSV` green `FileSpreadsheet`,
        `PPTX`/`PPT` amber/orange `Presentation`, code/text/archive fallback).
      - Middle: file name (strong, truncated cleanly), metadata line
        (`PDF • 128 KB` or extension + path tooltip).
      - Right: primary **«Открыть»** button (calls `window.prime.openPath`) and
        secondary folder icon button **«Открыть в проводнике»** (`window.prime.reveal`).
    - **Inline chip (`variant === 'inline'`):**
      - Clickable chip with colored file-type icon + filename (`openPath`) +
        compact adjacent folder icon button (`reveal`).
  - Translations added to `desktop/src/shared/translations.ts` where needed.

### 4.3 Chat Document Attachments (`.docx`, `.xlsx`, `.pptx`, `.pdf`)
- **Tauri Host (`desktop/src-tauri/src/attachments.rs`, `commands.rs`):**
  - Detect proper MIME types for `.pdf` (`application/pdf`), `.docx`
    (`application/vnd.openxmlformats-officedocument.wordprocessingml.document`),
    `.xlsx` (`application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`),
    and `.pptx` (`application/vnd.openxmlformats-officedocument.presentationml.presentation`)
    instead of labeling binary Office ZIP files as `text/plain`.
  - Support `importAttachments` (alongside `importImages`) so drag-and-drop and
    clipboard files work for documents as well as images.
- **Backend & Agent Prompt (`internal/backend/agent.go`, `internal/message/`, `internal/agent/agent.go`):**
  - Do not inline binary `.docx`/`.xlsx`/`.pptx` bytes as text attachments or
    send unsupported Office ZIP `FilePart`s to LLM providers.
  - Materialize attached document files to a persistent session-accessible
    directory (`<dataDir>/attachments/<sessionID>/<safe_filename>`) and inject
    `<attached_document name="..." mime="..." path="...">` into the user turn
    so `word_document`, `excel_workbook`, `powerpoint_presentation`, and
    `pdf_document` can inspect and edit the attached file directly via `input_path`.
- **Desktop Composer & Message Bubble (`Composer.tsx`, `MessageAttachment.tsx`):**
  - Allow dropping `.docx`, `.xlsx`, `.pptx`, `.pdf`, text, and image files onto
    the chat composer.
  - Display attached document chips with file-type icons in both the composer
    and sent user messages (`MessageAttachment.tsx`).

---

## 5. Testing & Verification

- **Go unit tests:**
  - `internal/documents/...`: round-trip create -> inspect -> update /
    fill_template / merge / split / rotate tests for all four formats, including
    Cyrillic text in PDF, DOCX, XLSX, and PPTX, live formula storage in XLSX,
    and valid ZIP/XML structure checks.
  - `internal/agent/tools/...`: schema, permission prompts, path traversal
    safety, and tool execution tests for `pdf_document`, `word_document`,
    `excel_workbook`, `powerpoint_presentation`.
  - `internal/skills/skills_test.go`: verify `prime-pdf`, `prime-docx`,
    `prime-xlsx`, `prime-pptx` load as builtins.
  - `internal/config/load_test.go`: verify tool list and read-only role
    filtering.
- **Rust tests (`desktop/src-tauri`):**
  - `media.rs` unit tests for `size` and `extension` fields in
    `inspect_local_path` on `.pdf`/`.docx`/`.xlsx`/`.pptx`.
- **Frontend unit + visual verification (`desktop/`):**
  - Vitest tests for `chatPaths.ts`, `ChatPathEntry.tsx` (block card and
    inline chip rendering for `.pdf`, `.docx`, `.xlsx`, `.pptx`, actions
    calling `openPath` and `reveal`), and `skillsFilter.ts` hiding the 4 new
    builtin skills.
  - Built-in browser visual check of rendered chat document cards at desktop and
    narrow widths (light/dark themes, long filenames, hover/focus states).
