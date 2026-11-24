import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { translate } from '../../src/shared/localization'
import { toolTitle } from '../../src/renderer/src/features/chat/toolTitle'
import { ToolCard } from '../../src/renderer/src/features/chat/ToolCard'
import { ToolExecutionGroup } from '../../src/renderer/src/features/chat/ToolExecutionGroup'
import { MessageItem } from '../../src/renderer/src/features/chat/MessageItem'
import type { Message } from '../../src/shared/contracts'

const ru = (value: string) => translate('ru', value)
const en = (value: string) => translate('en', value)

function makeUnfinishedSearchPart(): Message['parts'][number] {
  return {
    type: 'tool_call',
    data: {
      id: 'call-search-1',
      name: 'agentic_fetch',
      input: JSON.stringify({ prompt: 'Search the web for Google Antigravity IDE' })
    }
  }
}

describe('web search and fetch toolTitle', () => {
  it('renders agentic_fetch without url as web search in Russian and English', () => {
    const ruTitle = toolTitle('agentic_fetch', { prompt: 'Search the web for   Google Antigravity IDE' }, ru)
    expect(ruTitle.label).toBe('Поиск в интернете')
    expect(ruTitle.detail).toBe('Search the web for Google Antigravity IDE')

    const enTitle = toolTitle('agentic_fetch', { prompt: 'Search the web for Google Antigravity IDE' }, en)
    expect(enTitle.label).toBe('Web search')
    expect(enTitle.detail).toBe('Search the web for Google Antigravity IDE')
  })

  it('renders agentic_fetch with url as page analysis', () => {
    const title = toolTitle(
      'agentic_fetch',
      { url: 'https://example.com/docs', prompt: 'Summarize the docs' },
      ru
    )
    expect(title.label).toBe('Анализ страницы')
    expect(title.detail).toBe('https://example.com/docs')
  })

  it('renders web_search and web_fetch with expected labels', () => {
    const searchTitle = toolTitle('web_search', { query: 'Prime Desktop' }, ru)
    expect(searchTitle.label).toBe('Поиск в интернете')
    expect(searchTitle.detail).toBe('Prime Desktop')

    const fetchRu = toolTitle('web_fetch', { url: 'https://example.com' }, ru)
    expect(fetchRu.label).toBe('Загрузка')
    expect(fetchRu.detail).toBe('https://example.com')

    const fetchEn = toolTitle('web_fetch', { url: 'https://example.com' }, en)
    expect(fetchEn.label).toBe('Fetch')
    expect(fetchEn.detail).toBe('https://example.com')
  })

  it('provides English translation for interrupted execution note', () => {
    expect(en('Выполнение прервано до получения результата')).toBe(
      'Execution interrupted before receiving a result'
    )
  })
})

describe('ToolCard interrupted state safeguard', () => {
  it('renders failed state instead of spinner when message is finished without tool result', () => {
    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ToolCard part={makeUnfinishedSearchPart()} messageFinished={true} />
      </I18nProvider>
    )
    expect(markup).toContain('tool-error')
    expect(markup).toContain('class="tool-status failed"')
    expect(markup).toContain('lucide-x')
    expect(markup).toContain('Поиск в интернете')
    expect(markup).not.toContain('lucide-loader-circle')
    expect(markup).not.toContain('class="spin"')
  })

  it('renders spinning loader when message is not finished and tool has no result', () => {
    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ToolCard part={makeUnfinishedSearchPart()} messageFinished={false} />
      </I18nProvider>
    )
    expect(markup).toContain('class="tool-status running"')
    expect(markup).toContain('lucide-loader-circle')
    expect(markup).not.toContain('tool-error')
    expect(markup).not.toContain('class="tool-status failed"')
  })

  it('propagates finish state from MessageItem to ToolCard', () => {
    const finishedMessage: Message = {
      id: 'm1',
      session_id: 's1',
      role: 'assistant',
      parts: [
        makeUnfinishedSearchPart(),
        { type: 'finish', data: { reason: 'error', message: 'Interrupted' } }
      ]
    }
    const markup = renderToStaticMarkup(
      <I18nProvider language="en">
        <MessageItem message={finishedMessage} continued={false} fresh={false} live={false} matched={false} />
      </I18nProvider>
    )
    expect(markup).toContain('Completed 1 action')
    expect(markup).toContain('Interrupted')
    expect(markup).not.toContain('lucide-loader-circle')

    const expandedGroupMarkup = renderToStaticMarkup(
      <I18nProvider language="en">
        <ToolExecutionGroup parts={[makeUnfinishedSearchPart()]} messageFinished defaultOpen />
      </I18nProvider>
    )
    expect(expandedGroupMarkup).toContain('tool-error')
    expect(expandedGroupMarkup).toContain('class="tool-status failed"')
    expect(expandedGroupMarkup).toContain('Web search')
    expect(expandedGroupMarkup).not.toContain('lucide-loader-circle')
  })
})
