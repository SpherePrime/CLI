import {
  Download,
  FileCode2,
  FilePen,
  FilePlus2,
  FolderTree,
  Globe,
  ListChecks,
  Plug,
  Search,
  Sparkles,
  Terminal,
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

export function toolTitle(name: string, input: any, t: Translate): ToolTitle {
  const tool = name.toLowerCase()
  const path = input?.file_path || input?.path || input?.filePath
  if (tool.startsWith('mcp_')) {
    const [, server = '', ...rest] = name.split('_')
    const desktop = server === 'browser' ? desktopToolTitle(rest.join('_'), input, t) : undefined
    if (desktop) return desktop
    return { icon: Plug, label: `MCP · ${server}`, detail: rest.join('_') }
  }
  if (/^(view|read|cat)$/.test(tool))
    return { icon: FileCode2, label: t('Чтение'), detail: fileName(path) }
  if (tool === 'write') return { icon: FilePlus2, label: t('Запись'), detail: fileName(path) }
  if (/edit|replace/.test(tool))
    return { icon: FilePen, label: t('Правка'), detail: fileName(path) }
  if (tool === 'ls')
    return { icon: FolderTree, label: t('Список файлов'), detail: text(path) || '.' }
  if (tool === 'glob')
    return { icon: Search, label: t('Поиск файлов'), detail: text(input?.pattern) }
  if (/grep|rg|search/.test(tool) && !/web/.test(tool))
    return { icon: Search, label: t('Поиск в коде'), detail: text(input?.pattern || input?.query) }
  if (/bash|shell|job/.test(tool))
    return {
      icon: Terminal,
      label: t('Команда'),
      detail: text(input?.description) || text(input?.command)
    }
  if (/web_search|websearch/.test(tool))
    return { icon: Globe, label: t('Поиск в интернете'), detail: text(input?.query) }
  if (/fetch|download/.test(tool))
    return { icon: Download, label: t('Загрузка'), detail: text(input?.url) }
  if (/agent|task/.test(tool))
    return {
      icon: Sparkles,
      label: t('Подзадача'),
      detail: text(input?.description) || text(input?.prompt)
    }
  if (/todo/.test(tool)) return { icon: ListChecks, label: t('План задач'), detail: '' }
  return { icon: Wrench, label: name || t('Инструмент'), detail: '' }
}
