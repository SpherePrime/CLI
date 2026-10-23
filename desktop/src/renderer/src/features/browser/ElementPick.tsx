import { useState } from 'react'
import { LoaderCircle, Mic, Send, X } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import { useVoice } from '../plugins/useVoice'
import { VoiceWave } from '../chat/VoiceWave'
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
  workspace,
  onSend,
  onRemove,
  onClose
}: {
  results: PickedElementResult[]
  workspace: Workspace
  onSend(prompt: string, attachmentIds: string[]): Promise<unknown>
  onRemove(index: number): void
  onClose(): void
}) {
  const { t } = useI18n()
  const [comment, setComment] = useState('')
  const [sending, setSending] = useState(false)
  const multiple = results.length > 1
  const voice = useVoice(workspace, (text) =>
    setComment((value) => [value, text].filter(Boolean).join(' '))
  )
  return (
    <form
      className="element-pick"
      onSubmit={async (event) => {
        event.preventDefault()
        if ((!comment.trim() && !voice.recording) || !results.length) return
        setSending(true)
        try {
          const spoken = voice.recording ? await voice.finish() : ''
          const message = [comment, spoken].filter((part) => part.trim()).join(' ')
          if (!message.trim()) return
          await onSend(
            elementPrompt(results, message),
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
      {(voice.recording || voice.transcribing) && (
        <VoiceWave
          compact
          analyser={voice.analyser}
          transcribing={voice.transcribing}
          onCancel={() => void voice.cancel()}
        />
      )}
      <textarea
        hidden={voice.recording || voice.transcribing}
        autoFocus
        rows={2}
        aria-label={t('Замечание к элементу')}
        placeholder={
          multiple ? t('Например: выровняй эти блоки') : t('Например: переделай эту кнопку')
        }
        value={comment}
        onChange={(event) => setComment(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault()
            event.currentTarget.form?.requestSubmit()
          }
        }}
      />
      <div className="element-pick-actions">
        <button
          type="button"
          className={`icon small ${voice.recording ? 'danger' : ''}`}
          disabled={voice.transcribing || sending}
          aria-label={voice.recording ? t('Остановить запись') : t('Голосовой ввод')}
          title={voice.recording ? t('Остановить запись') : t('Голосовой ввод')}
          onClick={() => void voice.toggle()}
        >
          {voice.transcribing ? <LoaderCircle size={14} className="spin" /> : <Mic size={14} />}
        </button>
        <button
          type="submit"
          className="primary"
          disabled={sending || voice.transcribing || (!comment.trim() && !voice.recording)}
        >
          <Send size={13} />
          {multiple
            ? t('Отправить {value0} в чат', { value0: results.length })
            : t('Отправить в чат')}
        </button>
      </div>
    </form>
  )
}
