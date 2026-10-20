import { mkdtemp } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, it } from 'vitest'
import { SettingsStore } from '../../src/main/settings/store'

it('keeps independent chat reasoning choices after updates and restart', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'prime-chat-reasoning-'))
  const path = join(directory, 'preferences.json')
  const store = new SettingsStore(path)
  await store.update({ reasoningBySession: { first: 'high' } })
  await store.update({ reasoningBySession: { second: 'low' } })
  const restored = new SettingsStore(path)
  await restored.load()
  expect(restored.get().reasoningBySession).toEqual({ first: 'high', second: 'low' })
})
