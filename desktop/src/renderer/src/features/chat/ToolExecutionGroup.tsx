import { useMemo, useState } from 'react'
import { AlertCircle, Check, ChevronDown, LoaderCircle } from 'lucide-react'
import type { Message } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'
import { AgentCard } from './AgentCard'
import { backgroundJob } from './backgroundJob'
import { ToolCard } from './ToolCard'
import { extractToolDiffStats } from './toolDiff'
import { classifyToolCategory, type ToolCategory } from './toolOutputParser'

export interface ToolExecutionGroupProps {
  parts: NonNullable<Message['parts']>
  messageId?: string
  sessionId?: string
  messageFinished?: boolean
}

function parseInputObject(value: unknown): any {
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(value)
  } catch {
    return value
  }
}

function pluralRu(count: number, one: string, few: string, many: string): string {
  const mod100 = count % 100
  const mod10 = count % 10
  if (mod100 >= 11 && mod100 <= 19) return `${count} ${many}`
  if (mod10 === 1) return `${count} ${one}`
  if (mod10 >= 2 && mod10 <= 4) return `${count} ${few}`
  return `${count} ${many}`
}

function categoryChipLabel(category: ToolCategory, count: number, lang: string): string {
  if (lang !== 'ru') {
    const names: Record<ToolCategory, [string, string]> = {
      edit: ['edit', 'edits'],
      write: ['file', 'files'],
      read: ['read', 'reads'],
      command: ['cmd', 'cmds'],
      search: ['search', 'searches'],
      browser: ['browser', 'browser'],
      task: ['agent', 'agents'],
      other: ['step', 'steps']
    }
    const [singular, plural] = names[category]
    return `${count} ${count === 1 ? singular : plural}`
  }
  switch (category) {
    case 'edit':
      return pluralRu(count, 'правка', 'правки', 'правок')
    case 'write':
      return pluralRu(count, 'файл', 'файла', 'файлов')
    case 'read':
      return pluralRu(count, 'чтение', 'чтения', 'чтений')
    case 'command':
      return pluralRu(count, 'команда', 'команды', 'команд')
    case 'search':
      return pluralRu(count, 'поиск', 'поиска', 'поисков')
    case 'browser':
      return pluralRu(count, 'браузер', 'браузера', 'браузеров')
    case 'task':
      return pluralRu(count, 'агент', 'агента', 'агентов')
    default:
      return pluralRu(count, 'шаг', 'шага', 'шагов')
  }
}

function summarizeGroup(parts: NonNullable<Message['parts']>, messageFinished: boolean) {
  let running = 0
  let failed = 0
  let background = 0
  let added = 0
  let removed = 0
  const counts = new Map<ToolCategory, number>()

  for (const part of parts) {
    const data = part.data || {}
    const isShell = part.type === 'shell_command'
    const standalone = part.type === 'tool_result'
    const name = String(data.name || '')
    const input = parseInputObject(data.input)
    const result = standalone ? data : data.result
    const interrupted = !isShell && result === undefined && messageFinished
    const completed = isShell || result !== undefined || interrupted
    const isFailed = Boolean(result?.is_error || (isShell && data.exit_code) || interrupted)
    const job = isShell ? '' : backgroundJob(result?.metadata)

    if (!completed) running += 1
    if (isFailed) failed += 1
    if (job) background += 1

    const category = classifyToolCategory(name, isShell)
    counts.set(category, (counts.get(category) || 0) + 1)

    if (!standalone) {
      const diffStats = extractToolDiffStats(name, input)
      if (diffStats) {
        added += diffStats.added
        removed += diffStats.removed
      }
    }
  }

  return { running, failed, background, added, removed, counts }
}

