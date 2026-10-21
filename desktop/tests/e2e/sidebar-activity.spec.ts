import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('personal recent chats and project chats show the same live working indicator', async () => {
  const fixture = await launchWorkbench()
  const { page, project } = fixture
  let chats:
    { personalProject: string; personalSession: string; projectSession: string } | undefined
  try {
    chats = await page.evaluate(async (projectId) => {
      const personal = await window.prime.openChat()
      const personalSession = await window.prime.call(personal.id, 'createSession', {
        title: 'Personal activity'
      })
      await window.prime.call(personal.id, 'init')
      const projectSession = await window.prime.call(projectId, 'createSession', {
        title: 'Project activity'
      })
      await window.prime.call(projectId, 'currentSession', { sessionId: projectSession.id })
      return {
        personalProject: personal.id,
        personalSession: personalSession.id,
        projectSession: projectSession.id
      }
    }, project.id)
    await page.reload()
    if (!(await page.locator('.app-shell > .sidebar').isVisible()))
      await page.locator('.workspace-topbar [data-projects-toggle]').click()
    const personalRow = page.locator('.recent-chat').filter({ hasText: 'Personal activity' })
    const projectRow = page
      .locator('.project-group .session-row')
      .filter({ hasText: 'Project activity' })
    await expect(personalRow).toBeVisible()
    await expect(projectRow).toBeVisible()
    await personalRow.click({ button: 'right' })
    await page.getByRole('button', { name: 'Закрепить', exact: true }).click()
    await expect(personalRow.locator('.lucide-pin')).toBeVisible()
    await personalRow.click({ button: 'right' })
    await page.getByRole('button', { name: 'Переименовать', exact: true }).click()
    await page.locator('.recent-chat input').fill('Personal activity renamed')
    await page.locator('.recent-chat input').press('Enter')
    await expect(personalRow.locator('.session-link')).toHaveText('Personal activity renamed')
    await expect(page.locator('.recent-chat').filter({ hasText: 'Project activity' })).toHaveCount(
      0
    )
    await expect(
      page.locator('.project-group .session-row').filter({ hasText: 'Personal activity' })
    ).toHaveCount(0)
    await projectRow.locator('.session-link').click()
    await page.getByRole('textbox', { name: 'Сообщение Prime' }).fill('HOLD_FIXTURE')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await page.evaluate(async (chats) => {
      await window.prime.call(chats.personalProject, 'send', {
        sessionId: chats.personalSession,
        runId: crypto.randomUUID(),
        prompt: 'HOLD_FIXTURE'
      })
    }, chats)
    await expect
      .poll(
        () =>
          fixture.provider.prompts.filter(
            (body) => body.tools?.length && JSON.stringify(body.messages).includes('HOLD_FIXTURE')
          ).length
      )
      .toBeGreaterThanOrEqual(2)
    await expect(page.locator('.session-activity')).toHaveCount(2)
    const sizes = await page
      .locator('.session-activity')
      .evaluateAll((elements) =>
        elements.map((element) => parseFloat(getComputedStyle(element).width))
      )
    expect(sizes.every((width) => width === 14)).toBe(true)
    await page.screenshot({ path: 'test-results/sidebar-activity.png' })
    await page.evaluate(
      async ({ projectId, chats }) => {
        await Promise.all([
          window.prime.call(projectId, 'cancel', { sessionId: chats.projectSession }),
          window.prime.call(chats.personalProject, 'cancel', { sessionId: chats.personalSession })
        ])
      },
      { projectId: project.id, chats }
    )
    await expect(projectRow.locator('.session-activity')).toHaveCount(0)
    await expect(personalRow.locator('.session-activity')).toHaveCount(0)
  } finally {
    if (chats)
      await page
        .evaluate(
          async ({ projectId, chats }) => {
            await Promise.allSettled([
              window.prime.call(projectId, 'cancel', { sessionId: chats.projectSession }),
              window.prime.call(chats.personalProject, 'cancel', {
                sessionId: chats.personalSession
              })
            ])
          },
          { projectId: project.id, chats }
        )
        .catch(() => {})
    await fixture.close()
  }
})
