# Implementation Plan: PrimeAI Provider, Prime Key Tab & 5h/Weekly Limits

## Slice 1 (`freeai-prime-key-limits`) — `C:\Users\dwert\OneDrive\Documents\GitHub\freeAI-server`
1. Add migration `server/migrations/058_prime_key_and_tariff_windows.sql`:
   - `tariffs`: `requests_per_5h`, `tokens_per_5h`, `requests_per_week`, `tokens_per_week`, `cache_token_weight_percent` (default `0`).
   - `prime_upstream_config`: `id` PK (`1`), `enabled`, `base_url`, `api_key_encrypted`, `key_hint`, `default_model`, `updated_at`.
   - `plugin_window_usage`: `(client_id, window_kind)` PK, `window_start_ms`, `window_expires_ms`, `requests_used`, `tokens_used`.
2. Update `server/src/config.ts`, `server/src/db/types.ts`, `server/src/db/repository*.ts`:
   - Support `PRIME_UPSTREAM_API_KEY`, `PRIME_UPSTREAM_BASE_URL`, `PRIME_UPSTREAM_ENABLED`.
   - Encrypt/decrypt `prime_upstream_config` via `server/src/crypto/secrets.ts` (`MASTER_KEY`), never exposing raw keys; provide secret redaction helper for error strings.
3. Implement `server/src/providers/prime-key-gateway.ts`:
   - `fetchPrimeKeyModels`, `streamPrimeKeyCompletion` (OpenAI SSE -> `RouterEvent` including `cachedInputTokens`), `testPrimeKeyModel`.
4. Implement `server/src/routing/tariff-window-limit.ts` & `server/src/auth/roxy-tariff.ts`:
   - RoxyVPN plan resolution by email via `ROXY_API_URL` + `ROXY_API_TOKEN` with 60s cache and `free` fallback.
   - 5h & 7d sliding/first-request window limiter in Redis + Postgres persistence with `cache_token_weight_percent` calculation.
5. Wire `/api/admin/prime-key*` (`server/src/api/admin/prime-key.ts`), update `/api/admin/tariffs` (`server/src/api/admin/tariffs.ts`), update `/api/plugin/*` (`server/src/api/plugin.ts`: `GET /api/plugin/limits`, Prime Key routing, 5h/7d checks, SSE `limits` event), and add Vitest tests (`server/tests/`).
6. Update Admin SPA (`server/web/src/`):
   - Add `"Ключ от Prime"` (`prime-key`) tab in `navigation.ts` + `PrimeKeySettings.tsx` + `api.ts`.
   - Add 5h, weekly, and `Учёт кэша (%)` inputs to `Tariffs.tsx`.

## Slice 2 (`cli-primeai-provider-limits`) — `C:\Users\dwert\OneDrive\Documents\GitHub\CLI`
1. `internal/account`: add `Token() string` (Go-only, excluded from `State` JSON).
2. `internal/agent/primeai/`:
   - `provider.json` (embedded fallback), `types.go`, `session.go`, `limits.go`, `client.go`, `sse.go`, `provider.go`, `provider_test.go`.
   - Implement `fantasy.Provider` & `fantasy.LanguageModel` for `/api/plugin/chat/completions`, dynamic `FetchModels`, `FetchLimits`, and `__primeai_auto__`.
3. `internal/config/` & `internal/agent/coordinator.go`:
   - Register `primeai` syncer in `provider.go` & `load.go` using `account.Service` so `PrimeAI` is active out of the box when signed in.
   - Wire `primeai` into `coordinator.go` (`buildProvider`, 401 refresh, limit refresh callback).
4. `internal/proto/account.go` & `internal/server/account.go` & `endpoints.go`:
   - Add `PrimeAILimits` / `LimitWindow` types and `GET /v1/account/limits` + `POST /v1/account/limits/refresh`.
5. `internal/ui/`:
   - Show 5h & weekly PrimeAI limits in `sidebar.go` and `dialog/status.go`, plus `internal/i18n/catalogs.go` (`ru` + `en`).

## Slice 3 (`desktop-primeai-limits`) — `C:\Users\dwert\OneDrive\Documents\GitHub\CLI\desktop`
1. `desktop/src-tauri/src/commands.rs`, `desktop/src/shared/contracts.ts`, `translations.ts`, `installPreviewHost.ts`:
   - Support `account('limits' | 'refreshLimits')` -> `/v1/account/limits` & `/v1/account/limits/refresh`.
2. `desktop/src/renderer/src/features/auth/PrimeLimitsWidget.tsx` & `AccountCard.tsx` & `WorkspaceTopbar.tsx` & `auth-gate.css`:
   - Add compact topbar limits pill + popover (`5ч`, `Неделя`, reset countdown, plan name, refresh) and detailed limits panel in `AccountCard`.
   - Add unit tests in `desktop/tests/unit/authGate.test.tsx`.
