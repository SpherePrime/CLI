import { expect, it } from 'vitest'
import { githubOperation } from '../../src/main/github/service'
import type { CommandRunner } from '../../src/main/github/runner'
import { CommandError } from '../../src/main/github/runner'

it('requests the selected GitHub state and list limit without shell interpolation', async () => {
  const calls: string[][] = []
  const runner: CommandRunner = async (command, args) => {
    calls.push([command, ...args])
    return '[]'
  }
  await githubOperation(runner, { action: 'pulls', state: 'all', limit: 100 })
  await githubOperation(runner, { action: 'issues', state: 'closed', limit: 50 })
  expect(calls[0]).toEqual([
    'gh',
    'pr',
    'list',
    '--state',
    'all',
    '--limit',
    '100',
    '--json',
    expect.stringContaining('state')
  ])
  expect(calls[1]).toEqual([
    'gh',
    'issue',
    'list',
    '--state',
    'closed',
    '--limit',
    '50',
    '--json',
    expect.stringContaining('state')
  ])
})

it('rejects invalid states and unbounded list requests before running commands', async () => {
  let called = false
  const runner: CommandRunner = async () => {
    called = true
    return '[]'
  }
  await expect(githubOperation(runner, { action: 'pulls', state: '--all' })).rejects.toThrow()
  await expect(githubOperation(runner, { action: 'issues', limit: 1001 })).rejects.toThrow()
  expect(called).toBe(false)
})

it('retains failed and pending checks returned with nonzero gh exit codes', async () => {
  const checks = [
    { name: 'Build', bucket: 'fail', link: 'https://github.com/owner/repo/actions/runs/17' }
  ]
  const runner: CommandRunner = async (_command, args) => {
    if (args[0] === 'pr' && args[1] === 'checks')
      throw Object.assign(new CommandError('Checks failed', 1), { stdout: JSON.stringify(checks) })
    if (args[0] === 'repo') return '{"nameWithOwner":"owner/repo"}'
    if (args[0] === 'run') return 'build failure output'
    if (args[0] === 'api') return '[]'
    return '{"number":7}'
  }
  const detail: any = await githubOperation(runner, { action: 'pull', number: 7 })
  expect(detail.checks).toEqual(checks)
  expect(await githubOperation(runner, { action: 'failedLogs', number: 7 })).toEqual([
    { name: 'Build', log: 'build failure output' }
  ])
})

it('reports a check query failure without presenting it as no checks', async () => {
  const runner: CommandRunner = async (_command, args) => {
    if (args[0] === 'pr' && args[1] === 'checks') throw new CommandError('API rate limit', 1)
    if (args[0] === 'repo') return '{"nameWithOwner":"owner/repo"}'
    if (args[0] === 'api') return '[]'
    return '{"number":7}'
  }
  const detail: any = await githubOperation(runner, { action: 'pull', number: 7 })
  expect(detail.checksError).toContain('API rate limit')
  await expect(githubOperation(runner, { action: 'failedLogs', number: 7 })).rejects.toThrow(
    'API rate limit'
  )
})
