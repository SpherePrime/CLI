package i18n

// Catalogs maps a locale code to its string catalog.
var Catalogs = map[string]Catalog{
	En: {
		Locale:  En,
		Title:   "English",
		Strings: enStrings(),
	},
	Ru: {
		Locale:  Ru,
		Title:   "Русский",
		Strings: ruStrings(),
	},
}

// enStrings is the source of truth: every key must exist here so other
// catalogs can be checked against it.
func enStrings() map[string]string {
	return map[string]string{
		// Commands dialog.
		"cmd.new_session":               "New Session",
		"cmd.sessions":                  "Sessions",
		"cmd.switch_model":              "Switch Model",
		"cmd.model_settings":            "Model Settings",
		"cmd.summarize_session":         "Summarize Session",
		"cmd.toggle_sidebar":            "Toggle Sidebar",
		"cmd.open_file_picker":          "Open File Picker",
		"cmd.open_external_editor":      "Open External Editor",
		"cmd.enable_docker_mcp":         "Enable Docker MCP Catalog",
		"cmd.disable_docker_mcp":        "Disable Docker MCP Catalog",
		"cmd.toggle_todos_queue":        "Toggle To-Dos/Queue",
		"cmd.toggle_queue":              "Toggle Queue",
		"cmd.toggle_todos":              "Toggle To-Dos",
		"cmd.notification_style":        "Notification Style",
		"cmd.enable_smart_tools":        "Enable Smart Tools Mode",
		"cmd.disable_smart_tools":       "Disable Smart Tools Mode",
		"cmd.toggle_yolo_mode":          "Toggle Yolo Mode",
		"cmd.toggle_help":               "Toggle Help",
		"cmd.initialize_project":        "Initialize Project",
		"cmd.add_provider":              "Add Provider",
		"cmd.enable_background_color":   "Enable Background Color",
		"cmd.disable_background_color":  "Disable Background Color",
		"cmd.enable_mouse":              "Enable Mouse",
		"cmd.disable_mouse":             "Disable Mouse",
		"cmd.select_reasoning_effort":   "Select Reasoning Effort",
		"cmd.enable_thinking_mode":      "Enable Thinking Mode",
		"cmd.disable_thinking_mode":     "Disable Thinking Mode",
		"cmd.thinking_suffix":           "Thinking Mode",
		"cmd.quit":                      "Quit",
		"cmd.language":                  "Language",
		"cmd.type_to_filter":            "Type to filter",
		"lang.title":                    "Language",

		// Chat input placeholders.
		"input.ready":                  "Ready for instructions",
		"input.plan":                   "Let's plan",
		"input.yolo":                   "Go crazy",
		"input.shell":                  "Run a shell command",

		// Status / banners.
		"status.plan_mode":             "Plan with Prime before generating any code.",
		"status.yolo_mode":             "Skip permission prompts. System level commands will be blocked.",

		// Notifications.
		"notify.permission_required":   "Permission required to execute %q",
		"notify.questions_need_input":  "%d questions need your input",

		// Confirmation buttons.
		"btn.yes":       "Yes",
		"btn.no":        "No",
		"btn.yep":       "Yep!",
		"btn.nope":      "Nope",
		"btn.yup":       "Yup!",
		"btn.not_yet":   "Not yet",
		"btn.confirm":   "Confirm",
		"btn.reject":    "Reject",

		// Tabs.
		"tab.confirm": "Confirm",

		// Placeholders.
		"ph.something_else":   "Something else?",
		"ph.add_note":         "Add a note...",
		"ph.type_answer":      "Type your answer...",
		"ph.type_filter":      "Type to filter",

		// Quit dialog.
		"quit.confirm":         "Are you sure you want to quit?",
		"quit.hint":           "To quit without confirmation",
		"quit.hint2":          "press ctrl+c twice.",

		// Sessions.
		"sessions.enter_name":      "Enter session name",
		"sessions.new_title":       "New Session",
		"sessions.delete_confirm":  "Delete this session?",
		"sessions.rename_title":    "Rename Session",
		"sessions.delete_title":    "Delete Session",

		// Sidebar.
		"sidebar.modified_files": "Modified Files",
		"sidebar.none":           "None",
		"sidebar.more":           "…and %d more",

		// Pills.
		"pills.todo":    "To-Do",
		"pills.queued":  "%d Queued",

		// Onboarding.
		"onboard.init_title":  "Would you like to initialize this project?",
		"onboard.init_body":   "When I initialize your codebase I examine the project and put the result into an %s file which serves as general context.",
		"onboard.init_hint":   "You can also initialize anytime via ctrl+p.",
		"onboard.init_now":    "Would you like to initialize now?",

		// Permissions.
		"perm.allow":            "Allow",
		"perm.allow_session":    "Allow for Session",
		"perm.deny":             "Deny",

		// Update flow.
		"update.available":      "Prime update available: v%s → v%s.",
		"update.dev_version":    "This is a development version of Prime. The latest version is v%s.",
		"update.installed":      "Prime v%s installed. Restart Prime to run the new version.",
		"update.failed":         "Prime update failed: %v",
		"update.auto":           "Prime auto-updated to v%s. Restart Prime to run the new version.",

		// Info messages.
		"info.notifications_set":     "Notifications set to: %s",
		"info.thinking_mode":         "Thinking mode %s",
		"info.transparent_bg":        "Transparent background %s",
		"info.mouse_support":         "Mouse support %s",
		"info.smart_tools_mode":      "Smart Tools mode %s",
		"info.model_settings_saved":  "Model settings saved",
		"info.provider_added":        "Provider added to global config",
		"info.reasoning_effort":      "Reasoning effort set to %s",
		"info.yolo_disabled":         "Yolo mode disabled",
		"info.agent_busy":            "Agent is busy, please wait...",		"info.empty_message":         "Message is empty",
		"info.no_image_support":      "The current model does not support image attachments",
		"info.cannot_attach_dir":     "Cannot attach a directory",
		"info.clipboard_empty":       "Clipboard is empty or does not contain text",
		"info.file_too_large":       "File too large, max 5MB",
		"info.bad_image_format":      "File type is not a supported image format",
		"info.unable_read_file":      "Unable to read file: %v",
		"info.copied_to_clipboard":   "Selected text copied to clipboard",
		"info.selection_copied":      "Selection copied to clipboard",
		"info.selection_cut":         "Selection cut to clipboard",
		"info.docker_mcp_on":         "Docker MCP enabled and started successfully",
		"info.docker_mcp_off":        "Docker MCP disabled successfully",
		"info.plan_implement":        "Implement the plan.",
		"info.reconnect_failed":      "Can't restore the connection to the Prime server. Restart Prime to recover.",
		"info.reconnected":           "Reconnected to the Prime server.",
		"info.language_set":          "Language set to %s",
		"info.enabled":               "enabled",
		"info.disabled":              "disabled",
	}
}

