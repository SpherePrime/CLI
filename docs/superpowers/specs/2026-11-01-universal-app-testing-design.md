# Prime Desktop — универсальное тестирование приложений

Дата: 1 ноября 2026. Статус: архитектурная спецификация.

## 1. Цель

Добавить в Prime Desktop универсальный слой для запуска, исследования и тестирования приложений, которые не открываются внутри браузера. Пользователь или агент должны уметь:

- выбрать запущенное приложение;
- получить текстовое представление его интерфейса без обязательной vision-модели;
- нажимать элементы, вводить текст, выбирать значения и ждать состояния;
- включить режим выбора элемента и отправить выбранный элемент в чат;
- записать действия пользователя в воспроизводимый сценарий;
- запускать сценарии и получать артефакты, ошибки и снимки состояния;
- работать с приложениями, у которых нет доступного UI-дерева, через визуальный fallback.

Система не обещает одинаковую точность для всех приложений. Каждый результат обязан указывать источник обнаружения элемента и уровень уверенности.

## 2. Границы первой архитектуры

В существующем коде уже есть:

- Tauri host и единый `desktop_call` для renderer-to-Rust IPC;
- MCP endpoint Tauri-хоста для инструментов агента;
- запуск профилей через `launch_*`;
- управление окнами Windows через `computer/*`, включая разрешение на чат, захват окна и ввод;
- браузерный snapshot, picker и attachment store;
- скриптованные native UI-аудиты с Playwright и изолированным `PRIME_NATIVE_TEST_DATA`.

Новый слой должен переиспользовать эти границы. Нельзя переносить управление приложениями в React, добавлять второй IPC-протокол или передавать сырые изображения в модель вместо структурированных результатов.

В первой поставке поддерживаются Windows Desktop x64 как рабочая платформа и расширяемый контракт для Linux/macOS. Реализация платформенных провайдеров не должна попадать в общий тест-раннер.

## 3. Подход

### 3.1 Единый UI-граф

Все провайдеры переводят интерфейс в общий граф:

```text
UiSnapshot {
  target: UiTarget
  root: UiNode
  nodes: UiNode[]
  provider: "uia" | "atspi" | "ax" | "dom" | "vision"
  capturedAt
  truncated
  warnings[]
}

UiNode {
  ref
  parentRef
  role
  name
  text
  value
  bounds: { x, y, width, height }
  states[]
  actions[]
  automationId?
  className?
  path?
  children[]
  source
  confidence
}
```

`ref` является коротким идентификатором только для текущего snapshot. Он не считается постоянным идентификатором между запусками. Для сценариев используются устойчивые locator-стратегии в следующем порядке: automation id, accessibility path, role+name, текст, DOM selector, координаты.

`source` описывает провайдер и способ получения: `uia`, `dom`, `vision`, `coordinate`. `confidence` принимает значения `high`, `medium`, `low`; низкая уверенность требует явного сообщения агенту и может блокироваться политикой теста.

### 3.2 Провайдеры

Каждый провайдер реализует изолированный интерфейс:

```text
connect(target) -> ProviderSession
snapshot(session) -> UiSnapshot
elementAtPoint(session, point) -> UiNode
perform(session, action) -> ActionResult
capture(session, node?) -> ImageAttachment
wait(session, condition, timeout) -> WaitResult
```

Провайдеры:

1. **Windows UI Automation** — основной провайдер обычных Windows-приложений. Использует accessibility tree, control patterns, `ElementFromPoint`, Invoke/Value/Selection/Toggle и bounds.
2. **Chromium/DOM** — для Electron, Tauri, WebView2 и других Chromium-приложений. Переиспользует текущие browser snapshot/picker/evaluate механизмы, но преобразует результат в общий `UiNode`.
3. **Linux AT-SPI** — отдельный платформенный адаптер. Если accessibility bus недоступен, возвращает диагностическое состояние, а не молча пустое дерево.
4. **macOS AX** — отдельный адаптер для будущей macOS-сборки. Общий контракт не должен зависеть от Windows API.
5. **Vision/OCR** — fallback для canvas, игр, удалённого рабочего стола, старых приложений и интерфейсов без accessibility-дерева. Он возвращает OCR-ноды и визуальные области, но действия выполняются координатно и имеют низкую уверенность.

Выбор провайдера выполняется по типу цели и доступности данных. Для одного окна разрешается иметь несколько провайдеров: например, UIA для окна и DOM для вложенного WebView. Snapshot обязан показывать, какой провайдер фактически использован.

## 4. Инструменты агента

Новые MCP-инструменты размещаются в Tauri workbench и получают обычный Go-префикс `mcp_browser_`:

