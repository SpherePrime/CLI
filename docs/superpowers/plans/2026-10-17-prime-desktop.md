# Prime Desktop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Delegation is optional only when separately authorized.

**Goal:** Законченный Windows Desktop для Prime с сохранением поведения CLI, скрытым backend и установщиком.

**Architecture:** React общается с Electron main через узкий preload. Main запускает встроенный `prime.exe server`, использует HTTP/SSE через named pipe и управляет жизненным циклом. Существующая Go-логика Prime остаётся единственным источником поведения агента и его данных.

**Tech Stack:** Electron, React, TypeScript strict, Vite, Radix, Lucide, Zustand, TanStack Query, Zod, react-markdown, Monaco, Vitest, Testing Library, Playwright Electron, electron-builder/electron-updater; Go существующего CLI.

**Spec:** `../specs/2026-10-17-prime-desktop-design.md`. Читать оба документа. Проектное предложение; реализация в этом запросе не выполнялась.

## Global Constraints

- Windows x64 — обязательная целевая платформа. Windows arm64 и Linux — структура и сборочные профили; считать их поддержанными только после отдельных сборок и проверки. macOS в этот объём не входит.
- Рабочая логика остаётся в Prime: системные инструкции, контекст, модели, reasoning, агент, инструменты, сессии, очередь, разрешения, вопросы, MCP, LSP, Skills и плагины.
- Desktop не пишет напрямую в SQLite и не создаёт вторую БД чатов.
- Комментарии в код не добавлять без прямого запроса пользователя; пояснения в конфигурационных файлах разрешены.
- Не добавлять атрибуцию моделей в код, документацию, коммиты и PR. README писать кратко на английском.
- Следовать действующим AGENTS.md каждого репозитория; пользовательские изменения не перезаписывать. Перед реализацией получить remote state и сравнить его с текущим checkout.
- Это единый объём поставки: завершение отдельной задачи не означает готовность приложения к релизу.

## 1. Границы репозиториев и структура

Desktop root: `C:/Users/dwert/OneDrive/Documents/GitHub/CliDeckstop`.
CLI root: `C:/Users/dwert/OneDrive/Documents/GitHub/CLI`.
Пути ниже относительны к явно указанному root. Документы уже созданы; остальные файлы — будущие.

```text
CliDeckstop/
  package.json
  package-lock.json
  electron-builder.yml
  electron.vite.config.ts
  tsconfig.json
  vitest.config.ts
  playwright.config.ts
  runtime/manifest.json
  scripts/acquire-runtime.mjs
  scripts/verify-runtime.mjs
  src/shared/{contracts,schemas,errors}.ts
  src/main/index.ts
  src/main/runtime/{supervisor,manifest,lifecycle}.ts
  src/main/prime/{transport,sse,client,reconcile}.ts
  src/main/ipc/{register,validate}.ts
  src/main/files/{browser,diff,paths}.ts
  src/main/settings/{store,migrations}.ts
  src/main/updates/updater.ts
  src/preload/index.ts
  src/renderer/app/{App,WorkspaceShell}.tsx
  src/renderer/styles/{tokens,global}.css
  src/renderer/features/projects/
  src/renderer/features/sessions/
  src/renderer/features/chat/
  src/renderer/features/approvals/
  src/renderer/features/models/
  src/renderer/features/resources/
  src/renderer/features/files/
  src/renderer/features/plugins/
  src/renderer/features/settings/
  src/renderer/i18n/{ru,en}.json
  tests/unit/
  tests/integration/
  tests/e2e/
  tests/fixtures/
  docs/parity-matrix.md
  .github/workflows/{ci,release}.yml
```

Не объединять всё в App.tsx или main/index.ts. Компоненты и state каждой функции лежат рядом, общие transport и contracts независимы от React. Подробное наполнение feature-папок указано в задачах.

## 2. Контракт между модулями

Go DTO генерируются/сверяются с закреплённой OpenAPI и реальными SSE fixtures. Ниже — Desktop-контракт, который требуется создать, а не описание существующего SDK.