// ruStrings mirrors enStrings with Russian translations.
func ruStrings() map[string]string {
	return map[string]string{
		// Commands dialog.
		"cmd.new_session":               "Новая сессия",
		"cmd.sessions":                  "Сессии",
		"cmd.switch_model":              "Сменить модель",
		"cmd.model_settings":            "Настройки модели",
		"cmd.summarize_session":         "Сжать сессию",
		"cmd.toggle_sidebar":            "Боковая панель",
		"cmd.open_file_picker":          "Выбор файла",
		"cmd.open_external_editor":      "Открыть внешний редактор",
		"cmd.enable_docker_mcp":         "Включить Docker MCP",
		"cmd.disable_docker_mcp":        "Выключить Docker MCP",
		"cmd.toggle_todos_queue":        "Задачи/Очередь",
		"cmd.toggle_queue":              "Очередь",
		"cmd.toggle_todos":              "Задачи",
		"cmd.notification_style":        "Стиль уведомлений",
		"cmd.enable_smart_tools":        "Включить умные инструменты",
		"cmd.disable_smart_tools":       "Выключить умные инструменты",
		"cmd.toggle_yolo_mode":          "Режим Yolo",
		"cmd.toggle_help":               "Справка",
		"cmd.initialize_project":        "Инициализировать проект",
		"cmd.add_provider":              "Добавить провайдера",
		"cmd.enable_background_color":   "Включить цвет фона",
		"cmd.disable_background_color":  "Выключить цвет фона",
		"cmd.enable_mouse":              "Включить мышь",
		"cmd.disable_mouse":             "Выключить мышь",
		"cmd.select_reasoning_effort":   "Уровень рассуждений",
		"cmd.enable_thinking_mode":      "Включить размышления",
		"cmd.disable_thinking_mode":     "Выключить размышления",
		"cmd.thinking_suffix":           "режим размышлений",
		"cmd.quit":                      "Выйти",
		"cmd.language":                  "Язык",
		"cmd.type_to_filter":            "Введите для поиска",
		"lang.title":                    "Язык",

		// Chat input placeholders.
		"input.ready":                  "Готов к командам",
		"input.plan":                   "Спланируем",
		"input.yolo":                   "Без тормозов",
		"input.shell":                  "Выполнить команду",

		// Status / banners.
		"status.plan_mode":             "Спланируй с Prime перед генерацией кода.",
		"status.yolo_mode":             "Пропуск запросов разрешений. Системные команды будут блокироваться.",

		// Notifications.
		"notify.permission_required":   "Нужно разрешение для %q",
		"notify.questions_need_input":  "Вопросов, ждущих ответа: %d",

		// Confirmation buttons.
		"btn.yes":       "Да",
		"btn.no":        "Нет",
		"btn.yep":       "Да!",
		"btn.nope":      "Нет",
		"btn.yup":       "Угу!",
		"btn.not_yet":   "Пока нет",
		"btn.confirm":   "Подтвердить",
		"btn.reject":    "Отклонить",

		// Tabs.
		"tab.confirm": "Подтверждение",

		// Placeholders.
		"ph.something_else":   "Что-то ещё?",
		"ph.add_note":         "Добавить заметку...",
		"ph.type_answer":      "Введите ответ...",
		"ph.type_filter":      "Введите для поиска",

		// Quit dialog.
		"quit.confirm":         "Точно выйти?",
		"quit.hint":           "Чтобы выйти без подтверждения",
		"quit.hint2":          "нажми ctrl+c дважды.",

		// Sessions.
		"sessions.enter_name":      "Название сессии",
		"sessions.new_title":       "Новая сессия",
		"sessions.delete_confirm":  "Удалить эту сессию?",
		"sessions.rename_title":    "Переименовать сессию",
		"sessions.delete_title":    "Удалить сессию",

		// Sidebar.
		"sidebar.modified_files": "Изменённые файлы",
		"sidebar.none":           "Ничего",
		"sidebar.more":           "…ещё %d",

		// Pills.
		"pills.todo":    "Задачи",
		"pills.queued":  "В очереди: %d",

		// Onboarding.
		"onboard.init_title":  "Инициализировать этот проект?",
		"onboard.init_body":   "Я изучу проект и запишу результат в файл %s, который будет общим контекстом.",
		"onboard.init_hint":   "Можно инициализировать в любой момент через ctrl+p.",
		"onboard.init_now":    "Инициализировать сейчас?",

		// Permissions.
		"perm.allow":            "Разрешить",
		"perm.allow_session":    "Разрешить на сессию",
		"perm.deny":             "Запретить",

		// Update flow.
		"update.available":      "Доступна новая версия Prime: v%s → v%s.",
		"update.dev_version":    "Это dev-версия Prime. Последняя версия v%s.",
		"update.installed":      "Prime v%s установлен. Перезапусти Prime.",
		"update.failed":         "Ошибка обновления Prime: %v",
		"update.auto":           "Prime обновлён до v%s. Перезапусти Prime.",

		// Info messages.
		"info.notifications_set":     "Стиль уведомлений: %s",
		"info.thinking_mode":         "Режим размышлений %s",
		"info.transparent_bg":        "Прозрачный фон %s",
		"info.mouse_support":         "Поддержка мыши %s",
		"info.smart_tools_mode":      "Умные инструменты %s",
		"info.model_settings_saved":  "Настройки модели сохранены",
		"info.provider_added":        "Провайдер добавлен в глобальный конфиг",
		"info.reasoning_effort":      "Уровень рассуждений: %s",
		"info.yolo_disabled":         "Режим Yolo выключен",
		"info.agent_busy":            "Агент занят, подожди...",
		"info.empty_message":         "Сообщение пустое",
		"info.no_image_support":      "Текущая модель не поддерживает картинки",
		"info.cannot_attach_dir":     "Нельзя приложить каталог",
		"info.clipboard_empty":       "Буфер обмена пуст или там нет текста",
		"info.file_too_large":       "Файл слишком большой, макс. 5МБ",
		"info.bad_image_format":      "Формат файла не поддерживается как картинка",
		"info.unable_read_file":      "Не удалось прочитать файл: %v",
		"info.copied_to_clipboard":   "Текст скопирован в буфер",
		"info.selection_copied":      "Выделение скопировано",
		"info.selection_cut":         "Выделение вырезано",
		"info.docker_mcp_on":         "Docker MCP включён и запущен",
		"info.docker_mcp_off":        "Docker MCP выключен",
		"info.plan_implement":        "Реализовать план.",
		"info.reconnect_failed":      "Не удалось восстановить соединение с сервером Prime. Перезапусти Prime.",
		"info.reconnected":           "Соединение с сервером Prime восстановлено.",
		"info.language_set":          "Язык: %s",
		"info.enabled":               "включено",
		"info.disabled":              "выключено",
	}
}
