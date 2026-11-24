import { useEffect, useMemo, useRef, useState } from 'react'
import { Check, ChevronRight, Copy, FileCode, Folder, Globe, Terminal } from 'lucide-react'
import { useI18n } from '../../i18n'
import { extractToolDiff, tokenizeCodeLine, type DiffBlock } from './toolDiff'
import { parseToolOutput, splitFilePath } from './toolOutputParser'

export interface ToolBodyProps {
  name: string
  isShell: boolean
  standalone: boolean
  input: any
  description: string
  command: string | null
  output: unknown
  outputHidden: boolean
  completed: boolean
  interrupted: boolean
  failed: boolean
  exitCode?: number | null
}

function CopyButton({ value, label }: { value: string; label: string }) {
  const [copied, setCopied] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  useEffect(() => () => clearTimeout(timerRef.current), [])
  return (
    <button
      type="button"
      className={`tool-mini-btn${copied ? ' is-copied' : ''}`}
      onClick={(event) => {
        event.stopPropagation()
        void navigator.clipboard
          ?.writeText(value)
          .then(() => {
            setCopied(true)
            clearTimeout(timerRef.current)
            timerRef.current = setTimeout(() => setCopied(false), 1400)
          })
          .catch(() => {})
      }}
    >
      {copied ? <Check size={11} /> : <Copy size={11} />}
      <span>{copied ? '✓' : label}</span>
    </button>
  )
}

function SyntaxLine({ text }: { text: string }) {
  const tokens = tokenizeCodeLine(text)
  if (tokens.length === 0) return <>&nbsp;</>
  return (
    <>
      {tokens.map((token, index) =>
        token.kind === 'plain' ? (
          <span key={index}>{token.text}</span>
        ) : (
          <span key={index} className={`tool-syn-${token.kind}`}>
            {token.text}
          </span>
        )
      )}
    </>
  )
}

function formatRawOutput(output: unknown, interrupted: boolean, t: (text: string) => string): string {
  if (interrupted) return t('Выполнение прервано до получения результата')
  if (typeof output === 'string') return output
  if (output === undefined || output === null) return ''
  return JSON.stringify(output, null, 2)
}

function DiagnosticsStrip({
  diagnostics,
  summary,
  t
}: {
  diagnostics: string | null
  summary: string | null
  t: (text: string) => string
}) {
  if (!diagnostics && !summary) return null
  return (
    <details className="tool-diag-strip">
      <summary>
        <ChevronRight size={12} className="tool-diag-chevron" />
        <span>
          {t('Диагностика')}
          {summary ? ` · ${summary}` : ''}
        </span>
      </summary>
      {diagnostics && <pre>{diagnostics}</pre>}
    </details>
  )
}

