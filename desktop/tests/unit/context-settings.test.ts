import { expect, it } from 'vitest'
import { mkdtemp } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { SettingsStore } from '../../src/main/settings/store'
import { emptyAgentContext } from '../../src/shared/agentContext'

it('preserves working contexts of other projects during concurrent preference saves and reload', async () => {
  const path = join(await mkdtemp(join(tmpdir(), 'prime-context-settings-')), 'preferences.json')
  const store = new SettingsStore(path)
  await Promise.all([
    store.update({ agentContexts: { first: { ...emptyAgentContext(), goal: 'First project' } } }),
    store.update({ agentContexts: { second: { ...emptyAgentContext(), goal: 'Second project' } } }),
    store.update({ density: 'compact' })
  ])
  const restored = new SettingsStore(path)
  await restored.load()
  expect(restored.get().agentContexts?.first.goal).toBe('First project')
  expect(restored.get().agentContexts?.second.goal).toBe('Second project')
  expect(restored.get().density).toBe('compact')
})
