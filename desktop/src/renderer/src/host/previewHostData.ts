import type {
  BrowserLog,
  BrowserState,
  Message,
  Preferences,
  Project,
  Session
} from '../../../shared/contracts'
import type { AgentWorker } from '../../../shared/agents'
import type { AutomationResponse } from '../../../shared/automations'
import type { Job } from '../../../shared/jobs'
import type { LaunchResponse } from '../../../shared/launch'
import type { RemoteSnapshot } from '../../../shared/remote'
import type { TeamResponse } from '../../../shared/teams'
import type { TerminalSnapshot } from '../../../shared/terminal'

export type PreviewFileEntry = {
  path: string
  name: string
  directory: boolean
}

export function createPreviewPreferences(): Preferences {
  return {
    theme: 'dark',
    language: 'ru',
    density: 'comfortable',
    reduceMotion: false,
    recent: [
      'C:\\Users\\demo\\Projects\\prime-desktop',
      'C:\\Users\\demo\\Projects\\api-gateway',
      'C:\\Users\\demo\\Projects\\docs-site'
    ],
    projectOrder: [
      'C:\\Users\\demo\\Projects\\prime-desktop',
      'C:\\Users\\demo\\Projects\\api-gateway',
      'C:\\Users\\demo\\Projects\\docs-site'
    ],
    pinned: ['sess-1', 'sess-chat-1'],
    drafts: {},
    sidebarWidth: 264,
    inspectorWidth: 348,
    workbenchHeight: 280,
    sidebarVisible: true,
    inspectorVisible: true,
    agentMode: 'general',
    lastReasoningMode: 'high',
    reasoningBySession: {
      'sess-1': 'high',
      'sess-2': 'medium',
      'sess-3': 'low'
    },
    favoriteModels: [
      { provider: 'openai', model: 'gpt-5.4' },
      { provider: 'anthropic', model: 'claude-sonnet-4' }
    ],
    recentModels: [
      { provider: 'openai', model: 'gpt-5.4' },
      { provider: 'anthropic', model: 'claude-sonnet-4' }
    ],
    activeProject: 'C:\\Users\\demo\\Projects\\prime-desktop',
    activeSessions: {
      'C:\\Users\\demo\\Projects\\prime-desktop': 'sess-1',
      'C:\\Users\\demo\\Projects\\api-gateway': 'sess-3',
      chat: 'sess-chat-1'
    }
  }
}

