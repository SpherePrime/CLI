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
  browser_click: 'Нажимает на странице',
  browser_type: 'Вводит текст',
  browser_press_key: 'Нажимает клавишу',
  browser_scroll: 'Прокручивает страницу',
  browser_back: 'Назад',
  browser_evaluate: 'Выполняет скрипт',
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

export function desktopToolTitle(tool: string, input: any, t: Translate) {
  const group = groups.find((item) => tool.startsWith(item.prefix))
  if (!group) return undefined
  const action = actions[tool]
    ? t(actions[tool])
    : tool.slice(group.prefix.length).replaceAll('_', ' ')
  const value = detailKeys
    .map((key) => input?.[key])
    .find((item) => typeof item === 'string' && item.trim())
  const detail = typeof value === 'string' ? value.replace(/\s+/g, ' ').trim().slice(0, 120) : ''
  return {
    icon: group.icon,
    label: t(group.label),
    detail: detail ? `${action} · ${detail}` : action
  }
}
