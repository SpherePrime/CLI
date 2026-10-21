import { test } from '@playwright/test'
import { DatabaseSync } from 'node:sqlite'
import { join } from 'node:path'
import { launchWorkbench } from '../fixtures/workbench-app'

test('memory soak: chat switching and history scrolling', async () => {
  test.setTimeout(240000)
  const fixture = await launchWorkbench()
  const { page, project, root, app } = fixture
  try {
    const sessions: { id: string }[] = []
    for (const title of ['Soak A', 'Soak B', 'Soak C'])
      sessions.push(
        await page.evaluate(
          ({ id, name }: { id: string; name: string }) =>
            window.prime.call(id, 'createSession', { title: name }),
          { id: project.id, name: title }
        )
      )
    const database = new DatabaseSync(join(root, '.prime', 'prime.db'))
    try {
      const insert = database.prepare(
        'INSERT INTO messages(id,session_id,role,parts,created_at,updated_at,finished_at) VALUES(?,?,?,?,?,?,?)'
      )
      const start = Math.floor(Date.now() / 1000) - 100000
      for (const session of sessions)
        for (let index = 0; index < 600; index++) {
          const text = `Soak history ${String(index).padStart(3, '0')}`
          insert.run(`${session.id}-${index}`, session.id, index % 2 ? 'assistant' : 'user',
            JSON.stringify([{ type: 'text', data: { text } }]),
            start + index, start + index, start + index)
        }
    } finally {
      database.close()
    }
    await page.locator('.app-shell > .sidebar').waitFor({ state: 'visible', timeout: 30000 })
    const samples: number[] = []
    const sample = async () => {
      const metrics = await app.evaluate(({ app: electronApp }) =>
        electronApp.getAppMetrics().map((metric) => ({
          type: metric.type,
          memory: metric.memory?.workingSetSize || 0
        }))
      )
      const total = metrics.reduce((sum, metric) => sum + metric.memory, 0) / 1024
      samples.push(total)
      return { total, peak: Math.max(...metrics.map((metric) => metric.memory)) / 1024 }
    }
    let last: { total: number; peak: number } | null = null
    for (let cycle = 0; cycle < 20; cycle++) {
      const session = sessions[cycle % sessions.length]
      const link = page
        .locator('.project-group .session-link')
        .filter({ hasText: session.id ? '' : '' })
      void link
      await page
        .locator('.project-group .session-link')
        .filter({ hasText: `Soak ${['A', 'B', 'C'][cycle % 3]}` })
        .click()
      await page.locator('.conversation').waitFor({ state: 'visible', timeout: 15000 })
      if (cycle % 2 === 0) {
        await page.locator('.conversation').hover()
        await page.mouse.wheel(0, -20000)
        await page.waitForTimeout(400)
        await page.mouse.wheel(0, 40000)
      }
      last = await sample()
      await page.waitForTimeout(400)
    }
    const peak = Math.max(...samples)
    console.log('MEMORY_SOAK_MB', JSON.stringify({ last: last?.total, peak, samples: samples.length }))
  } finally {
    await fixture.close()
  }
})
