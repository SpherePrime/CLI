import { test, expect } from '@playwright/test'
import type { TerminalRead, TerminalSnapshot } from '../../src/shared/terminal'
import type { Project } from '../../src/shared/contracts'
import { createSshLinuxFixture, detectSshLinuxFixture } from '../fixtures/ssh-linux'
import { launchWorkbench } from '../fixtures/workbench-app'

test('local providers drive SSH commands and files without a remote agent, with chat restore', async () => {
  test.setTimeout(240000)
  const support = await detectSshLinuxFixture()
  test.skip(!support.available, support.reason)
  const ssh = await createSshLinuxFixture()
  const workbench = await launchWorkbench()
  const { page, errors } = workbench
  try {
    await ssh.execute(
      "printf 'export const version = 1\\n' > app.ts && git init -q && git add app.ts && git -c user.name=F -c user.email=f@example.test commit -qm init"
    )

    await page.getByRole('button', { name: 'SSH-серверы', exact: true }).click()
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('button', { name: 'Добавить сервер' }).first().click()
    const form = dialog.locator('form.remote-editor')
    await form.getByLabel('Название').fill('WSL fixture')
    await form.getByLabel('Хост').fill(ssh.host)
    await form.getByLabel('Порт').fill(String(ssh.port))
    await form.getByLabel('Пользователь').fill('root')
    await form.getByLabel('Папка проекта').fill(ssh.root)
    await form.getByRole('combobox', { name: 'Аутентификация' }).click()
    await dialog.getByRole('option', { name: 'Файл ключа' }).click()
    await form.getByLabel('Приватный ключ').fill(ssh.keyPath)
    await form.getByRole('button', { name: 'Сохранить' }).click()
    await dialog.getByRole('button', { name: 'Подключить', exact: true }).click()
    await dialog.getByRole('button', { name: 'Я проверил отпечаток' }).click()
    await dialog.getByRole('button', { name: 'Подтвердить доверие' }).click()
    await dialog.getByRole('button', { name: 'Подключить', exact: true }).click()
    await expect(dialog.getByRole('button', { name: 'Открыть проект' })).toBeVisible({
      timeout: 90000
    })
    await dialog.getByRole('button', { name: 'Открыть проект' }).click()
    await expect(page.locator('.project-item.selected')).toContainText('WSL fixture')

    const remote = (await page.evaluate(() => window.prime.projects())).find(
      (item: Project) => item.remote
    )!
    expect(remote.path.startsWith('prime-ssh://')).toBe(true)
    expect(remote.remote?.root).toBe(ssh.root)

    await page
      .getByRole('textbox', { name: 'Сообщение Prime', exact: true })
      .fill('SSH_TOOL_FIXTURE Создай тестовый файл')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Разрешить действие?' })).toBeVisible({
      timeout: 30000
    })
    await page.getByRole('button', { name: 'Разрешить', exact: true }).click()
    await expect
      .poll(() => ssh.execute('cat created-by-tool.txt').catch(() => ''), { timeout: 30000 })
      .toBe('Tool execution verified')
    await expect(page.getByRole('button', { name: 'Остановить задачу', exact: true })).toHaveCount(
      0,
      { timeout: 30000 }
    )

    const request = workbench.provider.prompts.find((item: any) =>
      Array.isArray(item.tools) && JSON.stringify(item.messages).includes('SSH_TOOL_FIXTURE')
    )
    expect(request).toBeTruthy()
    const names = request.tools.map((item: any) => item.function.name)
    expect(names.some((name: string) => name.endsWith('ssh_write_file'))).toBe(true)
    expect(names).not.toContain('write')
    expect(names).not.toContain('bash')
    expect(
      await ssh.execute(
        'test ! -f .prime/prime.db && test ! -f ../config/prime.json && echo LOCAL_AGENT'
      )
    ).toBe('LOCAL_AGENT')

    const opened = await page.evaluate((id) => window.prime.readFile(id, 'app.ts'), remote.id)
    expect(opened.content).toBe('export const version = 1\n')
    await page.evaluate(
      ({ id, revision }) =>
        window.prime.saveFile(id, 'app.ts', 'export const version = 2\n', revision),
      { id: remote.id, revision: opened.revision }
    )
    expect(await ssh.execute('cat app.ts')).toBe('export const version = 2')
    const changes = await page.evaluate((id) => window.prime.changes(id), remote.id)
    expect(changes.map((item) => item.path).sort()).toEqual(['app.ts', 'created-by-tool.txt'])
    const diff = await page.evaluate((id) => window.prime.diff(id, '', 'app.ts', 'git'), remote.id)
    expect(diff.patch).toContain('+export const version = 2')
    await expect(
      page.locator('.inspector').getByText('app.ts', { exact: true }).first()
    ).toBeVisible()

    const terminal = (await page.evaluate(
      (id) => window.prime.terminal(id, { action: 'create', options: { cols: 100, rows: 30 } }),
      remote.id
    )) as TerminalSnapshot
    await page.evaluate(
      ({ id, terminalId }) =>
        window.prime.terminal(id, {
          action: 'write',
          id: terminalId,
          data: 'pwd; echo REMOTE_$((40+2))\n'
        }),
      { id: remote.id, terminalId: terminal.id }
    )
    await expect
      .poll(
        async () => {
          const read = (await page.evaluate(
            ({ id, terminalId }) => window.prime.terminal(id, { action: 'read', id: terminalId }),
            { id: remote.id, terminalId: terminal.id }
          )) as TerminalRead
          return read.output.includes('REMOTE_42') && read.output.includes(ssh.root)
        },
        { timeout: 20000 }
      )
      .toBe(true)

    await page.evaluate(
      ({ id, terminalId }) => window.prime.terminal(id, { action: 'close', id: terminalId }),
      {
        id: remote.id,
        terminalId: terminal.id
      }
    )
    const previousSessions = await page.evaluate(
      (id) => window.prime.call(id, 'sessions'),
      remote.id
    )
    expect(previousSessions.length).toBeGreaterThan(0)
    const previousMessages = await page.evaluate(
      ({ id, sessionId }) => window.prime.call(id, 'messages', { sessionId }),
      { id: remote.id, sessionId: previousSessions[0].id }
    )
    const { page: reopenedPage } = await workbench.restart()
    await expect(reopenedPage.locator('.project-item.selected')).toContainText('WSL fixture', {
      timeout: 90000
    })
    const reopenedProject = (await reopenedPage.evaluate(() => window.prime.projects())).find(
      (item: Project) => item.remote
    )!
    expect(reopenedProject.path).toBe(remote.path)
    const restoredSessions = await reopenedPage.evaluate(
      (id) => window.prime.call(id, 'sessions'),
      reopenedProject.id
    )
    expect(restoredSessions.map((item: { id: string }) => item.id)).toEqual(
      expect.arrayContaining(previousSessions.map((item: { id: string }) => item.id))
    )
    expect(
      await reopenedPage.evaluate(
        ({ id, sessionId }) => window.prime.call(id, 'messages', { sessionId }),
        { id: reopenedProject.id, sessionId: previousSessions[0].id }
      )
    ).toEqual(previousMessages)
    await reopenedPage.evaluate((id) => window.prime.closeProject(id), reopenedProject.id)
    await expect
      .poll(async () =>
        (await reopenedPage.evaluate(() => window.prime.projects())).some(
          (item: Project) => item.remote
        )
      )
      .toBe(false)
    await expect(
      reopenedPage.locator('.project-item').filter({ hasText: 'WSL fixture' })
    ).toHaveCount(0)
    expect(errors).toEqual([])
  } finally {
    await workbench.close()
    await ssh.close()
  }
})
