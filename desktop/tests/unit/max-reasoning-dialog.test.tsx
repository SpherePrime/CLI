import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { MaxReasoningDialog } from '../../src/renderer/src/features/models/MaxReasoningDialog'
import { needsMaxReasoningNotice } from '../../src/renderer/src/features/models/maxReasoningNotice'

function render(language: 'ru' | 'en') {
  return renderToStaticMarkup(
    <I18nProvider language={language}>
      <MaxReasoningDialog close={() => {}} confirm={() => {}} />
    </I18nProvider>
  )
}

describe('maximum reasoning notice', () => {
  it('appears only when switching to max and not dismissed for good', () => {
    expect(needsMaxReasoningNotice('max', 'high')).toBe(true)
    expect(needsMaxReasoningNotice('max', 'max')).toBe(false)
    expect(needsMaxReasoningNotice('high', 'max')).toBe(false)
    expect(needsMaxReasoningNotice('max', 'off', true)).toBe(false)
  })
  it('explains the cost and offers cancel, enable and do not show again', () => {
    const markup = render('ru')
    expect(markup).toContain('<dialog')
    expect(markup).toContain('Максимальный уровень размышлений')
    expect(markup).toContain('агенты-ревьюеры')
    expect(markup).toContain('заметно больше токенов')
    expect(markup).toContain('точнее и аккуратнее')
    expect(markup).not.toContain('скриншоты')
    expect(markup).not.toContain('запускают проект')
    expect(markup).toContain('class="reasoning-meter"')
    expect(markup).toContain('id=')
    expect(markup).toContain('type="checkbox"')
    expect(markup).toContain('Больше не показывать')
    expect(markup).toContain('Отмена')
    expect(markup).toContain('Включить')
  })
  it('is translated to English', () => {
    const markup = render('en')
    expect(markup).toContain('Maximum reasoning level')
    expect(markup).toContain('noticeably more tokens')
    expect(markup).toContain('Do not show again')
    expect(markup).not.toContain('Отмена')
  })
})
