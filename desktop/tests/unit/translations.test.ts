import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import ts from 'typescript'
import { describe, expect, it } from 'vitest'
import { english } from '../../src/shared/translations'
import { translate } from '../../src/shared/localization'
import { formatDuration, triggerLabel } from '../../src/renderer/src/features/customAgents/presentation'

function walkSourceFiles(directory: string): string[] {
  const files: string[] = []
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const fullPath = join(directory, entry.name)
    if (entry.isDirectory()) {
      files.push(...walkSourceFiles(fullPath))
    } else if (/\.(ts|tsx)$/.test(entry.name) && !fullPath.endsWith('translations.ts')) {
      files.push(fullPath)
    }
  }
  return files
}

function collectTranslatedLiterals(expr: ts.Expression | undefined, found: Set<string>) {
  if (!expr) return
  if (ts.isStringLiteral(expr) || ts.isNoSubstitutionTemplateLiteral(expr)) {
    if (/[А-Яа-яЁё]/.test(expr.text)) found.add(expr.text)
    return
  }
  if (ts.isConditionalExpression(expr)) {
    collectTranslatedLiterals(expr.whenTrue, found)
    collectTranslatedLiterals(expr.whenFalse, found)
    return
  }
  if (ts.isParenthesizedExpression(expr)) {
    collectTranslatedLiterals(expr.expression, found)
  }
}

describe('translations integrity and coverage', () => {
  it('contains no duplicate keys in translations.ts', () => {
    const filePath = join(process.cwd(), 'src/shared/translations.ts')
    const sourceText = readFileSync(filePath, 'utf8')
    const sourceFile = ts.createSourceFile(filePath, sourceText, ts.ScriptTarget.Latest, true)
    const seen = new Set<string>()
    const duplicates: string[] = []

    const visit = (node: ts.Node) => {
      if (ts.isPropertyAssignment(node)) {
        const key =
          ts.isStringLiteral(node.name) || ts.isIdentifier(node.name) ? node.name.text : null
        if (key !== null) {
          if (seen.has(key)) duplicates.push(key)
          else seen.add(key)
        }
      }
      ts.forEachChild(node, visit)
    }
    visit(sourceFile)

    expect(duplicates).toEqual([])
  })

  it('covers all Russian t() and translate() keys used in src/', () => {
    const srcRoot = join(process.cwd(), 'src')
    const missing: string[] = []

    for (const file of walkSourceFiles(srcRoot)) {
      const sourceText = readFileSync(file, 'utf8')
      const sourceFile = ts.createSourceFile(file, sourceText, ts.ScriptTarget.Latest, true)
      const keys = new Set<string>()

      const visit = (node: ts.Node) => {
        if (ts.isCallExpression(node)) {
          const callee = node.expression
          const name = ts.isIdentifier(callee)
            ? callee.text
            : ts.isPropertyAccessExpression(callee)
              ? callee.name.text
              : ''
          if (name === 't' && node.arguments.length >= 1) {
            collectTranslatedLiterals(node.arguments[0], keys)
          } else if (name === 'translate' && node.arguments.length >= 2) {
            collectTranslatedLiterals(node.arguments[1], keys)
          }
        }
        ts.forEachChild(node, visit)
      }
      visit(sourceFile)

      for (const key of keys) {
        if (!Object.prototype.hasOwnProperty.call(english, key)) {
          missing.push(`${relative(srcRoot, file)}: ${key}`)
        }
      }
    }

    expect(missing).toEqual([])
  })

  it('localizes custom agent durations and intervals in English', () => {
    const tEn = (source: string, values?: Record<string, string | number>) =>
      translate('en', source, values)
    expect(
      triggerLabel(
        { trigger_type: 'interval', interval_minutes: 10, cron_expr: '', monitor_kind: '' },
        tEn
      )
    ).toBe('Every 10 min')
    expect(
      triggerLabel(
        { trigger_type: 'monitor', interval_minutes: 15, cron_expr: '', monitor_kind: 'http' },
        tEn
      )
    ).toBe('Monitor (HTTP) · 15 min')
    expect(formatDuration(100, 145, tEn)).toBe('45s')
    expect(formatDuration(100, 225, tEn)).toBe('2m 5s')
    expect(formatDuration(100, 220, tEn)).toBe('2m')
  })
})
