import {
  AlertTriangle,
  ArrowRightLeft,
  Brain,
  ClipboardList,
  Cpu,
  Download,
  FileCode2,
  FilePen,
  FilePlus2,
  FileSpreadsheet,
  FileText,
  FolderTree,
  GitBranch,
  Globe,
  HelpCircle,
  History,
  Image,
  Info,
  ListChecks,
  ListTree,
  Network,
  Plug,
  Presentation,
  ScrollText,
  Search,
  Send,
  ShieldAlert,
  Sparkles,
  Target,
  Terminal,
  UserCog,
  UserMinus,
  UserPlus,
  Users,
  Waypoints,
  Wrench,
  type LucideIcon
} from 'lucide-react'

import { desktopToolTitle } from './desktopTools'

type Translate = (key: string, values?: Record<string, string | number>) => string

export type ToolTitle = { icon: LucideIcon; label: string; detail: string }

function fileName(path: unknown) {
  if (typeof path !== 'string' || !path) return ''
  return path.split(/[\\/]/).filter(Boolean).pop() || path
}

function text(value: unknown) {
  return typeof value === 'string' ? value.replace(/\s+/g, ' ').trim() : ''
}

function humanizeSegment(value: unknown) {
  if (typeof value !== 'string') return ''
  return value.replace(/[_-]+/g, ' ').replace(/\s+/g, ' ').trim()
}

function humanizeToolName(raw: string) {
  const normalized = humanizeSegment(raw)
  if (!normalized) return ''
  return normalized.charAt(0).toUpperCase() + normalized.slice(1)
}

const memoryOps: Record<string, string> = {
  read: 'чтение',
  write: 'запись',
  add: 'добавление',
  append: 'дополнение',
  replace: 'замена',
  edit: 'правка',
  patch: 'правка',
  remove: 'удаление',
  delete: 'удаление',
  batch: 'пакетное обновление'
}

function memoryOpLabel(op: string, t: Translate) {
  if (!op) return ''
  const key = memoryOps[op]
  return key ? t(key) : humanizeSegment(op)
}

const documentActions: Record<string, string> = {
  create: 'создание',
  inspect: 'просмотр',
  update: 'обновление',
  fill_template: 'заполнение шаблона',
  merge: 'объединение',
  split: 'разделение',
  rotate: 'поворот'
}

function documentToolDetail(input: any, t: Translate) {
  const rawAction = text(input?.action).toLowerCase()
  const actionKey = documentActions[rawAction]
  const action = actionKey ? t(actionKey) : humanizeSegment(rawAction)
  const target = fileName(
    input?.output_path ||
      input?.path ||
      input?.input_path ||
      input?.output_dir ||
      (Array.isArray(input?.input_paths) ? input.input_paths[0] : '')
  )
  return [action, target].filter(Boolean).join(' · ')
}

function handoffDetail(input: any) {
  if (typeof input?.plan === 'string') {
    const firstLine = input.plan.split(/\r?\n/).find((line: string) => line.trim())
    const summary = text(firstLine)
    if (summary) return summary.slice(0, 120)
  }
  if (Array.isArray(input?.target_files)) {
    const files = input.target_files.map((item: unknown) => fileName(item)).filter(Boolean)
    if (files.length > 0) return files.join(', ').slice(0, 120)
  }
  return ''
}

