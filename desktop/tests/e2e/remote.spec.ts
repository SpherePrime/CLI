import { test, expect } from '@playwright/test'
import { readFile } from 'node:fs/promises'
import { Client } from 'ssh2'
import type { TerminalRead, TerminalSnapshot } from '../../src/shared/terminal'
import type { Project } from '../../src/shared/contracts'
import { createRemoteForward } from '../../src/main/remoteRuntime/forward'
import { createSshLinuxFixture, detectSshLinuxFixture } from '../fixtures/ssh-linux'
import { launchWorkbench } from '../fixtures/workbench-app'

const providerConfig = (url: string) =>
  JSON.stringify({
    options: {
      disable_default_providers: true,
      disable_provider_auto_update: true,
      disable_metrics: true,
      disable_auto_summarize: true
    },
    providers: {
      fixture: {
        id: 'fixture',
        name: 'Fixture',
        type: 'openai-compat',
        base_url: url,
        api_key: 'fixture-key',
        discover_models: false,
        models: [
          { id: 'desktop-fixture', name: 'Desktop Fixture', context_window: 128000, default_max_tokens: 8192 }
        ]
      }
    },
    models: {
      large: { provider: 'fixture', model: 'desktop-fixture' },
      small: { provider: 'fixture', model: 'desktop-fixture' }
    }
  })

test('remote SSH project: connect, agent, file edit, terminal, disconnect', async () => {
  test.setTimeout(240000)
  const support = await detectSshLinuxFixture()
  test.skip(!support.available, support.reason)
  const ssh = await createSshLinuxFixture()
  const tunnel = new Client()
  const workbench = await launchWorkbench()
  const { page, errors } = workbench
  try {
    await new Promise<void>((resolve, reject) => {
      tunnel.once('ready', resolve).once('error', reject)
      void readFile(ssh.keyPath).then((privateKey) =>
        tunnel.connect({ host: ssh.host, port: ssh.port, username: 'root', privateKey, hostVerifier: () => true })
      )
    })
    const forward = await createRemoteForward(tunnel, Number(new URL(workbench.provider.url).port))
    const config = providerConfig(`http://127.0.0.1:${forward.port}/v1`).replaceAll("'", "'\\''")
    await ssh.execute(`printf '%s' '${config}' > ../config/prime.json`)
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
    await form.getByLabel('Аутентификация').selectOption('key')
    await form.getByLabel('Приватный ключ').fill(ssh.keyPath)
    await form.getByRole('button', { name: 'Сохранить' }).click()
    await dialog.getByRole('button', { name: 'Подключить', exact: true }).click()
    await dialog.getByRole('button', { name: 'Я проверил отпечаток' }).click()
    await dialog.getByRole('button', { name: 'Подтвердить доверие' }).click()
    await dialog.getByRole('button', { name: 'Подключить', exact: true }).click()
    await expect(dialog.getByRole('button', { name: 'Открыть проект' })).toBeVisible({ timeout: 90000 })
    await dialog.getByRole('button', { name: 'Открыть проект' }).click()
    await expect(page.locator('.project-item.selected')).toContainText('WSL fixture')

    const remote = (await page.evaluate(() => window.prime.projects())).find(
      (item: Project) => item.remote
    )!
    expect(remote.path.startsWith('prime-ssh://')).toBe(true)
    expect(remote.remote?.root).toBe(ssh.root)

    await page.getByRole('textbox', { name: 'Сообщение Prime', exact: true }).fill('TOOL_FIXTURE Создай тестовый файл')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Разрешить действие?' })).toBeVisible({ timeout: 30000 })
    await page.getByRole('button', { name: 'Разрешить', exact: true }).click()
    await expect.poll(() => ssh.execute('cat created-by-tool.txt').catch(() => ''), { timeout: 30000 }).toBe('Tool execution verified')
    await expect(page.getByRole('button', { name: 'Остановить задачу', exact: true })).toHaveCount(0, { timeout: 30000 })

    const opened = await page.evaluate((id) => window.prime.readFile(id, 'app.ts'), remote.id)
    expect(opened.content).toBe('export const version = 1\n')
    await page.evaluate(
      ({ id, revision }) => window.prime.saveFile(id, 'app.ts', 'export const version = 2\n', revision),
      { id: remote.id, revision: opened.revision }
    )
    expect(await ssh.execute('cat app.ts')).toBe('export const version = 2')
    const changes = await page.evaluate((id) => window.prime.changes(id), remote.id)
    expect(changes.map((item) => item.path).sort()).toEqual(['app.ts', 'created-by-tool.txt'])
    const diff = await page.evaluate((id) => window.prime.diff(id, '', 'app.ts', 'git'), remote.id)
    expect(diff.patch).toContain('+export const version = 2')
    await expect(page.locator('.inspector').getByText('app.ts', { exact: true }).first()).toBeVisible()

    const terminal = (await page.evaluate(
      (id) => window.prime.terminal(id, { action: 'create', options: { cols: 100, rows: 30 } }),
      remote.id
    )) as TerminalSnapshot
    await page.evaluate(
      ({ id, terminalId }) => window.prime.terminal(id, { action: 'write', id: terminalId, data: 'pwd; echo REMOTE_$((40+2))\n' }),
      { id: remote.id, terminalId: terminal.id }
    )
    await expect
      .poll(async () => {
        const read = (await page.evaluate(
          ({ id, terminalId }) => window.prime.terminal(id, { action: 'read', id: terminalId }),
          { id: remote.id, terminalId: terminal.id }
        )) as TerminalRead
        return read.output.includes('REMOTE_42') && read.output.includes(ssh.root)
      }, { timeout: 20000 })
      .toBe(true)

    const runtimeCount = () =>
      ssh.execute("pgrep -fc '[p]rime-desktop/runtime/linux-.*/prime server' || true")
    expect(Number(await runtimeCount())).toBe(1)
    await page.evaluate(({ id, terminalId }) => window.prime.terminal(id, { action: 'close', id: terminalId }), {
      id: remote.id,
      terminalId: terminal.id
    })
    await page.evaluate((id) => window.prime.closeProject(id), remote.id)
    await expect.poll(async () => (await page.evaluate(() => window.prime.projects())).some((item: Project) => item.remote)).toBe(false)
    await expect.poll(async () => Number(await runtimeCount()), { timeout: 20000 }).toBe(0)
    await expect(page.locator('.project-item').filter({ hasText: 'WSL fixture' })).toHaveCount(0)
    await forward.close()
    expect(errors).toEqual([])
  } finally {
    tunnel.end()
    await workbench.close()
    await ssh.close()
  }
})