```ts
type ConnectionState = 'starting' | 'ready' | 'reconnecting' | 'failed'
type RunState = 'accepted' | 'queued' | 'running' | 'completed' | 'cancelled' | 'failed' | 'interrupted'
type RuntimeHandle = { endpoint: string; pid: number; owned: boolean; version: string }
type Scope = 'global' | 'workspace'
type SendInput = { workspaceId: string; sessionId: string; runId: string; prompt: string; attachmentIds: string[] }
type SendReceipt = { runId: string; state: RunState }
type Subscription = { unsubscribe(): void }
type FileEntry = { path: string; name: string; kind: 'file' | 'directory'; size: number }
type DiffView = { source: 'session' | 'git'; path: string; before: string; after: string }
```

Scope и имена полей преобразует main в реальные Go DTO; не посылать Desktop enum в API без адаптера. attachmentIds ссылаются на выбранные main вложения, которые client преобразует в настоящий `proto.Attachment`.

Публичные сервисы main:

```ts
interface RuntimeSupervisor {
  start(): Promise<RuntimeHandle>
  stopWhenIdle(): Promise<void>
}
interface PrimeClient {
  request<T>(method: string, path: string, body?: unknown): Promise<T>
  subscribe(workspaceId: string, clientId: string, onEvent: (event: unknown) => void): Subscription
}
interface DesktopBridge {
  chooseProject(): Promise<{ workspaceId: string; path: string } | null>
  sendMessage(input: SendInput): Promise<SendReceipt>
  cancelSession(workspaceId: string, sessionId: string): Promise<void>
  listFiles(workspaceId: string, relativePath: string): Promise<FileEntry[]>
  getDiff(workspaceId: string, sessionId: string, path: string, source: 'session' | 'git'): Promise<DiffView>
  onEvent(listener: (event: unknown) => void): () => void
}
```

Здесь перечислены ключевые интерфейсы; операции остальных экранов добавляются именованными методами: sessions.list/create/rename/delete, approvals.respond, questions.answer/cancel, models.list/select/setReasoning, resources.list/restart/authenticate, plugins.list/enable/disable/transcribe, settings.get/set и updates.check/install. Их входы и выходы определяются Zod-схемами по API; произвольный request в renderer не экспортируется.

## 3. Правило выполнения и проверок

Для каждой задачи: написать указанный поведенческий тест → выполнить его и зафиксировать ожидаемое падение → реализовать связанный функционал → выполнить scoped tests и typecheck → проверить diff → отдельный коммит. Не писать тесты на само наличие CSS-классов или конфигурационных строк. Проверки ниже являются будущими командами, а не отчётом об уже пройденных тестах.

CLI: `go test ./internal/server ./internal/backend ./internal/client ./internal/proto`, затем затронутые agent/config/plugins пакеты. Go брать по `go.mod` текущей версии, при изучении указан `go 1.27.0`.

Desktop npm scripts обязаны включать `dev`, `typecheck`, `lint`, `test:unit`, `test:integration`, `test:e2e`, `build`, `dist:win`, `runtime:verify`. Unit-команда: `vitest run tests/unit`; integration: `vitest run tests/integration`; e2e: `playwright test`. У e2e отдельный профиль данных и локальный тестовый provider; реальные ключи не нужны.

## Задача 1. Воспроизводимая основа и manifest runtime

**Файлы Desktop:** создать package/config файлы, `src/shared/contracts.ts`, `src/main/runtime/manifest.ts`, `runtime/manifest.json`, scripts runtime, `tests/unit/manifest.test.ts`, `.gitignore`, `README.md`. Документы сохранить. Создать локальный CHATS.md и исключить из Git вместе с локальными logs, runtime binaries, node_modules, dist, out, test-results и секретами.

**Интерфейс:** `readRuntimeManifest(path): Promise<RuntimeManifest>` проверяет platform, arch, version, SHA и protocolVersion; source commit обязателен. Из GitHub получать только закреплённый релиз/commit из manifest.

