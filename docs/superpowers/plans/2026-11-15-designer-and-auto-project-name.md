# Designer Mode & Auto-Incremented Default Project Name Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add automatic sequential default project naming (`Новый проект 1`, `2`, ...) and a dedicated, strictly isolated Designer mode tab with `generate_image` chats and one-click handoff (**«Сделать этот дизайн»**) to regular projects.

**Architecture:** The Go backend introduces `proto.Workspace.Kind` and a lightweight `AgentDesigner` (`"designer"`) agent locked exclusively to `"designer"` workspaces so it can never be selected or defaulted in regular projects. The Rust Tauri host exposes `projectCreationNextName`, auto-suffixed `createProject`, and `openDesigner`. The React UI adds the Designer rail tab, clean Designer chat view, and `DesignHandoffDialog` that transfers the selected design text and generated images into a chosen regular project chat.

**Tech Stack:** Go (`internal/config`, `internal/agent`, `internal/backend`, `internal/proto`, `internal/server`), Rust/Tauri (`desktop/src-tauri/src/`), React 19 + TypeScript + Vite (`desktop/src/`), Vitest, Go `testing`, Cargo `test`.

**Spec:** `docs/superpowers/specs/2026-11-15-designer-and-auto-project-name-design.md`

## Global Constraints
- Designer agent (`"designer"`) MUST NEVER be selectable or used as default in non-designer workspaces (`project`, `chat`, CLI).
- No AI/model attribution anywhere in code, docs, or commits.
- Keep code modular across focused files; do not add unrequested comments in code.
- Work only on branch `main`.
- All UI strings must be in Russian with English translations in `desktop/src/shared/translations.ts`.

## Review Focus
1. Attempting `POST /v1/workspaces/{id}/agent/main` with `agent_id: "designer"` on a regular project returns an error and leaves `general` active.
2. Creating a project when `Новый проект`, `Новый проект 1` already exist prefills `Новый проект 2` and creates `Новый проект 2` (and still auto-increments if submitted with `"Новый проект"`).
3. Selecting «Сделать этот дизайн» on a Designer message with generated `generate_image` outputs opens only regular projects (`project`, never `chat` or `designer`), attaches the images, switches to the target project's new chat in `general` mode, and starts the run.
4. Designer system prompt contains zero coding/bash/git/review/desktop-testing instructions, and Designer toolset contains only `generate_image`, `question`, `fetch`.
5. Opening/restarting the app on a regular project after using Designer always restores regular project behavior (`general`/`plan`), never Designer mode.

---

### Task 1: Go Backend — `AgentDesigner` & Workspace Kind Isolation

**Files:**
- Create: `internal/agent/templates/designer.md.tpl`
- Create: `internal/agent/designer_test.go`
- Modify: `internal/proto/proto.go`
- Modify: `internal/config/config.go`
- Modify: `internal/agent/prompts.go`
- Modify: `internal/agent/coordinator.go`
- Modify: `internal/agent/agent.go`
- Modify: `internal/backend/backend.go`
- Modify: `internal/backend/agent.go`
- Test: `internal/backend/designer_isolation_test.go`

**Interfaces:**
- `proto.Workspace.Kind string` (`json:"kind,omitempty"`)
- `config.AgentDesigner = "designer"`
- `Backend.SetMainAgent(workspaceID, agentID)` rejects `AgentDesigner` unless `ws.Kind == "designer"`, and rejects non-designer agents when `ws.Kind == "designer"`.

- [ ] Add `Kind string` to `proto.Workspace` and `backend.Workspace`.
- [ ] Add `AgentDesigner = "designer"` in `internal/config/config.go` and configure it in `SetupAgents()` with only `generate_image`, `question`, `fetch` (filtered against `allowedTools`) and empty `AllowedMCP`.
- [ ] Create `internal/agent/templates/designer.md.tpl` and `designerPrompt` in `internal/agent/prompts.go`.
- [ ] Register `AgentDesigner` in `coordinator.go`, keep `c.mainAgent = config.AgentGeneral`, and skip `mcpInstructionsBlock` / `visualCheckBlock` / `reviewPolicyBlock` / empty-`todos` reminder in `agent.go` when `a.agent.ID == config.AgentDesigner`.
- [ ] Update `CreateWorkspace` and `SetMainAgent` in `internal/backend/` to enforce strict workspace kind <-> agent isolation.
- [ ] Add unit tests in `internal/agent/designer_test.go` and `internal/backend/designer_isolation_test.go`.