function buildPreviewMockupPngBase64(): string {
  const width = 160
  const height = 100
  const rowSize = 1 + width * 3
  const raw = new Uint8Array(height * rowSize)

  for (let y = 0; y < height; y++) {
    const rowStart = y * rowSize
    raw[rowStart] = 0
    for (let x = 0; x < width; x++) {
      let r = 21
      let g = 22
      let b = 24

      if (y < 14) {
        r = 27
        g = 28
        b = 31
        if (x >= 12 && x <= 16 && y >= 5 && y <= 9) {
          r = 255
          g = 131
          b = 79
        }
      } else if (x >= 12 && x <= 46 && y >= 24 && y <= 29) {
        r = 60
        g = 43
        b = 36
      } else if (x >= 12 && x <= 78 && y >= 35 && y <= 42) {
        r = 245
        g = 243
        b = 238
      } else if (x >= 12 && x <= 64 && y >= 46 && y <= 53) {
        r = 255
        g = 173
        b = 133
      } else if (
        (x >= 12 && x <= 74 && y >= 59 && y <= 62) ||
        (x >= 12 && x <= 58 && y >= 66 && y <= 69)
      ) {
        r = 150
        g = 151
        b = 146
      } else if (x >= 12 && x <= 52 && y >= 76 && y <= 87) {
        r = 255
        g = 131
        b = 79
      } else if (x >= 88 && x <= 148 && y >= 22 && y <= 87) {
        const isBorder = x === 88 || x === 148 || y === 22 || y === 87
        if (isBorder) {
          r = 56
          g = 58
          b = 64
        } else {
          r = 34
          g = 35
          b = 39
          if (x >= 96 && x <= 132 && y >= 30 && y <= 34) {
            r = 245
            g = 243
            b = 238
          } else if (x >= 96 && x <= 104 && y >= 54 && y <= 78) {
            r = 56
            g = 58
            b = 64
          } else if (x >= 108 && x <= 116 && y >= 46 && y <= 78) {
            r = 255
            g = 173
            b = 133
          } else if (x >= 120 && x <= 128 && y >= 60 && y <= 78) {
            r = 56
            g = 58
            b = 64
          } else if (x >= 132 && x <= 140 && y >= 40 && y <= 78) {
            r = 255
            g = 131
            b = 79
          }
        }
      }

      const offset = rowStart + 1 + x * 3
      raw[offset] = r
      raw[offset + 1] = g
      raw[offset + 2] = b
    }
  }

  let s1 = 1
  let s2 = 0
  for (let i = 0; i < raw.length; i++) {
    s1 = (s1 + raw[i]) % 65521
    s2 = (s2 + s1) % 65521
  }
  const adler = ((s2 << 16) | s1) >>> 0

  const zlib = new Uint8Array(2 + 5 + raw.length + 4)
  zlib[0] = 0x78
  zlib[1] = 0x01
  zlib[2] = 0x01
  const len = raw.length
  const nlen = ~len & 0xffff
  zlib[3] = len & 0xff
  zlib[4] = (len >>> 8) & 0xff
  zlib[5] = nlen & 0xff
  zlib[6] = (nlen >>> 8) & 0xff
  zlib.set(raw, 7)
  const adlerOffset = 7 + raw.length
  zlib[adlerOffset] = (adler >>> 24) & 0xff
  zlib[adlerOffset + 1] = (adler >>> 16) & 0xff
  zlib[adlerOffset + 2] = (adler >>> 8) & 0xff
  zlib[adlerOffset + 3] = adler & 0xff

  const crcTable = new Uint32Array(256)
  for (let i = 0; i < 256; i++) {
    let c = i
    for (let k = 0; k < 8; k++) c = (c & 1) !== 0 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
    crcTable[i] = c >>> 0
  }

  const chunk = (type: string, data: Uint8Array) => {
    const out = new Uint8Array(12 + data.length)
    const view = new DataView(out.buffer)
    view.setUint32(0, data.length, false)
    for (let i = 0; i < 4; i++) out[4 + i] = type.charCodeAt(i)
    out.set(data, 8)
    let crc = 0xffffffff
    for (let i = 4; i < 8 + data.length; i++) {
      crc = crcTable[(crc ^ out[i]) & 0xff] ^ (crc >>> 8)
    }
    view.setUint32(8 + data.length, (crc ^ 0xffffffff) >>> 0, false)
    return out
  }

  const ihdr = new Uint8Array(13)
  const ihdrView = new DataView(ihdr.buffer)
  ihdrView.setUint32(0, width, false)
  ihdrView.setUint32(4, height, false)
  ihdr[8] = 8
  ihdr[9] = 2

  const sig = new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])
  const ihdrChunk = chunk('IHDR', ihdr)
  const idatChunk = chunk('IDAT', zlib)
  const iendChunk = chunk('IEND', new Uint8Array(0))

  const png = new Uint8Array(sig.length + ihdrChunk.length + idatChunk.length + iendChunk.length)
  let pos = 0
  for (const part of [sig, ihdrChunk, idatChunk, iendChunk]) {
    png.set(part, pos)
    pos += part.length
  }

  if (typeof btoa === 'function') {
    let binary = ''
    for (let offset = 0; offset < png.length; offset += 32768) {
      binary += String.fromCharCode(...png.subarray(offset, offset + 32768))
    }
    return btoa(binary)
  }
  return Buffer.from(png).toString('base64')
}

export const previewDesignPngBase64 = buildPreviewMockupPngBase64()

export function createPreviewDesignerProject(): Project {
  return {
    id: 'ws-designer',
    kind: 'designer',
    path: 'C:\\Users\\demo\\AppData\\Local\\Prime\\Designer',
    clientId: 'preview-client',
    connection: 'ready',
    currentSessionId: 'sess-design-1'
  }
}

