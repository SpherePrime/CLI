import { useState } from 'react'
import { Send, X } from 'lucide-react'
import type { PickedElementResult } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'

function elementDetails(result: PickedElementResult, index: number, total: number) {
  const { element } = result
  const styles = Object.entries(element.styles)
    .filter(([, value]) => value && value !== 'none' && value !== 'normal')
    .map(([name, value]) => `${name}: ${value}`)
    .join('; ')
  return [
    total > 1 ? `Element ${index + 1} on ${element.url}:` : `Element on ${element.url}:`,
    `- selector: \`${element.selector}\``,
    `- tag: <${element.tag}>, size ${Math.round(element.rect.width)}×${Math.round(element.rect.height)}`,
    element.text ? `- text: ${element.text.replace(/\s+/g, ' ').slice(0, 300)}` : '',
    styles ? `- computed styles: ${styles}` : '',
    '```html\n' + element.html + '\n```'
  ]
    .filter(Boolean)
    .join('\n')
}

export function elementPrompt(results: PickedElementResult[], comment: string) {
  const screenshots = results.filter((result) => result.attachmentId).length
  return [
    comment.trim(),
    ...results.map((result, index) => elementDetails(result, index, results.length)),
    screenshots === 1 ? 'A screenshot of the element is attached.' : '',
    screenshots > 1 ? 'Screenshots of the elements are attached in the same order.' : ''
  ]
    .filter(Boolean)
    .join('\n\n')
}

export function ElementPick({
  results,
  onSend,
  onRemove,
  onClose
}: {
  results: PickedElementResult[]
  onSend(prompt: string, attachmentIds: string[]): Promise<unknown>
  onRemove(index: number): void
  onClose(): void
}) {
  const { t } = useI18n()
  const [comment, setComment] = useState('')
  const [sending, setSending] = useState(false)
  const multiple = results.length > 1
  return (
    <form
      className="element-pick"
      onSubmit={async (event) => {
        event.preventDefault()
        if (!comment.trim() || !results.length) return
        setSending(true)
        try {
          await onSend(
            elementPrompt(results, comment),
            results.flatMap((result) => (result.attachmentId ? [result.attachmentId] : []))
          )
          onClose()
        } finally {
          setSending(false)
        }
      }}
      onKeyDown={(event) => {
        if (event.key === 'Escape') onClose()
      }}
    >
      <div className={`element-pick-list ${multiple ? 'multiple' : ''}`}>
        {results.map((result, index) => (
          <div className="element-pick-heading" key={`${result.element.selector}-${index}`}>
            {result.image && <img src={result.image} alt={t('Снимок элемента')} />}
            <div>
              <strong>
                &lt;{result.element.tag}&gt; · {Math.round(result.element.rect.width)}×
                {Math.round(result.element.rect.height)}
              </strong>
              <code title={result.element.selector}>{result.element.selector}</code>
              {result.element.text && <span>{result.element.text.slice(0, 120)}</span>}
            </div>
            <button
              type="button"
              className="icon small"
              aria-label={multiple ? t('Убрать элемент') : t('Закрыть')}
              onClick={() => (multiple ? onRemove(index) : onClose())}
            >
              <X size={14} />
            </button>
          </div>
        ))}
      </div>
      <textarea
        autoFocus
        rows={2}
        aria-label={t('Замечание к элементу')}
        placeholder={multiple ? t('Например: выровняй эти блоки') : t('Например: переделай эту кнопку')}
        value={comment}
        onChange={(event) => setComment(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault()
            event.currentTarget.form?.requestSubmit()
          }
        }}
      />
      <button type="submit" className="primary" disabled={sending || !comment.trim()}>
        <Send size={13} />
        {multiple ? t('Отправить {value0} в чат', { value0: results.length }) : t('Отправить в чат')}
      </button>
    </form>
  )
}
