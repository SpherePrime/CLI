import { test, expect, type Page } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

async function setLevel(page: Page, index: number) {
  await page.getByRole('button', { name: 'Модель и уровень размышлений', exact: true }).click()
  const slider = page.getByRole('slider', { name: 'Уровень размышлений', exact: true })
  await slider.press('Home')
  for (let step = 1; step <= index; step++) {
    await expect(slider).toHaveAttribute('aria-disabled', 'false')
    await slider.press('ArrowRight')
    await expect(slider).toHaveAttribute('aria-valuenow', String(step))
  }
  await page.keyboard.press('Escape')
}

test('keeps chat settings, sends, switches chats smoothly and manages recents', async () => {
  test.setTimeout(240000)
  const workbench = await launchWorkbench()
  let page = workbench.page
  const textbox = () => page.getByRole('textbox', { name: 'Сообщение Prime', exact: true })
  const sendButton = () => page.getByRole('button', { name: 'Отправить сообщение', exact: true })
  const sendPrompt = async (prompt: string) => {
    await textbox().fill(prompt)
    await sendButton().click()
    await expect
      .poll(
        () =>
          workbench.provider.prompts.some(
            (body) => body.stream && JSON.stringify(body.messages).includes(prompt)
          ),
        { timeout: 30000 }
      )
      .toBe(true)
    await expect(sendButton()).toBeVisible({ timeout: 30000 })
    await expect(textbox()).toHaveValue('')
  }
  try {
    await workbench.app.evaluate(({ BrowserWindow }) =>
      BrowserWindow.getAllWindows()[0].setSize(1200, 720)
    )
    for (let index = 0; index < 4; index++) await sendPrompt(`first-chat-request-${index}`)
    await setLevel(page, 3)
    await expect(page.locator('.model-control-level')).toHaveText('Высокий')

    await page.locator('.new-chat').click()
    await expect(page.locator('.model-control-level')).toHaveText('Высокий')
    await page.getByRole('combobox', { name: 'Режим агента', exact: true }).click()
    await page.getByRole('option', { name: 'План', exact: false }).click()
    await expect(page.getByRole('combobox', { name: 'Режим агента', exact: true })).toHaveText(
      'План'
    )
    await sendPrompt('second-chat-request')

    const sessions = await page.evaluate(
      (id) => window.prime.call(id, 'sessions'),
      workbench.project.id
    )
    const [second, first] = [...sessions].sort(
      (a: any, b: any) => (b.updated_at || 0) - (a.updated_at || 0)
    )
    for (const [session, title] of [
      [first, 'Alpha chat'],
      [second, 'Beta chat']
    ] as const)
      await page.evaluate(
        ({ id, session, title }) =>
          window.prime.call(id, 'renameSession', {
            sessionId: session.id,
            session: { ...session, title }
          }),
        { id: workbench.project.id, session, title }
      )
    await page.evaluate(() => window.dispatchEvent(new Event('focus')))
    const projectRows = page.locator('.project-group .session-row:not(.recent-chat)')
    await expect(projectRows.filter({ hasText: 'Alpha chat' })).toBeVisible({ timeout: 15000 })

    await page.evaluate(() => {
      const state = { welcome: false, positions: [] as number[] }
      ;(window as any).__switchProbe = state
      const observer = new MutationObserver(() => {
        if (document.querySelector('.welcome')) state.welcome = true
      })
      observer.observe(document.body, { childList: true, subtree: true })
      let frames = 0
      let tracked: Element | null = document.querySelector('.message-rows')
      const sample = () => {
        const rows = document.querySelector('.message-rows')
        if (rows !== tracked) {
          tracked = rows
          state.positions = []
        }
        const last = rows?.matches('.settling')
          ? null
          : rows?.lastElementChild?.getBoundingClientRect()
        if (last) state.positions.push(Math.round(last.bottom))
        if (++frames < 120) requestAnimationFrame(sample)
      }
      requestAnimationFrame(sample)
    })
    await projectRows.filter({ hasText: 'Alpha chat' }).locator('.session-link').click()
    await expect(page.locator('.message-rows')).toContainText('first-chat-request-3')
    await page.waitForTimeout(1600)
    const probe = await page.evaluate(() => (window as any).__switchProbe)
    expect(probe.welcome).toBe(false)
    const settledBottom = probe.positions.at(-1)
    expect(probe.positions.every((value: number) => Math.abs(value - settledBottom) <= 2)).toBe(
      true
    )
    await expect(page.locator('.model-control-level')).toHaveText('Высокий')

    await expect(projectRows.filter({ hasText: 'Alpha chat' })).toHaveClass(/active/)
    await expect(page.locator('.recent-chat.active')).toHaveCount(0)

    const conversation = page.locator('.conversation')
    await conversation.hover()
    await page.mouse.wheel(0, -4000)
    const jump = page.locator('.jump-bottom')
    await expect(jump).toBeVisible()
    await page.waitForTimeout(400)
    const centers = await page.evaluate(() => {
      const button = document.querySelector('.jump-bottom')!.getBoundingClientRect()
      const panel = document.querySelector('.conversation-panel')!.getBoundingClientRect()
      return {
        button: button.left + button.width / 2,
        panel: panel.left + panel.width / 2
      }
    })
    expect(Math.abs(centers.button - centers.panel)).toBeLessThanOrEqual(1)
    await page.screenshot({ path: 'test-results/chat-state-jump.png' })
    await jump.click()
    await expect(jump).toBeHidden()

    const recentBeta = page.locator('.recent-chat').filter({ hasText: 'Beta chat' })
    await recentBeta.hover()
    await recentBeta.getByRole('button', { name: 'Удалить чат', exact: true }).click()
    await expect(page.locator('.session-row').filter({ hasText: 'Beta chat' })).toHaveCount(0)
    await page.screenshot({ path: 'test-results/chat-state-sidebar.png' })

    await page.locator('.project-item').filter({ hasText: 'Личные чаты' }).click()
    await expect(page.locator('.workspace-topbar')).toContainText('Личные чаты')
    await sendPrompt('personal-chat-request')
    await page.screenshot({ path: 'test-results/chat-state-personal.png' })

    ;({ page } = await workbench.restart())
    await expect(page.getByRole('combobox', { name: 'Режим агента', exact: true })).toHaveText(
      'План'
    )
    await expect(page.locator('.model-control-level')).toHaveText('Высокий')
    const preferences = await page.evaluate(() => window.prime.preferences())
    expect(preferences.agentMode).toBe('plan')
    expect(preferences.lastReasoningMode).toBe('high')
    await page.locator('.new-chat').click()
    await expect(page.locator('.model-control-level')).toHaveText('Высокий')
    await sendPrompt('after-restart-request')
    expect(workbench.errors).toEqual([])
  } finally {
    await workbench.close()
  }
})
