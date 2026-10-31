# План реализации: тестирование приложений (этап A — ядро, Windows, UI Automation)

**Goal:** агент умеет привязаться к окну любого приложения, получить текстовое дерево интерфейса, нажимать, вводить и переключать элементы, а пользователь — выбрать элемент в приложении и отправить его в чат.

**Architecture:** новый домен Rust-хоста `desktop/src-tauri/src/appui/` с единым контрактом UI-графа и провайдерами; на этапе A реализуется провайдер Windows UI Automation, работающий на отдельном COM-потоке с таймаутами. Разрешения, отзыв доступа, проверка identity окна и аварийная остановка переиспользуются из существующего `computer/*`. Инструменты агента публикуются как MCP-инструменты Tauri-хоста (префикс `mcp_browser_`), UI — отдельная панель в доке.

**Tech Stack:** Rust (Tauri 2, `windows` 0.62.2, UI Automation), React 19 + TypeScript, Go (описание разрешений).

**Spec:** `docs/superpowers/specs/2026-11-01-universal-app-testing-design.md`

## Global Constraints

- Windows-only код — под `#[cfg(windows)]`; вне Windows явная ошибка, как в `computer/mod.rs`.
- Схемы инструментов: `additionalProperties: false`; никогда не принимают `workspaceId`/`root`.
- Новый хост-метод = запись в `desktop/src/shared/hostMethods.json` + литеральный arm в `commands.rs` + тип в `DesktopApi` (`desktop/src/shared/contracts.ts`).
- Ошибки Rust-хоста — английские; UI-строки — русские через `t()` или локальный `copy.ts` (ru/en).
- Код делить по файлам, без комментариев в коде.
- Коммит и push в `main` после каждой задачи; без атрибуции ИИ в коммитах.
- Лимиты: 4000 узлов, глубина 40, таймаут снапшота 20 с, действия 15 с, бюджет 200 действий на сессию.

## Review Focus

1. Пустое UIA-дерево (игра, canvas, приложение без accessibility) → понятная диагностика, а не пустой успех. Тест — задача 4.
2. Окно с повышенными правами (UAC) → явная ошибка, а не падение. Тест — задача 6.
3. Зависшее приложение → таймаут, хост-поток не блокируется. Тест — задача 4.
4. Неоднозначный локатор (две кнопки с одинаковым именем) → ошибка со списком кандидатов, без случайного клика. Тест — задача 2.
5. Окно закрылось или пересоздано между снапшотом и действием → отказ, а не действие в другое окно. Тесты — задачи 5 и 6.

---

## Задача 1. Типы UI-графа и текстовый снапшот

**Files:**
- Create: `desktop/src-tauri/src/appui/mod.rs`
- Create: `desktop/src-tauri/src/appui/types.rs`
- Create: `desktop/src-tauri/src/appui/render.rs`
- Modify: `desktop/src-tauri/src/lib.rs` (добавить `pub mod appui;`)

**Interfaces:**
- Produces: `UiBounds`, `UiNode`, `UiSnapshot`, `Confidence`, `Provider`, `Locator`; `render::snapshot_text(&UiSnapshot) -> String`.

```rust
pub struct UiBounds { pub x: i32, pub y: i32, pub width: i32, pub height: i32 }

pub struct UiNode {
    pub reference: String,          // serde rename "ref"
    pub parent: Option<String>,
    pub role: String,               // нормализованная роль
    pub name: String,
    pub value: Option<String>,
    pub bounds: UiBounds,
    pub states: Vec<String>,        // enabled, focused, checked, offscreen, password
    pub actions: Vec<String>,       // invoke, setValue, toggle, select, expand, scroll
    pub automation_id: Option<String>,
    pub class_name: Option<String>,
    pub path: String,               // "window/group/button[Save]"
    pub children: Vec<UiNode>,
}

pub struct UiSnapshot {
    pub provider: Provider,
    pub hwnd: String,
    pub title: String,
    pub captured_at: u64,
    pub truncated: bool,
    pub warnings: Vec<String>,
    pub nodes: Vec<UiNode>,
    pub count: usize,
}
```

- [ ] **Step 1: Написать падающий тест формата текста**