const namedTools: Record<string, (input: any, t: Translate) => ToolTitle> = {
  agent: (input, t) => ({
    icon: Sparkles,
    label: t('Подзадача'),
    detail: text(input?.description) || text(input?.prompt)
  }),
  agent_jobs: (input, t) => ({
    icon: Users,
    label: t('Команда агентов'),
    detail: [text(input?.op) || 'list', text(input?.job_id)].filter(Boolean).join(' · ')
  }),
  agent_message: (input, t) => ({
    icon: Users,
    label: input?.op === 'send' ? t('Сообщение агенту') : t('Состав команды'),
    detail:
      input?.op === 'send'
        ? [text(input?.worker_id), text(input?.text).slice(0, 120)].filter(Boolean).join(' · ')
        : ''
  }),
  goal_complete: (input, t) => ({
    icon: Target,
    label: t('Цель выполнена'),
    detail: text(input?.summary).slice(0, 160)
  }),
  job_output: (input, t) => ({
    icon: Terminal,
    label: t('Вывод фоновой задачи'),
    detail: text(input?.shell_id) ? `#${text(input?.shell_id)}` : ''
  }),
  job_kill: (input, t) => ({
    icon: Terminal,
    label: t('Остановка задачи'),
    detail: text(input?.shell_id) ? `#${text(input?.shell_id)}` : ''
  }),
  mcp_job: (input, t) => ({
    icon: Plug,
    label: t('Фоновый вызов'),
    detail: [text(input?.op) || 'status', text(input?.job_id)].filter(Boolean).join(' · ')
  }),
  prime_info: (_input, t) => ({ icon: Info, label: t('Состояние Prime'), detail: '' }),
  prime_logs: (_input, t) => ({ icon: ScrollText, label: t('Логи Prime'), detail: '' }),
  question: (input, t) => ({
    icon: HelpCircle,
    label: t('Вопрос'),
    detail: text(input?.questions?.[0]?.question)
  }),
  generate_image: (input, t) => ({
    icon: Image,
    label: t('Генерация изображения'),
    detail: text(input?.prompt)
  }),
  pdf_document: (input, t) => ({
    icon: FileText,
    label: t('PDF-документ'),
    detail: documentToolDetail(input, t)
  }),
  word_document: (input, t) => ({
    icon: FileText,
    label: t('Word-документ'),
    detail: documentToolDetail(input, t)
  }),
  excel_workbook: (input, t) => ({
    icon: FileSpreadsheet,
    label: t('Excel-таблица'),
    detail: documentToolDetail(input, t)
  }),
  powerpoint_presentation: (input, t) => ({
    icon: Presentation,
    label: t('Презентация'),
    detail: documentToolDetail(input, t)
  }),
  lsp_diagnostics: (_input, t) => ({ icon: Wrench, label: t('Диагностика LSP'), detail: '' }),
  lsp_references: (input, t) => ({
    icon: Network,
    label: t('Ссылки символа'),
    detail: text(input?.symbol)
  }),
  lsp_restart: (_input, t) => ({ icon: Wrench, label: t('Перезапуск LSP'), detail: '' }),
  lsp_symbols: (input, t) => ({
    icon: ListTree,
    label: t('Символы файла'),
    detail: fileName(input?.file_path || input?.path)
  }),
  lsp_definition: (input, t) => ({
    icon: Network,
    label: t('Определение символа'),
    detail: text(input?.symbol)
  }),
  lsp_call_hierarchy: (input, t) => ({
    icon: Waypoints,
    label: t('Иерархия вызовов'),
    detail: text(input?.symbol)
  }),
  lsp_rename: (input, t) => ({
    icon: FilePen,
    label: t('Переименование символа'),
    detail: text(input?.symbol)
  }),
  lsp_replace_symbol: (input, t) => ({
    icon: FilePen,
    label: t('Замена символа'),
    detail: text(input?.symbol) || fileName(input?.file_path || input?.path || input?.filePath)
  }),
  custom_agent_memory: (input, t) => {
    const rawTarget = text(input?.target).toLowerCase()
    const target =
      rawTarget === 'user' || rawTarget === 'user_profile'
        ? t('Профиль пользователя')
        : rawTarget === 'memory' || rawTarget === 'global_memory'
          ? t('Общая память')
          : text(input?.slug || input?.agent_id)
    const op = memoryOpLabel(text(input?.op).toLowerCase(), t)
    return {
      icon: Brain,
      label: t('Память агентов'),
      detail: [target, op].filter(Boolean).join(' · ')
    }
  },
  custom_agent_list: (_input, t) => ({
    icon: Users,
    label: t('Список агентов'),
    detail: ''
  }),
  custom_agent_list_models: (_input, t) => ({
    icon: Cpu,
    label: t('Модели агентов'),
    detail: ''
  }),
  custom_agent_create: (input, t) => ({
    icon: UserPlus,
    label: t('Создание агента'),
    detail: text(input?.name) || text(input?.slug)
  }),
  custom_agent_update: (input, t) => ({
    icon: UserCog,
    label: t('Обновление агента'),
    detail: text(input?.name) || text(input?.slug) || text(input?.id)
  }),
  custom_agent_delete: (input, t) => ({
    icon: UserMinus,
    label: t('Удаление агента'),
    detail: text(input?.slug) || text(input?.id)
  }),
  custom_agent_dispatch: (input, t) => ({
    icon: Send,
    label: t('Запуск агента'),
    detail: [text(input?.slug || input?.agent_id), text(input?.prompt).slice(0, 100)]
      .filter(Boolean)
      .join(' · ')
  }),
  custom_agent_swarm: (input, t) => ({
    icon: Network,
    label: t('Рой агентов'),
    detail: text(input?.goal).slice(0, 140)
  }),
  custom_agent_workflow: (input, t) => ({
    icon: GitBranch,
    label: t('Цепочка агентов'),
    detail: [text(input?.name || input?.slug || input?.workflow_id), text(input?.op)]
      .filter(Boolean)
      .join(' · ')
  }),
  custom_agent_runs: (input, t) => ({
    icon: History,
    label: t('История запусков'),
    detail: text(input?.slug || input?.agent_id)
  }),
  custom_agent_blackboard: (input, t) => ({
    icon: ClipboardList,
    label: t('Доска координации'),
    detail: [text(input?.topic), text(input?.op)].filter(Boolean).join(' · ')
  }),
  handoff_to_executor: (input, t) => ({
    icon: ArrowRightLeft,
    label: t('Передача исполнителю'),
    detail: handoffDetail(input)
  }),
  escalate_to_planner: (input, t) => ({
    icon: AlertTriangle,
    label: t('Возврат планировщику'),
    detail: text(input?.reason).slice(0, 140)
  }),
  sudo: (input, t) => ({
    icon: ShieldAlert,
    label: t('Команда администратора'),
    detail: text(input?.description) || text(input?.command)
  }),
  list_mcp_resources: (_input, t) => ({ icon: Plug, label: t('Ресурсы MCP'), detail: '' }),
  read_mcp_resource: (input, t) => ({
    icon: Plug,
    label: t('Ресурс MCP'),
    detail: text(input?.uri)
  }),
  sourcegraph: (input, t) => ({
    icon: Search,
    label: t('Поиск по репозиториям'),
    detail: text(input?.query)
  }),
  search_instructions: (input, t) => ({
    icon: Search,
    label: t('Поиск инструкций'),
    detail: text(input?.query)
  }),
  read_instruction: (input, t) => ({
    icon: FileCode2,
    label: t('Чтение инструкции'),
    detail: text(input?.id)
  }),
  read_tool_documentation: (input, t) => ({
    icon: FileCode2,
    label: t('Чтение документации инструмента'),
    detail: text(input?.tool_name)
  }),
  read_tool_result: (input, t) => ({
    icon: ScrollText,
    label: t('Чтение полного результата'),
    detail: text(input?.artifact_id)
  }),
  read_file: (input, t) => ({
    icon: FileCode2,
    label: t('Чтение'),
    detail: fileName(input?.file_path || input?.path)
  }),
  read: (input, t) => ({
    icon: FileCode2,
    label: t('Чтение'),
    detail: fileName(input?.file_path || input?.path)
  }),
  search_skills: (input, t) => ({
    icon: Search,
    label: t('Поиск навыков'),
    detail: text(input?.query)
  }),
  search_mcp: (input, t) => ({
    icon: Search,
    label: t('Поиск MCP'),
    detail: text(input?.query)
  }),
  search_tools: (input, t) => ({
    icon: Search,
    label: t('Поиск инструментов'),
    detail: text(input?.query)
  })
}