### Task 2: Rust Tauri Host & Shared Contracts — Auto-Numbered Projects & `openDesigner`

**Files:**
- Modify: `desktop/src-tauri/src/project_creation.rs`
- Modify: `desktop/src-tauri/src/projects.rs`
- Modify: `desktop/src-tauri/src/commands.rs`
- Modify: `desktop/src/shared/contracts.ts`
- Modify: `desktop/src/shared/hostMethods.json`

**Interfaces:**
- `DesktopApi.projectCreationNextName(defaultName: string): Promise<string>`
- `DesktopApi.openDesigner(): Promise<Project>`
- `Project.kind?: 'chat' | 'project' | 'designer'`
- Rust `project_creation::next_available_name(&documents, base_name)` and `create_directory_auto(&documents, name)`.

- [ ] Add `next_available_name` and `create_directory_auto` + unit tests in `project_creation.rs`.
- [ ] Update `projects::workspace_request` to send `"kind": kind` to `/v1/workspaces`, and add `projects::designer_directory`.
- [ ] Wire `projectCreationNextName`, `createProject` (auto-increment), and `openDesigner` in `commands.rs`, `hostMethods.json`, and `contracts.ts`.

### Task 3: Desktop UI — Auto-Name Dialog, Designer Tab & «Сделать этот дизайн» Handoff

**Files:**
- Create: `desktop/src/renderer/src/features/designer/DesignerWelcome.tsx`
- Create: `desktop/src/renderer/src/features/designer/DesignHandoffDialog.tsx`
- Create: `desktop/src/renderer/src/features/designer/designHandoff.ts`
- Create: `desktop/src/renderer/src/features/designer/designer.css`
- Modify: `desktop/src/renderer/src/features/projects/CreateProjectDialog.tsx`
- Modify: `desktop/src/renderer/src/features/projects/ProjectRail.tsx`
- Modify: `desktop/src/renderer/src/features/projects/SidebarChats.tsx`
- Modify: `desktop/src/renderer/src/features/projects/Welcome.tsx`
- Modify: `desktop/src/renderer/src/features/chat/Composer.tsx`
- Modify: `desktop/src/renderer/src/features/chat/MessageItem.tsx`
- Modify: `desktop/src/renderer/src/features/chat/MessageList.tsx`
- Modify: `desktop/src/renderer/src/features/chat/HistoryMessageList.tsx`
- Modify: `desktop/src/renderer/src/app/useWorkspace.ts`
- Modify: `desktop/src/renderer/src/app/App.tsx`
- Modify: `desktop/src/renderer/src/host/installPreviewHost.ts`
- Modify: `desktop/src/renderer/src/host/previewHostData.ts`
- Modify: `desktop/src/shared/translations.ts`
- Create: `desktop/tests/unit/designerHandoff.test.ts`

**Interfaces:**
- `Workspace.openDesigner: () => Promise<void>`
- `Workspace.newDesignerChat: () => Promise<void>`
- `Workspace.implementDesign: (targetProjectPath: string, design: DesignPayload) => Promise<void>`
- `DesignPayload`: `{ title: string; text: string; images: { name: string; mimeType: string; base64: string }[] }`

- [ ] Update `CreateProjectDialog.tsx` to prefill `projectCreationNextName(t('Новый проект'))`.
- [ ] Add `openDesigner`, `newDesignerChat`, `implementDesign` in `useWorkspace.ts`, ensuring non-designer workspaces strictly use `'general' | 'plan'` and never `'designer'`.
- [ ] Add Designer rail tab in `ProjectRail.tsx`, Designer sidebar view in `SidebarChats.tsx`, `DesignerWelcome.tsx`, topbar label, and hide developer Inspector/WorkbenchDock/mode-toggle when `kind === 'designer'`.
- [ ] Add «Сделать этот дизайн» button on assistant messages in Designer mode (`MessageItem.tsx`) opening `DesignHandoffDialog.tsx` (pick existing/recent regular project, create new project, or open folder, then switch and start build run).
- [ ] Update `installPreviewHost.ts` and `translations.ts`, and write unit tests in `desktop/tests/unit/designerHandoff.test.ts`.