export function createPreviewProjects(): Project[] {
  return [
    {
      id: 'ws-chat',
      kind: 'chat',
      path: 'chat',
      clientId: 'preview-client',
      connection: 'ready',
      currentSessionId: 'sess-chat-1'
    },
    createPreviewDesignerProject(),
    {
      id: 'ws-prime',
      kind: 'project',
      path: 'C:\\Users\\demo\\Projects\\prime-desktop',
      clientId: 'preview-client',
      connection: 'ready',
      currentSessionId: 'sess-1'
    },
    {
      id: 'ws-api',
      kind: 'project',
      path: 'C:\\Users\\demo\\Projects\\api-gateway',
      clientId: 'preview-client',
      connection: 'ready',
      currentSessionId: 'sess-3'
    },
    {
      id: 'ws-ssh',
      kind: 'project',
      path: 'ssh://prod-eu/srv/app',
      clientId: 'preview-client',
      connection: 'ready',
      remote: {
        profileId: 'ssh-1',
        name: 'prod-eu',
        host: '10.0.4.18',
        port: 22,
        user: 'deploy',
        root: '/srv/app'
      }
    }
  ]
}

const nowSec = Math.floor(Date.now() / 1000)

export function createPreviewSessions(): Record<string, Session[]> {
  return {
    'ws-prime': [
      {
        id: 'sess-1',
        title: 'Полировка интерфейса и док-панели',
        prompt_tokens: 18420,
        completion_tokens: 4120,
        cost: 0.14,
        created_at: nowSec - 3600,
        updated_at: nowSec - 120,
        todos: [
          { content: 'Сохранение высоты WorkbenchDock', status: 'completed' },
          { content: 'Исправление Escape и компактных flyout-панелей', status: 'completed' },
          { content: 'Проверка адаптива в светлой и тёмной темах', status: 'in_progress' }
        ]
      },
      {
        id: 'sess-2',
        title: 'Диагностика WebSocket-стриминга',
        prompt_tokens: 6200,
        completion_tokens: 1450,
        created_at: nowSec - 7200,
        updated_at: nowSec - 3200
      }
    ],
    'ws-api': [
      {
        id: 'sess-3',
        title: 'API Gateway rate limiter',
        prompt_tokens: 4100,
        completion_tokens: 980,
        created_at: nowSec - 9000,
        updated_at: nowSec - 5400
      }
    ],
    'ws-ssh': [
      {
        id: 'sess-4',
        title: 'Проверка systemd-юнитов на prod-eu',
        created_at: nowSec - 12000,
        updated_at: nowSec - 10000
      }
    ],
    'ws-chat': [
      {
        id: 'sess-chat-1',
        title: 'Подготовка релизных заметок',
        created_at: nowSec - 5000,
        updated_at: nowSec - 1800
      },
      {
        id: 'sess-chat-2',
        title: 'Чеклист производительности UI',
        created_at: nowSec - 14000,
        updated_at: nowSec - 11000
      }
    ],
    'ws-designer': [
      {
        id: 'sess-design-1',
        title: 'Концепт лендинга Prime Cloud',
        created_at: nowSec - 4200,
        updated_at: nowSec - 900
      },
      {
        id: 'sess-design-2',
        title: 'Мобильный экран аналитики',
        created_at: nowSec - 10800,
        updated_at: nowSec - 7200
      }
    ]
  }
}

export function createClosedProjectSessions(): Record<string, Session[]> {
  return {
    'C:\\Users\\demo\\Projects\\docs-site': [
      {
        id: 'sess-closed-1',
        title: 'Обновление документации API',
        created_at: nowSec - 86400,
        updated_at: nowSec - 43200
      }
    ]
  }
}

const fence = '```'