- `app_targets` — список доступных приложений/окон;
- `app_attach` — привязать сессию к окну и запросить view/control permission;
- `app_snapshot` — получить UI-граф в текстовом формате с refs;
- `app_click` — активировать элемент по locator/ref;
- `app_type` — ввести текст в элемент;
- `app_set_value` — установить значение, если поддерживается control pattern;
- `app_toggle` — изменить checkbox/switch/radio состояние;
- `app_select` — выбрать option/list item;
- `app_press_key` — отправить клавишу или chord с текущими ограничениями безопасности;
- `app_scroll` — прокрутить контейнер или окно;
- `app_wait_for` — ждать появления, исчезновения или изменения элемента;
- `app_screenshot` — получить окно или область элемента как attachment;
- `app_pick` — запустить интерактивный выбор элемента;
- `app_test_run` — запустить сохранённый сценарий;
- `app_test_explore` — выполнить ограниченное безопасное исследование интерфейса;
- `app_test_record_start/stop` — включить и завершить запись действий.

Схемы инструментов используют `additionalProperties: false`, не принимают workspace/root от модели и получают workspace/session из MCP binding и `prime/sessionId`. Инструменты управления требуют существующий grant и повторно проверяют identity окна перед каждым действием.

## 5. Разрешения и безопасность

Сохраняются два уровня защиты:

1. стандартное Go-разрешение на вызов MCP-инструмента, включая режимы подтверждения и полный доступ;
2. нативный grant на конкретное окно, чат и режим `view` или `control`.

Для `control` обязательно сохраняются текущие ограничения `computer/*`: аварийная остановка `Ctrl+Alt+Escape`, проверка foreground/uncovered target, запрет опасных системных сочетаний, лимиты текста и времени действия.

Новые политики:

- чтение дерева и screenshot требуют `view`;
- клик, ввод, toggle, select и сценарии требуют `control`;
- `app_test_explore` по умолчанию работает только с безопасными действиями;
- удаление, покупка, отправка, выход, выключение и подтверждение побочных эффектов блокируются или требуют отдельного подтверждения;
- vision/coordinate действия с `low` confidence не выполняются без явного разрешения;
- каждая сессия имеет timeout, лимит шагов, лимит скриншотов и общий budget;
- завершение/краш приложения, потеря окна или смена process identity немедленно останавливают сценарий.

Все действия записываются в test log с locator, provider, результатом, временем и причиной fallback.

## 6. Пикер элемента

Пикер размещается в новой панели «Тесты приложений», а быстрый вход также доступен из панели «Окна» после выбора окна. Это отделяет управление доступом к окну от списка сценариев, но сохраняет короткий путь пользователя.

Поток:

1. пользователь выбирает окно и выдаёт view-доступ;
2. нажимает «Выбрать элемент» или использует глобальный хоткей;
3. host переводит провайдер в режим `elementAtPoint`;
4. элемент под курсором подсвечивается рамкой, если провайдер предоставляет bounds;
5. подтверждение фиксирует node, provider, locator candidates, bounds, доступные actions и optional crop;
6. renderer показывает карточку по образцу `ElementPick.tsx`;
7. отправка создаёт обычное сообщение с structured context и attachment id.

Пикер не должен вставлять непроверенный текст в исходник. Для native-элемента в чат отправляется описание и locator. Связь с исходным файлом является отдельным диагностическим результатом с confidence и не является обязательной для управления.

Для Chromium можно дополнительно передавать DOM selector, HTML и source metadata. Для UIA/AT-SPI/AX передаются automation id, role/name path и control patterns. Для vision передаются OCR text, bounds, crop и confidence.

## 7. Сценарии

Сценарий хранится в проекте в отдельном файле, например `prime-app-test.yaml`. Формат должен быть версионированным и поддерживать:

```yaml
version: 1
app: calculator
launch: calculator
steps:
  - wait_for:
      role: window
      name: Calculator
  - click:
      role: button
      name: "7"
  - click:
      role: button
      name: "+"
  - type: "5"
  - assert:
      role: text
      value: "12"
  - screenshot: result
```

Минимальный набор шагов: `launch`, `attach`, `wait_for`, `click`, `type`, `set_value`, `toggle`, `select`, `press_key`, `scroll`, `assert`, `screenshot`, `stop`.

Locator в сценарии может быть ref только для отладки текущего запуска. Для сохранённого сценария используются устойчивые поля и ordered fallback. При неоднозначности runner останавливается с диагностикой, показывает совпавшие nodes и не выбирает случайный элемент.

Артефакты каждого запуска:

- `results.json` с шагами, временем, locator resolution и ошибками;
- `tree-<step>.json`;
- `NN-<step>.png`;
- stdout/stderr профиля запуска;
- итоговый manifest с provider/platform/version;
- trace действий без секретов и содержимого защищённых полей.

## 8. Запись и исследование

Режим записи сохраняет действия пользователя в нормализованном виде и одновременно прикладывает альтернативные locators, snapshot до/после и screenshot при необходимости. После записи агент может преобразовать черновик в устойчивый сценарий, но исходный raw trace сохраняется отдельно.

Режим exploration выполняет BFS/ограниченный обход доступных nodes:

