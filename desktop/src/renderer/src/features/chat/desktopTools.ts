import {
  AppWindow,
  Clock3,
  Github,
  GitCommitHorizontal,
  Globe,
  Rocket,
  Server,
  SlidersHorizontal,
  Target,
  TerminalSquare,
  Users,
  type LucideIcon
} from 'lucide-react'

type Translate = (key: string, values?: Record<string, string | number>) => string

const groups: { prefix: string; icon: LucideIcon; label: string }[] = [
  { prefix: 'browser_', icon: Globe, label: 'Браузер' },
  { prefix: 'terminal_', icon: TerminalSquare, label: 'Терминал' },
  { prefix: 'automation_', icon: Clock3, label: 'Автоматизации' },
  { prefix: 'teams_', icon: Users, label: 'Команды' },
  { prefix: 'launch_', icon: Rocket, label: 'Запуск' },
  { prefix: 'github_', icon: Github, label: 'GitHub' },
  { prefix: 'git_', icon: GitCommitHorizontal, label: 'Git' },
  { prefix: 'ssh_', icon: Server, label: 'SSH' },
  { prefix: 'window_', icon: AppWindow, label: 'Окна' },
  { prefix: 'app_', icon: AppWindow, label: 'Проверка приложения' },
  { prefix: 'context_', icon: Target, label: 'Контекст задачи' },
  { prefix: 'desktop_', icon: SlidersHorizontal, label: 'Prime Desktop' }
]

const actions: Record<string, string> = {
  browser_navigate: 'Открывает страницу',
  browser_tabs: 'Список вкладок',
  browser_new_tab: 'Открывает вкладку',
  browser_select_tab: 'Выбирает вкладку',
  browser_close_tab: 'Закрывает вкладку',
  browser_snapshot: 'Читает страницу',
  browser_screenshot: 'Снимок страницы',
  browser_screenshot_full: 'Снимок всей страницы',
  browser_screenshot_element: 'Снимок элемента',
  browser_screenshot_region: 'Снимок области',
  browser_record_start: 'Записывает видео',
  browser_record_stop: 'Завершает запись видео',
  browser_run: 'Проверка сценарием',
  browser_click: 'Нажимает на странице',
  browser_type: 'Вводит текст',
  browser_press_key: 'Нажимает клавишу',
  browser_scroll: 'Прокручивает страницу',
  browser_back: 'Назад',
  browser_evaluate: 'Выполняет скрипт',
  browser_health: 'Проверяет отклик страницы',
  browser_resize: 'Меняет ширину окна',
  app_launch: 'Запускает приложение',
  app_targets: 'Список окон приложений',
  app_attach: 'Подключается к окну',
  app_close: 'Закрывает приложение',
  app_output: 'Читает вывод приложения',
  app_snapshot: 'Снимок интерфейса',
  app_find: 'Ищет элемент',
  app_wait: 'Ожидает элемент',
  app_audit: 'Аудит макета',
  app_check: 'Записывает проверку',
  app_click: 'Нажимает на элемент',
  app_type: 'Вводит текст',
  app_set_value: 'Устанавливает значение',
  app_toggle: 'Переключает элемент',
  app_select: 'Выбирает элемент',
  app_focus: 'Фокус на элементе',
  app_scroll: 'Прокручивает область',
  app_expand: 'Раскрывает элемент',
  app_press_key: 'Нажимает клавишу',
  app_screenshot: 'Снимок элемента',
  app_test_explore: 'Авто-обход интерфейса',
  app_test_list: 'Список сценариев',
  app_test_run: 'Запускает сценарий',
  terminal_list: 'Список терминалов',
  terminal_read: 'Читает вывод',
  terminal_write: 'Вводит команду',
  terminal_resize: 'Меняет размер',
  terminal_create: 'Открывает терминал',
  terminal_close: 'Закрывает терминал',
  terminal_release: 'Возвращает управление',
  automation_list: 'Список автоматизаций',
  automation_save: 'Создаёт автоматизацию',
  automation_update: 'Изменяет автоматизацию',
  automation_pause: 'Приостанавливает автоматизацию',
  automation_run: 'Запускает автоматизацию',
  automation_delete: 'Удаляет автоматизацию',
  automation_cancel: 'Останавливает запуск',
  automation_open: 'Открывает автоматизацию',
  teams_list: 'Список групп',
  teams_create: 'Создаёт группу агентов',
  teams_cancel: 'Останавливает группу',
  teams_open: 'Открывает группу',
  launch_list: 'Команды запуска',
  launch_save: 'Сохраняет команду запуска',
  launch_logs: 'Читает логи приложения',
  launch_start: 'Запускает приложение',
  launch_stop: 'Останавливает приложение',
  launch_restart: 'Перезапускает приложение',
  launch_remove: 'Удаляет команду запуска',
  git_history: 'История коммитов',
  git_show_commit: 'Просматривает коммит',
  git_restore_commit: 'Восстанавливает состояние проекта',
  git_revert_commit: 'Отменяет изменения коммита',
  github_status: 'Состояние репозитория',
  github_issues: 'Список задач',
  github_pulls: 'Список pull request',
  github_branches: 'Список веток',
  github_issue: 'Просматривает задачу',
  github_pull: 'Просматривает pull request',
  github_diff: 'Читает изменения',
  github_failed_logs: 'Читает логи ошибок',
  github_create_pull: 'Создаёт pull request',
  github_comment: 'Комментирует pull request',
  ssh_profiles: 'Список серверов',
  ssh_connect: 'Подключается к серверу',
  ssh_disconnect: 'Отключается от сервера',
  ssh_exec: 'Выполняет команду',
  ssh_read_file: 'Читает файл',
  ssh_write_file: 'Записывает файл',
  ssh_edit_file: 'Изменяет файл',
  ssh_list_files: 'Список файлов',
  ssh_search: 'Ищет в файлах',
  ssh_glob: 'Ищет файлы',
  ssh_download: 'Скачивает файл на сервер',
  ssh_upload_file: 'Загружает файл на сервер',
  ssh_create_entry: 'Создаёт файл или папку',
  ssh_download_file: 'Скачивает файл',
  ssh_forward_port: 'Пробрасывает порт',
  ssh_forwards: 'Активные пробросы',
  ssh_close_forward: 'Закрывает проброс',
  window_list: 'Список окон',
  window_request_access: 'Запрашивает доступ к окну',
  window_view: 'Смотрит окно',
  window_action: 'Действие в окне',
  context_get: 'Читает контекст',
  context_update: 'Обновляет контекст',
  desktop_capabilities: 'Возможности приложения'
}

