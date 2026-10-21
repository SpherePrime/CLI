import { test, expect } from '@playwright/test'
import { writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { launchWorkbench } from '../fixtures/workbench-app'
import sharp from 'sharp'

test('browser tools target background tabs concurrently without changing the selected tab', async () => {
  const fixture = await launchWorkbench()
  try {
    for (const name of ['Alpha', 'Beta'])
      await writeFile(
        join(fixture.root, `${name}.html`),
        `<!doctype html><title>${name}</title><input placeholder="Name" onkeydown="if(event.key==='Enter')document.body.dataset.enter='yes'"><button onclick="document.querySelector('output').textContent='${name}: '+document.querySelector('input').value">Apply</button><output>waiting</output>`
      )
    const endpoint = await fixture.app.evaluate(() => ({
      url: process.env.PRIME_DESKTOP_BROWSER_URL!,
      token: process.env.PRIME_DESKTOP_BROWSER_TOKEN!
    }))
    const call = async (name: string, args: Record<string, unknown> = {}) => {
      const response = await fetch(endpoint.url, {
        signal: AbortSignal.timeout(12000),
        method: 'POST',
        headers: { Authorization: `Bearer ${endpoint.token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          jsonrpc: '2.0',
          id: Math.random(),
          method: 'tools/call',
          params: { name, arguments: args }
        })
      })
      const body = await response.json()
      if (body.error || body.result?.isError) throw new Error(JSON.stringify(body))
      return body.result.content
    }
    const json = async (name: string, args: Record<string, unknown> = {}) =>
      JSON.parse((await call(name, args))[0].text)
    const alpha = await json('browser_new_tab', {
      url: join(fixture.root, 'Alpha.html'),
      activate: true
    })
    const beta = await json('browser_new_tab', { url: join(fixture.root, 'Beta.html') })
    expect(beta.tabId).not.toBe(alpha.tabId)
    expect((await json('browser_tabs')).activeTab).toBe(alpha.tabId)
    const snapshots = await Promise.all(
      [alpha, beta].map((tab) => call('browser_snapshot', { tabId: tab.tabId }))
    )
    const refs = snapshots.map((content) => ({
      input: Number(content[0].text.match(/\[(\d+)\] input:text "Name"/)[1]),
      button: Number(content[0].text.match(/\[(\d+)\] button "Apply"/)[1])
    }))
    await Promise.all(
      [alpha, beta].map((tab, index) =>
        call('browser_type', {
          tabId: tab.tabId,
          ref: refs[index].input,
          text: index ? 'Second' : 'First',
          submit: true
        })
      )
    )
    await Promise.all(
      [alpha, beta].map((tab, index) =>
        call('browser_click', { tabId: tab.tabId, ref: refs[index].button })
      )
    )
    for (const [index, tab] of [alpha, beta].entries()) {
      const value = await json('browser_evaluate', {
        tabId: tab.tabId,
        expression:
          '({text:document.querySelector("output").textContent, enter:document.body.dataset.enter})'
      })
      expect(value).toEqual({ text: index ? 'Beta: Second' : 'Alpha: First', enter: 'yes' })
    }
    const shot = await call('browser_screenshot', { tabId: beta.tabId })
    expect(shot[0].type).toBe('image')
    const image = sharp(Buffer.from(shot[0].data, 'base64'))
    expect((await image.metadata()).width).toBeGreaterThan(0)
    expect((await image.stats()).channels.some((channel) => channel.stdev > 1)).toBe(true)
    await writeFile('test-results/browser-background.png', Buffer.from(shot[0].data, 'base64'))
    expect((await json('browser_tabs')).activeTab).toBe(alpha.tabId)
    await call('browser_close_tab', { tabId: beta.tabId })
    await expect(
      call('browser_evaluate', { tabId: beta.tabId, expression: 'document.title' })
    ).rejects.toThrow()
    expect((await json('browser_tabs')).activeTab).toBe(alpha.tabId)
    expect(await json('browser_evaluate', { expression: 'document.title' })).toBe('Alpha')
  } finally {
    await fixture.close()
  }
})
