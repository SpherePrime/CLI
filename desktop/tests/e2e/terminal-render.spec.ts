import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('agent-created terminals render their output', async () => {
  test.setTimeout(120000)
  const fixture = await launchWorkbench()
  const { page, project } = fixture
  try {
    for (const title of ['dev', 'tests']) {
      const terminal = (await page.evaluate(({ id, title }) => window.prime.terminal(id, { action: 'create', options: { title } }), { id: project.id, title })) as { id: string }
      await page.evaluate(({ id, tid }) => window.prime.terminal(id, { action: 'write', id: tid, data: "Write-Output ('READY_' + 'TERMINAL')\r" }), { id: project.id, tid: terminal.id })
    }
    await page.getByRole('button', { name: 'Терминал проекта', exact: true }).click()
    await expect.poll(() => page.locator('.terminal-session:not([hidden]) .xterm-rows').innerText(), { timeout: 20000 }).toContain('READY_TERMINAL')
    await page.screenshot({ path: 'test-results/dock-terminals.png' })
  } finally {
    await fixture.close()
  }
})