- посещает только actionable элементы;
- не повторяет один locator в том же состоянии;
- после шага сравнивает snapshot и проверяет, что процесс жив;
- пытается вернуться назад или восстановить начальное состояние;
- останавливается на модальных окнах, ошибках, неизвестных переходах и опасных действиях;
- пишет отчёт о непроверенных ветках.

Это не заменяет ручные сценарии и не должен включаться как бесконечный автономный цикл.

## 9. Renderer и Rust layout

Renderer получает отдельный домен в `desktop/src/renderer/src/features/appTesting/`: панель, target list, snapshot tree, picker card, scenario editor, run artifacts и permission state. Типы добавляются отдельными файлами в `desktop/src/shared/`.

Rust host получает отдельный домен `desktop/src-tauri/src/appui/`:

- `mod.rs` — lifecycle, target sessions и provider selection;
- `types.rs` — serde DTO и validation;
- `provider.rs` — общий trait;
- `uia/`, `atspi/`, `ax/`, `dom/`, `vision/` — платформенные адаптеры;
- `picker.rs` — point-to-element и highlight state;
- `scenario.rs` — parser/executor/artifacts;
- `recording.rs` — action capture;
- `tools.rs` — MCP definitions and dispatch.

Интеграция соблюдает текущий контракт: host method добавляется в `hostMethods.json`, типизируется в `contracts.ts`, получает dispatch arm в `commands.rs`, а agent tools публикуются через workbench MCP. Renderer не получает произвольный process handle и не вызывает OS API.

## 10. Тестирование реализации

Каждый провайдер тестируется отдельно на fixture application:

- UIA fixture с кнопками, полями, toggle, list, modal и automation ids;
- Chromium fixture, проверяющий соответствие DOM и общего UI-графа;
- vision fixture с заранее известным screenshot/OCR;
- Linux/macOS fixtures добавляются вместе с соответствующими build profiles.

Обязательные уровни проверки:

- Rust unit tests для locator resolution, schema validation, identity checks, safety policy и scenario transitions;
- Go tests для permission description, MCP tool registration и result serialization;
- renderer unit tests для snapshot tree, picker card, scenario state и artifact display;
- native scripted audits через существующий CDP runner для attach → snapshot → pick → run → failure report;
- packaging tests для наличия provider-specific ресурсов и отсутствия секретов;
- manual matrix для elevated windows, empty accessibility tree, high DPI, multiple monitors, modal dialogs, minimized/covered windows и application restart.

Модель без vision должна получать полноценные text results. Vision attachment является дополнительным материалом и не может быть единственным способом понять snapshot или ошибку.

## 11. Ограничения и честные гарантии

- Игры, canvas и remote desktop могут предоставлять только vision/OCR/coordinate fallback.
- UI Automation не гарантирует доступ к приложениям с более высоким уровнем привилегий без соответствующих прав.
- Accessibility tree может быть неполным или содержать нестабильные имена; runner обязан показывать ambiguity вместо случайного клика.
- Координаты не являются стабильным locator и используются последними.
- Сопоставление элемента с исходником для native-приложений эвристическое и не входит в гарантию первой версии.
- Текущая поставка поддерживает Windows и Linux; macOS требует отдельной сборки Desktop и AX-провайдера.
- Новые platform dependencies должны учитывать текущую упаковку engine, закрытый список ресурсов installer-а, build tags и checksum manifest.

## 12. Этапы реализации

1. Контракт UI-графа, provider trait, locator model, result serialization и permission policy без реального OS-провайдера.
2. Windows UIA provider и fixture application: snapshot, elementAtPoint, click, type, wait, screenshot.
3. `app_*` MCP tools, target session, picker UI и отправка structured context в чат.
4. Scenario parser/runner, artifacts, assertions и native scripted audit.
5. Recording и безопасный exploration.
6. DOM provider, переиспользование browser picker/snapshot и единый результат для Chromium.
7. Vision/OCR fallback с явными low-confidence действиями.
8. Linux AT-SPI и затем macOS AX после появления поддерживаемой macOS-сборки.

Каждый этап должен заканчиваться проверяемым пользовательским потоком и отдельным тестовым набором. Переход к следующему провайдеру не должен менять контракт инструментов и сценариев.

## 13. Критерии приёмки архитектуры

Архитектура считается пригодной к реализации, когда:

- один scenario runner принимает UIA, DOM и vision snapshots через один контракт;
- модель без vision может получить и использовать snapshot, выполнить безопасный click/type/wait и понять ошибку;
- выбранный элемент отправляется в чат с provider, locator candidates, bounds, actions и confidence;
- разрешения привязаны к workspace/session/window identity и могут быть отозваны;
- неуспешный locator не приводит к случайному действию;
- каждый запуск оставляет воспроизводимые JSON/PNG/log artifacts;
- добавление нового провайдера не требует изменения сценариев и renderer-контракта;
- Windows fixture проходит полный путь launch → attach → snapshot → picker → scenario → report.
