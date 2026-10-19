import { afterEach, expect, it } from 'vitest'
import { mkdtemp, mkdir, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { ProjectChanges } from '../../src/main/workbench/projectChanges'

const monitors: ProjectChanges[] = []
afterEach(() => monitors.splice(0).forEach((monitor) => monitor.dispose()))

it('observes source edits while ignoring runtime state and project exclusions', async () => {
  const root = await mkdtemp(join(tmpdir(), 'prime-project-events-'))
  await mkdir(join(root, '.prime'))
  await writeFile(join(root, '.gitignore'), 'ignored.txt\n')
  const monitor = new ProjectChanges()
  monitors.push(monitor)
  const initial = await monitor.fingerprint(root)
  await writeFile(join(root, '.prime', 'state.json'), '{}')
  await writeFile(join(root, 'ignored.txt'), 'ignored')
  await new Promise((resolve) => setTimeout(resolve, 100))
  expect(await monitor.fingerprint(root)).toBe(initial)
  await writeFile(join(root, 'source.ts'), 'export const value = 1')
  await expect.poll(() => monitor.fingerprint(root)).not.toBe(initial)
  const changed = await monitor.fingerprint(root)
  await writeFile(join(root, 'source.ts'), 'export const value = 2')
  await expect.poll(() => monitor.fingerprint(root)).not.toBe(changed)
})

it('reloads ignore rules and releases watchers on disposal', async () => {
  const root = await mkdtemp(join(tmpdir(), 'prime-project-exclusions-'))
  const monitor = new ProjectChanges()
  monitors.push(monitor)
  await monitor.fingerprint(root)
  await writeFile(join(root, '.primeignore'), 'generated/\n')
  await mkdir(join(root, 'generated'))
  await new Promise((resolve) => setTimeout(resolve, 150))
  const baseline = await monitor.fingerprint(root)
  await writeFile(join(root, 'generated', 'file.txt'), 'ignored')
  await new Promise((resolve) => setTimeout(resolve, 100))
  expect(await monitor.fingerprint(root)).toBe(baseline)
  monitor.dispose()
  await expect(monitor.fingerprint(root)).rejects.toThrow('closed')
})