function DiffView({
  diff,
  filePath,
  parsed,
  completed,
  outputHidden,
  t
}: {
  diff: DiffBlock
  filePath: string
  parsed: ReturnType<typeof parseToolOutput>
  completed: boolean
  outputHidden: boolean
  t: (text: string) => string
}) {
  const [tab, setTab] = useState<'diff' | 'output'>('diff')
  const { dir, base } = splitFilePath(filePath)
  const hasSecondaryOutput = completed && !outputHidden && Boolean(parsed.cleanText)

  return (
    <div className="tool-frame">
      <div className="tool-frame-bar">
        <div className="tool-frame-file" title={filePath}>
          <FileCode size={12} />
          <span className="tool-frame-path">
            {dir && <span className="tool-path-dir">{dir}</span>}
            <span className="tool-path-base">{base || filePath}</span>
          </span>
        </div>
        <div className="tool-frame-actions">
          <span className="tool-diff-nums">
            <span className="tool-num-add">+{diff.added}</span>
            <span className="tool-num-del">−{diff.removed}</span>
          </span>
          {hasSecondaryOutput && (
            <div className="tool-tab-pair">
              <button
                type="button"
                className={`tool-mini-btn${tab === 'diff' ? ' is-active' : ''}`}
                onClick={() => setTab('diff')}
              >
                Diff
              </button>
              <button
                type="button"
                className={`tool-mini-btn${tab === 'output' ? ' is-active' : ''}`}
                onClick={() => setTab('output')}
              >
                {t('Вывод')}
              </button>
            </div>
          )}
        </div>
      </div>

      {tab === 'diff' ? (
        <div className="tool-diff-grid">
          {diff.lines.map((line, index) => (
            <div key={index} className={`tool-diff-line is-${line.kind}`}>
              <span className="tool-diff-ln">{line.oldLine ?? ''}</span>
              <span className="tool-diff-ln">{line.newLine ?? ''}</span>
              <span className="tool-diff-sign">
                {line.kind === 'add' ? '+' : line.kind === 'del' ? '−' : ''}
              </span>
              <span className="tool-diff-code">
                <SyntaxLine text={line.text} />
              </span>
            </div>
          ))}
          {diff.truncated && (
            <div className="tool-diff-truncated">{t('Показаны первые 240 строк изменений')}</div>
          )}
        </div>
      ) : (
        <pre className="tool-terminal-out">{parsed.cleanText}</pre>
      )}

      <DiagnosticsStrip diagnostics={parsed.diagnostics} summary={parsed.diagnosticSummary} t={t} />
    </div>
  )
}

function CommandView({
  command,
  description,
  parsed,
  completed,
  outputHidden,
  failed,
  isShell,
  exitCode,
  t
}: {
  command: string | null
  description: string
  parsed: ReturnType<typeof parseToolOutput>
  completed: boolean
  outputHidden: boolean
  failed: boolean
  isShell: boolean
  exitCode?: number | null
  t: (text: string, values?: Record<string, string | number>) => string
}) {
  return (
    <div className={`tool-frame${failed ? ' is-error' : ''}`}>
      {description && <div className="tool-frame-desc">{description}</div>}
      {command && (
        <div className="tool-terminal-cmd">
          <span className="tool-terminal-prompt">$</span>
          <code title={command}>{command}</code>
          <CopyButton value={command} label={t('Копировать')} />
        </div>
      )}
      {parsed.cwd && (
        <div className="tool-terminal-cwd">
          <Folder size={11} />
          <span>{parsed.cwd}</span>
        </div>
      )}
      {completed && !outputHidden && (
        <pre className={`tool-terminal-out${failed ? ' is-error' : ''}`}>
          {parsed.cleanText || t('Без вывода')}
        </pre>
      )}
      {isShell && exitCode !== undefined && exitCode !== null && (
        <div className="tool-terminal-cwd">
          {t('Код выхода: {code}', { code: exitCode })}
        </div>
      )}
    </div>
  )
}

function FileContentView({
  filePath,
  content,
  parsed,
  completed,
  outputHidden,
  t
}: {
  filePath: string
  content: string
  parsed: ReturnType<typeof parseToolOutput>
  completed: boolean
  outputHidden: boolean
  t: (text: string) => string
}) {
  const { dir, base } = splitFilePath(filePath)
  const displayCode = content || (completed && !outputHidden ? parsed.cleanText : '')
  const lines = displayCode ? displayCode.replace(/\r\n/g, '\n').split('\n') : []
  const capped = lines.length > 160 ? lines.slice(0, 160) : lines

  return (
    <div className="tool-frame">
      {filePath && (
        <div className="tool-frame-bar">
          <div className="tool-frame-file" title={filePath}>
            <FileCode size={12} />
            <span className="tool-frame-path">
              {dir && <span className="tool-path-dir">{dir}</span>}
              <span className="tool-path-base">{base || filePath}</span>
            </span>
          </div>
          <div className="tool-frame-actions">
            {lines.length > 0 && <span className="tool-line-count">{lines.length} ln</span>}
            {displayCode && <CopyButton value={displayCode} label={t('Копировать')} />}
          </div>
        </div>
      )}
      {capped.length > 0 && <pre className="tool-terminal-out">{capped.join('\n')}</pre>}
      {lines.length > capped.length && (
        <div className="tool-diff-truncated">
          {t('Показаны первые 160 строк')} ({lines.length})
        </div>
      )}
      <DiagnosticsStrip diagnostics={parsed.diagnostics} summary={parsed.diagnosticSummary} t={t} />
    </div>
  )
}

