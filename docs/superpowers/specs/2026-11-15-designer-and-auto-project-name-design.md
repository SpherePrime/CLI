# Designer Mode & Auto-Incremented Default Project Name

## Overview

Two additions to Prime Desktop and its Go backend:

1. **Auto-incremented default project naming** when creating projects in `Documents/Prime`, eliminating duplicate-name friction.
2. **Designer mode (`kind: 'designer'`)** — a dedicated workspace tab on the left rail with standalone design chats, a lightweight non-coding `designer` agent with `generate_image`, and one-click handoff (**«Сделать этот дизайн»**) into any regular project.

---

## 1. Auto-Incremented Project Names

### Behavior
- `CreateProjectDialog` queries the host for the next available name based on the localized default (`Новый проект` / `New project`) inside `<Documents>/Prime` (`projectCreationNextName(defaultName)`).
- Sequence: `Новый проект` -> `Новый проект 1` -> `Новый проект 2` -> `Новый проект 3` ...
- On creation (`createProject`), `project_creation::create_directory_auto` attempts `create_dir(&base.join(name))`. On `AlreadyExists`, it atomically increments `"{name} 1"`, `"{name} 2"`, ... until a new directory is created. Existing directories and files are never overwritten or reused.
- Existing `project_creation::create_directory` stays unchanged for strict duplicate-rejection tests.

---

## 2. Designer Mode Architecture

### Isolation Guarantee
The `designer` agent (`config.AgentDesigner = "designer"`) **must never** run as the default or selectable agent for regular projects or personal chats:
- `proto.Workspace.Kind` (`"project" | "chat" | "designer"`) is sent on `POST /v1/workspaces` and stored on `backend.Workspace`.
- `coordinator.NewCoordinator` always initializes `mainAgent` to `AgentGeneral`.
- `Backend.CreateWorkspace` sets `ws.mainAgent = config.AgentDesigner` **only** when `args.Kind == "designer"`.
- `Backend.SetMainAgent` rejects `AgentDesigner` for any workspace where `ws.Kind != "designer"`, and rejects switching a `"designer"` workspace to coding agents (`general`, `plan`, `task`).
- Desktop `Preferences.agentMode` remains strictly `'general' | 'plan'`. `useWorkspace.send()` sends `mode: 'designer'` only when `targetProject.kind === 'designer'`; for all other workspaces it uses `general` or `plan`.
- `Composer` hides the `Общий / План` toggle in Designer workspaces.

### Backend (`internal/config`, `internal/agent`, `internal/backend`, `internal/proto`)
- **Agent definition (`AgentDesigner`):**
  - `AllowedTools`: `generate_image`, `question`, `fetch` (only tools that exist in `allToolNames()` minus `DisabledTools`).
  - `AllowedMCP`: empty map (`map[string][]string{}` — no coding/browser/app MCPs).
- **Prompt (`internal/agent/templates/designer.md.tpl`):**
  - Concise visual/UI/UX designer persona focused on concepting layouts, visual systems, illustrations, and calling `generate_image` (including `source_images` edits) to iterate on designs.
  - Excludes coding instructions, `instructionRoutingTmpl`, `desktopContext`, `cliContext`, `visualCheckBlock`, `reviewPolicyBlock`, `mcpInstructionsBlock`, and empty-`todos` `<system_reminder>`.
  - Any chat model (`large` slot) can be selected in Designer; image generation runs through `generate_image`.

### Desktop Host (`desktop/src-tauri/src/`)
- `projects::designer_directory(&state)` -> `<app_state_dir>/Designer`.
- `openDesigner` host method creates the directory and opens it with `kind = "designer"`.
- `workspace_request` includes `"kind": kind` in `POST /v1/workspaces`.
- `"designer"` workspaces are excluded from `recent` projects (`if kind == "project"`).
- New host helper `importAttachmentFiles(paths)` or reusing `attachments` / `importImages` allows passing generated images (base64 data URLs or saved files) cleanly into a target project's run as `attachmentIds`.

### Desktop UI (`desktop/src/renderer/src/`)
- **Left Icon Rail (`ProjectRail.tsx`):**
  - Dedicated **«Дизайнер»** tab button (`Palette` icon) activating `w.openDesigner()`.
- **Sidebar (`SidebarChats.tsx`):**
  - When `w.project?.kind === 'designer'`, the sidebar focuses on Designer chats: «Новый дизайн» button, search, and session list (without mixing coding project folders). Regular projects remain accessible when switching back to Projects or Personal Chat.
- **Workspace Layout (`App.tsx`, `Welcome.tsx`, `WorkspaceTopbar.tsx`):**
  - Topbar shows `Дизайнер` (`Designer`).
  - `Welcome` shows designer-oriented starter prompts (website hero, SaaS dashboard, mobile app UI, brand illustration) and calls out `generate_image`.
  - Right Inspector (`files`, `changes`, `browser`, `app`, `agents`) and bottom Developer `WorkbenchDock` are suppressed in `kind === 'designer'` so the workspace stays focused on design conversation and visual previews.
- **«Сделать этот дизайн» Handoff (`features/designer/` + `MessageItem.tsx`):**
  - Assistant messages in Designer chats render a **«Сделать этот дизайн»** action button (in the message footer alongside copy, styled cleanly with Prime tokens).
  - Clicking opens `DesignHandoffDialog` listing regular projects (`kind === 'project'` from open + recent projects, excluding `chat` and `designer`), with search and quick actions to create a new project or open a folder.
  - Selecting a project:
    1. Extracts the message's text specification and any generated/tool images (converted via `window.prime.importImages`).
    2. Opens the target project, creates a new chat titled from the design topic, switches to that project and chat, ensures mode `'general'`, and dispatches the implementation prompt + attached design images so the project agent immediately starts building it.