export function ToolExecutionGroup({
  parts,
  messageId = '',
  sessionId,
  messageFinished = false
}: ToolExecutionGroupProps) {
  const { t, language } = useI18n()
  const stats = useMemo(() => summarizeGroup(parts, messageFinished), [parts, messageFinished])
  const [open, setOpen] = useState(false)

  const heading =
    stats.running > 0
      ? language === 'ru'
        ? `Выполняется · ${pluralRu(parts.length, 'действие', 'действия', 'действий')}`
        : `Running · ${parts.length} ${parts.length === 1 ? 'action' : 'actions'}`
      : stats.failed > 0
        ? language === 'ru'
          ? `Выполнено с ошибками · ${pluralRu(parts.length, 'действие', 'действия', 'действий')}`
          : `Completed with errors · ${parts.length} ${parts.length === 1 ? 'action' : 'actions'}`
        : language === 'ru'
          ? `Выполнено ${pluralRu(parts.length, 'действие', 'действия', 'действий')}`
          : `Completed ${parts.length} ${parts.length === 1 ? 'action' : 'actions'}`

  const orderedCategories: ToolCategory[] = [
    'edit',
    'write',
    'command',
    'search',
    'read',
    'browser',
    'task',
    'other'
  ]
  const hasDiffChip = stats.added > 0 || stats.removed > 0
  const maxCategoryChips = hasDiffChip ? 2 : 3
  const chips = orderedCategories
    .filter((category) => (stats.counts.get(category) || 0) > 0)
    .slice(0, maxCategoryChips)
    .map((category) => ({
      category,
      label: categoryChipLabel(category, stats.counts.get(category) || 0, language)
    }))

  return (
    <details
      className={`tool-exec-group${stats.failed > 0 ? ' has-error' : ''}${stats.running > 0 ? ' is-running' : ''}`}
      open={open}
      onToggle={(event) => {
        const nextOpen = event.currentTarget.open
        if (nextOpen !== open) {
          setOpen(nextOpen)
        }
      }}
    >
      <summary className="tool-exec-group-summary">
        <span
          className={`tool-exec-status${stats.failed > 0 ? ' is-error' : stats.running > 0 ? ' is-running' : ' is-done'}`}
          aria-hidden="true"
        >
          {stats.running > 0 ? (
            <LoaderCircle size={13} className="spin" />
          ) : stats.failed > 0 ? (
            <AlertCircle size={13} />
          ) : (
            <Check size={13} />
          )}
        </span>
        <span className="tool-exec-heading">{heading}</span>
        <span className="tool-exec-chips">
          {(stats.added > 0 || stats.removed > 0) && (
            <span className="tool-exec-chip is-diff">
              {stats.added > 0 && <span className="tool-num-add">+{stats.added}</span>}
              {stats.removed > 0 && <span className="tool-num-del">−{stats.removed}</span>}
            </span>
          )}
          {chips.map((chip) => (
            <span key={chip.category} className={`tool-exec-chip is-${chip.category}`}>
              {chip.label}
            </span>
          ))}
        </span>
        <ChevronDown
          size={14}
          className="tool-exec-chevron"
          aria-label={open ? t('Свернуть') : t('Показать')}
        />
      </summary>

      <div className="tool-exec-tree">
        {parts.map((part, index) => {
          if (part.type === 'tool_call' && part.data?.name === 'agent') {
            const targetMessageId = String((part.data as { _sourceMessageId?: string })._sourceMessageId || messageId)
            return (
              <div
                key={index}
                className={`tool-tree-node cat-task${index === parts.length - 1 ? ' is-last' : ''}`}
              >
                <span className="tool-tree-rail" aria-hidden="true">
                  <span className="tool-tree-dot" />
                </span>
                <div className="tool-tree-card">
                  <AgentCard
                    part={part}
                    messageId={targetMessageId}
                    sessionId={sessionId}
                    messageFinished={messageFinished}
                  />
                </div>
              </div>
            )
          }
          return (
            <ToolCard
              key={index}
              part={part}
              messageFinished={messageFinished}
              grouped
              isLast={index === parts.length - 1}
            />
          )
        })}
      </div>
    </details>
  )
}