test('remote runtime stops when the SSH connection drops and reconnects cleanly', async () => {
  test.setTimeout(240000)
  const support = await detectSshLinuxFixture()
  test.skip(!support.available, support.reason)
  const ssh = await createSshLinuxFixture()
  const workbench = await launchWorkbench()
  const { page } = workbench
  try {
    const saved = await page.evaluate(
      (input) => window.prime.remote({ action: 'save', input }),
      { name: 'Drop fixture', host: ssh.host, port: ssh.port, user: 'root', root: ssh.root, auth: 'key' as const, keyPath: ssh.keyPath, runtime: 'bundled' as const }
    )
    const id = saved.profiles[0].id
    const challenged = await page.evaluate((id) => window.prime.remote({ action: 'connect', id }), id)
    await page.evaluate(
      ({ id, fingerprint }) => window.prime.remote({ action: 'trust', id, fingerprint }),
      { id, fingerprint: challenged.profiles[0].challenge!.fingerprint }
    )
    const connect = async () => {
      const result = await page.evaluate((id) => window.prime.remote({ action: 'connect', id }), id)
      expect(result.profiles[0].status).toBe('connected')
      return page.evaluate((id) => window.prime.openRemote(id), id)
    }
    const project = await connect()
    await page.evaluate((id) => window.prime.terminal(id, { action: 'create' }), project.id)
    const runtimeCount = () =>
      ssh.execute("pgrep -fc '[p]rime-desktop/runtime/linux-.*/prime server' || true")
    expect(Number(await runtimeCount())).toBe(1)
    await ssh.execute("pkill -9 -f '[s]shd-session: root' || true")
    await expect
      .poll(async () => (await page.evaluate(() => window.prime.remote({ action: 'list' }))).profiles[0].status, { timeout: 60000 })
      .toBe('error')
    await expect.poll(async () => Number(await runtimeCount()), { timeout: 30000 }).toBe(0)
    expect((await page.evaluate(() => window.prime.projects())).some((item: Project) => item.remote)).toBe(false)
    const terminals = await page.evaluate((id) => window.prime.terminal(id, { action: 'list' }).catch(() => []), project.id)
    expect(terminals).toEqual([])
    const reopened = await connect()
    expect(reopened.path).toBe(project.path)
    const sessions = await page.evaluate((id) => window.prime.call(id, 'sessions'), reopened.id)
    expect(Array.isArray(sessions)).toBe(true)
    await page.evaluate((id) => window.prime.remote({ action: 'disconnect', id }), id)
    await expect.poll(async () => Number(await runtimeCount()), { timeout: 30000 }).toBe(0)
  } finally {
    await workbench.close()
    await ssh.close()
  }
})