export function createPreviewMessages(): Record<string, Message[]> {
  return {
    'sess-1': [
      {
        id: 'msg-1',
        session_id: 'sess-1',
        role: 'user',
        created_at: nowSec - 480,
        parts: [
          {
            type: 'text',
            data: {
              text: 'Проверь сохранение высоты `WorkbenchDock`, исправь перекрытия в узких окнах и покажи сводку правок.'
            }
          }
        ]
      },
      {
        id: 'msg-2',
        session_id: 'sess-1',
        role: 'assistant',
        created_at: nowSec - 420,
        parts: [
          {
            type: 'reasoning',
            data: {
              thinking:
                '1. Проверить `Preferences` в `contracts.ts` и валидацию в Rust `settings.rs`.\n2. Добавить `setPointerCapture`, `pointercancel` и клавиатурный resize (`ArrowUp` / `ArrowDown`) на разделитель дока.\n3. Выровнять брейкпоинт настроек на `860px` и проверить компактный инспектор.',
              started_at: nowSec - 450,
              finished_at: nowSec - 435
            }
          },
          {
            type: 'tool_call',
            data: {
              id: 'call-read-1',
              name: 'read_file',
              input: { path: 'src/renderer/src/features/workbench/WorkbenchDock.tsx' },
              result: {
                content: 'Read 214 lines from WorkbenchDock.tsx',
                is_error: false
              }
            }
          },
          {
            type: 'tool_call',
            data: {
              id: 'call-agent-1',
              name: 'agent',
              input: {
                name: 'UI Audit',
                agent: 'explore',
                prompt: 'Проверить responsive-layout.css и shell.css на ширинах 390px, 820px и 1440px'
              },
              result: {
                content: 'Background agent job started',
                metadata: '{"background":true}',
                is_error: false
              }
            }
          },
          {
            type: 'text',
            data: {
              text: [
                '### Что уже обновлено',
                '',
                'Высота док-панели теперь сохраняется в `preferences.workbenchHeight` (диапазон `140..1200`) и восстанавливается при повторном открытии.',
                '',
                `${fence}ts`,
                'export function clampWorkbenchHeight(value: number, availableHeight: number) {',
                '  const max = Math.max(140, Math.min(1200, Math.round(availableHeight)))',
                '  return Math.max(140, Math.min(max, Math.round(value)))',
                '}',
                `${fence}`,
                '',
                '| Область | Изменение | Ширина |',
                '| --- | --- | --- |',
                '| `WorkbenchDock` | Сохранение высоты + `ArrowUp`/`ArrowDown` | Все |',
                '| `SettingsDialog` | Единый брейкпоинт навигации `860px` | `761–860px` |',
                '| `Inspector` | Flyout fallback для `PanelBoundary` | `< 1050px` |',
                '',
                '> Подсказка: используйте перетаскивание разделителя или клавиши `↑` / `↓` в фокусе разделителя.'
              ].join('\n')
            }
          },
          {
            type: 'finish',
            data: { reason: 'stop' }
          }
        ]
      }
    ],
    'sess-2': [
      {
        id: 'msg-3',
        session_id: 'sess-2',
        role: 'user',
        created_at: nowSec - 3300,
        parts: [{ type: 'text', data: { text: 'Проверь задержку доставки событий в стриме.' } }]
      },
      {
        id: 'msg-4',
        session_id: 'sess-2',
        role: 'assistant',
        created_at: nowSec - 3250,
        parts: [
          {
            type: 'text',
            data: {
              text: 'Поток событий работает стабильно, средняя задержка менее `12 ms`.'
            }
          },
          { type: 'finish', data: { reason: 'stop' } }
        ]
      }
    ],
    'sess-design-1': [
      {
        id: 'msg-design-u1',
        session_id: 'sess-design-1',
        role: 'user',
        created_at: nowSec - 1000,
        parts: [
          {
            type: 'text',
            data: {
              text: 'Спроектируй главный экран лендинга для облачной платформы: тёплая тёмная палитра, чёткая иерархия и визуальный концепт.'
            }
          }
        ]
      },
      {
        id: 'msg-design-a1',
        session_id: 'sess-design-1',
        role: 'assistant',
        created_at: nowSec - 950,
        parts: [
          {
            type: 'tool_call',
            data: {
              id: 'call-gen-1',
              name: 'generate_image',
              input: {
                prompt: 'Minimalist warm dark SaaS hero mockup with terracotta accent card and clean typography'
              },
              result: {
                content: 'Generated hero-concept.png',
                mime_type: 'image/png',
                data: previewDesignPngBase64,
                images: [
                  {
                    data: previewDesignPngBase64,
                    mime_type: 'image/png',
                    path: 'hero-concept.png'
                  }
                ],
                is_error: false
              }
            }
          },
          {
            type: 'text',
            data: {
              text: [
                '### Концепт главного экрана Prime Cloud',
                '',
                '- **Палитра:** графитовый фон `#151618`, карточки `#222327`, тёплый акцент `#ff834f` для основного CTA.',
                '- **Композиция:** левая колонка с крупным заголовком, подзаголовком и двумя кнопками действий; справа — интерактивная карточка телеметрии.',
                '- **Типографика:** выразительный заголовок без засечек с плотным трекингом и моноширинные метки для метрик.'
              ].join('\n')
            }
          },
          { type: 'finish', data: { reason: 'stop' } }
        ]
      }
    ],
    'sess-design-2': [
      {
        id: 'msg-design-u2',
        session_id: 'sess-design-2',
        role: 'user',
        created_at: nowSec - 7500,
        parts: [
          {
            type: 'text',
            data: { text: 'Сделай концепт мобильного экрана аналитики с карточками ключевых метрик.' }
          }
        ]
      },
      {
        id: 'msg-design-a2',
        session_id: 'sess-design-2',
        role: 'assistant',
        created_at: nowSec - 7450,
        parts: [
          {
            type: 'text',
            data: {
              text: '### Мобильный экран аналитики\n\nКомпактная верхняя сводка, два KPI-блока в сетке 2×1 и прокручиваемая лента событий с быстрыми фильтрами.'
            }
          },
          { type: 'finish', data: { reason: 'stop' } }
        ]
      }
    ]
  }
}

