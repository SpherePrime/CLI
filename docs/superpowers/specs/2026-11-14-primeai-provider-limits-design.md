# PrimeAI Provider, Prime Key Tab & 5h/Weekly Tariff Limits

## Goal

Add out-of-the-box `PrimeAI` provider support to `CLI` and `Desktop` using the authenticated Prime Account session, and upgrade `freeAI-server` with a secure **Prime Key** ("Ключ от Prime") routing tab, RoxyVPN user/tariff resolution, and configurable 5-hour / weekly tariff limits with configurable cache-token weighting.

## 1. `freeAI-server` Architecture

### 1.1 Prime Key ("Ключ от Prime")
- **Storage & Env**:
  - Supports environment variables (`PRIME_UPSTREAM_API_KEY`, `PRIME_UPSTREAM_BASE_URL`, `PRIME_UPSTREAM_ENABLED`) and encrypted database persistence (`prime_upstream_config` table, AES-256-GCM encrypted with `MASTER_KEY`).
  - Secret policy: plaintext API keys are NEVER returned in any HTTP response, admin GET endpoint, error message, request log, or event log. Admin endpoints only return `{ enabled, configured, source: "env" | "db" | "none", maskedKey, baseUrl, defaultModel, updatedAt }`.
- **Routing**:
  - When Prime Key is enabled and configured, `/api/plugin/models`, `/api/plugin/chat/completions`, and `/api/plugin/test` route directly through the configured OpenAI-compatible gateway (`baseUrl`, e.g., `https://185-246-67-53.sslip.io/v1`) with `Authorization: Bearer <primeKey>`.
  - OpenAI SSE stream (`choices[0].delta.content`, `reasoning_content`, `<think>` tags, `tool_calls`, `usage.prompt_tokens_details.cached_tokens`) is translated to plugin `RouterEvent` (`route`, `text`, `thinking`, `tool_call`, `usage`, `limits`, `done`, `error`).
- **Admin Tab**:
  - Separate navigation item `"Ключ от Prime"` (`prime-key`) in `server/web` with `GET /api/admin/prime-key`, `PUT /api/admin/prime-key`, `DELETE /api/admin/prime-key`, and `POST /api/admin/prime-key/test`.

### 1.2 RoxyVPN User & Tariff Resolution
- On `/api/plugin/session`, `/api/plugin/limits`, and `/api/plugin/chat/completions`, user email from the verified Prime/VPN session is matched against RoxyVPN users (`ROXY_API_URL` + `ROXY_API_TOKEN`, cached with 60s TTL) to read `planName` and `subscriptionExpiresAt`.
- If `ROXY_API_URL` is absent or the user has no active subscription, falls back to `client_users.tariff_id` by email or the `free` tariff.

### 1.3 5-Hour & Weekly Tariff Limits with Cache Weighting
- **Tariff Schema (`tariffs` table & `Tariffs.tsx`)**:
  - `requests_per_5h` (integer, 0 = unlimited)
  - `tokens_per_5h` (bigint, 0 = unlimited)
  - `requests_per_week` (integer, 0 = unlimited)
  - `tokens_per_week` (bigint, 0 = unlimited)
  - `cache_token_weight_percent` (integer 0..100, default `0`)
- **Effective Token Formula**:
  - `uncached = max(0, inputTokens - cachedInputTokens)`
  - `effective = uncached + Math.round(cachedInputTokens * (cacheWeightPercent / 100)) + outputTokens`
- **Window Enforcement**:
  - First-request-anchored 5h (`18_000_000` ms) and 7-day (`604_800_000` ms) windows tracked atomically in Redis and persisted in `plugin_window_usage`.
  - Pre-request check rejects with `429 PLUGIN_LIMIT_EXCEEDED`, `window: "5h" | "7d"`, `resetInSeconds`, and full `limits` snapshot.
  - Post-completion emits SSE `event: limits` before `done`.
- **Client Limits Endpoint**:
  - `GET /api/plugin/limits` returns `{ planName, cacheWeightPercent, fiveHour, weekly }` where each window has `{ usedTokens, limitTokens, usedRequests, limitRequests, percent, resetAt, resetInSeconds }`.

## 2. `CLI` Built-in `PrimeAI` Provider & Account Limits
- `internal/account`: exposes internal `Token()` and `Refresh(ctx)` for Go callers (never serialized in `account.State`).
- `internal/agent/primeai`: implements `SessionManager` (`POST <PRIMEAI_URL>/api/plugin/session`), `FetchModels` (`GET /api/plugin/models` + `__primeai_auto__`), `FetchLimits` (`GET /api/plugin/limits`), and `fantasy.Provider` / `fantasy.LanguageModel` over custom SSE `/api/plugin/chat/completions`.
- `internal/config`: registers `primeai` in `Providers()` and enables it out of the box whenever Prime Account is signed in (survives `prime update`).
- `internal/server` & `internal/proto`: exposes `GET /v1/account/limits` and `POST /v1/account/limits/refresh`.
- CLI TUI: displays 5h & weekly PrimeAI limits in sidebar (`internal/ui/model/sidebar.go`) and `/status` (`internal/ui/dialog/status.go`), refreshing after agent runs.

## 3. `Desktop` Limits UI
- `desktop/src-tauri/src/commands.rs`, `desktop/src/shared/*`: adds `"limits"` action (or `account('limits' | 'refreshLimits')` / `limits` endpoint) bridged to `/v1/account/limits`.
- `WorkspaceTopbar`: compact PrimeAI 5h/weekly usage pill with status color (`normal`, `warn >= 75%`, `critical >= 90%`) and interactive popover showing both progress bars, reset countdowns, plan name, and manual refresh.
- `AccountCard`: shows expanded PrimeAI tariff and 5h/weekly quota bars inside Settings -> Account.
