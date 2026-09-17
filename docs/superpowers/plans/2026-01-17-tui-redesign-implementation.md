# TUI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create modern TUI with interactive forms, fuzzy autocomplete, and separate CLI commands for provider/model management.

**Architecture:** Event-driven TUI using ratatui/crossterm. Business logic isolated in model_wizard.rs. New CLI commands added to agent-cli/src/cmd/.

**Tech Stack:** ratatui 0.29, crossterm 0.28, dialoguer 0.11, rustyline 14, serde/toml, dirs 5

**Spec:** docs/superpowers/specs/2026-01-17-tui-redesign-design.md

## Global Constraints

- Цвета: VS Code Dark `#1e1e1e` фон, `#569cd6` акцент
- Навигация: Tab/Shift+Tab для полей, Enter подтверждает, Esc отменяет
- ID провайдера: простое имя (например, `my-openai`)
- CLI: agent provider add/list/use, agent model add/list/use
- Хранение: секция `[providers.<id>]` в config.toml

---

## File Structure

```
crates/agent-ui/src/
├── lib.rs              (добавить pub use для новых модулей)
├── app.rs              (создать — AgentApp с state machine)
├── theme.rs            (доработать цвета)
└── ui/
    ├── mod.rs          (создать)
    ├── components.rs   (создать — InputField, SelectList, FormField)
    ├── banner.rs       (создать — BannerScreen)
    ├── input.rs        (создать — InputScreen с автодополнением)
    ├── model_wizard.rs (перенести логику из interactive_tui/model_wizard.rs)
    └── provider_wizard.rs (создать — ProviderWizardScreen)

crates/agent-cli/src/
├── cmd/
│   ├── mod.rs          (создать — модуль команд)
│   ├── provider.rs     (создать — CLI для provider)
│   └── model.rs        (создать — CLI для model)
├── main_loop.rs        (модифицировать — использовать run_tui_app)
└── main.rs             (добавить subcommands)
```

---

## Task 1: Создать UI-компоненты (InputField, SelectList, FormField)

**Files:**
- Create: crates/agent-ui/src/ui/components.rs
- Create: crates/agent-ui/src/ui/mod.rs

**Interfaces:**
- Consumes: crossterm, ratatui
- Produces: Component trait, InputField struct, SelectList struct

- [ ] **Step 1: Write the failing test**

Создаём тест: `crate::ui::components::InputField::new("prompt").default("value").render()`

- [ ] **Step 2: Run test to verify it fails**

Run: `cargo test --package agent-ui components:: tests --test-threads=1`
Expected: FAIL "module not found"

- [ ] **Step 3: Write minimal implementation**

Создаём базовую структуру Component'ов

- [ ] **Step 4: Run test to verify it passes**

Run: cargo test

- [ ] **Step 5: Commit**

```bash
git add crates/agent-ui/src/ui/
git commit -m "feat: add UI components (InputField, SelectList, FormField)"
```

---

## Task 2: Создать баннер и welcome-сообщение

**Files:**
- Create: crates/agent-ui/src/ui/banner.rs
- Modify: crates/agent-ui/src/ui/mod.rs (добавить pub mod banner)

**Interfaces:**
- Consumes: theme
- Produces: draw_welcome(model, provider, version) -> Block

- [ ] **Step 1: Write test for draw_welcome**

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Write minimal implementation**

Используем существующий код из welcome.rs с цветами темы

- [ ] **Step 4: Run test**

- [ ] **Step 5: Commit**

```bash
git add crates/agent-ui/src/ui/banner.rs crates/agent-ui/src/ui/mod.rs
git commit -m "feat: add welcome banner screen"
```

---

## Task 3: Создать AgentApp с state machine

**Files:**
- Create: crates/agent-ui/src/app.rs

**Interfaces:**
- Consumes: AppState enum, Event enum
- Produces: AgentApp struct с методами run(), event(), render()

- [ ] **Step 1: Define AppState enum (пишем тест)**

```rust
enum AppState {
    Banner,
    Input,
    ModelWizard,
    ProviderWizard,
    ConfigMenu,
}
```

- [ ] **Step 2: Run test to verify**

- [ ] **Step 3: Write minimal implementation**

- [ ] **Step 4: Run test**

- [ ] **Step 5: Commit**

---

## Task 4: Реализовать InputScreen с fuzzy автодополнением