- [ ] Проверить Git и remote обоих каталогов; Desktop инициализировать как отдельный репозиторий, если он всё ещё пуст; remote не придумывать.
- [ ] Зафиксировать полный каталог CLI-функций в `docs/parity-matrix.md` из workspace interface, command dialog, config schema и plugins. Для каждой строки указать экран и тест из задач ниже.
- [ ] Тестировать отклонение manifest с неверной архитектурой или checksum и отсутствие запуска неподтверждённого exe. Команда: `npm run test:unit -- manifest`.
- [ ] Поднять Electron окно с локальными assets, изоляцией, CSP и пустым shell; добавить build/typecheck/lint и runtime acquisition scripts. Проверить `npm run build`.
- [ ] Проверить сгенерированный пакет на отсутствие секретов и случайного CLI.exe из соседней папки; коммит `chore: initialize desktop shell and pinned runtime`.

## Задача 2. Надёжный контракт Go-сервера

**Файлы CLI:** изменить `internal/server/endpoints.go`, `proto.go`, `events.go`, `internal/backend/agent.go`, `backend.go`, `internal/client/proto.go`, `internal/proto/proto.go`; создать `internal/proto/desktop.go`, `internal/backend/run_registry.go`, `event_journal.go`, `snapshot.go`, `internal/server/desktop.go`, тесты рядом. Snapshot pending requests потребует адаптеров в `internal/permission` и `internal/question` без UI-зависимости.

**Производит:** capabilities, snapshot/cursor/generation, replay, durable runs, идемпотентный send. Точные маршруты и поля — раздел 5 spec. Existing clients должны продолжать работать без обязательных новых полей.

- [ ] Написать интеграционные тесты: одинаковый run_id дважды запускает tool ровно один раз; другой payload даёт 409; поздний reconnect восстанавливает pending question/permission; старый cursor требует snapshot; сервер после рестарта возвращает interrupted для незаконченного run.
- [ ] Выполнить Go scoped tests, убедиться в поведенческом падении новых сценариев, затем добавить registry и атомарное принятие перед исполнением. Использовать существующий слой хранения и миграции, не отдельный файл с незащищёнными записями.
- [ ] Добавить snapshot и event journal под одной согласованной границей синхронизации. Ограничить replay 10 000 событиями / 16 MiB; обработать смену generation и пропавшую сессию.
- [ ] Добавить HTTP/SSE-контракт, OpenAPI, Go-клиент, fixtures из реального test server. Проверить `go test ./internal/server ./internal/backend ./internal/client ./internal/proto` и существующие multiclient/cancellation тесты.
- [ ] Коммит `feat: add resumable desktop protocol and idempotent runs`; публикация CLI-изменений по правилам его AGENTS только после проверок и сравнения remote.

Пример обязательного ожидаемого поведения fixture-driven теста Desktop:

```ts
it('keeps one logical message after snapshot and replay', () => {
  const state = reconcile(snapshotAtCursor12, replayFromCursor10)
  expect(state.messages.map(message => message.id)).toEqual(['user-1', 'assistant-1'])
  expect(state.cursor).toBe(14)
})
```

`snapshotAtCursor12` и `replayFromCursor10` создать в `tests/fixtures/reconnect.ts` из ответов Go test server: сообщения с ID user-1/assistant-1, cursors 11–14 и повторное обновление assistant-1. Это тест дедупликации, а не полное текстовое воспроизведение JSON в плане.

## Задача 3. Скрытый backend, named pipe и supervision

**Файлы Desktop:** `src/main/runtime/{supervisor,lifecycle}.ts`, `src/main/prime/{transport,sse,client}.ts`, `tests/integration/runtime.test.ts`, `tests/unit/sse.test.ts`. **CLI:** `internal/server/net_windows.go` и ACL-тест Windows.

**Потребляет:** manifest и protocol задачи 2. **Производит:** RuntimeSupervisor, PrimeClient и ConnectionState.