```rust
#[test]
fn renders_nodes_with_role_name_states_bounds_and_actions() {
    let snapshot = fixture_snapshot();
    let text = snapshot_text(&snapshot);
    assert!(text.contains("n3 button \"Save\" [enabled] (120,300,50,50) actions=invoke"));
    assert!(text.contains("n4 edit \"Name\" [enabled] value=\"Иван\""));
}
```

- [ ] **Step 2: Убедиться, что тест не компилируется**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::render`
Expected: ошибка компиляции, `snapshot_text` не найдена.

- [ ] **Step 3: Реализовать типы и `snapshot_text`**

Формат строки: `{ref} {role} "{name}" [{states}] ({x},{y},{w},{h})` + `value="..."` + `actions=...`; отступ — два пробела на уровень вложенности; заголовок — `{title} ({provider}, {count} nodes)`, при `truncated` добавляется `truncated`.

- [ ] **Step 4: Прогнать тест**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::render`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add desktop/src-tauri/src/appui desktop/src-tauri/src/lib.rs
git commit -m "feat: add the app UI graph model and text snapshot"
```

---

## Задача 2. Локаторы элементов

**Files:**
- Create: `desktop/src-tauri/src/appui/locator.rs`

**Interfaces:**
- Consumes: `UiSnapshot`, `UiNode`, `Locator` (задача 1).
- Produces: `resolve(snapshot: &UiSnapshot, locator: &Locator) -> Result<String, String>` — возвращает `ref`.

Порядок разрешения: `reference` → `automationId` → `path` → `role`+`name` → `name`. Сравнение имени — без учёта регистра и пробелов по краям. Неоднозначность → ошибка со списком кандидатов. Устаревший `reference` → `Stale reference; take a new snapshot`. Пустой локатор → `Provide reference, automationId, path, role or name`.

- [ ] **Step 1: Написать падающие тесты**

```rust
#[test]
fn ambiguous_name_lists_candidates_instead_of_choosing_one() {
    let snapshot = snapshot_with_two_save_buttons();
    let error = resolve(&snapshot, &Locator { name: Some("Save".into()), ..Default::default() }).unwrap_err();
    assert!(error.contains("n3") && error.contains("n7"));
}

#[test]
fn stale_reference_is_rejected() {
    let snapshot = snapshot_with_two_save_buttons();
    assert!(resolve(&snapshot, &Locator { reference: Some("n99".into()), ..Default::default() }).is_err());
}

#[test]
fn empty_locator_is_rejected() {
    assert!(resolve(&snapshot_with_two_save_buttons(), &Locator::default()).is_err());
}
```

- [ ] **Step 2: Прогнать тесты — должны упасть**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::locator`
Expected: FAIL / ошибка компиляции.

- [ ] **Step 3: Реализовать `resolve`**

- [ ] **Step 4: Прогнать тесты**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::locator`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add desktop/src-tauri/src/appui/locator.rs
git commit -m "feat: resolve app elements by stable locators"
```

---

## Задача 3. Тестовое окно-фикстура

**Files:**
- Create: `desktop/src-tauri/src/appui/testkit.rs` (`#[cfg(test)]`)

**Interfaces:**
- Produces: `testkit::window() -> FixtureWindow` с `hwnd() -> String`, `set_label(&self, text: &str)`, `label(&self) -> String`, `checkbox_state(&self) -> bool`.

По образцу `computer/windows.rs:156`: `CreateWindowExW` в отдельном потоке с циклом сообщений (`GetMessageW`), дочерние контролы `BUTTON "Save"`, `EDIT`, `CHECKBOX "Enabled"`, `STATIC "Ready"`. `Drop` → `DestroyWindow` + `PostThreadMessageW(WM_QUIT)`.

- [ ] **Step 1: Написать падающий тест**

```rust
#[test]
fn fixture_window_exposes_standard_controls() {
    let fixture = testkit::window();
    assert!(unsafe { IsWindow(handle(&fixture.hwnd()).unwrap()) }.as_bool());
    assert_eq!(fixture.label(), "Ready");
}
```