const detailKeys = [
  'description',
  'url',
  'title',
  'name',
  'path',
  'file_path',
  'localPath',
  'host',
  'pattern',
  'text',
  'command',
  'data',
  'query',
  'key',
  'action'
]

function screenshotAction(tool: string, input: any) {
  if (tool !== 'browser_screenshot') return tool
  if (input?.fullPage === true) return 'browser_screenshot_full'
  if (input?.ref) return 'browser_screenshot_element'
  if (input?.clip) return 'browser_screenshot_region'
  return tool
}

export function desktopToolTitle(tool: string, input: any, t: Translate) {
  const group = groups.find((item) => tool.startsWith(item.prefix))
  if (!group) return undefined
  const actionKey = screenshotAction(tool, input)
  const action = actions[actionKey]
    ? t(actions[actionKey])
    : tool.slice(group.prefix.length).replace(/[_-]+/g, ' ').trim()
  const value = detailKeys
    .map((key) => input?.[key])
    .find((item) => typeof item === 'string' && item.trim())
  let detail = typeof value === 'string' ? value.replace(/\s+/g, ' ').trim().slice(0, 120) : ''
  if (!detail && actionKey === 'browser_run' && Array.isArray(input?.steps) && input.steps.length > 0) {
    const firstNav = input.steps.find(
      (step: any) => step?.action === 'navigate' && typeof step?.url === 'string' && step.url.trim()
    )
    const countLabel = t('Шагов: {count}', { count: input.steps.length })
    detail = firstNav ? `${countLabel} · ${text(firstNav.url).slice(0, 80)}` : countLabel
  }
  return {
    icon: group.icon,
    label: t(group.label),
    detail: detail ? `${action} · ${detail}` : action
  }
}

function text(value: unknown) {
  return typeof value === 'string' ? value.replace(/\s+/g, ' ').trim() : ''
}
