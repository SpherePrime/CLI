import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { Message } from '../../src/shared/contracts'
import { consolidateToolMessages } from '../../src/shared/toolMessages'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { MessageItem } from '../../src/renderer/src/features/chat/MessageItem'

const makeMessage = (
  id: string,
  parts: Message['parts'],
  overrides: Partial<Message> = {}
): Message =>
  ({
    id,
    session_id: 's1',
    role: 'assistant',
    parts,
    ...overrides
  }) as Message

function renderItem(message: Message, language: 'ru' | 'en' = 'ru') {
  return renderToStaticMarkup(
    <I18nProvider language={language}>
      <MessageItem
        message={message}
        continued={false}
        fresh={false}
        live={false}
        matched={false}
      />
    </I18nProvider>
  )
}

describe('MessageItem summary and empty assistant rendering', () => {
  it('renders .compaction-progress when is_summary_message has no finish part', () => {
    const html = renderItem(makeMessage('sum-1', [], { is_summary_message: true }))
    expect(html).toContain('compaction-progress')
    expect(html).toContain('Автоматическое сжатие контекста')
  })

  it('renders .compaction-status.is-done with "Контекст сжат" on end_turn finish', () => {
    const html = renderItem(
      makeMessage('sum-2', [{ type: 'finish', data: { reason: 'end_turn' } }], {
        is_summary_message: true
      })
    )
    expect(html).toContain('compaction-status is-done')
    expect(html).toContain('Контекст сжат')
    expect(html).not.toContain('compaction-progress')

    const enHtml = renderItem(
      makeMessage('sum-2-en', [{ type: 'finish', data: { reason: 'end_turn' } }], {
        is_summary_message: true
      }),
      'en'
    )
    expect(enHtml).toContain('Context compacted')
  })

  it('renders .compaction-status.is-error with error message or fallback on error finish', () => {
    const html = renderItem(
      makeMessage('sum-3', [{ type: 'finish', data: { reason: 'error', message: 'boom' } }], {
        is_summary_message: true
      })
    )
    expect(html).toContain('compaction-status is-error')
    expect(html).toContain('boom')

    const fallbackHtml = renderItem(
      makeMessage('sum-4', [{ type: 'finish', data: { reason: 'error' } }], {
        is_summary_message: true
      })
    )
    expect(fallbackHtml).toContain('compaction-status is-error')
    expect(fallbackHtml).toContain('Не удалось сжать контекст')
  })

  it('renders .finish-note for finished assistant message with no visible content', () => {
    const html = renderItem(makeMessage('a-empty', [{ type: 'finish', data: { reason: 'end_turn' } }]))
    expect(html).toContain('class="finish-note"')
    expect(html).toContain('Модель завершила ответ без текста')
  })

  it('does not render empty-response .finish-note when assistant message has text or attachments', () => {
    const textHtml = renderItem(
      makeMessage('a-text', [
        { type: 'text', data: { text: 'Готово' } },
        { type: 'finish', data: { reason: 'end_turn' } }
      ])
    )
    expect(textHtml).toContain('Готово')
    expect(textHtml).not.toContain('finish-note')
    expect(textHtml).not.toContain('Модель завершила ответ без текста')

    const attachmentHtml = renderItem(
      makeMessage('a-bin', [
        { type: 'binary', data: { mime_type: 'application/pdf', data: 'abc' } },
        { type: 'finish', data: { reason: 'end_turn' } }
      ])
    )
    expect(attachmentHtml).not.toContain('finish-note')
    expect(attachmentHtml).not.toContain('Модель завершила ответ без текста')
  })

  it('keeps summary and empty finished assistant messages through consolidateToolMessages and renders them', () => {
    const consolidated = consolidateToolMessages([
      makeMessage('s-progress', [], { is_summary_message: true }),
      makeMessage('s-done', [{ type: 'finish', data: { reason: 'end_turn' } }], {
        is_summary_message: true
      }),
      makeMessage('s-error', [{ type: 'finish', data: { reason: 'error', message: 'boom' } }], {
        is_summary_message: true
      }),
      makeMessage('a-empty', [{ type: 'finish', data: { reason: 'end_turn' } }])
    ])

    expect(consolidated.map((item) => item.id)).toEqual(['s-progress', 's-done', 's-error', 'a-empty'])
    const html = consolidated.map((item) => renderItem(item)).join('')
    expect(html).toContain('compaction-progress')
    expect(html).toContain('compaction-status is-done')
    expect(html).toContain('compaction-status is-error')
    expect(html).toContain('class="finish-note"')
  })
})
