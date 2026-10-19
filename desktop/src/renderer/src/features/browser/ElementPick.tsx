import { useState } from 'react'
import { Send, X } from 'lucide-react'
import type { PickedElementResult } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'

export function elementPrompt(result: PickedElementResult, comment: string) {
  const { element } = result
  const styles = Object.entries(element.styles)
    .filter(([, value]) => value && value !== 'none' && value !== 'normal')
    .map(([name, value]) => `${name}: ${value}`)
    .join('; ')
  return [
    comment.trim(),
    `Element on ${element.url}:`,
    `- selector: \`${element.selector}\``,
    `- tag: <${element.tag}>, size ${Math.round(element.rect.width)}×${Math.round(element.rect.height)}`,
    element.text ? `- text: ${element.text.replace(/\s+/g, ' ').slice(0, 300)}` : '',
    styles ? `- computed styles: ${styles}` : '',
    '```html\n' + element.html + '\n```',
    result.attachmentId ? 'A screenshot of the element is attached.' : ''
  ]
    .filter(Boolean)
    .join('\n')
}

export function ElementPick({
  result,
  onSend,
  onClose
}: {
  result: PickedElementResult
  onSend(prompt: string, attachmentIds: string[]): Promise<unknown>
  onClose(): void
}) {
  const { t } = useI18n()
  const [comment, setComment] = useState('')
  const [sending, setSending] = useState(false)
  const { element } = result
  return (
    <form
      className="element-pick"
      onSubmit={async (event) => {
        event.preventDefault()
        if (!comment.trim()) return
        setSending(true)
        try {
          await onSend(elementPrompt(result, comment), result.attachmentId ? [result.attachmentId] : [])
          onClose()
        } finally {
          setSending(false)
        }
      }}
      onKeyDown={(event) => {
        if (event.key === 'Escape') onClose()
      }}
    >
      <div className="element-pick-heading">
        {result.image && <img src={result.image} alt={t('Снимок элемента')} />}
        <div>
          <strong>
            &lt;{element.tag}&gt; · {Math.round(element.rect.width)}×{Math.round(element.rect.height)}
          </strong>
          <code title={element.selector}>{element.selector}</code>
          {element.text && <span>{element.text.slice(0, 120)}</span>}
        </div>
        <button type="button" className="icon small" aria-label={t('Закрыть')} onClick={onClose}>
          <X size={14} />
        </button>
      </div>
      <textarea
        autoFocus
        rows={2}
        aria-label={t('Замечание к элементу')}
        placeholder={t('Например: переделай эту кнопку')}
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
        {t('Отправить в чат')}
      </button>
    </form>
  )
}