export function createPreviewWorkers(): AgentWorker[] {
  return [
    {
      id: 'msg-2$$call-agent-1',
      session_id: 'worker-sess-1',
      parent_session_id: 'sess-1',
      root_session_id: 'sess-1',
      name: 'UI Audit',
      agent: 'explore',
      task: 'Аудит адаптивных брейкпоинтов 390px / 820px / 1440px',
      status: 'running',
      started_at: nowSec - 300,
      updated_at: nowSec - 5
    },
    {
      id: 'msg-2$$call-agent-2',
      session_id: 'worker-sess-2',
      parent_session_id: 'sess-1',
      root_session_id: 'sess-1',
      name: 'CSS Polish',
      agent: 'code',
      task: 'Проверка контрастности светлой и тёмной темы',
      status: 'done',
      started_at: nowSec - 600,
      updated_at: nowSec - 240
    }
  ]
}

export function createPreviewFiles(): Record<string, PreviewFileEntry[]> {
  return {
    '': [
      { path: 'src', name: 'src', directory: true },
      { path: 'package.json', name: 'package.json', directory: false },
      { path: 'README.md', name: 'README.md', directory: false },
      { path: 'tsconfig.json', name: 'tsconfig.json', directory: false }
    ],
    src: [
      { path: 'src/app', name: 'app', directory: true },
      { path: 'src/components', name: 'components', directory: true },
      { path: 'src/styles', name: 'styles', directory: true }
    ],
    'src/app': [
      { path: 'src/app/App.tsx', name: 'App.tsx', directory: false },
      { path: 'src/app/useWorkspace.ts', name: 'useWorkspace.ts', directory: false }
    ],
    'src/components': [
      { path: 'src/components/PanelBoundary.tsx', name: 'PanelBoundary.tsx', directory: false }
    ],
    'src/styles': [
      { path: 'src/styles/shell.css', name: 'shell.css', directory: false },
      { path: 'src/styles/workbench-dock.css', name: 'workbench-dock.css', directory: false }
    ]
  }
}

