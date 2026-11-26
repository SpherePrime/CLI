import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { VoiceTranscriptPill } from '../../src/renderer/src/features/chat/VoiceTranscriptPill'
import { VoiceWave } from '../../src/renderer/src/features/chat/VoiceWave'
import { VoiceSettings } from '../../src/renderer/src/features/settings/VoiceSettings'

describe('VoiceTranscriptPill & VoiceWave', () => {
  it('renders floating live transcript with committed, open, and emerging tail words', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <VoiceTranscriptPill
          analyser={null}
          transcribing={false}
          transcript={{
            committed: 'Привет мир',
            open: 'это живой',
            tail: 'ввод текста'
          }}
        />
      </I18nProvider>
    )

    expect(html).toContain('voice-transcript-pill')
    expect(html).toContain('voice-transcript-canvas')
    expect(html).toContain('Привет мир')
    expect(html).toContain('это живой')
    expect(html).toContain('ввод')
    expect(html).toContain('текста')
    expect(html).toContain('voice-live-fresh')
    expect(html).toContain('voice-live-caret')
  })

  it('omits floating transcript pill when no words recognized yet', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <VoiceTranscriptPill
          analyser={null}
          transcribing={false}
          transcript={{ committed: '', open: '', tail: '' }}
        />
      </I18nProvider>
    )

    expect(html).toBe('')
  })

  it('renders clean VoiceWave recording bar', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <VoiceWave analyser={null} transcribing={false} onCancel={vi.fn()} />
      </I18nProvider>
    )

    expect(html).toContain('voice-wave')
    expect(html).toContain('Говорите…')
  })
})

describe('VoiceSettings simplified TinyASR section', () => {
  it('shows local en-ru-uk model with Russian default and zero legacy providers', () => {
    const workspace = {
      call: vi.fn().mockResolvedValue({
        engine: 'tinyasr',
        model: 'en-ru-uk',
        language: 'ru',
        languages: ['ru', 'en', 'uk', 'auto'],
        installed: true
      }),
      preferences: {},
      setPreference: vi.fn()
    } as any

    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <VoiceSettings workspace={workspace} />
      </I18nProvider>
    )

    expect(html).toContain('en-ru-uk')
    expect(html).toContain('Язык распознавания')
    expect(html).toContain('Русский')
    expect(html).not.toMatch(/Google STT|Whisper STT|Vosk STT|CUDA|Vulkan/i)
  })
})
