import {
  Download,
  FileCode2,
  FilePen,
  FilePlus2,
  FolderTree,
  Globe,
  HelpCircle,
  Image,
  Info,
  ListChecks,
  ListTree,
  Network,
  Plug,
  ScrollText,
  Search,
  Sparkles,
  Terminal,
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

// Exact labels win over the regex fallbacks below, so names like
// search_instructions do not get swallowed by the generic code search rule.
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
  job_output: (_input, t) => ({ icon: Terminal, label: t('Вывод фоновой задачи'), detail: '' }),
  job_kill: (_input, t) => ({ icon: Terminal, label: t('Остановка задачи'), detail: '' }),
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
  if (tool.startsWith('mcp_')) {
    const bridge = /^mcp_(?:browser|prime-desktop(?:-\d+)?)_(.+)$/.exec(tool)
    if (bridge) {
      return (
        desktopToolTitle(bridge[1], input, t) ?? {
          icon: Wrench,
          label: t('Prime Desktop'),
          detail: bridge[1].replaceAll('_', ' ')
        }
      )
    }
    const [, server = '', ...rest] = name.split('_')
    return { icon: Plug, label: `MCP · ${server}`, detail: rest.join('_') }
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
  if (/^agentic_?fetch$/.test(tool))
    return {
      icon: Globe,
      label: t('Анализ страницы'),
      detail: text(input?.url) || text(input?.prompt)
    }
  if (/^fetch$|^download$/.test(tool))
    return { icon: Download, label: t('Загрузка'), detail: text(input?.url) }
  if (/^task$/.test(tool))
    return {
      icon: Sparkles,
      label: t('Подзадача'),
      detail: text(input?.description) || text(input?.prompt)
    }
  if (/todo/.test(tool)) return { icon: ListChecks, label: t('План задач'), detail: '' }
  if (/lsp_/.test(tool)) return { icon: Wrench, label: `LSP · ${tool.slice(4)}`, detail: '' }
  return { icon: Wrench, label: name || t('Инструмент'), detail: '' }
}
