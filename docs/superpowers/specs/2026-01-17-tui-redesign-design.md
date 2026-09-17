# TUI Redesign — Design Specification

**Date:** 2026-01-17  
**Status:** Implemented ✓  
**Author:** Agent  

## Executive Summary

Переработка терминального интерфейса с целью создания приложения-подобного UX:
- Ретроспективный welcome-баннер при старте
- Интерактивное меню с навигацией стрелками
- Пошаговые формы для добавления провайдеров и моделей
- Fuzzy-автодополнение команд в реальном времени
- CLI-команды: `agent provider add/list/use`, `agent model add/list/use`

---

## 1. Архитектура UI

### 1.1. Основные экраны (Screens)

| Экран | Описание | Компоненты |
|-------|----------|------------|
| `BannerScreen` | Приветствие, текущая модель, доступные команды | Header, Prompt, Footer |
| `InputScreen` | Приём команд от пользователя | MessageInput, AutocompleteList |
| `ModelWizardScreen` | Настройка модели и провайдера | Form, SelectList, InputField |
| `ProviderWizardScreen` | Добавление нового провайдера | MultiStepForm |
| `ConfigMenuScreen` | Меню настроек | MenuItemList, StatusPanel |
| `MessageListScreen` | Отображение диалога | ChatBubble, ToolCallWidget |

### 1.2. Цикл событий (Event Loop)

```
loop {
    render(state);
    event = polls_events(timeout);
    match event {
        KeyPress(key) -> handle_key(key, state),
        Mouse(event) -> handle_mouse(event, state),
        Resize(w, h) -> state.update_size(w, h),
    }
}
```

---

## 2. UI Composition

### 2.1. Компоненты

#### InputField
- Поддержка placeholder'а
- Авто-дополнение в выпадающем списке
- Валидация значения

#### SelectList
- Fuzzy-поиск по элементам
- Навигация стрелками
- Множественный выбор (для тулз)

#### FormField
- Метка + поле ввода
- Поддержка разных типов: string, number, select
- Валидация и подсказки

#### ChatBubble
- Аватарка (👤/🤖)
- Цветовая схема по роли
- Поддержка markdown

---

## 3. Цветовая схема

### 3.1. Основные цвета (VS Code Dark)

| Элемент | Цвет (RGB) | Код |
|---------|-----------|-----|
| Фон | 30, 30, 30 | `#1e1e1e` |
| Текст | 200, 200, 200 | `#c8c8c8` |
| Акцент (prompt, ссылки) | 86, 158, 255 | `#569cd6` |
| Success | 33, 194, 124 | `#21c27c` |
| Error | 214, 67, 73 | `#d64349` |
| Warning | 230, 149, 7 | `#e69507` |

### 3.2. Применение

- Приветственный баннер: `┌──────────┐` с акцентным заголовком
- Prompt: `❯ ` в ярком синем
- Сообщения: слева `👤 you` (синий) / `🤖 agent` (зеленый)
- Статус: серым цветом внизу экрана

---

## 4. CLI-команды

### 4.1. Управление провайдерами

```bash
# Показать список
agent provider list

# Добавить (интерактивно)
agent provider add
# или с параметрами
agent provider add --id my-azure --base-url "https://my.openai.cn" --api-key-env AZURE_KEY

# Использовать
agent provider use my-azure

# Удалить
agent provider remove my-azure
```

### 4.2. Управление моделями

```bash
# Показать список
agent model list

# Добавить (интерактивно)
agent model add

# Использовать
agent model use my-azure/gpt-4o

# Установить дефолт
agent model set-default my-openai/claude-3.5-sonnet
```

---

## 5. Формат хранения

### 5.1. config.toml (дополненный)

```toml
model = { provider = "my-azure", model = "gpt-4o", base_url = "https://...", ... }

[providers.my-azure]
kind = "openai"
base_url = "https://my.azure.openai.cn/v1"
api_key_env = "AZURE_API_KEY"

[providers.my-openai]
kind = "openai"
base_url = "https://api.openai.com/v1"
api_key_env = "OPENAI_API_KEY"
```

### 5.2. CLI API (Rust)

```
crates/agent-cli/src/cmd/provider.rs   # add, list, use, remove
crates/agent-cli/src/cmd/model.rs      # add, list, use, set-default
```

---

## 6. Пошаговые формы (Example Flow)

### 6.1. provider add — интерактивный wizard

```
┌────────────────────────────────────┐
│  Добавить провайдер                │
├────────────────────────────────────┤
│  Provider ID: [my-openai     ] ──▶│
│  Base URL:    [https://...   ]   │
│  API Key Env: [OPENAI_KEY    ]   │
├────────────────────────────────────┤
│  [✓] Save   [Cancel]              │
└────────────────────────────────────┘
```

Навигация:
- Tab/Shift+Tab — переключить поле
- Стрелки вниз/вверх — пролистывание вариантов в select'ах
- Enter — подтвердить/создать
- Esc — отменить

---

## 7. Файловая структура

```
crates/agent-ui/
├── Cargo.toml
└── src/
    ├── lib.rs              # публичный API
    ├── app.rs              # AgentApp со state и lifecycle
    ├── ui/
    │   ├── mod.rs          # экраны
    │   ├── banner.rs       # BannerScreen
    │   ├── input.rs        # InputScreen + автодополнение
    │   ├── model_wizard.rs # ModelWizardScreen
    │   ├── provider_wizard.rs # ProviderWizardScreen
    │   └── components.rs   # InputField, SelectList, Form
    ├── theme.rs            # цветовая схема
    └── events.rs           # обработка клавиатуры

crates/agent-cli/
├── src/cmd/
│   ├── provider.rs         # CLI команды для провайдеров
│   └── model.rs            # CLI команды для моделей
└── src/main.rs
```

---

## 8. Миграция

### 8.1. Сохранение обратной совместимости

- Старый `run_main_loop` остаётся, но помечается `#[deprecated]`
- Новый `run_tui_app` активен по умолчанию
- Флаг `--legacy` восстанавливает старый режим

### 8.2. Изоляция бизнес-логики

- `model_wizard.rs` → `provider_list.rs` (чтение списка)
- `config_wizard.rs` → `config_applier.rs` (применение настроек)
- UI-логика вынесена в новые файлы в `agent-ui/src/ui/`

---

## 9. Тесты

- Unit-тесты для `app.rs` (state transitions)
- Integration-тесты для CLI команд (`provider add --id test --dry-run`)
- Snapshot-тесты UI (сериализация терминального вывода)

---

## 10. План реализации (подробнее в отдельном плане)

1. Создать модуль `agent-ui/src/ui/` с базовыми компонентами
2. Реализовать `AgentApp` с state machine
3. Перенести welcome-баннер в `BannerScreen`
4. Реализовать `InputScreen` с fuzzy-автодополнением
5. Переписать `model_wizard.rs` → `ModelWizardScreen`
6. Создать `provider_wizard.rs` (пошаговая форма)
7. Добавить CLI команды в `agent-cli/src/cmd/`
8. Обновить `main_loop.rs` → использовать `run_tui_app`

---

## 11. Открытые вопросы

- [ ] Нужно ли сохранять курсор/историю ввода при смене экранов?
- [ ] Как обрабатывать огромные списки провайдеров/моделей? (пагинация?)
- [ ] Нужен ли режим "read-only" для CI?

---

*Дизайн готов к согласованию. После одобрения перейдём к написанию плана реализации (writing-plans skill).*