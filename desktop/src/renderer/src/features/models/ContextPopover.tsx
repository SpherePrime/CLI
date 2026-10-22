import { useEffect, useState, type RefObject } from 'react'
import { Archive, LoaderCircle } from 'lucide-react'
import { Popover } from '../../components/Popover'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'
import { compactTokens } from './ContextMeter'

type Category = { key: string; tokens: number; items: number }
type Breakdown = {
  categories: Category[]
  estimated_total: number
  reported_total: number
  context_window: number
  auto_summarize_percent: number
  estimated_usage: boolean
}

/**
 * Totals for the context popover. The meter and header use the provider's
 * reported usage of the latest request when it exists (the true current
 * context); the category list keeps the estimated proportions unscaled so
 * its percentages always sum to 100% of the estimate instead of inflating
 * with the reported-vs-estimate ratio.
 */
export function contextTotals(data: Breakdown | null, fallbackWindow: number) {
  const capacity = data?.context_window || fallbackWindow
  const estimated = data?.estimated_total || 0
  const reported = data?.reported_total || 0
  const total = reported || estimated
  const shareOfEstimate = estimated ? (category: Category) => category.tokens / estimated : () => 0
  const percentOfTotal = (value: number) => (capacity ? (value / capacity) * 100 : 0)
  return {
    capacity,
    estimated,
    total,
    estimatedUsage: Boolean(data?.estimated_usage),
    shareOfEstimate,
    percentOfTotal,
    threshold: data?.auto_summarize_percent || 0,
    free: Math.max(0, capacity - total)
  }
}

const colors: Record<string, string> = {
  system_prompt: '#ff6a35',
  mcp_instructions: '#f59e0b',
  tools: '#8b5cf6',
  mcp_tools: '#a78bfa',
  summary: '#14b8a6',
  user_messages: '#60a5fa',
  assistant_messages: '#34d399',
  reasoning: '#f472b6',
  tool_calls: '#facc15',
  tool_results: '#38bdf8',
  attachments: '#94a3b8'
}

export function ContextPopover({
  workspace: w,
  anchor,
  close,
  fallbackWindow
}: {
  workspace: Workspace
  anchor: RefObject<HTMLElement | null>
  close: () => void
  fallbackWindow: number
}) {
  const { t, locale } = useI18n()
  const [data, setData] = useState<Breakdown | null>(null)
  const [error, setError] = useState('')
  const [compacting, setCompacting] = useState(false)
  const sessionId = w.session?.id
  const tokens = (w.session?.prompt_tokens || 0) + (w.session?.completion_tokens || 0)
  useEffect(() => {
    let current = true
    if (!sessionId) return
    void w
      .call('context', { sessionId })
      .then((result) => {
        if (current)
          setData(
            result && Array.isArray(result.categories) ? result : { ...result, categories: [] }
          )
      })
      .catch((reason) => {
        if (current) setError(reason?.message || String(reason))
      })
    return () => {
      current = false
    }
  }, [sessionId, tokens])
  const labels: Record<string, string> = {
    system_prompt: t('Системный промпт'),
    mcp_instructions: t('Инструкции MCP'),
    tools: t('Инструменты'),
    mcp_tools: t('Инструменты MCP'),
    summary: t('Сводка после сжатия'),
    user_messages: t('Ваши сообщения'),
    assistant_messages: t('Ответы Prime'),
    reasoning: t('Размышления'),
    tool_calls: t('Вызовы инструментов'),
    tool_results: t('Результаты инструментов'),
    attachments: t('Вложения')
  }
  const totals = contextTotals(data, fallbackWindow)
  const { capacity, total, estimated, estimatedUsage, threshold } = totals
  const percentOf = totals.percentOfTotal
  const reported = data?.reported_total || 0
  // Bars show each category's share of the window using the reported total,
  // keeping the bar and header consistent even when the estimate differs.
  const barWidth = (category: Category) => {
    if (!capacity || !estimated) return 0
    return ((category.tokens / estimated) * (total / capacity)) * 100
  }
  return (
    <Popover anchor={anchor} close={close} width={340} align="end" label={t('Контекст')}>
      <div className="context-popover">
        <div className="context-popover-head">
          <strong>{t('Контекст')}</strong>
          <span>
            {compactTokens(total)} / {compactTokens(capacity)} · {Math.round(percentOf(total))}%
          </span>
        </div>
        <div className="context-bar" aria-hidden="true">
          {data?.categories.map((category) => (
            <span
              key={category.key}
              style={{
                width: `${Math.min(100, barWidth(category))}%`,
                background: colors[category.key]
              }}
            />
          ))}
          {threshold > 0 && threshold < 100 && (
            <i className="context-bar-threshold" style={{ left: `${threshold}%` }} />
          )}
        </div>
        {threshold > 0 && threshold < 100 && (
          <p className="context-note">
            {t('Автосжатие при {percent}% · свободно {free}', {
              percent: threshold,
              free: compactTokens(totals.free)
            })}
          </p>
        )}
        {w.session && (
          <p className="context-note context-usage">
            {t('{value0} токенов · ${value1}', {
              value0: (w.session.prompt_tokens || 0).toLocaleString(locale),
              value1: (w.session.cost || 0).toLocaleString(locale, {
                minimumFractionDigits: 4,
                maximumFractionDigits: 4
              })
            })}
          </p>
        )}
        {error && <p className="error-text">{error}</p>}
        {!data && !error && (
          <p className="context-note">
            <LoaderCircle size={13} className="spin" /> {t('Считаем…')}
          </p>
        )}
        {data && (
          <ul className="context-list">
            {data.categories.map((category) => (
              <li key={category.key}>
                <i style={{ background: colors[category.key] }} />
                <span className="context-list-label">
                  {labels[category.key] || category.key}
                  {category.items > 1 && <small>×{category.items}</small>}
                </span>
                <span className="context-list-value">
                  {compactTokens(Math.round(category.tokens))}
                </span>
                <span className="context-list-percent">
                  {(totals.shareOfEstimate(category) * 100).toFixed(1)}%
                </span>
              </li>
            ))}
          </ul>
        )}
        {data && (
          <p className="context-note">
            {estimatedUsage && reported
              ? t('Расход оценён, так как провайдер не сообщил точные токены.')
              : data.reported_total
                ? t('Итог — по данным провайдера, разбивка приблизительная.')
                : t('Провайдер не сообщил расход, показана оценка.')}
          </p>
        )}
        <button
          className="context-compact"
          disabled={!sessionId || w.busy || compacting || !total}
          onClick={() => {
            setCompacting(true)
            void w
              .call('summarize', { sessionId })
              .then(w.refresh)
              .then(close)
              .catch(w.errorHandler)
              .finally(() => setCompacting(false))
          }}
        >
          {compacting ? <LoaderCircle size={14} className="spin" /> : <Archive size={14} />}
          {t('Сжать контекст сейчас')}
        </button>
      </div>
    </Popover>
  )
}