export function createPreviewFileContents(): Record<string, { content: string; revision: string }> {
  return {
    'package.json': {
      revision: 'rev-1',
      content: JSON.stringify(
        {
          name: 'prime-desktop',
          version: '0.4.105',
          scripts: {
            'ui:preview': 'vite --config vite.preview.config.ts',
            'native:ui': 'tsc --noEmit && vite build --config vite.native.config.ts'
          }
        },
        null,
        2
      )
    },
    'README.md': {
      revision: 'rev-1',
      content: '# Prime Desktop\n\nЛокальное рабочее пространство для разработки с Prime.\n'
    },
    'tsconfig.json': {
      revision: 'rev-1',
      content: '{\n  "compilerOptions": {\n    "strict": true,\n    "jsx": "react-jsx"\n  }\n}\n'
    },
    'src/app/App.tsx': {
      revision: 'rev-1',
      content: [
        "import { useState } from 'react'",
        "import { WorkbenchDock } from '../features/workbench/WorkbenchDock'",
        '',
        'export function WorkspaceShell() {',
        '  const [workbench, setWorkbench] = useState(true)',
        '  return workbench ? <WorkbenchDock /> : null',
        '}',
        ''
      ].join('\n')
    },
    'src/app/useWorkspace.ts': {
      revision: 'rev-1',
      content: 'export const defaultWorkbenchHeight = 340\n'
    },
    'src/components/PanelBoundary.tsx': {
      revision: 'rev-1',
      content: 'export function PanelBoundary({ children }: { children: React.ReactNode }) {\n  return <>{children}</>\n}\n'
    },
    'src/styles/shell.css': {
      revision: 'rev-1',
      content: '.workspace-topbar {\n  display: flex;\n  align-items: center;\n}\n'
    },
    'src/styles/workbench-dock.css': {
      revision: 'rev-1',
      content: '.workbench-dock {\n  display: flex;\n  flex-direction: column;\n}\n'
    }
  }
}

export function createPreviewConfig() {
  return {
    models: {
      large: { provider: 'openai', model: 'gpt-5.4', context_window: 200000 },
      small: { provider: 'anthropic', model: 'claude-sonnet-4', context_window: 200000 }
    },
    session_models: {},
    options: {
      disable_multi_agent: false,
      smart_tools: true,
      auto_lsp: true,
      language: 'ru',
      notifications: 'auto'
    },
    providers: {
      openai: {
        name: 'OpenAI',
        base_url: 'https://api.openai.com/v1',
        type: 'openai-compat',
        discover_models: true,
        models: [
          {
            id: 'gpt-5.4',
            name: 'GPT-5.4',
            context_window: 200000,
            default_max_tokens: 16384,
            can_reason: true,
            reasoning_levels: ['off', 'low', 'medium', 'high', 'max']
          }
        ]
      },
      anthropic: {
        name: 'Anthropic',
        base_url: 'https://api.anthropic.com/v1',
        type: 'anthropic',
        discover_models: true,
        models: [
          {
            id: 'claude-sonnet-4',
            name: 'Claude Sonnet 4',
            context_window: 200000,
            default_max_tokens: 8192,
            can_reason: true,
            reasoning_levels: ['off', 'low', 'medium', 'high']
          }
        ]
      }
    },
    mcp: {
      context7: { type: 'stdio', command: 'npx context7-mcp', disabled: false }
    },
    lsp: {
      typescript: { command: 'typescript-language-server', disabled: false }
    }
  }
}

export function createPreviewProviders() {
  return [
    {
      id: 'openai',
      name: 'OpenAI',
      models: [
        {
          id: 'gpt-5.4',
          name: 'GPT-5.4',
          context_window: 200000,
          can_reason: true,
          reasoning_levels: ['off', 'low', 'medium', 'high', 'max']
        }
      ]
    },
    {
      id: 'anthropic',
      name: 'Anthropic',
      models: [
        {
          id: 'claude-sonnet-4',
          name: 'Claude Sonnet 4',
          context_window: 200000,
          can_reason: true,
          reasoning_levels: ['off', 'low', 'medium', 'high']
        }
      ]
    }
  ]
}

export function createPreviewTerminal(workspaceId: string): TerminalSnapshot {
  return {
    id: 'term-1',
    workspaceId,
    title: 'pwsh',
    shell: 'pwsh.exe',
    cwd: 'C:\\Users\\demo\\Projects\\prime-desktop',
    pid: 14820,
    cols: 100,
    rows: 24,
    owner: 'user',
    status: 'running',
    createdAt: Date.now() - 120000,
    sequence: 1
  }
}