- [ ] **Step 2: Прогнать тест — должен упасть**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::testkit`
Expected: FAIL / ошибка компиляции.

- [ ] **Step 3: Реализовать фикстуру**

- [ ] **Step 4: Прогнать тест**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::testkit`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add desktop/src-tauri/src/appui/testkit.rs
git commit -m "test: add a Win32 fixture window for app UI tests"
```

---

## Задача 4. Провайдер UI Automation: снапшот дерева

**Files:**
- Create: `desktop/src-tauri/src/appui/uia/mod.rs`
- Create: `desktop/src-tauri/src/appui/uia/worker.rs`
- Create: `desktop/src-tauri/src/appui/uia/tree.rs`
- Create: `desktop/src-tauri/src/appui/uia/roles.rs`
- Modify: `desktop/src-tauri/Cargo.toml` (features `Win32_UI_Accessibility`, `Win32_System_Com`, `Win32_System_Variant`)

**Interfaces:**
- Consumes: типы задачи 1, фикстура задачи 3.
- Produces: `UiaWorker::start() -> Result<UiaWorker, String>`, `UiaWorker::call(&self, UiaRequest, Duration) -> Result<Value, String>`, `UiaRequest::Snapshot { hwnd, max_nodes, max_depth }`, `tree::snapshot(&IUIAutomation, HWND, usize, usize) -> Result<UiSnapshot, String>`, `roles::role(UIA_CONTROLTYPE_ID) -> &'static str`.

Воркер: отдельный поток, `CoInitializeEx(None, COINIT_MULTITHREADED)`, `CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER)`, приём заданий через `mpsc::Receiver`, ответ через канал задания. `call` использует `recv_timeout`; на таймауте возвращает `UI Automation did not respond within Ns` и помечает воркер сломанным, чтобы следующий вызов создал новый поток.

Обход: `ElementFromHandle(hwnd)` → BFS через `FindAll(TreeScope_Children, CreateTrueCondition())`. Свойства: `CurrentName`, `CurrentControlType`, `CurrentAutomationId`, `CurrentClassName`, `CurrentBoundingRectangle`, `CurrentIsEnabled`, `CurrentIsOffscreen`, `CurrentHasKeyboardFocus`, `CurrentIsPassword`. Значение — `GetCurrentPatternAs::<IUIAutomationValuePattern>(UIA_ValuePatternId)`; для паролей не читается. Действия — по наличию паттернов `UIA_InvokePatternId`, `UIA_ValuePatternId`, `UIA_TogglePatternId`, `UIA_SelectionItemPatternId`, `UIA_ExpandCollapsePatternId`, `UIA_ScrollPatternId`.

- [ ] **Step 1: Написать падающие тесты**

```rust
#[test]
fn snapshot_contains_fixture_controls_with_roles_and_actions() {
    let fixture = testkit::window();
    let worker = UiaWorker::start().unwrap();
    let value = worker.call(UiaRequest::Snapshot { hwnd: fixture.hwnd(), max_nodes: 4000, max_depth: 40 }, Duration::from_secs(20)).unwrap();
    let text = value["text"].as_str().unwrap();
    assert!(text.contains("button \"Save\""));
    assert!(text.contains("checkbox \"Enabled\""));
    assert!(text.contains("actions=invoke"));
}

#[test]
fn empty_tree_reports_a_warning_instead_of_success() {
    let snapshot = tree::snapshot(&automation(), invisible_window(), 4000, 40).unwrap();
    assert!(snapshot.count <= 1);
    assert!(!snapshot.warnings.is_empty());
}
```

- [ ] **Step 2: Прогнать тесты — должны упасть**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::uia`
Expected: FAIL / ошибка компиляции.

- [ ] **Step 3: Реализовать воркер, обход и маппинг ролей**

- [ ] **Step 4: Прогнать тесты**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::uia`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add desktop/src-tauri/src/appui/uia desktop/src-tauri/Cargo.toml
git commit -m "feat: read the UI Automation tree of any Windows application"
```

---

## Задача 5. Действия по элементу и скриншот элемента

**Files:**
- Create: `desktop/src-tauri/src/appui/uia/action.rs`
- Create: `desktop/src-tauri/src/appui/uia/capture.rs`

**Interfaces:**
- Consumes: `tree::snapshot`, `locator::resolve`, `computer::input::perform`, `computer::capture::capture`.
- Produces: `action::perform(&IUIAutomation, HWND, &Locator, UiAction) -> Result<Value, String>`, `capture::element_png(&IUIAutomation, HWND, &Locator) -> Result<Vec<u8>, String>`.

Порядок: свежий снапшот → `resolve` → поиск элемента по индексу BFS → сверка `role`+`name`+`automationId` → выполнение паттерна. При расхождении — `The element changed; take a new snapshot`. Если паттерна нет, а режим `control` — координатный клик по центру элемента через `computer::input::perform(hwnd, &Action::Click { x, y, button: None, count: None }, guard)`; результат помечается `confidence: medium`.

- [ ] **Step 1: Написать падающие тесты**

```rust
#[test]
fn invoke_changes_the_fixture_label() {
    let fixture = testkit::window();
    let worker = UiaWorker::start().unwrap();
    worker.call(UiaRequest::Invoke { hwnd: fixture.hwnd(), locator: by_name("Save") }, Duration::from_secs(15)).unwrap();
    assert_eq!(fixture.label(), "Saved");
}

