import { useState } from 'react'
import { LoaderCircle, Mic, Send, X } from 'lucide-react'
import type { PickedAppElement } from '../../../../shared/appTesting'
import type { Workspace } from '../../app/useWorkspace'
import { useVoice } from '../plugins/useVoice'
import { VoiceWave } from '../chat/VoiceWave'
import type { AppTestingCopy } from './copy'

export function locatorText(picked: PickedAppElement): string {
  const { element } = picked
  if (element.automationId) return `automationId=${element.automationId}`
  if (element.role && element.name) return `role=${element.role}, name="${element.name}"`
  if (picked.reference) return `reference=${picked.reference}`
  return `path=${element.path}`
}

export function appElementPrompt(picked: PickedAppElement, comment: string): string {
  const { element } = picked
  return [
    comment.trim(),
    `Element in the application window "${picked.window}":`,
    `- role: ${element.role}`,
    `- name: ${element.name}`,
    element.automationId ? `- automationId: \`${element.automationId}\`` : '',
    element.className ? `- class: \`${element.className}\`` : '',
    element.value ? `- value: ${element.value}` : '',
    `- bounds: ${element.bounds.x},${element.bounds.y} ${element.bounds.width}×${element.bounds.height}`,
    element.states.length ? `- states: ${element.states.join(', ')}` : '',
    element.actions.length ? `- actions: ${element.actions.join(', ')}` : '',
    `- path: ${element.path}`,
    `- locator: ${locatorText(picked)}`,
    `- provider: ${element.source}, confidence: ${picked.confidence}`,
    picked.attachmentId ? 'A screenshot of the element is attached.' : ''
  ]
    .filter(Boolean)
    .join('\n')
}

type Props = {
  copy: AppTestingCopy
  workspace: Workspace
  picked: PickedAppElement
  onSend(prompt: string, attachmentIds: string[]): Promise<unknown>
  onClose(): void
}

export function PickedElementCard({ copy, workspace, picked, onSend, onClose }: Props) {
  const [comment, setComment] = useState('')
  const [sending, setSending] = useState(false)
  const voice = useVoice(workspace, (text) => setComment((value) => [value, text].filter(Boolean).join(' ')))
  const { element } = picked
  return (
    <form
      className="element-pick app-testing-pick"
      onSubmit={async (event) => {
        event.preventDefault()
        if (!comment.trim() && !voice.recording) return
        setSending(true)
        try {
          const spoken = voice.recording ? await voice.finish() : ''
          const message = [comment, spoken].filter((part) => part.trim()).join(' ')
          if (!message.trim()) return
          await onSend(appElementPrompt(picked, message), picked.attachmentId ? [picked.attachmentId] : [])
          onClose()
        } finally {
          setSending(false)
        }
      }}
      onKeyDown={(event) => {
        if (event.key === 'Escape') onClose()
      }}
    >
      <div className="element-pick-list">
        <div className="element-pick-heading">
          {picked.image && <img src={picked.image} alt={element.name} />}
          <div>
            <strong>
              {element.role} · {element.bounds.width}×{element.bounds.height}
            </strong>
            <code title={element.path}>{locatorText(picked)}</code>
            {element.name && <span>{element.name.slice(0, 120)}</span>}
          </div>
          <button type="button" className="icon small" aria-label={copy.close} onClick={onClose}>
            <X size={14} />
          </button>
        </div>
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
        aria-label={copy.comment}
        placeholder={copy.commentHint}
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
          aria-label={voice.recording ? copy.voiceStop : copy.voice}
          title={voice.recording ? copy.voiceStop : copy.voice}
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
          {copy.sendToChat}
        </button>
      </div>
    </form>
  )
}