function SearchView({
  input,
  parsed,
  completed,
  outputHidden,
  t
}: {
  input: any
  parsed: ReturnType<typeof parseToolOutput>
  completed: boolean
  outputHidden: boolean
  t: (text: string) => string
}) {
  const chips: Array<{ key: string; value: string }> = []
  if (input && typeof input === 'object') {
    for (const [key, label] of [
      ['pattern', t('Паттерн')],
      ['query', t('Запрос')],
      ['path', t('Путь')],
      ['include', t('Маска')]
    ]) {
      const value = input[key]
      if (typeof value === 'string' && value.trim()) {
        chips.push({ key: label, value: value.trim() })
      }
    }
  }
  return (
    <div className="tool-frame">
      {chips.length > 0 && (
        <div className="tool-param-chips">
          {chips.map((chip) => (
            <span key={`${chip.key}-${chip.value}`} className="tool-param-chip">
              <span className="tool-param-chip-key">{chip.key}</span>
              <code>{chip.value}</code>
            </span>
          ))}
        </div>
      )}
      {completed && !outputHidden && (
        <pre className="tool-terminal-out">{parsed.cleanText || t('Без вывода')}</pre>
      )}
    </div>
  )
}

function formatScenarioStepTarget(step: Record<string, unknown>): string {
  const action = String(step.action || '')
  const singleLine = (raw: unknown) =>
    typeof raw === 'string' ? raw.replace(/\s+/g, ' ').trim() : ''
  switch (action) {
    case 'resize':
      return step.width !== undefined ? `${step.width}px` : ''
    case 'navigate':
      return singleLine(step.url)
    case 'wait':
      return step.ms !== undefined ? `${step.ms} ms` : ''
    case 'wait_for':
      return [singleLine(step.selector), singleLine(step.text), step.gone ? 'gone' : '']
        .filter(Boolean)
        .join(' · ')
    case 'click':
    case 'hover':
    case 'focus':
      return (
        singleLine(step.selector) ||
        singleLine(step.text) ||
        (step.x !== undefined && step.y !== undefined ? `${step.x}, ${step.y}` : '')
      )
    case 'type':
      return [singleLine(step.selector), singleLine(step.text)].filter(Boolean).join(' ← ')
    case 'select':
      return [singleLine(step.selector), singleLine(step.option ?? step.value)]
        .filter(Boolean)
        .join(' = ')
    case 'press_key':
      return [singleLine(step.key), step.holdMs ? `${step.holdMs} ms` : '']
        .filter(Boolean)
        .join(' · ')
    case 'scroll':
      return (
        singleLine(step.to) ||
        (step.y !== undefined ? `${step.x ?? 0}, ${step.y}` : '')
      )
    case 'assert':
      return [
        singleLine(step.selector),
        singleLine(step.contains ?? step.text),
        step.absent ? 'absent' : ''
      ]
        .filter(Boolean)
        .join(' · ')
    case 'evaluate':
      return singleLine(step.expression)
    case 'screenshot':
      return [
        singleLine(step.name),
        singleLine(step.selector),
        step.fullPage ? 'fullPage' : '',
        typeof step.zoom === 'number' && step.zoom > 1 ? `zoom ${step.zoom}` : ''
      ]
        .filter(Boolean)
        .join(' · ')
    default: {
      const rest = Object.entries(step)
        .filter(([key, value]) => key !== 'action' && value !== undefined && value !== null && value !== '')
        .map(([key, value]) =>
          `${key}=${typeof value === 'string' ? singleLine(value) : JSON.stringify(value)}`
        )
      return rest.join(' · ')
    }
  }
}

