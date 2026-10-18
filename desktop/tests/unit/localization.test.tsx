import { describe, expect, it } from 'vitest'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider, useI18n, translate } from '../../src/renderer/src/i18n'
import { english } from '../../src/renderer/src/i18n/catalog'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import ts from 'typescript'

describe('desktop localization', () => {
  it('defaults to Russian and interpolates values without translating their content', () => {
    expect(translate(undefined, 'Новый чат')).toBe('Новый чат')
    expect(translate('en', 'Действия: {value}', { value: 'Мой проект' })).toBe(
      'Actions: Мой проект'
    )
  })

  it('renders translated text and accessible labels through React context', () => {
    function Button() {
      const { t } = useI18n()
      return createElement('button', { 'aria-label': t('Закрыть') }, t('Новый чат'))
    }
    const render = (language: 'ru' | 'en') =>
      renderToStaticMarkup(
        createElement(I18nProvider, { language, children: createElement(Button) })
      )
    expect(render('ru')).toBe('<button aria-label="Закрыть">Новый чат</button>')
    expect(render('en')).toBe('<button aria-label="Close">New chat</button>')
  })

  it('provides nonempty English translations and preserves every interpolation', () => {
    for (const [source, translated] of Object.entries(english)) {
      expect(translated.trim(), source).not.toBe('')
      expect(translated.match(/\{\w+\}/g)?.sort() || [], source).toEqual(
        source.match(/\{\w+\}/g)?.sort() || []
      )
    }
  })

  it('covers every literal translation key used by renderer components', () => {
    const root = join(process.cwd(), 'src/renderer/src')
    const files = readdirSync(root, { recursive: true }).filter((file) =>
      /\.tsx?$/.test(String(file))
    )
    const keys: string[] = []
    for (const file of files) {
      const filename = join(root, String(file))
      const source = ts.createSourceFile(
        filename,
        readFileSync(filename, 'utf8'),
        ts.ScriptTarget.Latest,
        true
      )
      const visit = (node: ts.Node) => {
        if (
          ts.isCallExpression(node) &&
          node.expression.getText(source) === 't' &&
          ts.isStringLiteral(node.arguments[0])
        )
          keys.push(node.arguments[0].text)
        ts.forEachChild(node, visit)
      }
      visit(source)
    }
    expect(keys.length).toBeGreaterThan(150)
    for (const key of keys) expect(english[key], key).toBeDefined()
  })
})