export function toolTitle(name: string, input: any, t: Translate): ToolTitle {
  const tool = name.toLowerCase()
  const path = input?.file_path || input?.path || input?.filePath
  if (tool === 'mcp_call') {
    if (typeof input?.tool === 'string' && input.tool.toLowerCase().startsWith('mcp_')) {
      return toolTitle(input.tool, input.arguments ?? {}, t)
    }
    const server = humanizeSegment(input?.server)
    return {
      icon: Plug,
      label: server ? `MCP · ${server}` : t('Вызов MCP'),
      detail: humanizeSegment(text(input?.tool))
    }
  }
  if (tool.startsWith('mcp_')) {
    const bridge = /^mcp_(?:browser|project|terminal|git|ssh|windows|apps|prime-desktop(?:-[a-z]+)?(?:-\d+)?)_(.+)$/.exec(tool)
    if (bridge) {
      return (
        desktopToolTitle(bridge[1], input, t) ?? {
          icon: Wrench,
          label: t('Prime Desktop'),
          detail: humanizeSegment(bridge[1])
        }
      )
    }
    const [, server = '', ...rest] = name.split('_')
    const formattedServer = humanizeSegment(server)
    return {
      icon: Plug,
      label: formattedServer ? `MCP · ${formattedServer}` : t('Вызов MCP'),
      detail: humanizeSegment(rest.join('_'))
    }
  }
  const desktop = desktopToolTitle(tool, input, t)
  if (desktop) return desktop
  const named = namedTools[tool]
  if (named) return named(input, t)
  if (/^(view|read|cat)$/.test(tool))
    return { icon: FileCode2, label: t('Чтение'), detail: fileName(path) }
  if (tool === 'write') return { icon: FilePlus2, label: t('Запись'), detail: fileName(path) }
  if (/^multi|^edit|replace/.test(tool))
    return { icon: FilePen, label: t('Правка'), detail: fileName(path) }
  if (tool === 'ls')
    return { icon: FolderTree, label: t('Список файлов'), detail: text(path) || '.' }
  if (tool === 'glob')
    return { icon: Search, label: t('Поиск файлов'), detail: text(input?.pattern) }
  if (/grep|^rg$|^rg-/.test(tool))
    return { icon: Search, label: t('Поиск в коде'), detail: text(input?.pattern || input?.query) }
  if (/^bash$|^shell$/.test(tool))
    return {
      icon: Terminal,
      label: t('Команда'),
      detail: text(input?.description) || text(input?.command)
    }
  if (/^web_?search$/.test(tool))
    return { icon: Globe, label: t('Поиск в интернете'), detail: text(input?.query) }
  if (/^agentic_?fetch$/.test(tool)) {
    const url = text(input?.url)
    return url
      ? { icon: Globe, label: t('Анализ страницы'), detail: url }
      : { icon: Globe, label: t('Поиск в интернете'), detail: text(input?.prompt) }
  }
  if (/^(?:web_?)?fetch$|^download$/.test(tool))
    return { icon: Download, label: t('Загрузка'), detail: text(input?.url) }
  if (/^task$/.test(tool))
    return {
      icon: Sparkles,
      label: t('Подзадача'),
      detail: text(input?.description) || text(input?.prompt)
    }
  if (/todo/.test(tool)) return { icon: ListChecks, label: t('План задач'), detail: '' }
  if (/lsp_/.test(tool)) return { icon: Wrench, label: `LSP · ${humanizeSegment(tool.slice(4))}`, detail: '' }
  return { icon: Wrench, label: humanizeToolName(name) || t('Инструмент'), detail: '' }
}