#[test]
fn missing_element_is_rejected_without_guessing() {
    let fixture = testkit::window();
    let worker = UiaWorker::start().unwrap();
    let error = worker.call(UiaRequest::Invoke { hwnd: fixture.hwnd(), locator: by_name("Нет такой") }, Duration::from_secs(15)).unwrap_err();
    assert!(error.contains("No element matches"));
}
```

- [ ] **Step 2: Прогнать тесты — должны упасть**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::uia::action`
Expected: FAIL / ошибка компиляции.

- [ ] **Step 3: Реализовать действия и кроп скриншота элемента**

- [ ] **Step 4: Прогнать тесты**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::uia`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add desktop/src-tauri/src/appui/uia
git commit -m "feat: act on app elements through UI Automation patterns"
```

---

## Задача 6. AppUiManager: привязка, разрешения, лимиты, лог

**Files:**
- Create: `desktop/src-tauri/src/appui/manager.rs`
- Modify: `desktop/src-tauri/src/computer/mod.rs` (сделать `access` публичным как `share_for`)
- Modify: `desktop/src-tauri/src/state.rs` (поле `pub appui: Arc<crate::appui::AppUiManager>`)
- Modify: `desktop/src-tauri/src/lifecycle.rs` (запуск и остановка)

**Interfaces:**
- Consumes: `UiaWorker`, `tree`, `action`, `WindowManager`.
- Produces:
  - `AppUiManager::attach(&self, app, context, hwnd, pid, identity, title, mode) -> Result<Value, String>` — переиспользует `state.windows.tool(app, context, "window_request_access", args)`, затем сохраняет цель `{workspaceId, sessionId, hwnd, title, pid, identity, generation}`.
  - `AppUiManager::target(&self, app, workspace, session, control) -> Result<Target, String>` — берёт share через `share_for`, проверяет identity и generation перед каждым действием.
  - `AppUiManager::snapshot(&self, app, context) -> Result<Value, String>`, `AppUiManager::act(&self, app, context, request) -> Result<Value, String>`.

- [ ] **Step 1: Написать падающие тесты**

```rust
#[test]
fn actions_require_an_active_share() {
    assert!(manager().act(&app(), &context(), click("Save")).is_err());
}

#[test]
fn revoked_share_stops_further_actions() {
    let manager = attached_manager();
    manager.revoke();
    assert!(manager.act(&app(), &context(), click("Save")).is_err());
}

#[test]
fn replaced_window_identity_is_rejected() {
    let manager = attached_manager();
    manager.replace_identity_for_test();
    assert!(manager.act(&app(), &context(), click("Save")).is_err());
}
```

- [ ] **Step 2: Прогнать тесты — должны упасть**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::manager`
Expected: FAIL / ошибка компиляции.

- [ ] **Step 3: Реализовать менеджер, лимиты и лог**

- [ ] **Step 4: Прогнать тесты**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add desktop/src-tauri/src/appui desktop/src-tauri/src/computer/mod.rs desktop/src-tauri/src/state.rs desktop/src-tauri/src/lifecycle.rs
git commit -m "feat: attach app testing sessions to the existing window permission"
```

---

## Задача 7. Инструменты агента `app_*`

**Files:**
- Create: `desktop/src-tauri/src/appui/tools.rs`
- Modify: `desktop/src-tauri/src/browser/workbench.rs` (`tools.extend(crate::appui::tools::definitions())` и arm `name if name.starts_with("app_")` с ранним возвратом, как у `window_`)
- Modify: `internal/agent/tools/mcp_window_permission.go`
- Modify: `internal/agent/tools/mcp_window_permission_test.go`

