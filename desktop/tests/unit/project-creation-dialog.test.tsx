import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { CreateProjectDialog } from '../../src/renderer/src/features/projects/CreateProjectDialog'
import type { Workspace } from '../../src/renderer/src/app/useWorkspace'

function render(language: 'ru' | 'en') {
  return renderToStaticMarkup(<I18nProvider language={language}><CreateProjectDialog workspace={{} as Workspace} /></I18nProvider>)
}

describe('project creation dialog', () => {
  it('renders a labelled modal, safe default, path status and separate open-folder action', () => {
    const markup = render('ru')
    expect(markup).toContain('<dialog')
    expect(markup).toContain('aria-labelledby=')
    expect(markup).toContain('value="Новый проект"')
    expect(markup).toContain('aria-describedby="project-creation-validation project-creation-path"')
    expect(markup).toContain('aria-live="polite"')
    expect(markup).toContain('Открыть папку')
    expect(markup).toContain('type="submit" class="primary" disabled=""')
  })
  it('translates the creation flow to English', () => {
    const markup = render('en')
    expect(markup).toContain('value="New project"')
    expect(markup).toContain('Create project')
    expect(markup).toContain('Open folder')
    expect(markup).toContain('Locating your Documents folder')
    expect(markup).not.toContain('Создать проект')
  })
})
