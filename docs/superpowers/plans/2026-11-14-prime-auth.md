# Prime Unified Account Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add mandatory Google browser OAuth login (`https://vpn.sphereprime.ru`) with shared encrypted session storage across both CLI (`prime`, `prime run`, `prime login`, `prime logout`) and Desktop (Tauri + React `AuthGate` + account settings).

**Architecture:** A single Go package `internal/account` owns the OAuth protocol, background polling, and DPAPI/`0600` encrypted file storage in `<GlobalDataDir>/prime-account.bin`. Both CLI/TUI and `internal/server` (`/v1/account*` via `desktop-engine`) consume `internal/account`, while Desktop's Rust host bridges `window.prime.account(...)` directly to `desktop-engine` without altering Tauri's network CSP.

**Tech Stack:** Go 1.25+, Bubble Tea v2 / Lipgloss v2, Rust (Tauri 2), React 19 + TypeScript + Vite + CSS design tokens.

**Spec:** `docs/superpowers/specs/2026-11-14-prime-auth-design.md`

## Global Constraints

- Base auth site URL defaults to `https://vpn.sphereprime.ru`, overridable via `PRIME_ACCOUNT_URL` for tests/dev.
- Login is mandatory on startup in both CLI TUI and Desktop UI: no "Skip" / guest button anywhere.
- Session file `<config.GlobalDataDir()>/prime-account.bin` is shared by CLI and Desktop and survives `prime update`.
- Windows storage uses Win32 DPAPI (`CurrentUser`, `CRYPTPROTECT_UI_FORBIDDEN`); Unix uses `0600` atomic writes.
- No AI/model attribution anywhere (`AGENTS.md`); Russian is the primary UI source language with English translations in `desktop/src/shared/translations.ts` and `internal/i18n`.
- Work only in `main` branch; split code cleanly across focused files; no comments unless genuinely needed.

## Review Focus

- Race between concurrent `StartLogin` / `CancelOrLogout` / `Restore` calls or cross-process disk writes from CLI and Desktop.
- Offline / `5xx` error during startup `Restore` must preserve the stored token (`hasStoredSession: true`, `networkError: true`) instead of deleting it, whereas `401`/`403` must delete the token.
- `desktop-engine` JSON responses must never expose the raw bearer token.
- Desktop `AuthGate` must work seamlessly at `320px`, `390px`, `768px`, `1280px`, `1440px` in both dark and light themes with zero overflow/overlap and full keyboard accessibility.
- CLI `uiAuthGate` must block chat/landing/init until authenticated, support copy-URL fallback (`c`) for headless/SSH terminals, and transition cleanly upon login (including login completed in another window).

---

### Task 1: Core `internal/account` & Server `/v1/account` Endpoints

**Files:**
- Create: `internal/account/types.go`
- Create: `internal/account/storage_windows.go`
- Create: `internal/account/storage_unix.go`
- Create: `internal/account/service.go`
- Create: `internal/account/service_test.go`
- Create: `internal/proto/account.go`
- Create: `internal/server/account.go`
- Create: `internal/server/account_test.go`
- Modify: `internal/server/endpoints.go`
- Modify: `internal/server/server.go` (attach shared `*account.Service` to `server` / `controllerV1`)

**Interfaces:**
- Produces:
  - `account.State` (`SignedIn`, `Waiting`, `Checking`, `Email`, `LoginURL`, `Error`, `NetworkError`, `HasStoredSession` with JSON tags `signedIn`, `waiting`, `checking`, `email`, `loginUrl`, `error`, `networkError`, `hasStoredSession`).
  - `account.NewService(opts ...Option) *Service` (`WithStoragePath(path string)`, `WithBaseURL(url string)`, `WithOpenBrowser(fn func(string) error)`, `WithHTTPClient(c *http.Client)`, `WithPollInterval(d time.Duration)`, `WithMaxAttempts(n int)`).
  - `(s *Service) Restore(ctx context.Context) State`
  - `(s *Service) State(ctx context.Context) State`
  - `(s *Service) StartLogin(ctx context.Context) (State, error)`
  - `(s *Service) CancelOrLogout(ctx context.Context) State`
  - `(s *Service) DefaultPath() string` -> `filepath.Join(config.GlobalDataDir(), "prime-account.bin")`
  - Server endpoints: `GET /v1/account`, `POST /v1/account/restore`, `POST /v1/account/login`, `POST /v1/account/logout` returning `proto.AccountState`.

- [ ] **Step 1:** Write failing unit tests in `internal/account/service_test.go` covering:
  - `Restore` with missing file -> `SignedIn: false`
  - `StartLogin` -> opens URL `${base}/auth/google?app_hash=desktop_<64hex>`, polls `/api/auth/desktop-auth/<hash>`, saves encrypted token on `status: "success"`, sets `SignedIn: true` and `Email`
  - `Restore` with valid token -> `GET /api/auth/me` with `Bearer` -> `SignedIn: true`, `Email`
  - `Restore` with `401` -> deletes token file, `SignedIn: false`
  - `Restore` with `503`/connection error -> preserves token file, `HasStoredSession: true`, `NetworkError: true`
  - `CancelOrLogout` cancels active poll and deletes token file
  - Cross-instance disk sync: instance B sees token saved by instance A when `State(ctx)` is called.
- [ ] **Step 2:** Implement `internal/account/types.go`, `storage_windows.go`, `storage_unix.go`, `service.go` and run `go test -race ./internal/account/...`.
- [ ] **Step 3:** Add `internal/proto/account.go`, `internal/server/account.go`, register endpoints in `internal/server/endpoints.go`, wire `*account.Service` in `internal/server/server.go`, add `internal/server/account_test.go`, and run `go test -race ./internal/server/...`.

---

