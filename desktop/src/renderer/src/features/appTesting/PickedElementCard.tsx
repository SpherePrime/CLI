import { useState } from 'react'
import type { PickedAppElement } from '../../../../shared/appTesting'
import { appTestingCopy } from './copy'

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
  language: 'ru' | 'en'
  picked: PickedAppElement
  onSend(prompt: string, attachmentIds: string[]): Promise<unknown>
  onClose(): void
}

export function PickedElementCard({ language, picked, onSend, onClose }: Props) {
  const c = appTestingCopy[language]
  const [comment, setComment] = useState('')
  const [sending, setSending] = useState(false)
  return (
    <form
      className="element-pick app-testing-pick"
      onSubmit={async (event) => {
        event.preventDefault()
        if (!comment.trim()) return
        setSending(true)
        try {
          await onSend(appElementPrompt(picked, comment), picked.attachmentId ? [picked.attachmentId] : [])
          onClose()
        } finally {
          setSending(false)
        }
      }}
    >
      <div className="element-pick-list">
        <div className="element-pick-heading">
          {picked.image && <img src={picked.image} alt={picked.element.name} />}
          <strong>
            {picked.element.role} · {picked.element.bounds.width}×{picked.element.bounds.height}
          </strong>
          <code title={picked.element.path}>{locatorText(picked)}</code>
          <button type="button" aria-label={c.cancel} title={c.cancel} onClick={onClose}>
            ×
          </button>
        </div>
        {picked.element.name && <p className="app-testing-pick-name">{picked.element.name}</p>}
        {picked.confidence !== 'high' && (
          <p className="app-testing-warning">
            confidence: {picked.confidence}
            {picked.candidates > 1 ? ` · ${picked.candidates}` : ''}
          </p>
        )}
      </div>
      <textarea
        value={comment}
        placeholder={c.pick}
        aria-label={c.pick}
        onChange={(event) => setComment(event.target.value)}
      />
      <button type="submit" disabled={sending || !comment.trim()}>
        {c.sendToChat}
      </button>
    </form>
  )
}
