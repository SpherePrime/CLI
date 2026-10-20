import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('edits providers and restores favorite and recent models after restart', async () => {
  test.setTimeout(120000)
  const workbench = await launchWorkbench()
  let page = workbench.page
  const id = workbench.project.id
  try {
    await page.evaluate(
      async ({ id, url }) => {
        await window.prime.call(id, 'providerSave', {
          scope: 0,
          originalId: 'fixture',
          provider: {
            id: 'fixture',
            name: 'Fixture',
            base_url: url,
            type: 'openai-compat',
            discover_models: false,
            models: [
              { id: 'desktop-fixture', name: 'Desktop Fixture', supports_attachments: true },
              { id: 'secondary-fixture', name: 'Secondary Fixture' }
            ]
          }
        })
        await window.prime.preferences({
          favoriteModels: [{ provider: 'fixture', model: 'desktop-fixture' }]
        })
      },
      { id, url: workbench.provider.url }
    )
    await page.reload()
    await page.getByRole('button', { name: 'Настройки', exact: false }).first().click()
    await page.getByRole('button', { name: 'Провайдеры', exact: true }).click()
    await page.getByRole('button', { name: 'Изменить', exact: true }).click()
    await expect(page.getByLabel('API ключ', { exact: true })).toHaveValue('')
    await page.getByLabel('ID провайдера', { exact: true }).fill('renamed-fixture')
    await page.getByRole('button', { name: 'Сохранить изменения', exact: true }).click()
    await expect(page.getByRole('status')).toContainText('Настройки провайдера сохранены')
    const saved = await page.evaluate((id) => window.prime.call(id, 'config'), id)
    expect(saved.providers.fixture).toBeUndefined()
    expect(saved.providers['renamed-fixture'].models).toHaveLength(2)
    expect(saved.models.large.provider).toBe('renamed-fixture')
    await page.getByRole('dialog').getByRole('button', { name: 'Закрыть', exact: true }).click()
    await page.evaluate(() => window.dispatchEvent(new Event('prime:models')))
    await expect(page.getByRole('group', { name: 'Избранное', exact: true })).toBeVisible()
    await page
      .getByRole('button', { name: 'Добавить в избранное: Secondary Fixture', exact: true })
      .click()
    await expect(
      page.getByRole('button', { name: 'Удалить из избранного: Secondary Fixture', exact: true })
    ).toBeVisible()
    await page.getByRole('option', { name: 'Secondary Fixture', exact: false }).click()
    await expect(page.getByRole('listbox', { name: 'Модели', exact: true })).toHaveCount(0)
    ;({ page } = await workbench.restart())
    await page.evaluate(() => window.dispatchEvent(new Event('prime:models')))
    const groups = page.locator('.model-picker-group')
    await expect(groups.first()).toHaveAttribute('aria-label', 'Недавние')
    await expect(groups.first().getByRole('option')).toContainText('Secondary Fixture')
    await expect(
      page.getByRole('group', { name: 'Избранное', exact: true }).getByRole('option')
    ).toContainText('Desktop Fixture')
    const preferences = await page.evaluate(() => window.prime.preferences())
    expect(preferences.recentModels?.[0]).toEqual({
      provider: 'renamed-fixture',
      model: 'secondary-fixture'
    })
    expect(preferences.favoriteModels).toHaveLength(2)
    expect(workbench.errors).toEqual([])
  } finally {
    await workbench.close()
  }
})
