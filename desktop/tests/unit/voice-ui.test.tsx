import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { VoiceWave } from '../../src/renderer/src/features/chat/VoiceWave'
import { VoiceSettings } from '../../src/renderer/src/features/settings/VoiceSettings'
import { formatLiveTranscript } from '../../src/renderer/src/features/plugins/useVoice'
import type { Workspace } from '../../src/renderer/src/app/useWorkspace'

function createWorkspaceStub(): Workspace {
  return {
    project: { id: 'ws-1', path: '/repo', name: 'repo', kind: 'project', connection: 'ready' },
    session: { id: 'sess-1', title: 'Chat', prompt_tokens: 0, completion_tokens: 0 },
    busy: false,
    preferences: { theme: 'dark', language: 'ru', drafts: {}, recent: [], pinned: [], sidebarWidth: 260, inspectorWidth: 320 },
    savePreferences: vi.fn().mockResolvedValue(undefined),
    call: vi.fn().mockResolvedValue({
      engine: 'tinyasr',
      model: 'en-ru-uk',
      language: 'auto',
      languages: ['auto', 'ru', 'en', 'uk'],
      installed: true
    }),
    errorHandler: vi.fn()
  } as unknown as Workspace
}

describe('VoiceWave live transcript', () => {
  it('omits live transcript strip when no words have arrived yet', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <VoiceWave
          analyser={null}
          transcribing={false}
          liveTranscript={{ committed: '', open: '', tail: '' }}
        />
      </I18nProvider>
    )
    expect(html).not.toContain('voice-live-transcript')
    expect(html).toContain('Говорите…')
  })

  it('renders committed, pending, and tail live transcript directly above the waveform bar', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <VoiceWave
          analyser={null}
          transcribing={false}
          liveTranscript={{
            committed: 'Привет, давай настроим',
            open: 'распознавание речи',
            tail: 'через gRPC'
          }}
        />
      </I18nProvider>
    )
    const transcriptIdx = html.indexOf('voice-live-transcript')
    const waveIdx = html.indexOf('voice-wave')
    expect(transcriptIdx).toBeGreaterThan(-1)
    expect(waveIdx).toBeGreaterThan(transcriptIdx)
    expect(html).toContain('Привет, давай настроим')
    expect(html).toContain('распознавание речи')
    expect(html).toContain('через gRPC')
    expect(html).toContain('voice-live-caret')
  })

  it('joins live transcript parts cleanly', () => {
    expect(formatLiveTranscript({ committed: 'Привет.', open: 'Как', tail: 'дела' })).toBe(
      'Привет. Как дела'
    )
  })
})

describe('VoiceSettings simplified TinyASR UI', () => {
  it('renders only Auto/RU/EN/UK language options with zero Google/Vosk/Whisper traces', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <VoiceSettings workspace={createWorkspaceStub()} />
      </I18nProvider>
    )
    expect(html).toContain('Локальная модель en-ru-uk')
    expect(html).toContain('Язык распознавания')
    expect(html).toContain('Авто')
    expect(html).not.toContain('Google STT')
    expect(html).not.toContain('Vosk STT')
    expect(html).not.toContain('Whisper STT')
    expect(html).not.toContain('Ускорение')
    expect(html).not.toContain('CUDA')
    expect(html).not.toContain('Vulkan')
  })
})