export function createPreviewJobs(): Job[] {
  return [
    {
      id: 'job-1',
      kind: 'agent',
      title: 'UI Audit',
      projectKey: 'C:\\Users\\demo\\Projects\\prime-desktop',
      projectName: 'prime-desktop',
      workspaceId: 'ws-prime',
      sessionId: 'sess-1',
      status: 'running',
      startedAt: Date.now() - 48000,
      detail: 'Проверка адаптивных брейкпоинтов',
      stoppable: true
    },
    {
      id: 'job-2',
      kind: 'terminal',
      title: 'pwsh · npm run test:unit',
      projectKey: 'C:\\Users\\demo\\Projects\\prime-desktop',
      projectName: 'prime-desktop',
      workspaceId: 'ws-prime',
      status: 'completed',
      startedAt: Date.now() - 180000,
      finishedAt: Date.now() - 172000,
      stoppable: false
    }
  ]
}

export function createPreviewAutomations(): AutomationResponse {
  return {
    root: 'C:\\Users\\demo\\Projects\\prime-desktop',
    rules: [
      {
        id: 'auto-1',
        root: 'C:\\Users\\demo\\Projects\\prime-desktop',
        title: 'Ночной аудит тестов',
        prompt: 'Запусти unit-тесты и проверь отчёт.',
        enabled: true,
        trigger: { type: 'daily', time: '03:00' },
        createdAt: Date.now() - 86400000,
        updatedAt: Date.now() - 3600000,
        nextAt: Date.now() + 28800000
      }
    ],
    runs: [
      {
        id: 'run-1',
        ruleId: 'auto-1',
        root: 'C:\\Users\\demo\\Projects\\prime-desktop',
        title: 'Ночной аудит тестов',
        trigger: 'daily',
        status: 'completed',
        createdAt: Date.now() - 3600000,
        finishedAt: Date.now() - 3540000,
        workspaceId: 'ws-prime',
        sessionId: 'sess-1'
      }
    ]
  }
}

export function createPreviewLaunch(): LaunchResponse {
  return {
    root: 'C:\\Users\\demo\\Projects\\prime-desktop',
    configs: [
      {
        id: 'launch-1',
        name: 'UI Preview',
        command: 'npm run ui:preview',
        url: 'http://127.0.0.1:5173'
      }
    ],
    processes: [
      {
        configId: 'launch-1',
        terminalId: 'term-1',
        status: 'running',
        startedAt: Date.now() - 90000,
        url: 'http://127.0.0.1:5173'
      }
    ]
  }
}

export function createPreviewTeams(): TeamResponse {
  return {
    root: 'C:\\Users\\demo\\Projects\\prime-desktop',
    teams: [
      {
        id: 'team-1',
        root: 'C:\\Users\\demo\\Projects\\prime-desktop',
        title: 'Полировка Desktop UI',
        goal: 'Проверить компоненты, фокус и контрастность обеих тем',
        concurrency: 2,
        createdAt: Date.now() - 240000,
        tasks: [
          {
            id: 'task-1',
            title: 'Аудит Dock и панелей',
            instruction: 'Проверить сохранение высоты и адаптив',
            status: 'completed',
            startedAt: Date.now() - 230000,
            finishedAt: Date.now() - 180000
          },
          {
            id: 'task-2',
            title: 'Проверка диалога настроек',
            instruction: 'Проверить ширину 820px и 390px',
            status: 'running',
            startedAt: Date.now() - 120000
          }
        ]
      }
    ]
  }
}

export function createPreviewRemote(): RemoteSnapshot {
  return {
    credentialStorageAvailable: true,
    profiles: [
      {
        id: 'ssh-1',
        name: 'prod-eu',
        host: '10.0.4.18',
        port: 22,
        user: 'deploy',
        root: '/srv/app',
        auth: 'agent',
        runtime: 'bundled',
        status: 'connected'
      }
    ]
  }
}

export function createPreviewBrowserState(): BrowserState {
  return {
    url: 'http://127.0.0.1:5173',
    title: 'Prime Desktop Preview',
    loading: false,
    canGoBack: false,
    canGoForward: false,
    activeTab: 'tab-1',
    tabs: [{ id: 'tab-1', url: 'http://127.0.0.1:5173', title: 'Preview', loading: false }]
  }
}

export function createPreviewBrowserLogs(): BrowserLog[] {
  return []
}