**Files:**
- Modify: crates/agent-cli/src/main_loop.rs (импортировать InputScreen)
- Create: crates/agent-ui/src/ui/input.rs

**Interfaces:**
- Consumes: SlashCommandPalette из input.rs
- Produces: InputScreen::handle_key() с автодополнением

- [ ] **Step 1: Write test для автодополнения**

```rust
#[test]
fn test_fuzzy_complete() {
    let mut input = InputScreen::new();
    input.set_input("/mo");
    let suggestions = input.complete();
    assert_eq!(suggestions, vec!["/model", "/monitor"]);
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Write implementation**

Используем fuzzy_matcher для фильтрации команд

- [ ] **Step 4: Run test**

- [ ] **Step 5: Commit**

---

## Task 5: Перенести и доработать ModelWizardScreen

**Files:**
- Move: crates/agent-ui/src/interactive_tui/model_wizard.rs -> crates/agent-ui/src/ui/model_wizard.rs
- Create: crates/agent-ui/src/ui/model_wizard.rs (с ratatui рендером)
- Modify: crates/agent-ui/src/lib.rs (обновить импорт)

**Interfaces:**
- Consumes: ModelConfig, ProviderKind из agent_config
- Produces: run_model_wizard_ui() -> ModelConfig

- [ ] **Step 1: Write test для формы модели**

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Реализовать UI формы**

- [ ] **Step 4: Run test**

- [ ] **Step 5: Commit**

---

## Task 6: Создать ProviderWizardScreen (пошаговая форма)

**Files:**
- Create: crates/agent-ui/src/ui/provider_wizard.rs

**Interfaces:**
- Consumes: Dialoguer Input/Select (переносим логику)
- Produces: run_provider_wizard() -> ProviderConfig

- [ ] **Step 1: Write test для пошаговой формы**

```rust
#[test]
fn test_provider_wizard_steps() {
    let mut wizard = ProviderWizardScreen::new();
    assert_eq!(wizard.current_step(), 0); // ID провайдера
    
    wizard.next_step();
    assert_eq!(wizard.current_step(), 1); // base_url
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Write implementation**

Форма: 1. ID провайдера, 2. Base URL, 3. API Key Env, 4. Проверка и сохранение

- [ ] **Step 4: Run test**

- [ ] **Step 5: Commit**

---

## Task 7: Создать CLI команды для provider (agent provider add/list/use)

**Files:**
- Create: crates/agent-cli/src/cmd/mod.rs
- Create: crates/agent-cli/src/cmd/provider.rs
- Modify: crates/agent-cli/src/main.rs (добавить subcommands)

**Interfaces:**
- Consumes: ConfigLoader, ProviderKind
- Produces: provider add/list/use команды

- [ ] **Step 1: Write test для CLI**

```rust
#[test]
fn test_provider_add_cli() {
    let args = vec!["agent", "provider", "add", "--id", "test"];
    let result = parse_provider_args(&args);
    assert_eq!(result.id, "test");
}
```

- [ ] **Step 2: Run test**

- [ ] **Step 3: Реализовать парсинг аргументов**

- [ ] **Step 4: Run test**

- [ ] **Step 5: Commit**

---

## Task 8: Создать CLI команды для model (agent model add/list/use)

**Files:**
- Create: crates/agent-cli/src/cmd/model.rs

**Interfaces:**
- Consumes: ModelConfig, ConfigLoader
- Produces: model add/list/use команды

---

## Task 9: Интегрировать TUI в main_loop

**Files:**
- Modify: crates/agent-cli/src/main_loop.rs

**Interfaces:**
- Consumes: AgentApp из agent-ui
- Produces: run_tui_app() как основной цикл

- [ ] **Step 1: Write test для smooth переключения**

- [ ] **Step 2: Run test**

- [ ] **Step 3: Заменить старый run_main_loop на run_tui_app**

- [ ] **Step 4: Run test**

- [ ] **Step 5: Commit**

---

## Task 10: Обновить документацию и CI

**Files:**
- Modify: README.md (добавить новые команды)
- Modify: crates/agent-cli/Cargo.toml (обновить зависимости если нужно)

---

## Self-Review Checklist

1. [ ] Spec coverage: все разделы design doc покрыты задачами
2. [ ] No placeholders: все шаги содержат конкретный код
3. [ ] Type consistency: имена и типы совпадают между задачами