function BrowserScenarioView({
  input,
  parsed,
  completed,
  outputHidden,
  failed,
  t
}: {
  input: any
  parsed: ReturnType<typeof parseToolOutput>
  completed: boolean
  outputHidden: boolean
  failed: boolean
  t: (text: string, values?: Record<string, string | number>) => string
}) {
  const steps = Array.isArray(input?.steps) ? input.steps : []
  const extraChips: Array<{ key: string; value: string }> = []
  if (typeof input?.tabId === 'string' && input.tabId.trim()) {
    extraChips.push({ key: 'tabId', value: input.tabId.trim() })
  }
  if (typeof input?.continueOnError === 'boolean') {
    extraChips.push({ key: 'continueOnError', value: String(input.continueOnError) })
  }
  const stepsJson = useMemo(() => JSON.stringify(steps, null, 2), [steps])

  return (
    <div className={`tool-frame${failed ? ' is-error' : ''}`}>
      <div className="tool-frame-bar">
        <div className="tool-frame-file">
          <Globe size={12} />
          <span className="tool-path-base">{t('Шаги выполнения')}</span>
        </div>
        <div className="tool-frame-actions">
          <span className="tool-line-count">{t('Шагов: {count}', { count: steps.length })}</span>
          {steps.length > 0 && <CopyButton value={stepsJson} label={t('Копировать')} />}
        </div>
      </div>

      {extraChips.length > 0 && (
        <div className="tool-param-chips">
          {extraChips.map((chip) => (
            <span key={chip.key} className="tool-param-chip">
              <span className="tool-param-chip-key">{chip.key}</span>
              <code>{chip.value}</code>
            </span>
          ))}
        </div>
      )}

      {steps.length > 0 && (
        <div className="tool-scenario-list">
          {steps.map((step: any, index: number) => {
            const record = step && typeof step === 'object' ? (step as Record<string, unknown>) : {}
            const action = String(record.action || 'step')
            const target = formatScenarioStepTarget(record)
            return (
              <div key={index} className="tool-scenario-step">
                <span className="tool-scenario-idx">{index + 1}</span>
                <span className="tool-scenario-action">{action}</span>
                {target && (
                  <code className="tool-scenario-target" title={target}>
                    {target}
                  </code>
                )}
              </div>
            )
          })}
        </div>
      )}

      {completed && !outputHidden && parsed.cleanText && (
        <div className="tool-generic-out">
          <div className="tool-section-label">
            <Terminal size={11} />
            <span>{t('Вывод')}</span>
          </div>
          <pre className={`tool-terminal-out${failed ? ' is-error' : ''}`}>{parsed.cleanText}</pre>
        </div>
      )}
      <DiagnosticsStrip diagnostics={parsed.diagnostics} summary={parsed.diagnosticSummary} t={t} />
    </div>
  )
}

