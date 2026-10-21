import { test, expect } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { readFile, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { launchWorkbench } from '../fixtures/workbench-app'

test('history rows stay readable and restore preserves local work in a stash', async () => {
  test.setTimeout(90000)
  const fixture = await launchWorkbench()
  const { page, app, root, project } = fixture
  const git = (...args: string[]) =>
    execFileSync('git', args, { cwd: root, encoding: 'utf8' }).trim()
  try {
    git('branch', '-M', 'main')
    await writeFile(
      join(root, '.gitignore'),
      'config/\ndata/\nprofile/\ncache/\nxdg-config/\nxdg-data/\n.prime/\n'
    )
    await writeFile(
      join(root, '.git', 'info', 'exclude'),
      await readFile(join(root, '.gitignore'), 'utf8')
    )
    await writeFile(join(root, 'app.ts'), 'export const version = 2\n')
    git('add', 'app.ts', '.gitignore')
    git(
      '-c',
      'user.name=Fixture',
      '-c',
      'user.email=fixture@example.test',
      'commit',
      '-qm',
      'Current work with a long commit title that must wrap without overlapping its author and timestamp'
    )
    await writeFile(join(root, 'later.txt'), 'later independent work\n')
    git('add', 'later.txt')
    git(
      '-c',
      'user.name=Fixture',
      '-c',
      'user.email=fixture@example.test',
      'commit',
      '-qm',
      'Later independent work'
    )
    await writeFile(join(root, 'app.ts'), 'unsaved work\n')
    await writeFile(join(root, 'notes.txt'), 'untracked notes\n')
    await page.evaluate(async (id) => {
      const session = await window.prime.call(id, 'createSession', { title: 'Git history fixture' })
      await window.prime.call(id, 'currentSession', { sessionId: session.id })
    }, project.id)
    await page.reload()
    await expect(
      page.locator('.session-link').filter({ hasText: 'Git history fixture' })
    ).toBeVisible()
    for (const trigger of [
      page.getByRole('button', { name: 'Режим доступа', exact: true }),
      page.getByRole('combobox', { name: 'Режим агента', exact: true })
    ]) {
      await trigger.click()
      await expect(trigger).toHaveAttribute('aria-expanded', 'true')
      await expect
        .poll(() =>
          trigger
            .locator('.menu-chevron')
            .evaluate((element) => getComputedStyle(element).transform)
        )
        .toBe('matrix(-1, 0, 0, -1, 0, 0)')
      await page.keyboard.press('Escape')
      await expect(trigger).toHaveAttribute('aria-expanded', 'false')
    }
    await page.getByRole('button', { name: 'Инструменты проекта', exact: false }).click()
    await page.getByRole('tab', { name: 'GitHub', exact: true }).first().click()
    await page.getByRole('button', { name: 'Развернуть панель', exact: true }).click()
    const rows = page.locator('.git-history-item')
    await expect(rows).toHaveCount(3)
    await expect(page.locator('.git-history-card')).toBeVisible()
    await expect(rows.first()).toBeEnabled()
    await rows
      .filter({ has: page.locator('.git-history-title').filter({ hasText: /^Current work/ }) })
      .click()
    await expect(page.locator('.git-history-card h3')).toContainText('Current work')
    const restoreButton = page.getByRole('button', {
      name: 'Восстановить весь проект',
      exact: true
    })
    await restoreButton.hover()
    await expect
      .poll(() =>
        restoreButton.evaluate((element) => {
          const canvas = document.createElement('canvas')
          canvas.width = canvas.height = 1
          const context = canvas.getContext('2d')!
          context.fillStyle = getComputedStyle(element).backgroundColor
          context.fillRect(0, 0, 1, 1)
          const channels = [...context.getImageData(0, 0, 1, 1).data].slice(0, 3)
          return Math.max(...channels) - Math.min(...channels)
        })
      )
      .toBeGreaterThan(40)
    await expect(page.locator('.patch-file')).toHaveCount(2)
    await expect(page.locator('.patch-row.patch-added').first()).toBeVisible()
    await expect(page.locator('.patch-row.patch-removed').first()).toBeVisible()
    const assertRows = async () => {
      const bounds = await rows.evaluateAll((elements) =>
        elements.map((element) => {
          const row = element.getBoundingClientRect()
          const title = element.querySelector('.git-history-title')!.getBoundingClientRect()
          const author = element.querySelector('.git-history-author')!.getBoundingClientRect()
          return {
            top: row.top,
            bottom: row.bottom,
            titleBottom: title.bottom,
            authorTop: author.top
          }
        })
      )
      expect(bounds[0].bottom).toBeLessThanOrEqual(bounds[1].top + 1)
      for (const item of bounds) expect(item.titleBottom).toBeLessThanOrEqual(item.authorTop + 1)
    }
    await assertRows()
    await page.screenshot({ path: 'test-results/git-history-wide.png' })
    await app.evaluate(({ BrowserWindow }) => {
      const window = BrowserWindow.getAllWindows()[0]
      window.setMinimumSize(620, 620)
      window.setSize(760, 860)
    })
    await assertRows()
    await page.screenshot({ path: 'test-results/git-history-narrow.png' })
    await page.getByRole('button', { name: 'Отменить этот коммит', exact: true }).click()
    await page.getByRole('button', { name: 'Подтвердить отмену коммита', exact: true }).click()
    await expect(page.getByRole('status')).toContainText('Изменения коммита отменены')
    expect((await readFile(join(root, 'app.ts'), 'utf8')).replace(/\r\n/g, '\n')).toBe(
      'export const version = 1\n'
    )
    expect(await readFile(join(root, 'later.txt'), 'utf8')).toBe('later independent work\n')
    expect(git('log', '-1', '--format=%s')).toMatch(/^Revert project changes from /)
    await rows
      .filter({ has: page.locator('.git-history-title').filter({ hasText: /^fixture$/ }) })
      .click()
    await page.getByRole('button', { name: 'Восстановить весь проект', exact: true }).click()
    await page.getByRole('button', { name: 'Подтвердить восстановление', exact: true }).click()
    await expect(page.getByRole('status')).toContainText('Состояние восстановлено')
    expect((await readFile(join(root, 'app.ts'), 'utf8')).replace(/\r\n/g, '\n')).toBe(
      'export const version = 1\n'
    )
    expect(git('log', '-1', '--format=%s')).toMatch(/^Restore project to /)
    expect(git('ls-files', 'later.txt')).toBe('')
    expect(git('stash', 'show', '--include-untracked', '-p', 'stash@{0}')).toContain('unsaved work')
    expect(git('stash', 'show', '--include-untracked', '-p', 'stash@{0}')).toContain(
      'untracked notes'
    )
    expect(fixture.errors).toEqual([])
  } finally {
    await fixture.close()
  }
})
