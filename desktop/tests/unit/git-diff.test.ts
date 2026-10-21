import { expect, it } from 'vitest'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { GitDiffView } from '../../src/renderer/src/features/github/GitDiffView'
import { parseGitDiff } from '../../src/renderer/src/features/github/parseGitDiff'

it('renders large diffs in bounded batches while keeping complete change counts', () => {
  const diff = `diff --git a/large.txt b/large.txt\n--- /dev/null\n+++ b/large.txt\n@@ -0,0 +1,5000 @@\n${Array.from({ length: 5000 }, (_, index) => `+line${index}`).join('\n')}`
  const html = renderToStaticMarkup(createElement(GitDiffView, { diff }))
  expect((html.match(/class="patch-row /g) || []).length).toBe(2000)
  expect(html).toContain('+5000')
  expect(html).toContain('Показать ещё')
  expect(html).not.toContain('line4999')
})

it('separates files and numbers additions, deletions and context without counting headers', () => {
  const files = parseGitDiff(
    ` app.ts | 2 +-\n diff summary\ndiff --git a/app.ts b/app.ts\nindex abc..def 100644\n--- a/app.ts\n+++ b/app.ts\n@@ -4,2 +4,2 @@ function app\n-old\n+new\n same\ndiff --git a/new.txt b/new.txt\nnew file mode 100644\n--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1 @@\n+hello\n`
  )
  expect(files.map(({ path, added, removed }) => ({ path, added, removed }))).toEqual([
    { path: 'app.ts', added: 1, removed: 1 },
    { path: 'new.txt', added: 1, removed: 0 }
  ])
  expect(files[0].rows.filter((row) => row.kind !== 'hunk')).toEqual([
    { kind: 'removed', text: 'old', oldLine: 4 },
    { kind: 'added', text: 'new', newLine: 4 },
    { kind: 'context', text: 'same', oldLine: 5, newLine: 5 }
  ])
})

it('keeps binary changes, renamed paths and truncated hunks readable', () => {
  const files = parseGitDiff(
    `diff --git a/old name.png b/new name.png\nsimilarity index 100%\nrename from old name.png\nrename to new name.png\nBinary files a/old name.png and b/new name.png differ\ndiff --git a/app.ts b/app.ts\n--- a/app.ts\n+++ b/app.ts\n@@ -10,4 +12,4 @@\n+incomplete`
  )
  expect(files[0].path).toBe('new name.png')
  expect(files[0].rows.some((row) => row.text.startsWith('Binary files'))).toBe(true)
  expect(files[1].rows.at(-1)).toEqual({ kind: 'added', text: 'incomplete', newLine: 12 })
})
