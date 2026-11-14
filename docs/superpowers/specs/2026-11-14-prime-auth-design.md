# Единая авторизация аккаунта Prime (CLI и Desktop)

Статус: утверждено. Авторизация отвечает только за вход в аккаунт Prime (`https://vpn.sphereprime.ru`); настройка модельных провайдеров остаётся отдельной.

## Цели и ограничения

- Обязательный экран входа при запуске **Desktop** и **CLI** (без кнопки «Пропустить»).
- Единая защищённая сессия для CLI и Desktop: вход в одном приложении автоматически авторизует второе; `prime update` не затрагивает файл сессии.
- Точное соответствие протоколу из `freeAI-server/plugin`:
  - Базовый URL: `https://vpn.sphereprime.ru` (с возможностью переопределения для тестов/отладки через переменную окружения `PRIME_ACCOUNT_URL`).
  - Инициация входа: `hash = "desktop_" + 64 hex`, открытие `https://vpn.sphereprime.ru/auth/google?app_hash=<hash>`.
  - Опрос статуса: `GET /api/auth/desktop-auth/<hash>` каждые 2 с, до 150 попыток (~5 мин), без редиректов (`CheckRedirect`), таймаут запроса 10 с.
  - Проверка сессии: `GET /api/auth/me` с `Authorization: Bearer <token>`.
- Никакого дублирования HTTP-логики или DPAPI в Rust/TS: всё сетевое взаимодействие и хранение токена живёт в Go-пакете `internal/account`.

## Архитектура

### 1. Общий Go-пакет `internal/account`

- `service.go` — `Service`:
  - Хранилище: `<config.GlobalDataDir()>/prime-account.bin` (или переданный путь для тестов). При изолированном запуске Desktop (`PRIME_NATIVE_TEST_DATA`) `PRIME_GLOBAL_DATA` уже указывает на временную папку, поэтому тесты не трогают реальную сессию пользователя.
  - Методы: `State(ctx) State`, `Restore(ctx) State`, `StartLogin(ctx) (State, error)`, `CancelOrLogout(ctx) State`, `CheckNow(ctx) State`.
  - Потокобезопасность и защита от гонок: монотонный `flow uint64` отменяет устаревшие циклы опроса при повторном `StartLogin` или `CancelOrLogout`.
  - Поведение при `Restore` / `CheckNow`:
    - Если файла сессии нет или токен пуст → `signedIn = false`.
    - `GET /api/auth/me` вернул `200` → `signedIn = true`, `email = payload.user.email || payload.email`.
    - `GET /api/auth/me` вернул `401`/`403` → файл сессии удаляется, `signedIn = false`.
    - Сетевая ошибка / таймаут / `5xx` при наличии локального токена → токен не удаляется, выставляется `networkError = true` и `error`, чтобы пользователь мог нажать «Повторить проверку» или «Войти заново».
- `storage_windows.go` / `storage_unix.go`:
  - Windows: Win32 DPAPI (`CryptProtectData` / `CryptUnprotectData`, флаг `CRYPTPROTECT_UI_FORBIDDEN`, scope `CurrentUser`) над JSON `{"token":"..."}` через `vendordeps/x/sys/windows`.
  - Unix: атомарная запись JSON `{"token":"..."}` с правами `0600`.
- `types.go`:
  - `State`: `SignedIn`, `Waiting`, `Checking`, `Email`, `LoginURL`, `Error`, `NetworkError`, `HasStoredSession`.

### 2. Server / Desktop Engine (`internal/server` и `internal/proto`)

- `GET /v1/account` → возвращает текущий `account.State` (сессия синхронизируется с диском, если файл появился/исчез из CLI).
- `POST /v1/account/restore` → запускает повторную проверку сохранённой сессии (`/api/auth/me`).
- `POST /v1/account/login` → запускает Google OAuth (генерирует `app_hash`, возвращает `loginUrl`, пытается открыть системный браузер через `browser.OpenURL` и запускает фоновый опрос).
- `POST /v1/account/logout` → отменяет опрос и удаляет сохранённую сессию.
- Токен никогда не передаётся в JSON-ответах `desktop-engine`.

