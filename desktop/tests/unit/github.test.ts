import { expect, it } from 'vitest'
import { resolve } from 'node:path'
import { githubOperation } from '../../src/main/github/service'
import { localRunner } from '../../src/main/github/runner'

it('reads the real repository through gh', async () => {
  const runner = localRunner(resolve('..'))
  const status: any = await githubOperation(runner, { action: 'status' })
  if (!status.authenticated) return
  expect(status.repository).toMatch(/\//)
  expect(Array.isArray(await githubOperation(runner, { action: 'pulls' }))).toBe(true)
  expect(Array.isArray(await githubOperation(runner, { action: 'issues' }))).toBe(true)
  expect(await githubOperation(runner, { action: 'branches' })).toContain('main')
  await expect(githubOperation(runner, { action: 'createPull', head: '--force', base: 'main', title: 'x', body: '', draft: true })).rejects.toThrow()
}, 60000)
