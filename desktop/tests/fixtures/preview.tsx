import '../../src/renderer/src/styles/global.css'
import '../../src/renderer/src/styles/brand.css'
import '../../src/renderer/src/styles/composer.css'
import '../../src/renderer/src/styles/files.css'
import '../../src/renderer/src/styles/tasks.css'
import '../../src/renderer/src/styles/motion.css'
import { createRoot } from 'react-dom/client'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { MessageList } from '../../src/renderer/src/features/chat/MessageList'
import { Composer } from '../../src/renderer/src/features/chat/Composer'
import type { Message } from '../../src/shared/contracts'

const now = Math.floor(Date.now() / 1000) - 120
const todos = [
  { content: 'Собрать структуру страницы', status: 'completed', active_form: 'Собираю структуру' },
  {
    content: 'Написать анимацию частиц на canvas',
    status: 'in_progress',
    active_form: 'Пишу анимацию частиц на canvas'
  },
  { content: 'Проверить страницу в браузере', status: 'pending', active_form: 'Проверяю' }
]
const messages: Message[] = [
  {
    id: 'u1',
    session_id: 's',
    role: 'user',
    created_at: now,
    parts: [{ type: 'text', data: { text: 'Сделай красивую интерактивную html-страницу' } }]
  },
  {
    id: 'a1',
    session_id: 's',
    role: 'assistant',
    created_at: now + 2,
    parts: [
      {
        type: 'reasoning',
        data: {
          thinking: 'Нужна одна страница без зависимостей. Сделаю canvas с частицами.',
          started_at: now + 2,
          finished_at: now + 24
        }
      },
      { type: 'tool_call', data: { id: 'c1', name: 'ls', input: '{"path":"."}' } }
    ]
  },
  {
    id: 't1',
    session_id: 's',
    role: 'tool',
    parts: [
      { type: 'tool_result', data: { tool_call_id: 'c1', name: 'ls', content: 'index.html' } }
    ]
  },
  {
    id: 'a2',
    session_id: 's',
    role: 'assistant',
    created_at: now + 30,
    parts: [
      { type: 'text', data: { text: 'Продолжаю — пишу сам HTML-файл.' } },
      {
        type: 'tool_call',
        data: {
          id: 'c2',
          name: 'write',
          input: '{"file_path":"C:\\\\project\\\\index.html","content":"<!doctype html>"}'
        }
      }
    ]
  },
  {
    id: 't2',
    session_id: 's',
    role: 'tool',
    parts: [
      { type: 'tool_result', data: { tool_call_id: 'c2', name: 'write', content: 'File written' } }
    ]
  },
  {
    id: 'a3',
    session_id: 's',
    role: 'assistant',
    created_at: now + 60,
    parts: [
      { type: 'text', data: { text: 'Запускаю локальный сервер в фоне:' } },
      {
        type: 'tool_call',
        data: {
          id: 'c3',
          name: 'bash',
          input:
            '{"command":"python -m http.server 8931","description":"Start local HTTP server on port 8931"}'
        }
      },
      {
        type: 'tool_call',
        data: { id: 'c4', name: 'grep', input: '{"pattern":"requestAnimationFrame"}' }
      },
      {
        type: 'reasoning',
        data: { thinking: 'Проверяю, что анимация запускается…', started_at: now + 90 }
      }
    ]
  },
  {
    id: 't3',
    session_id: 's',
    role: 'tool',
    parts: [{ type: 'tool_result', data: { tool_call_id: 'c3', name: 'bash', content: 'ok' } }]
  }
]

const models = [
  {
    provider_id: 'anthropic',
    provider_name: 'Anthropic',
    model_id: 'claude-fable-5-1',
    name: 'Claude Fable 5.1',
    context_window: 200000,
    default_reasoning_mode: 'high'
  }
]
const workspace: any = {
  project: { id: 'p', kind: 'project', path: 'C:\\project', connection: 'ready' },
  session: {
    id: 's',
    title: 'Preview',
    todos,
    prompt_tokens: 48200,
    completion_tokens: 1900,
    updated_at: now
  },
  sessions: [],
  messages,
  preferences: { drafts: {} },
  config: { models: { large: { provider: 'anthropic', model: 'claude-fable-5-1' } } },
  agent: {},
  busy: true,
  loading: false,
  activeSessionId: 's',
  permissions: [],
  queue: [],
  refreshQueue: async () => {},
  questions: [],
  savePreferences: async () => {},
  setBusy: () => {},
  refresh: async () => {},
  errorHandler: () => {},
  send: async () => true,
  call: async (operation: string) => {
    if (operation === 'models') return models
    if (operation === 'getSkip') return { skip: false }
    if (operation === 'context')
      return {
        categories: [
          { key: 'system_prompt', tokens: 9800, items: 1 },
          { key: 'tools', tokens: 14200, items: 24 },
          { key: 'mcp_tools', tokens: 6100, items: 12 },
          { key: 'user_messages', tokens: 1200, items: 3 },
          { key: 'assistant_messages', tokens: 3400, items: 4 },
          { key: 'reasoning', tokens: 5200, items: 2 },
          { key: 'tool_calls', tokens: 2100, items: 4 },
          { key: 'tool_results', tokens: 8100, items: 4 }
        ],
        estimated_total: 50100,
        reported_total: 50100,
        context_window: 200000,
        auto_summarize_percent: 70,
        estimated_usage: false
      }
    return []
  }
}

function Preview() {
  return (
    <I18nProvider language="ru">
      <div
        style={{
          height: '100vh',
          display: 'flex',
          flexDirection: 'column',
          background: 'var(--bg)'
        }}
      >
        <MessageList messages={messages} busy />
        <Composer workspace={workspace} />
      </div>
    </I18nProvider>
  )
}

createRoot(document.getElementById('root')!).render(<Preview />)
