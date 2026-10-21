import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('desktop runtime sends its desktop identity and feature context to the provider', async () => {
  const fixture = await launchWorkbench()
  try {
    await fixture.page
      .getByRole('textbox', { name: 'Сообщение Prime' })
      .fill('Кто ты и какие возможности есть в приложении?')
    await fixture.page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect
      .poll(() =>
        fixture.provider.prompts.some((body) =>
          body.messages?.some(
            (message: any) =>
              ['system', 'developer'].includes(message.role) &&
              JSON.stringify(message.content).includes(
                'You are Prime Desktop, a coding agent in the Prime Desktop application.'
              )
          )
        )
      )
      .toBe(true)
    const request = fixture.provider.prompts.find((body) => body.tools?.length)
    const context = JSON.stringify(request?.messages)
    expect(context).toContain('GitHub')
    expect(context).toContain('SSH')
  } finally {
    await fixture.close()
  }
})
