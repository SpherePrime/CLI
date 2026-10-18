import { Check, ChevronDown, FileCode2, Terminal, Wrench, X } from 'lucide-react'
import type { Message } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'

function parameters(value: unknown) {
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(value)
  } catch {
    return value
  }
}

export function ToolCard({ part }: { part: Message['parts'][number] }) {
  const { t } = useI18n()
  const data = part.data
  const isShell = part.type === 'shell_command'
  const standalone = part.type === 'tool_result'
  const result = standalone ? data : data.result
  const input = parameters(data.input)
  const command = isShell ? data.command : typeof input?.command === 'string' ? input.command : null
  const description = typeof input?.description === 'string' ? input.description : ''
  const completed = isShell || result !== undefined
  const failed = Boolean(result?.is_error || (isShell && data.exit_code))
  const output = isShell ? data.output : result?.content
  return (
    <details className={`tool-card ${failed ? 'tool-error' : ''}`}>
      <summary>
        <span className="tool-icon">
          {isShell || /bash|shell/i.test(data.name) ? (
            <Terminal size={15} />
          ) : /edit|write|read|view/i.test(data.name) ? (
            <FileCode2 size={15} />
          ) : (
            <Wrench size={15} />
          )}
        </span>
        <span>{isShell ? t('Команда') : data.name || t('Инструмент')}</span>
        <small>{failed ? t('Ошибка') : completed ? t('Готово') : t('Выполняется')}</small>
        {completed ? (
          failed ? (
            <X size={14} />
          ) : (
            <Check size={14} />
          )
        ) : (
          <span className="dot pulse" />
        )}
        <ChevronDown size={13} />
      </summary>
      {description && <p className="tool-description">{description}</p>}
      {!standalone && (
        <section className="tool-section">
          <h4>{command !== null ? t('Команда') : t('Параметры')}</h4>
          <pre>
            {command ?? (typeof input === 'string' ? input : JSON.stringify(input, null, 2))}
          </pre>
        </section>
      )}
      {completed && (
        <section className="tool-section tool-output">
          <h4>{t('Вывод')}</h4>
          <pre>
            {typeof output === 'string'
              ? output || t('Без вывода')
              : output === undefined
                ? t('Без вывода')
                : JSON.stringify(output, null, 2)}
          </pre>
        </section>
      )}
      {isShell && (
        <small className="tool-exit">{t('Код выхода: {code}', { code: data.exit_code })}</small>
      )}
    </details>
  )
}
