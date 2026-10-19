import { _electron as electron, expect } from '@playwright/test'
import { createServer } from 'vite'
import react from '@vitejs/plugin-react'
import { build } from 'esbuild'
import { resolve } from 'node:path'

const manager = resolve('.superpowers/workbench/terminal-manager.cjs')
await build({ entryPoints: ['src/main/terminal/manager.ts'], outfile: manager, bundle: true, platform: 'node', format: 'cjs', external: ['node-pty', 'koffi'], logLevel: 'silent' })
const server = await createServer({ configFile: false, plugins: [react()], logLevel: 'error', server: { host: '127.0.0.1', port: 0, watch: null } })
await server.listen()
const env = { ...process.env, PRIME_TERMINAL_MANAGER: manager, PRIME_TERMINAL_UI_URL: `${server.resolvedUrls.local[0]}tests/fixtures/terminal-ui.html` }
delete env.ELECTRON_RUN_AS_NODE
const desktop = await electron.launch({ args: [resolve('tests/fixtures/terminal-ui-main.cjs')], env })
try {
  const page = await desktop.firstWindow()
  const errors = []
  page.on('pageerror', (error) => { errors.push(error.message); console.error('pageerror', error.message) })
  page.on('console', (message) => { if (message.type() === 'error') console.error(message.text()) })
  await expect(page.getByRole('button', { name: 'Новый терминал', exact: true }).first()).toBeEnabled({ timeout: 15000 })
  await page.screenshot({ path: '.superpowers/workbench/terminal-empty.png' })
  await page.getByRole('button', { name: 'Новый терминал', exact: true }).first().click()
  await expect(page.locator('.xterm-helper-textarea')).toHaveCount(1)
  await expect(page.getByRole('tab')).toHaveCount(1)
  await page.locator('.xterm-helper-textarea').focus()
  await page.keyboard.type("Write-Output ('UI_'+'ROUNDTRIP')")
  await page.keyboard.press('Enter')
  await expect(page.locator('.xterm-screen')).toContainText('UI_ROUNDTRIP', { timeout: 15000 })
  await page.getByRole('button', { name: 'Передать агенту', exact: true }).click()
  await expect(page.getByText('Управляет агент', { exact: true })).toBeVisible()
  const sessions = await page.evaluate(() => window.terminalFixture.api({ action: 'list' }))
  expect(sessions[0].owner).toBe('agent')
  await page.screenshot({ path: '.superpowers/workbench/terminal-agent.png' })
  await page.getByRole('button', { name: 'Вернуть управление', exact: true }).click()
  const writesBeforeRemount = (await page.evaluate(() => window.terminalFixture.writes())).filter((data) => !/^\x1b\[[IO]$/.test(data))
  await page.getByRole('button', { name: 'Toggle panel', exact: true }).click()
  await expect(page.getByRole('tab')).toHaveCount(0)
  const alive = await page.evaluate(() => window.terminalFixture.api({ action: 'list' }))
  expect(alive[0].id).toBe(sessions[0].id)
  await page.getByRole('button', { name: 'Toggle panel', exact: true }).click()
  await expect(page.locator('.xterm-screen')).toContainText('UI_ROUNDTRIP', { timeout: 10000 })
  expect((await page.locator('.xterm-screen').textContent()).match(/UI_ROUNDTRIP/g)).toHaveLength(1)
  await page.waitForTimeout(250)
  const writesAfterRemount = (await page.evaluate(() => window.terminalFixture.writes())).filter((data) => !/^\x1b\[[IO]$/.test(data))
  expect(writesAfterRemount).toEqual(writesBeforeRemount)
  expect(writesAfterRemount.some((data) => data === '\x1b[?1;2c')).toBe(false)
  await expect(page.getByRole('tab')).toHaveCount(1)
  await page.getByRole('button', { name: 'Новый терминал', exact: true }).click()
  await expect(page.getByRole('tab')).toHaveCount(2)
  await expect(page.locator('.terminal-session:not([hidden]) .xterm-helper-textarea')).toHaveCount(1)
  await page.getByRole('tab').first().click()
  await page.getByRole('button', { name: 'Копировать вывод', exact: true }).click()
  await expect(page.getByTitle('Скопировано', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Очистить экран', exact: true }).click()
  await page.screenshot({ path: '.superpowers/workbench/terminal-tabs.png' })
  await page.getByRole('button', { name: 'Закрыть терминал powershell.exe', exact: true }).first().click()
  await expect(page.getByRole('tab')).toHaveCount(1)
  await page.getByRole('button', { name: 'Закрыть терминал powershell.exe', exact: true }).click()
  await expect(page.getByText('Команды в контексте проекта', { exact: true })).toBeVisible()
  expect(errors).toEqual([])
  console.log(JSON.stringify({ result: 'terminal-ui-ok', input: true, tabs: true, owner: true, remount: true, replayWithoutProtocolInput: true, copy: true, clear: true, close: true, pageErrors: errors }))
} finally {
  await desktop.close()
  await server.close()
}