- [ ] Проверить split SSE chunks, несколько событий в chunk, UTF-8 на границе буферов, EOF и reconnect. Тест runtime запускает настоящий тестовый prime server и получает capabilities через pipe.
- [ ] Реализовать запуск аргументами, windowsHide, ограничения ACL, 15-секундный health timeout, rotation logs, single-instance lock и owned/attached различие.
- [ ] Реализовать bounded retries, process exit, сон/пробуждение, client retirement и shutdown_if_idle; stdout/stderr всегда дренируются. Не завершать весь tree при закрытии одного окна или чата.
- [ ] Выполнить `npm run test:unit -- sse`, `npm run test:integration -- runtime`; на Windows проверить в Process Explorer отсутствие консольного окна и ownership процессов.
- [ ] Коммит `feat: supervise hidden Prime runtime over local pipes`.

## Задача 4. IPC, согласование состояния и хранилище окна

**Файлы:** `src/shared/{schemas,errors}.ts`, `src/preload/index.ts`, `src/main/ipc/{register,validate}.ts`, `src/main/prime/reconcile.ts`, `src/main/settings/{store,migrations}.ts`; `tests/unit/{ipc,reconcile,settings}.test.ts`.

**Потребляет:** PrimeClient. **Производит:** DesktopBridge, нормализованные workspace/session/message/run maps и события UI. Event ключ включает workspace ID, generation, cursor, entity ID.

- [ ] Тестировать отказ неизвестному IPC-методу, чужому frame, неверному workspace и path; повторное событие, позднее событие другой сессии, сброс generation, malformed event.
- [ ] Создать preload методы и Zod-валидацию на main; отписки обязательны. Sanitized config исключает credentials перед передачей UI.
- [ ] Реализовать snapshot+buffer+replay, обновление по ID/revision, состояние disconnected независимо от run state. При timeout POST запросить runs/{id}; не генерировать новый run ID автоматически.
- [ ] Сохранить черновики по project path/session ID, recent/pinned и положение окна атомарно; миграцию проверить на профиле предыдущей версии. Не хранить полные сообщения второй копией.
- [ ] Выполнить scoped unit tests и typecheck; коммит `feat: add validated desktop bridge and state recovery`.

## Задача 5. Рабочая оболочка, проекты и сессии

**Файлы:** `src/renderer/app/{App,WorkspaceShell}.tsx`, `styles/{tokens,global}.css`, `features/projects/{ProjectSidebar,ProjectPicker,projectStore}.tsx`, `features/sessions/{SessionList,SessionActions}.tsx`, `i18n/{ru,en}.json`, `tests/e2e/projects.spec.ts`. State-модули без JSX сохранить с `.ts` при реализации.

**Потребляет:** DesktopBridge sessions/projects, settings. **Производит:** выбранные workspace/session и устойчивую навигацию.

- [ ] E2E: открыть папку с кириллицей и пробелами, создать два чата, переименовать, переключить, перезапустить приложение и восстановить выбранный чат.
- [ ] Реализовать layout из spec, resize, collapsed panels, dark/light/system, русские строки и palette с поиском; подключить local fonts с лицензиями.
- [ ] Создать first-run discovery, provider setup entry, empty/loading/error states, recent projects, pinned sessions и подтверждение удаления. Удаление сессии и закрытие проекта — разные действия.
- [ ] Проверить window 800×600 и 1440×900, DPI 125/150/200%, keyboard navigation, сохранение размеров.
- [ ] Выполнить `npm run test:e2e -- projects`, build; коммит `feat: add project and session workspace`.

## Задача 6. Полный чат, composer, очередь и инструменты

**Файлы:** `features/chat/{ChatView,MessageList,MessageItem,ToolCallCard,ReasoningBlock,Composer,QueuePanel,RunStatus,TaskTree}.tsx`, `features/chat/{chatStore,messageAdapter}.ts`, `tests/unit/messageAdapter.test.ts`, `tests/e2e/chat.spec.ts`.

**Потребляет:** session/message/run snapshots и events, send/cancel/queue/summarize. **Производит:** полный пользовательский цикл задачи.

