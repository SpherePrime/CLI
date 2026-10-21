import { test, expect } from '@playwright/test'
import { mkdir } from 'node:fs/promises'
import { join } from 'node:path'
import { launchWorkbench } from '../fixtures/workbench-app'

test('projects move in both directions and retain order after opening and restarting', async () => {
  const fixture = await launchWorkbench()
  let page = fixture.page
  const paths = [
    fixture.root,
    join(fixture.root, 'second-project'),
    join(fixture.root, 'third-project')
  ]
  try {
    for (const path of paths.slice(1)) {
      await mkdir(path)
      await fixture.app.evaluate(({ dialog }, path) => {
        dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [path] })
      }, path)
      await page.evaluate(() => window.prime.openProject())
    }
    await page.reload()
    if (!(await page.locator('.app-shell > .sidebar').isVisible()))
      await page.locator('.workspace-topbar [data-projects-toggle]').click()
    const headings = () => page.locator('.project-reorder-heading')
    const order = () =>
      headings().evaluateAll((elements) =>
        elements.map((element) => element.getAttribute('data-project-path'))
      )
    await expect.poll(order).toEqual([...paths].reverse())
    await headings()
      .last()
      .locator('.project-item')
      .dragTo(headings().first(), { targetPosition: { x: 60, y: 3 } })
    await expect.poll(order).toEqual([paths[0], paths[2], paths[1]])
    const targetHeight = (await headings().last().boundingBox())!.height
    await headings()
      .first()
      .locator('.project-item')
      .dragTo(headings().last(), { targetPosition: { x: 60, y: targetHeight - 3 } })
    await expect.poll(order).toEqual([paths[2], paths[1], paths[0]])
    await page.evaluate((path) => window.prime.openProject(path), paths[0])
    ;({ page } = await fixture.restart())
    await expect.poll(order).toEqual([paths[2], paths[1], paths[0]])
    await page.screenshot({ path: 'test-results/project-order.png' })
  } finally {
    await fixture.close()
  }
})
