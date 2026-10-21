import { test, expect, type Page } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

async function sendPrompt(page: Page, prompts: any[], prompt: string) {
  await page.getByRole('textbox', { name: 'Сообщение Prime', exact: true }).fill(prompt)
  await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
  await expect
    .poll(() => prompts.some((body) => body.stream && JSON.stringify(body.messages).includes(prompt)))
    .toBe(true)
  await expect(page.getByRole('button', { name: 'Отправить сообщение', exact: true })).toBeVisible({
    timeout: 30000
  })
}

async function rename(page: Page, projectId: string, title: string) {
  await page.evaluate(
    async ({ projectId, title }) => {
      const [session] = await window.prime.call(projectId, 'sessions')
      await window.prime.call(projectId, 'renameSession', {
        sessionId: session.id,
        session: { ...session, title }
      })
    },
    { projectId, title }
  )
}

async function startProbe(page: Page) {
  await page.evaluate(() => {
    const state = { welcome: false, emptyGroups: 0, blankRows: 0, frames: 0 }
    ;(window as any).__projectProbe = state
    const sample = () => {
      if (document.querySelector('.welcome')) state.welcome = true
      const groups = [...document.querySelectorAll('.project-group')].filter((group) =>
        group.querySelector('.project-item[aria-expanded="true"]')
      )
      if (groups.some((group) => !group.querySelector('.session-row'))) state.emptyGroups++
      const rows = document.querySelector('.message-rows')
      if (!rows || !rows.childElementCount) state.blankRows++
      if (++state.frames < 120) requestAnimationFrame(sample)
    }
    requestAnimationFrame(sample)
  })
}

test('switching chats across projects keeps the layout steady', async () => {
  test.setTimeout(240000)
  const workbench = await launchWorkbench()
  const page = workbench.page
  try {
    await sendPrompt(page, workbench.provider.prompts, 'project-chat-request')
    await rename(page, workbench.project.id, 'Project chat')
    await page.locator('.project-item').filter({ hasText: 'Личные чаты' }).click()
    await expect(page.locator('.workspace-topbar')).toContainText('Личные чаты')
    await sendPrompt(page, workbench.provider.prompts, 'personal-chat-request')
    const personal = await page.evaluate(async () =>
      (await window.prime.projects()).find((item) => item.kind === 'chat')!
    )
    await rename(page, personal.id, 'Personal chat')
    const projectGroup = page.locator('.project-group').filter({ hasText: 'prime-workbench' })
    if ((await projectGroup.locator('.session-row').count()) === 0)
      await projectGroup.locator('.project-item').click()
    const projectLink = page
      .locator('.project-group .session-row:not(.recent-chat)')
      .filter({ hasText: 'Project chat' })
      .locator('.session-link')
    const personalLink = page
      .locator('.project-group .session-row:not(.recent-chat)')
      .filter({ hasText: 'Personal chat' })
      .locator('.session-link')
    await expect(projectLink).toBeVisible({ timeout: 15000 })
    await expect(personalLink).toBeVisible({ timeout: 15000 })

    for (let round = 0; round < 3; round++) {
      for (const [link, text] of [
        [projectLink, 'project-chat-request'],
        [personalLink, 'personal-chat-request']
      ] as const) {
        await startProbe(page)
        await link.click()
        await expect(page.locator('.message-rows')).toContainText(text)
        await page.waitForTimeout(1500)
        const probe = await page.evaluate(() => (window as any).__projectProbe)
        expect(probe.welcome).toBe(false)
        expect(probe.emptyGroups).toBe(0)
        if (round > 0) expect(probe.blankRows).toBe(0)
      }
    }
    await page.screenshot({ path: 'test-results/chat-project-switch.png' })
    expect(workbench.errors).toEqual([])
  } finally {
    await workbench.close()
  }
})