- [ ] Написать сценарий: prompt → streaming → tool call → результат → RunComplete; затем очередь второго запроса и отмена первого. Проверять по run ID, а не только по session ID.
- [ ] Отобразить все content parts реального proto.Message, включая attachments, видимый thinking, tools, errors, todos, дочерние сессии и usage. Raw HTML не включать.
- [ ] Реализовать Enter/Shift+Enter/IME, drag-and-drop, paste images, @files, slash/custom/MCP prompts, черновики и disabled/loading Send; прямой shell использует API Prime.
- [ ] Реализовать Stop/двойной Esc, очередь и clear, summarize progress, некрадущий scroll; большие outputs сворачиваются, список виртуализирован.
- [ ] E2E доказывает, что отмена A не останавливает B; 1000 сообщений не ломают ввод. Выполнить unit/e2e по chat; коммит `feat: implement streaming agent chat and task controls`.

## Задача 7. Разрешения, вопросы и безопасные режимы

**Файлы:** `features/approvals/{PermissionCard,QuestionForm,PermissionMode}.tsx`, `features/approvals/requestStore.ts`, `tests/e2e/approvals.spec.ts`.

**Потребляет:** pending snapshot, permission/question events и API ответов. **Производит:** один разрешённый ответ на один request ID без зависших tools.

- [ ] Тестировать allow, allow_session, deny, batch question и отмену; два клиента отвечают на один запрос; запрос восстановлен после disconnect.
- [ ] Карточка показывает именно command/path/diff запрошенного действия; состояние «Уже обработано» при разрешении другим клиентом. Действие после ошибки можно безопасно повторить по тому же request ID.
- [ ] Plan/Coder и YOLO привязать к backend; переключение busy отклоняется без рассинхронизации UI. YOLO имеет явный постоянный индикатор.
- [ ] Проверить focus management, keyboard-only и screen reader name; выполнить approvals e2e; коммит `feat: add agent approvals and structured questions`.

## Задача 8. Провайдеры, модели и все настройки Prime

**CLI файлы:** `internal/server/endpoints.go`, создать `internal/server/model_capabilities.go`, `internal/proto/model_capabilities.go`, общий reasoning-сервис в `internal/config`, адаптировать `internal/cmd/model_reasoning.go` и Go-клиент. **Desktop:** `features/models/{ProviderSetup,ModelPicker,ReasoningPicker}.tsx`, `features/settings/{SettingsDialog,PrimeSettings}.tsx`, `tests/e2e/models.spec.ts`.

**Потребляет:** config scope, provider-key/OAuth, capabilities. **Производит:** validated model selection и настройки без потери существующего primerc.

- [ ] Go-тест сравнивает нормализацию reasoning через CLI и API для off/low/medium/high/xhigh/max и модели без reasoning. Проверить недопустимое значение и сохранение scope.
- [ ] Вынести общую нормализацию, добавить capabilities API и contract tests. Custom base URL и localhost provider проверяются через backend.
- [ ] Реализовать подключение, проверку, редактирование/удаление provider, large/small model, OAuth и refresh. Секреты не возвращать в UI после сохранения.
- [ ] Все применимые настройки из schema и palette внести в parity matrix: permissions lists, context/ignore, compact, language, notifications, channels, commands. Для редких настроек допустима форма «Дополнительно» с backend validation; фиктивные переключатели запрещены.
- [ ] Выполнить Go config/cmd/server tests, models e2e и проверку logs на утечку тестового ключа; коммит `feat: expose model capabilities and desktop settings` в соответствующих репозиториях.

## Задача 9. MCP, LSP, Skills, jobs и каналы

**Desktop файлы:** `features/resources/{ResourcesPanel,McpDetails,McpAuth,LspDiagnostics,SkillsCatalog,JobsPanel}.tsx`, `features/resources/resourceAdapter.ts`, `tests/e2e/resources.spec.ts`. **CLI:** endpoints/proto/backend для jobs и поддержанных MCP channel events, плюс тесты.

**Потребляет:** config API и snapshots; existing MCP/LSP/Skills services. **Производит:** реальные состояния ресурсов и дочерних работ.

