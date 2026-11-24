import { useMemo, useState } from 'react'
import { Check, ChevronDown, Clock3, LoaderCircle, Terminal, X } from 'lucide-react'
import type { Message } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'
import { backgroundJob } from './backgroundJob'
import { hidesToolOutput } from './hiddenToolOutput'
import { ToolBody } from './ToolBody'
import { buildDiffMeter, extractToolDiffStats } from './toolDiff'
import { classifyToolCategory, parseToolOutput } from './toolOutputParser'
import { toolTitle } from './toolTitle'
import './toolCard.css'

export { backgroundJob }

function parameters(value: unknown) {
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(value)
  } catch {
    return value
  }
}

export type ToolCardProps = {
  part: Message['parts'][number]
  messageFinished?: boolean
  grouped?: boolean
  isLast?: boolean
}

export function ToolCard({
  part,
  messageFinished = false,
  grouped = false,
  isLast = false
}: ToolCardProps) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const data = part.data
  const isShell = part.type === 'shell_command'
  const standalone = part.type === 'tool_result'
  const result = standalone ? data : data.result
  const input = useMemo(() => parameters(data.input), [data.input])
  const toolName = String(data.name || '')
  const command = isShell ? data.command : typeof input?.command === 'string' ? input.command : null
  const description = typeof input?.description === 'string' ? input.description : ''
  const interrupted = !isShell && result === undefined && Boolean(messageFinished)
  const completed = isShell || result !== undefined || interrupted
  const running = !completed
  const failed = Boolean(result?.is_error || (isShell && data.exit_code) || interrupted)
  const output = isShell ? data.output : result?.content
  const outputHidden = !isShell && !failed && hidesToolOutput(toolName, input)
  const job = isShell ? '' : backgroundJob(result?.metadata)
  const title = isShell
    ? { icon: Terminal, label: t('Команда'), detail: String(data.command || '') }
    : toolTitle(toolName, input, t)
  const Icon = title.icon
  const category = classifyToolCategory(toolName, isShell)
  const diffStats = useMemo(
    () => (!standalone ? extractToolDiffStats(toolName, input) : null),
    [standalone, toolName, input]
  )
  const diffMeter = useMemo(() => (diffStats ? buildDiffMeter(diffStats) : null), [diffStats])
  const parsedOutput = useMemo(
    () => (failed && !isShell && typeof output === 'string' ? parseToolOutput(output) : null),
    [failed, isShell, output]
  )
  const exitCode = isShell && data.exit_code ? Number(data.exit_code) : parsedOutput?.exitCode ?? null

  const card = (
    <details
      className={`tool-card cat-${category}${grouped ? ' is-grouped' : ''}${failed ? ' tool-error is-error' : ''}${running ? ' is-running' : ''}`}
      onToggle={(event) => setOpen(event.currentTarget.open)}
    >
      <summary>
        <span className="tool-badge tool-icon" aria-hidden="true">
          <Icon size={13} />
        </span>
        <span className="tool-title" title={data.name}>
          <span className="tool-label">{title.label}</span>
          {title.detail && (
            <>
              {' '}
              <code className="tool-detail">{title.detail}</code>
            </>
          )}
        </span>
        <span className="tool-right">
          {diffStats && diffMeter && (
            <span className="tool-diff-pill">
              <span className="tool-diff-meter" aria-hidden="true">
                {diffMeter.map((slot, index) => (
                  <i key={index} className={`is-${slot}`} />
                ))}
              </span>
              {diffStats.added > 0 && <span className="tool-num-add">+{diffStats.added}</span>}
              {diffStats.removed > 0 && <span className="tool-num-del">−{diffStats.removed}</span>}
            </span>
          )}
          {failed && exitCode !== null && exitCode > 0 && (
            <span className="tool-exit-badge">exit {exitCode}</span>
          )}
          {failed && <small className="tool-error-pill">{t('Ошибка')}</small>}
          {job && <small className="tool-background">{t('В фоне · {id}', { id: job })}</small>}
          <span
            className={`tool-status ${failed ? 'failed' : job ? 'background' : completed ? 'done' : 'running'}`}
            title={
              failed
                ? t('Ошибка')
                : job
                  ? t('Выполняется в фоне')
                  : completed
                    ? t('Готово')
                    : t('Выполняется')
            }
          >
            {failed ? (
              <X size={12} strokeWidth={2.5} />
            ) : job ? (
              <Clock3 size={12} strokeWidth={2.5} />
            ) : completed ? (
              <Check size={12} strokeWidth={2.5} />
            ) : (
              <LoaderCircle size={12} className="spin" />
            )}
          </span>
          <ChevronDown size={13} className="tool-chevron" />
        </span>
      </summary>
      {open && (
        <ToolBody
          name={toolName}
          isShell={isShell}
          standalone={standalone}
          input={input}
          description={description}
          command={command}
          output={output}
          outputHidden={outputHidden}
          completed={completed}
          interrupted={interrupted}
          failed={failed}
          exitCode={isShell ? Number(data.exit_code) : exitCode}
        />
      )}
    </details>
  )

  if (!grouped) {
    return card
  }

  return (
    <div
      className={`tool-tree-item cat-${category}${isLast ? ' is-last' : ''}${failed ? ' is-error' : ''}${running ? ' is-running' : ''}`}
    >
      <span className="tool-tree-node" aria-hidden="true" />
      {card}
    </div>
  )
}
