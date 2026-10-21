import { Check, ChevronDown, LoaderCircle, Terminal, X } from 'lucide-react'
import { toolTitle } from './toolTitle'
import { ImageLightbox } from '../../components/ImageLightbox'
import type { Message } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'
import { useState } from 'react'

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
  const [zoomed, setZoomed] = useState(-1)
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
  const gallery = [
    ...(typeof result?.mime_type === 'string' &&
    result.mime_type.startsWith('image/') &&
    typeof result?.data === 'string' &&
    result.data
      ? [{ data: result.data, mime_type: result.mime_type, path: '' }]
      : []),
    ...((Array.isArray(result?.images) ? result.images : []) as any[]).filter(
      (image) =>
        typeof image?.mime_type === 'string' &&
        image.mime_type.startsWith('image/') &&
        typeof image?.data === 'string' &&
        image.data
    )
  ]
  const imageSrcs = gallery.map((image) => `data:${image.mime_type};base64,${image.data}`)
  const title = isShell
    ? { icon: Terminal, label: t('Команда'), detail: String(data.command || '') }
    : toolTitle(String(data.name || ''), input, t)
  const Icon = title.icon
  return (
    <details className={`tool-card ${failed ? 'tool-error' : ''}`}>
      <summary>
        <span className="tool-icon">
          <Icon size={15} />
        </span>
        <span className="tool-title" title={data.name}>
          <span>{title.label}</span>
          {title.detail && (
            <>
              {' '}
              <code>{title.detail}</code>
            </>
          )}
        </span>
        {failed && <small>{t('Ошибка')}</small>}
        <span
          className={`tool-status ${failed ? 'failed' : completed ? 'done' : 'running'}`}
          title={failed ? t('Ошибка') : completed ? t('Готово') : t('Выполняется')}
        >
          {failed ? (
            <X size={13} strokeWidth={2.5} />
          ) : completed ? (
            <Check size={13} strokeWidth={2.5} />
          ) : (
            <LoaderCircle size={13} className="spin" />
          )}
        </span>
        <ChevronDown size={13} className="tool-chevron" />
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
          {imageSrcs.length > 0 && (
            <div className={`tool-gallery ${imageSrcs.length > 1 ? 'multi' : ''}`}>
              {imageSrcs.map((src, index) => (
                <button
                  key={index}
                  className="tool-image"
                  onClick={() => setZoomed(index)}
                  aria-label={t('Открыть изображение')}
                >
                  <img src={src} alt={gallery[index].path || title.label} />
                </button>
              ))}
            </div>
          )}
          <pre>
            {typeof output === 'string'
              ? output || t('Без вывода')
              : output === undefined
                ? imageSrcs.length
                  ? ''
                  : t('Без вывода')
                : JSON.stringify(output, null, 2)}
          </pre>
        </section>
      )}
      {isShell && (
        <small className="tool-exit">{t('Код выхода: {code}', { code: data.exit_code })}</small>
      )}
      {zoomed >= 0 && imageSrcs[zoomed] && (
        <ImageLightbox
          src={imageSrcs[zoomed]}
          alt={gallery[zoomed]?.path || title.label}
          close={() => setZoomed(-1)}
        />
      )}
    </details>
  )
}