### Task 2: CLI & TUI Mandatory Auth Gate + Commands

**Files:**
- Create: `internal/ui/model/auth_gate.go`
- Create: `internal/ui/model/auth_gate_test.go`
- Modify: `internal/ui/model/ui.go` (add `uiAuthGate`, check `accountService` on startup, handle `authStateMsg`, block entering chat/landing/init while signed out, handle `/logout` and status email)
- Modify: `internal/ui/dialog/commands.go` & `internal/ui/dialog/status.go` & `internal/ui/dialog/actions.go` (show account email in Status; add `Выйти из аккаунта` / `Log out` action)
- Modify: `internal/i18n/catalogs.go` (EN + RU keys for auth gate, login, logout, errors, hints)
- Modify: `internal/cmd/login.go` (default `prime login` / `prime` platform uses `internal/account` Google OAuth flow; keep `hyper`/`copilot`/`openai`)
- Modify: `internal/cmd/logout.go` (default `prime logout` logs out of Prime account; keep provider logout when platform arg given)
- Modify: `internal/cmd/run.go` (require active Prime account session before running non-interactive agent)

**Interfaces:**
- Consumes: `internal/account` (`Service`, `State`, `Restore`, `StartLogin`, `CancelOrLogout`).

- [ ] **Step 1:** Write failing tests in `internal/ui/model/auth_gate_test.go` and `internal/cmd/` verifying `uiAuthGate` starts active when signed out, has no skip action, transitions to `uiLanding`/`uiInitialize` on `SignedIn: true`, and `prime run` rejects unauthenticated invocation.
- [ ] **Step 2:** Implement `internal/ui/model/auth_gate.go` with themed card, Prime logo + shimmer, 3-step live progress indicator, `Enter`/`Esc`/`c` (copy URL)/`o` (re-open)/`q`, and disk-sync polling tick.
- [ ] **Step 3:** Wire `uiAuthGate`, `/logout`, `commands.go`, `status.go`, `i18n/catalogs.go`, `cmd/login.go`, `cmd/logout.go`, `cmd/run.go`.
- [ ] **Step 4:** Run `go test -race ./internal/ui/... ./internal/cmd/... ./internal/i18n/...`.

---

### Task 3: Desktop Host Bridge + `AuthGate` & Account Settings UI

**Files:**
- Modify: `desktop/src/shared/hostMethods.json` (add `"account": true`)
- Modify: `desktop/src/shared/contracts.ts` (add `AccountState`, `AccountAction`, `DesktopApi.account`)
- Modify: `desktop/src/shared/translations.ts` (EN translations for all Russian auth strings)
- Modify: `desktop/src-tauri/src/commands.rs` (handle `"account"` -> `/v1/account*` via `state.require_engine()`)
- Modify: `desktop/src/renderer/src/host/installPreviewHost.ts` (full preview host `account` simulation + `?auth=signed-out` query support for testing)
- Create: `desktop/src/renderer/src/features/auth/AuthGate.tsx`
- Create: `desktop/src/renderer/src/features/auth/auth-gate.css`
- Create: `desktop/src/renderer/src/features/auth/AccountCard.tsx`
- Modify: `desktop/src/renderer/src/main.tsx` (import `auth-gate.css`)
- Modify: `desktop/src/renderer/src/app/useWorkspace.ts` (manage `account` state, restore on engine ready, poll while waiting/unauthenticated, expose `loginAccount`, `logoutAccount`, `retryAccountRestore`)
- Modify: `desktop/src/renderer/src/app/App.tsx` (render `<AuthGate>` when runtime is ready and `!w.account.signedIn`, before `<WorkspaceView>`)
- Modify: `desktop/src/renderer/src/app/WorkspaceTopbar.tsx` & `desktop/src/renderer/src/features/settings/SettingsDialog.tsx` (compact account badge + Settings account tab/section with email and logout button)
- Create: `desktop/tests/unit/authGate.test.tsx`

**Interfaces:**
- Consumes: `GET /v1/account`, `POST /v1/account/restore`, `POST /v1/account/login`, `POST /v1/account/logout`.

- [ ] **Step 1:** Update `hostMethods.json`, `contracts.ts`, `translations.ts`, `commands.rs`, `installPreviewHost.ts`.
- [ ] **Step 2:** Build `AuthGate.tsx`, `auth-gate.css`, `AccountCard.tsx` using Prime design tokens, 3-step live flow tracker, single primary CTA ("Войти через Google"), waiting/retry/copy-link states, full light/dark and 320px–1920px responsive support, `prefers-reduced-motion` support.
- [ ] **Step 3:** Wire into `useWorkspace.ts`, `App.tsx`, `WorkspaceTopbar.tsx`, `SettingsDialog.tsx`.
- [ ] **Step 4:** Add unit tests in `desktop/tests/unit/authGate.test.tsx` and run `npm run typecheck`, `npm run test:unit`, and `cargo check` in `desktop/src-tauri`.

---

### Task 4: Full Verification, Visual Gate, Build & Git Push

- [ ] **Step 1:** Run full Go test suite `go test -race -failfast ./internal/account/... ./internal/server/... ./internal/cmd/... ./internal/ui/... ./internal/i18n/...` and build `prime.exe` + `desktop-engine` (`node desktop/scripts/build-desktop-engine.mjs`).
- [ ] **Step 2:** Run Desktop preview server (`npm run ui:preview`), run `prime-designer` Playwright layout audit (`audit.mjs` or `browser_run` across 320/390/768/1280/1440 in dark & light themes), inspect screenshots (overview + close-ups), test all states (signed-out, waiting, error, signed-in, settings logout).
- [ ] **Step 3:** Update `CHATS.md`, check `git status` and `git log` (verify zero AI attribution), commit and push to `main`.
