import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('model picker stays still while the workspace refreshes', async () => {
  test.setTimeout(120000)
  const workbench = await launchWorkbench()
  const page = workbench.page
  try {
    await expect(page.locator('.model-control-name')).toHaveText('Desktop Fixture')
    await page.getByRole('button', { name: 'Модель и уровень размышлений', exact: true }).click()
    await page.evaluate(() => window.dispatchEvent(new Event('prime:models')))
    await expect(page.getByRole('option', { name: 'Desktop Fixture', exact: false })).toBeVisible()
    await page.waitForTimeout(500)
    await page.evaluate(() => {
      const state = { notes: 0, moves: 0, labels: 0, frames: 0 }
      ;(window as any).__pickerProbe = state
      let lastTop = -1
      let lastLabel = document.querySelector('.model-control-name')?.textContent
      const sample = () => {
        const popover = document.querySelector('.prime-popover')
        if (popover?.querySelector('.picker-note')) state.notes++
        const top = Math.round(parseFloat((popover as HTMLElement | null)?.style.top || '-1'))
        if (lastTop !== -1 && top !== lastTop) state.moves++
        lastTop = top
        const label = document.querySelector('.model-control-name')?.textContent
        if (label !== lastLabel) state.labels++
        lastLabel = label
        state.frames++
        requestAnimationFrame(sample)
      }
      requestAnimationFrame(sample)
    })
    await page.waitForTimeout(12000)
    const probe = await page.evaluate(() => (window as any).__pickerProbe)
    expect(probe.frames).toBeGreaterThan(100)
    expect(probe.notes).toBe(0)
    expect(probe.moves).toBe(0)
    expect(probe.labels).toBe(0)
    await page.keyboard.press('Escape')
    await expect(page.locator('.prime-popover')).toHaveCount(0)
    const reopened = await page.evaluate(async () => {
      window.dispatchEvent(new Event('prime:models'))
      const frames: { open: boolean; note: boolean }[] = []
      for (let index = 0; index < 20; index++) {
        await new Promise((resolve) => requestAnimationFrame(resolve))
        const popover = document.querySelector('.prime-popover')
        frames.push({ open: !!popover, note: !!popover?.querySelector('.picker-note') })
      }
      return frames
    })
    expect(reopened.some((frame) => frame.open)).toBe(true)
    expect(reopened.some((frame) => frame.note)).toBe(false)
    await page.screenshot({ path: 'test-results/model-picker.png' })
    expect(workbench.errors).toEqual([])
  } finally {
    await workbench.close()
  }
})