- [ ] Поднять fixtures: MCP с tool/resource/prompt и OAuth, LSP diagnostic, один enabled и один disabled skill, background shell job, MCP channel message при поддержке CLI.
- [ ] Реализовать config CRUD через существующий config слой, tools/resources/prompts, auth, refresh, Docker enable/disable, restart systems, diagnostics и skill read.
- [ ] Пробросить недостающие jobs/channel DTO без выдуманного fallback события; snapshots для состояний после reconnect. Источник и причина unavailable видимы.
- [ ] Выполнить resources e2e с процессами, а не только статическими mock objects; остановить fixtures после теста. Коммит `feat: add resource management and background job visibility`.

## Задача 10. Файлы, вложения и сравнение изменений

**Файлы:** `src/main/files/{browser,diff,paths}.ts`, `features/files/{FileTree,FilePreview,DiffViewer,AttachmentPicker}.tsx`, `tests/unit/paths.test.ts`, `tests/integration/diff.test.ts`, `tests/e2e/files.spec.ts`.

**Потребляет:** history Prime, filesystem main, Git binary при наличии. **Производит:** listFiles/getDiff, managed attachment IDs и безопасные previews.

- [ ] Проверить traversal, symlink escape, UNC, другой drive, удалённый файл, binary и большой файл; разрешённый явный внешний attachment отличать от обхода root.
- [ ] Реализовать lazy tree и @-поиск с ignore policy; выбранные attachments валидировать по реальным возможностям модели и proto.Attachment. Preview до 2 MiB, большие файлы открывать явно внешним действием.
- [ ] Реализовать session history diff и Git diff как два подписанных источника. В Git обработать untracked/renamed/deleted и отсутствие Git. Не писать в рабочие файлы ради просмотра.
- [ ] Выполнить files e2e и integration diff на временном Git repo с пользовательским изменением рядом с агентным. Коммит `feat: add file previews attachments and session diffs`.

## Задача 11. Плагины и голосовой ввод

**CLI:** адаптировать `internal/plugins/{plugins,voice,task}.go`; создать `internal/backend/plugins.go`, `internal/server/plugins.go`, `internal/proto/plugins.go`, tests; обновить endpoints/client. **Desktop:** `features/plugins/{PluginManager,PluginCard,VoiceRecorder}.tsx`, `tests/e2e/plugins.spec.ts`.

**Потребляет:** существующая система плагинов Prime, микрофон Electron. **Производит:** установки/toggle/progress и распознавание в черновик.

- [ ] Тестировать включение/выключение, progress, download fail, повтор установки и сохранение enabled state. Voice contract принимает bounded audio upload до 20 MiB/120 секунд и проверяет MIME до передачи существующему engine.
- [ ] Отделить plugin services от TUI messages; переиспользовать install/deactivate и настройки engine/model/language. Конкретный engine определяет допустимый аудиоформат через capability; не обещать универсальную поддержку browser recognition.
- [ ] Реализовать микрофон только после явного действия, Alt+V, Stop/Cancel, timeout, denied permission, отсутствие устройства и editable transcript. При выключении плагина запись прекращается.
- [ ] Выполнить тест с аудиофайлом fixture и provider stub; на Windows вручную проверить реальный микрофон и существующий voice engine. Не объявлять voice готовым по одному fake transcript.
- [ ] Коммит `feat: support Prime plugins and desktop dictation` в соответствующих репозиториях.

## Задача 12. Tray, установка, обновления и диагностика

**Файлы:** `src/main/runtime/lifecycle.ts`, `src/main/updates/updater.ts`, `features/settings/{AboutDialog,DiagnosticsView,UpdateStatus}.tsx`, `electron-builder.yml`, `.github/workflows/{ci,release}.yml`, `tests/e2e/lifecycle.spec.ts`, `docs/release-checklist.md`.

**Потребляет:** runtime ownership, run state, manifest. **Производит:** установщик, update feed и единый версионированный пакет.

