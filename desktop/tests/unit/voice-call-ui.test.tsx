import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it } from 'vitest'
import {
  ConversationCall,
  ConversationCallPill
} from '../../src/renderer/src/features/voice/ConversationCall'
import type { Conversation } from '../../src/renderer/src/features/voice/useConversation'

function conversation(overrides: Partial<Conversation> = {}): Conversation {
  return {
    state: 'listening',
    muted: false,
    heard: '',
    question: '',
    analyser: null,
    outputAnalyser: null,
    startedAt: Date.now(),
    preparing: undefined,
    start: async () => {},
    stop: async () => {},
    toggleMute: () => {},
    stopSpeaking: () => {},
    ...overrides
  }
}

it('keeps the assistant speaking status visible when the microphone is muted', () => {
  const html = renderToStaticMarkup(
    <ConversationCall
      conversation={conversation({ state: 'speaking', muted: true })}
      onOpenChat={() => {}}
    />
  )
  expect(html).toContain('Prime отвечает')
  expect(html).toContain('Включить микрофон')
  expect(html).toContain('Остановить озвучку')
  expect(html).toContain('Открыть чат')
  expect(html).toContain('aria-pressed="true"')
})

it('exposes preparation progress and disables mute until the call connects', () => {
  const html = renderToStaticMarkup(
    <ConversationCall
      conversation={conversation({
        state: 'preparing',
        preparing: { state: 'downloading', percent: 43 } as Conversation['preparing']
      })}
      onOpenChat={() => {}}
    />
  )
  expect(html).toContain('43%')
  expect(html).toContain('disabled=""')
  expect(html).toContain('Завершить')
})

it('keeps a clear return-to-call action and call controls available in chat', () => {
  const html = renderToStaticMarkup(
    <ConversationCallPill conversation={conversation({ muted: true })} onReturnToCall={() => {}} />
  )
  expect(html).toContain('Вернуться к разговору')
  expect(html).toContain('Микрофон выключен')
  expect(html).toContain('Включить микрофон')
  expect(html).toContain('Завершить')
})