**Interfaces:**
- Consumes: `AppUiManager`.
- Produces: `appui::tools::definitions() -> Vec<Value>`, `appui::tools::run(app, context, name, args) -> Result<Value, String>`.

Инструменты: `app_targets`, `app_attach`, `app_detach`, `app_snapshot`, `app_click`, `app_type`, `app_set_value`, `app_toggle`, `app_select`, `app_focus`, `app_scroll`, `app_press_key`, `app_wait_for`, `app_screenshot`. Общий набор полей локатора: `reference`, `automationId`, `role`, `name`, `path`.

Go: ветка в `permissionDescription` для `app_attach` → `Allow view|control of application window "X" (PID n, HWND h) in this chat`.

- [ ] **Step 1: Написать падающие тесты**

```rust
#[test]
fn app_tools_reject_workspace_and_root_overrides() {
    for tool in definitions() {
        let name = tool["name"].as_str().unwrap();
        assert!(name.starts_with("app_"));
        assert!(tool["inputSchema"]["properties"].get("workspaceId").is_none());
    }
}
```

```go
func TestAppAttachUsesNormalChatPermission(t *testing.T) {
	// подтверждение, полный доступ и отказ — по образцу TestWindowMCPUsesNormalChatPermission
}
```

- [ ] **Step 2: Прогнать тесты — должны упасть**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib appui::tools`
Run: `go test ./internal/agent/tools`
Expected: FAIL / ошибка компиляции.

- [ ] **Step 3: Реализовать схемы, диспетчер и Go-описание разрешения**

- [ ] **Step 4: Прогнать тесты**

Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib`
Run: `go test ./internal/agent/tools`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add desktop/src-tauri/src/appui/tools.rs desktop/src-tauri/src/browser/workbench.rs internal/agent/tools
git commit -m "feat: let the agent inspect and drive any application window"
```

---

## Задача 8. Хост-метод и панель «Тесты приложений»

**Files:**
- Create: `desktop/src/renderer/src/features/appTesting/AppTestingPanel.tsx`
- Create: `desktop/src/renderer/src/features/appTesting/TargetList.tsx`
- Create: `desktop/src/renderer/src/features/appTesting/SnapshotTree.tsx`
- Create: `desktop/src/renderer/src/features/appTesting/copy.ts`
- Create: `desktop/src/shared/appTesting.ts`
- Modify: `desktop/src/shared/hostMethods.json`
- Modify: `desktop/src/shared/contracts.ts`
- Modify: `desktop/src-tauri/src/commands.rs` (arm `"appTesting"`)
- Modify: `desktop/src/renderer/src/features/workbench/WorkbenchDock.tsx`
- Modify: `desktop/src/renderer/src/app/App.tsx`

**Interfaces:**
- Consumes: `appui` host method.
- Produces: `window.prime.appTesting(workspaceId, request)`; `WorkbenchView` включает `'appTesting'`.

Панель: список целей → привязка view/control → дерево снапшота с поиском → кнопки действий → лог. Заголовок вкладки — «Тесты приложений».

- [ ] **Step 1: Написать падающий юнит-тест рендера дерева**

```ts
it('renders nested nodes with roles and actions', () => {
  const rows = flattenSnapshot(snapshotFixture())
  expect(rows[0].label).toContain('window')
  expect(rows.some(row => row.label.includes('actions=invoke'))).toBe(true)
})
```

- [ ] **Step 2: Прогнать тест — должен упасть**

Run: `cd desktop && npm run test:unit`
Expected: FAIL.

- [ ] **Step 3: Реализовать типы, хост-метод, панель и регистрацию вкладки**

- [ ] **Step 4: Прогнать тесты и типизацию**

Run: `cd desktop && npm run test:unit && npm run typecheck`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add desktop/src/renderer/src/features/appTesting desktop/src/shared desktop/src-tauri/src/commands.rs desktop/src/renderer/src/features/workbench desktop/src/renderer/src/app
git commit -m "feat: add the application testing panel"
```

---

## Задача 9. Пикер элемента

