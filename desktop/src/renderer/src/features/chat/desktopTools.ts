import {
  AppWindow,
  Clock3,
  Github,
  Globe,
  Rocket,
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
  { prefix: 'window_', icon: AppWindow, label: 'Окна' },
  { prefix: 'context_', icon: Target, label: 'Контекст задачи' },
  { prefix: 'desktop_', icon: SlidersHorizontal, label: 'Prime Desktop' }
]

const actions: Record<string, string> = {
  browser_navigate: 'Открывает страницу',
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
  automation_cancel: 'Останавливает запуск',
  automation_open: 'Открывает автоматизацию',
  teams_list: 'Список групп',
  teams_create: 'Создаёт группу агентов',
  teams_cancel: 'Останавливает группу',
  teams_open: 'Открывает группу',
  launch_list: 'Команды запуска',
  launch_save: 'Сохраняет команду запуска',
  launch_logs: 'Читает логи приложения',
  window_list: 'Список окон',
  window_request_access: 'Запрашивает доступ к окну',
  window_view: 'Смотрит окно',
  window_action: 'Действие в окне',
  context_get: 'Читает контекст',
  context_update: 'Обновляет контекст',
  desktop_capabilities: 'Возможности приложения'
}

const detailKeys = ['url', 'title', 'name', 'text', 'command', 'data', 'query', 'key', 'action']

export function desktopToolTitle(tool: string, input: any, t: Translate) {
  const group = groups.find((item) => tool.startsWith(item.prefix))
  if (!group) return undefined
  const action = actions[tool] ? t(actions[tool]) : tool.slice(group.prefix.length).replaceAll('_', ' ')
  const value = detailKeys.map((key) => input?.[key]).find((item) => typeof item === 'string' && item.trim())
  const detail = typeof value === 'string' ? value.replace(/\s+/g, ' ').trim().slice(0, 120) : ''
  return { icon: group.icon, label: t(group.label), detail: detail ? `${action} · ${detail}` : action }
}