- [ ] Тестировать Close → tray при busy, возврат, explicit exit, другой клиент на сервере, renderer crash и backend crash. Проверить отсутствие orphan-процессов после idle exit.
- [ ] Собрать per-user NSIS installer, иконки, shortcuts, uninstall; runtime поставляется resources и проверяется после упаковки. Подпись настраивается CI secret; отсутствие сертификата не маскировать успешной подписанной сборкой.
- [ ] Обновлять Desktop + bundled CLI атомарным пакетом после завершения работ. Внешний CLI не менять. Создавать backup перед DB migration; запрещать несовместимый downgrade.
- [ ] Добавить диагностику версии shell/runtime/protocol, пути проекта и redacted logs; экспорт только по действию пользователя. Notifications ведут в нужную сессию.
- [ ] Выполнить `npm run runtime:verify`, `npm run build`, `npm run dist:win`; проверить установку и обновление на чистой VM. Коммит `feat: package desktop lifecycle and coordinated updates`.

## Задача 13. Сквозная приёмка всего приложения

**Файлы:** `tests/e2e/parity.spec.ts`, `tests/fixtures/provider-server.ts`, `docs/parity-matrix.md`, `docs/release-checklist.md`, README.

**Потребляет:** весь spec. **Производит:** проверяемая готовность одного полного релиза.

- [ ] Fixtures provider обслуживает реальные форматы вызовов Prime: streaming, tool calls, error, cancellation, queue и вложения. Сетевой доступ к платным провайдерам для базовых CI tests не нужен.
- [ ] Пройти цепочку project → provider → session → tool approval → question → file edit → diff → run complete → reopen → update. Сверить состояние с API, а не только с текстом в DOM.
- [ ] Пройти отдельные сценарии из всех строк spec §6, все commands/settings parity matrix. Для каждого иметь automated evidence либо точную ручную процедуру и результат.
- [ ] Выполнить `npm run typecheck`, `npm run lint`, `npm run test:unit`, `npm run test:integration`, `npm run test:e2e`, `npm run build`, `npm run dist:win`; в CLI — scoped tests, `go vet` затронутых пакетов и release build. Не повторять неизменённые успешные проверки без причины.
- [ ] На VM: install/uninstall/reinstall, shared existing Prime data, обновление предыдущей версии, DPI/keyboard, 1000 сообщений, sleep/wake, network offline, failed MCP, missing Git, missing microphone.
- [ ] Обновить краткий английский README, локальную историю CHATS, сверить diff на секреты и запрещённые комментарии/атрибуцию. Проверить последний commit message и состояние remote перед публикацией согласно AGENTS.
- [ ] Релиз готов только при выполнении spec §11. Публикация публичного установщика и подключение update feed выполняются по разрешённому пользователем каналу; сейчас репозиторий Desktop и release endpoint не назначены.

## 4. Зависимости и единая поставка

Порядок: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12 → 13. Это порядок технической реализации, а не отдельные продукты. Обязательные изменения CLI идут в его репозиторий, Desktop — в свой; версии связывает runtime manifest. До готовности 13-й задачи приложение не считается завершённым.

Не входят: новый agent engine, облачные аккаунты/синхронизация, произвольный marketplace, расписания из Harness, встроенный полноценный IDE-редактор, Git write operations и macOS. Эти функции не требуются для сохранения текущей работы Prime и не должны отвлекать от полноценного Desktop.

## 5. Самопроверка плана

| Требование spec | Покрытие |
|---|---|
| Скрытый CLI и private transport | 1–4 |
| Восстановление и отсутствие повторных действий | 2–4, 6–7, 12–13 |
| Интерфейс и навигация | 5–7, 10 |
| Все рабочие функции Prime | 1 parity inventory, 6–11, 13 |
| Общая история и безопасные настройки | 2, 4, 8, 12–13 |
| Плагины и voice | 11 |
| Windows install/update | 12–13 |
| Язык, DPI, клавиатура и длинные чаты | 5–7, 13 |

Исходники при планировании не изменялись. Новые маршруты, схемы и файлы в этом плане — запланированные работы; существующими считаются только явно подтверждённые в spec §2 и §5.