**Files:**
- Create: `desktop/src-tauri/src/appui/picker.rs`
- Create: `desktop/src/renderer/src/features/appTesting/PickedElementCard.tsx`
- Modify: `desktop/src-tauri/src/commands.rs` (методы `appPick`, `appPickCancel`)
- Modify: `desktop/src/shared/hostMethods.json`, `desktop/src/shared/contracts.ts`
- Modify: `desktop/src-tauri/src/computer/mod.rs` (второй хоткей в существующем потоке `super_start`)

**Interfaces:**
- Produces: `window.prime.appPick(): Promise<PickedAppElement | null>`, `window.prime.appPickCancel(): Promise<void>`; событие `prime:event` типа `appui` с `{ action: "picked", element, attachmentId, image }`.

Поток: кнопка «Выбрать элемент» → arm → хоткей **Ctrl+Alt+P** → `GetCursorPos` + `ElementFromPoint` → описание элемента и кроп-скриншот через `attachments.image("element.png", bytes)` → карточка в панели → «Отправить в чат» → `w.send(prompt, attachmentIds)`. Escape — отмена.

- [ ] **Step 1: Написать падающий тест промпта карточки**

```ts
it('describes provider, role, locator candidates and confidence', () => {
  const prompt = appElementPrompt([elementFixture()], 'поправь отступ')
  expect(prompt).toContain('role: button')
  expect(prompt).toContain('automationId')
  expect(prompt).toContain('confidence: high')
})
```

- [ ] **Step 2: Прогнать тест — должен упасть**

Run: `cd desktop && npm run test:unit`
Expected: FAIL.

- [ ] **Step 3: Реализовать пикер, хоткей, хост-методы и карточку**

- [ ] **Step 4: Прогнать тесты и типизацию**

Run: `cd desktop && npm run test:unit && npm run typecheck`
Run: `cargo test --manifest-path src-tauri/Cargo.toml --lib`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add desktop/src-tauri/src/appui/picker.rs desktop/src-tauri/src/commands.rs desktop/src/shared desktop/src/renderer/src/features/appTesting
git commit -m "feat: pick an element in any application and send it to the chat"
```

---

## Этап B: сценарные тесты

`appui/scenario.rs` — парсер `prime-app-test.yaml` (версия 1) и исполнитель. Шаги: `launch`, `attach`, `wait_for`, `click`, `type`, `set_value`, `toggle`, `select`, `press_key`, `scroll`, `assert`, `screenshot`, `stop`. Артефакты: `results.json`, `tree-<step>.json`, `NN-<step>.png`, логи профиля запуска. Инструмент `app_test_run`. Неоднозначный локатор останавливает прогон с диагностикой.

## Этап C: запись и исследование

`appui/recording.rs` — подписка на события UI Automation (focus, invoke, value change) → черновик сценария с альтернативными локаторами. `app_test_explore` — ограниченный обход actionable-элементов с блокировкой опасных действий (удаление, отправка, покупка, выход, выключение) и лимитом шагов.

## Этап D: остальные провайдеры

1. DOM-провайдер для Electron, Tauri и WebView2: переиспользует `browser/snapshot.js` и `browser/picker.js`, результат приводится к общему `UiNode`.
2. Vision/OCR fallback для canvas, игр и удалённого рабочего стола: действия координатные, `confidence: low`, требуют явного разрешения.
3. Linux AT-SPI, затем macOS AX — тот же контракт, без изменений в инструментах и сценариях.

---

## Проверка

- Rust: `cargo test --manifest-path src-tauri/Cargo.toml --lib`
- Renderer: `cd desktop && npm run typecheck && npm run test:unit`
- Go: `go test ./internal/agent/tools`
- Сквозная проверка на реальном приложении: скриптованный аудит в стиле `.workbench-native-ui-audit.mjs` (CDP 9239, изолированный `PRIME_NATIVE_TEST_DATA`): привязка к фикстурному окну → снапшот → клик → проверка результата → пикер → карточка → сообщение в чат.

## Риски

- Electron без `--force-renderer-accessibility` и игры дают пустое дерево: возвращается диагностика, а не успех.
- UIA-вызовы требуют таймаутов и отдельного COM-потока, иначе зависшее приложение блокирует хост.
- Окна с повышенными правами недоступны из неэлевированного хоста.
- macOS-десктоп не собирается, поэтому AX-провайдер до появления сборки будет мёртвым кодом.
- Этап A — девять задач; этапы B, C и D оформляются отдельными планами.