function GenericParamsView({
  standalone,
  input,
  parsed,
  completed,
  outputHidden,
  failed,
  t
}: {
  standalone: boolean
  input: any
  parsed: ReturnType<typeof parseToolOutput>
  completed: boolean
  outputHidden: boolean
  failed: boolean
  t: (text: string) => string
}) {
  const entries =
    !standalone && input && typeof input === 'object' && !Array.isArray(input)
      ? Object.entries(input).filter(([, value]) => value !== undefined && value !== null && value !== '')
      : []

  const hasRawFallback =
    !standalone && entries.length === 0 && input !== undefined && input !== null && input !== ''

  return (
    <div className={`tool-frame${failed ? ' is-error' : ''}`}>
      {entries.length > 0 && (
        <div className="tool-kv-list">
          {entries.map(([key, value]) => {
            const formatted =
              typeof value === 'string'
                ? value
                : typeof value === 'number' || typeof value === 'boolean'
                  ? String(value)
                  : JSON.stringify(value, null, 2)
            return (
              <div key={key} className="tool-kv-row">
                <span className="tool-kv-key">{key}</span>
                <code className="tool-kv-val">{formatted}</code>
              </div>
            )
          })}
        </div>
      )}
      {hasRawFallback && (
        <pre className="tool-terminal-out">
          {typeof input === 'string' ? input : JSON.stringify(input, null, 2)}
        </pre>
      )}
      {completed && !outputHidden && (
        <div className="tool-generic-out">
          <div className="tool-section-label">
            <Terminal size={11} />
            <span>{t('Вывод')}</span>
          </div>
          <pre className={`tool-terminal-out${failed ? ' is-error' : ''}`}>
            {parsed.cleanText || t('Без вывода')}
          </pre>
        </div>
      )}
      <DiagnosticsStrip diagnostics={parsed.diagnostics} summary={parsed.diagnosticSummary} t={t} />
    </div>
  )
}

export function ToolBody({
  name,
  isShell,
  standalone,
  input,
  description,
  command,
  output,
  outputHidden,
  completed,
  interrupted,
  failed,
  exitCode
}: ToolBodyProps) {
  const { t } = useI18n()
  const rawOutputText = useMemo(
    () => formatRawOutput(output, interrupted, t),
    [output, interrupted, t]
  )
  const parsed = useMemo(() => parseToolOutput(rawOutputText), [rawOutputText])
  const diff = useMemo(
    () => (!standalone ? extractToolDiff(name, input) : null),
    [standalone, name, input]
  )
  const filePath = String(input?.file_path || input?.path || input?.filePath || '')

  let bodyNode: React.ReactNode
  if (diff) {
    bodyNode = (
      <DiffView
        diff={diff}
        filePath={filePath}
        parsed={parsed}
        completed={completed}
        outputHidden={outputHidden}
        t={t}
      />
    )
  } else if (
    isShell ||
    name === 'bash' ||
    name === 'sudo' ||
    name === 'job_output' ||
    name === 'job_kill'
  ) {
    bodyNode = (
      <CommandView
        command={command}
        description={description}
        parsed={parsed}
        completed={completed}
        outputHidden={outputHidden}
        failed={failed}
        isShell={isShell}
        exitCode={exitCode}
        t={t}
      />
    )
  } else if (name === 'write' && !standalone) {
    bodyNode = (
      <FileContentView
        filePath={filePath}
        content={typeof input?.content === 'string' ? input.content : ''}
        parsed={parsed}
        completed={completed}
        outputHidden={outputHidden}
        t={t}
      />
    )
  } else if (name === 'view' || name === 'read' || name === 'read_file') {
    bodyNode = (
      <FileContentView
        filePath={filePath}
        content=""
        parsed={parsed}
        completed={completed}
        outputHidden={outputHidden}
        t={t}
      />
    )
  } else if (name === 'grep' || name === 'glob' || name === 'ls' || name === 'web_search') {
    bodyNode = (
      <SearchView
        input={input}
        parsed={parsed}
        completed={completed}
        outputHidden={outputHidden}
        t={t}
      />
    )
  } else if (
    !standalone &&
    (name === 'browser_run' || name.endsWith('_browser_run')) &&
    Array.isArray(input?.steps)
  ) {
    bodyNode = (
      <BrowserScenarioView
        input={input}
        parsed={parsed}
        completed={completed}
        outputHidden={outputHidden}
        failed={failed}
        t={t}
      />
    )
  } else {
    bodyNode = (
      <GenericParamsView
        standalone={standalone}
        input={input}
        parsed={parsed}
        completed={completed}
        outputHidden={outputHidden}
        failed={failed}
        t={t}
      />
    )
  }

  return <div className="tool-body">{bodyNode}</div>
}