### 3. Desktop (Rust Tauri + React UI)

- **Мост (`desktop/src-tauri/src/commands.rs`, `desktop/src/shared/hostMethods.json`, `desktop/src/shared/contracts.ts`):**
  - Метод `account(action: 'status' | 'restore' | 'login' | 'logout')` проксируется в `/v1/account*` через `state.require_engine()`.
  - Превью-хост (`installPreviewHost.ts`) поддерживает полный сценарий авторизации для разработки и браузерных проверок (включая `?auth=signed-out` / сброс / вход / выход).
- **Визуальный дизайн (`AuthGate.tsx` + `auth-gate.css`):**
  - Встраивается в существующую систему токенов (`brand.css`, `design/tokens.css`, `glass/tokens.css`, шрифты `Inter` + `JetBrains Mono`, тёмная/светлая тема, локализация `ru`/`en`).
  - Асимметричная двухколоночная сцена под `Titlebar` (на `< 760px` — компактная вертикальная колонка, без горизонтального скролла от `320px`):
    - **Левая панель:** знак Prime, лаконичная типографика и фирменный 3-шаговый индикатор этапа входа (`01 Браузер Google` → `02 Подтверждение` → `03 Сессия активна`).
    - **Правая карточка:** «Войдите, чтобы начать», **единственная кнопка действия «Войти через Google»** (без кнопки «Пропустить»), в процессе ожидания — «Отменить вход» и вспомогательные действия «Открыть ссылку снова» / «Скопировать ссылку», а при сетевом сбое проверки — «Повторить проверку» и «Войти заново».
  - После входа в `WorkspaceView`:
    - Компактный блок аккаунта в `SettingsDialog` (вкладка/карточка аккаунта: email + «Выйти») и аккуратный индикатор профиля в шапке/навигации, нажатие «Выйти» возвращает на `AuthGate`.

### 4. CLI и TUI (`internal/cmd`, `internal/ui`)

- **TUI (`internal/ui/model/ui.go`, `auth_gate.go`, `internal/i18n`):**
  - Новое стартовое состояние `uiAuthGate`. При запуске `prime` проверяется `account.Service.Restore`; если `SignedIn == false`, TUI сразу открывает `uiAuthGate` без кнопки «Пропустить» и не пускает к `uiLanding` / `uiInitialize` / `uiChat`.
  - В карточке `uiAuthGate` (в цветах активной темы TUI): логотип Prime с шиммером, 3-шаговый статус (`Браузер` → `Подтверждение` → `Готово`), горячие клавиши: `Enter` (начать вход / повторить), `Esc` (отменить ожидание), `c` (скопировать ссылку входа в буфер — важно для SSH/headless-терминалов), `o` (открыть ссылку повторно), `Ctrl+C`/`q` (выход из CLI).
  - По завершении входа (или обнаружении входа, выполненного параллельно в Desktop) — мгновенный переход в `uiInitialize` / `uiLanding` и запуск отложенной инициализации.
  - Команда `/logout` и пункт в палитре команд (`Ctrl+P` → «Выйти из аккаунта») выходят из аккаунта и возвращают на `uiAuthGate`.
  - Отображение текущего email в диалоге статуса (`Status`).
- **Команды CLI:**
  - `prime run`: перед запуском проверяет `account.Service.Restore`; при отсутствии авторизации возвращает понятную ошибку с подсказкой выполнить `prime login` или запустить `prime`.
  - `prime login`: без аргументов (или `prime`) выполняет вход в аккаунт Prime (с красивым терминальным потоком и отображением ссылки); платформы `hyper`, `copilot`, `openai` сохраняют прежнее поведение при явном указании.
  - `prime logout`: без аргументов выходит из аккаунта Prime (при указании платформы провайдера — удаляет учётные данные провайдера, как раньше).
