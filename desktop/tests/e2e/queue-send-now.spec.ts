import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

const textbox = (page: import('@playwright/test').Page) =>
  page.getByRole('textbox', { name: 'Сообщение Prime', exact: true })
const sendButton = (page: import('@playwright/test').Page) =>
  page.getByRole('button', { name: 'Отправить сообщение', exact: true })

async function queueFollowUp(page: import('@playwright/test').Page, prompt: string) {
  await textbox(page).fill(prompt)
  await sendButton(page).click()
  const queue = page.locator('.prompt-queue')
  await expect(queue).toBeVisible({ timeout: 15000 })
  const row = queue.locator('li').filter({ hasText: prompt })
  await expect(row).toBeVisible()
  await row.locator('.queue-send-now').click()
  await expect(row.locator('.queue-send-now')).toBeDisabled()
  await expect(row.locator('.queue-send-now').locator('.spin')).toBeVisible()
  return row
}

test('send now delivers the queued prompt after the current model step and continues the task', async () => {
  test.setTimeout(180000)
  const fixture = await launchWorkbench()
  const page = fixture.page
  try {
    await textbox(page).fill('SLOW_FIXTURE сделай долгую задачу')
    await sendButton(page).click()
    await expect
      .poll(
        () =>
          fixture.provider.prompts.some((body) => {
            const users = (body.messages || []).filter((message: any) => message.role === 'user')
            const last = users.at(-1)
            const text = typeof last?.content === 'string' ? last.content : ''
            return text.startsWith('SLOW_FIXTURE')
          }),
        { timeout: 30000 }
      )
      .toBe(true)

    const row = await queueFollowUp(page, 'send-now-target продолжай после этого')

    await page.getByRole('heading', { name: 'Разрешить действие?' }).waitFor({ timeout: 30000 })
    await page.getByRole('button', { name: 'Разрешить', exact: true }).click()

    await expect
      .poll(
        () =>
          fixture.provider.prompts
            .slice(1)
            .some((body) =>
              JSON.stringify(body.messages).includes('send-now-target продолжай после этого')
            ),
        { timeout: 60000 }
      )
      .toBe(true)

    await expect(page.locator('.prompt-queue')).toHaveCount(0, { timeout: 15000 })
    await expect(page.locator('.message-rows')).toContainText('send-now-target продолжай после этого', {
      timeout: 30000
    })
    await expect(page.locator('.message-rows')).toContainText('Соединение работает', {
      timeout: 60000
    })
    expect(fixture.errors).toEqual([])
  } finally {
    await fixture.close()
  }
})

test('send now delivers the queued prompt when the task waits for an answer', async () => {
  test.setTimeout(180000)
  const fixture = await launchWorkbench()
  const page = fixture.page
  try {
    await textbox(page).fill('QUESTION_FIXTURE задай вопрос')
    await sendButton(page).click()
    await expect(page.locator('.approval-card')).toBeVisible({ timeout: 30000 })

    await queueFollowUp(page, 'send-now-target примени это сейчас')

    await page.locator('.approval-card input[type=radio]').first().check()
    await page.locator('.approval-card').getByRole('button', { name: 'Ответить' }).click()

    await expect
      .poll(
        () =>
          fixture.provider.prompts.some((body) =>
            JSON.stringify(body.messages).includes('send-now-target примени это сейчас')
          ),
        { timeout: 60000 }
      )
      .toBe(true)
    await expect(page.locator('.message-rows')).toContainText('send-now-target примени это сейчас', {
      timeout: 30000
    })
    expect(fixture.errors).toEqual([])
  } finally {
    await fixture.close()
  }
})
