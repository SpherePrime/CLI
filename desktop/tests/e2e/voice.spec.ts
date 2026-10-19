import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('local speech model synthesizes Russian audio inside Electron', async () => {
  test.setTimeout(180000)
  const fixture = await launchWorkbench()
  const { page } = fixture
  try {
    const result = await page.evaluate(() =>
      window.prime.speech({ action: 'speak', text: 'Проверка голоса Prime.', voice: 'F1', speed: 1 })
    )
    expect(result.mime).toBe('audio/wav')
    expect(atob(result.audio.slice(0, 8)).startsWith('RIFF')).toBe(true)
    expect(result.audio.length).toBeGreaterThan(50000)
    const voices = await page.evaluate(() => window.prime.speech({ action: 'voices' }))
    expect(voices.map((voice: { id: string }) => voice.id)).toEqual(expect.arrayContaining(['F1', 'M1']))
    await page.getByRole('button', { name: 'Настройки' }).first().click()
    await page.getByRole('button', { name: 'Голос', exact: true }).click()
    await expect(page.getByText('Модель готова')).toBeVisible()
    await page.screenshot({ path: 'test-results/voice-settings.png' })
  } finally {
    await fixture.close()
  }
})
