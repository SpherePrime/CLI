import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { mkdir, writeFile, readFile } from 'node:fs/promises'
import { join } from 'node:path'
import { expect } from '@playwright/test'

const execute = promisify(execFile)
export async function createProject(root) {
  const directory = join(root, 'UI Fixture Project')
  await mkdir(directory)
  await execute('git', ['init', '-b', 'main', directory], { windowsHide: true })
  await writeFile(join(directory, 'fixture.txt'), 'original fixture\n')
  await execute('git', ['-C', directory, 'add', 'fixture.txt'], { windowsHide: true })
  await execute('git', ['-C', directory, '-c', 'user.name=Fixture', '-c', 'user.email=fixture@localhost', 'commit', '-m', 'Initial fixture'], { windowsHide: true })
  return directory
}

export async function auditProject(audit) {
  await audit.step('17-file-editor', async page => {
    const project = page.getByRole('group', { name: 'Проект UI Fixture Project', exact: true })
    if (!(await project.isVisible())) await page.getByRole('button', { name: 'Панель проектов', exact: true }).first().click()
    await project.locator('.project-item').click()
    await project.getByRole('button', { name: 'Создать первый чат', exact: true }).click()
    await expect(page.locator('.breadcrumb-project')).toHaveText('UI Fixture Project')
    const files = page.getByRole('button', { name: 'Файлы', exact: true })
    if (await files.getAttribute('aria-pressed') !== 'true') await files.click()
    await page.getByRole('button', { name: /fixture.txt/ }).first().click()
    const editor = page.getByRole('region', { name: 'Редактор файлов', exact: true })
    await expect(editor).toBeVisible()
    const input = editor.getByRole('textbox', { name: 'Код файла', exact: true })
    await input.focus()
    await input.press('Control+A')
    await input.pressSequentially('edited through native UI')
    await editor.getByRole('button', { name: 'Сохранить', exact: true }).click()
    await expect.poll(() => readFile(join(audit.project, 'fixture.txt'), 'utf8')).toBe('edited through native UI')
    await page.getByRole('button', { name: 'Закрыть файл fixture.txt', exact: true }).click()
  })
  await audit.step('18-real-git-history', async page => {
    await page.getByRole('button', { name: 'Инструменты проекта', exact: true }).click()
    const dock = page.locator('.workbench-dock')
    await dock.getByRole('tablist', { name: 'Инструменты проекта', exact: true }).getByRole('tab', { name: 'GitHub', exact: true }).click()
    await expect(dock.getByText('Initial fixture', { exact: true })).toBeVisible()
    await dock.getByText('Initial fixture', { exact: true }).click()
    await expect(dock.getByText('fixture.txt', { exact: true }).first()).toBeVisible()
    await audit.capture('18-git-commit-detail')
    await dock.getByRole('button', { name: 'Скрыть инструменты', exact: true }).click()
  })
}
