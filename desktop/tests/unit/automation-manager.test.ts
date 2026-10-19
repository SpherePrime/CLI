import { afterEach, expect, it } from 'vitest'
import { mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { AutomationManager, type AgentAdapter } from '../../src/main/automations/manager'

const directories: string[] = []
afterEach(async () => {
  await Promise.all(
    directories.splice(0).map((directory) => rm(directory, { recursive: true, force: true }))
  )
})
async function setup() {
  const root = await mkdtemp(join(tmpdir(), 'prime-automation-'))
  directories.push(root)
  let now = 1000000
  const dispatched: string[] = []
  const adapter: AgentAdapter = {
    prepare: async ({ runId }) => ({ workspaceId: 'workspace', sessionId: runId }),
    dispatch: async ({ runId }) => {
      dispatched.push(runId)
    },
    status: async () => ({ status: 'running' }),
    cancel: async () => {}
  }
  const path = join(root, 'automations.json')
  const manager = new AutomationManager({ path, adapter, emit: () => {}, now: () => now })
  await manager.load()
  const input = {
    title: 'Review',
    prompt: 'Review project',
    enabled: true,
    trigger: { type: 'interval' as const, minutes: 1 }
  }
  return {
    root,
    path,
    manager,
    adapter,
    dispatched,
    input,
    advance: (milliseconds: number) => {
      now += milliseconds
    }
  }
}

it('claims one durable run before concurrent ticks and manual runs dispatch', async () => {
  const fixture = await setup()
  const saved = await fixture.manager.save(fixture.root, fixture.input)
  fixture.adapter.dispatch = async ({ runId }) => {
    const disk = JSON.parse(await readFile(fixture.path, 'utf8'))
    expect(disk.runs.find((run: { id: string }) => run.id === runId).sessionId).toBe(runId)
    fixture.dispatched.push(runId)
  }
  fixture.advance(60000)
  await Promise.allSettled([
    fixture.manager.tick(),
    fixture.manager.tick(),
    fixture.manager.run(fixture.root, saved.rules[0].id)
  ])
  expect(fixture.dispatched).toHaveLength(1)
  expect((await fixture.manager.list(fixture.root)).runs[0].status).toBe('running')
})

it('does not replay overdue schedules on restart or pause and resume', async () => {
  const fixture = await setup()
  const saved = await fixture.manager.save(fixture.root, fixture.input)
  await fixture.manager.save(fixture.root, {
    ...fixture.input,
    id: saved.rules[0].id,
    enabled: false
  })
  fixture.advance(3600000)
  await fixture.manager.save(fixture.root, { ...fixture.input, id: saved.rules[0].id })
  await fixture.manager.tick()
  expect(fixture.dispatched).toHaveLength(0)
  const restarted = new AutomationManager({
    path: fixture.path,
    adapter: fixture.adapter,
    emit: () => {},
    now: () => 999999999
  })
  await restarted.load()
  await restarted.tick()
  expect(fixture.dispatched).toHaveLength(0)
})

it('recovers known sessions and never resubmits claimed runs', async () => {
  const fixture = await setup()
  const saved = await fixture.manager.save(fixture.root, fixture.input)
  await fixture.manager.run(fixture.root, saved.rules[0].id)
  fixture.adapter.status = async () => ({ status: 'completed' })
  const restarted = new AutomationManager({
    path: fixture.path,
    adapter: fixture.adapter,
    emit: () => {},
    now: () => 1000000
  })
  await restarted.load()
  expect((await restarted.list(fixture.root)).runs[0].status).toBe('completed')
  const state = JSON.parse(await readFile(fixture.path, 'utf8'))
  state.runs[0].status = 'claimed'
  delete state.runs[0].sessionId
  await writeFile(fixture.path, JSON.stringify(state))
  const interrupted = new AutomationManager({
    path: fixture.path,
    adapter: fixture.adapter,
    emit: () => {},
    now: () => 1000000
  })
  await interrupted.load()
  expect((await interrupted.list(fixture.root)).runs[0].status).toBe('interrupted')
  expect(fixture.dispatched).toHaveLength(1)
})

it('shows corrupt state and refuses to overwrite it', async () => {
  const fixture = await setup()
  await writeFile(fixture.path, '{broken')
  const manager = new AutomationManager({
    path: fixture.path,
    adapter: fixture.adapter,
    emit: () => {}
  })
  await manager.load()
  expect((await manager.list(fixture.root)).error).toContain('automation')
  await expect(manager.save(fixture.root, fixture.input)).rejects.toThrow()
  expect(await readFile(fixture.path, 'utf8')).toBe('{broken')
})

it('filters own failures, deduplicates events and applies cooldown', async () => {
  const fixture = await setup()
  const saved = await fixture.manager.save(fixture.root, {
    ...fixture.input,
    trigger: { type: 'runFailed', cooldownMinutes: 1 }
  })
  await fixture.manager.run(fixture.root, saved.rules[0].id)
  fixture.adapter.status = async () => ({ status: 'failed' })
  await fixture.manager.tick()
  await fixture.manager.event({
    root: fixture.root,
    type: 'runFailed',
    sourceRunId: fixture.dispatched[0]
  })
  expect(fixture.dispatched).toHaveLength(1)
  fixture.advance(60000)
  await Promise.all([
    fixture.manager.event({
      root: fixture.root,
      type: 'runFailed',
      sourceRunId: 'external',
      fingerprint: 'one'
    }),
    fixture.manager.event({
      root: fixture.root,
      type: 'runFailed',
      sourceRunId: 'external',
      fingerprint: 'one'
    })
  ])
  expect(fixture.dispatched).toHaveLength(2)
})

it('keeps paused active work tracked and requires cancellation before deletion', async () => {
  const fixture = await setup()
  const saved = await fixture.manager.save(fixture.root, fixture.input)
  await fixture.manager.run(fixture.root, saved.rules[0].id)
  const paused = await fixture.manager.save(fixture.root, {
    ...fixture.input,
    id: saved.rules[0].id,
    enabled: false
  })
  expect(paused.runs[0].status).toBe('running')
  await expect(fixture.manager.remove(fixture.root, saved.rules[0].id)).rejects.toThrow(
    'automation.active'
  )
  await fixture.manager.cancel(fixture.root, paused.runs[0].id)
  expect((await fixture.manager.remove(fixture.root, saved.rules[0].id)).rules).toHaveLength(0)
})

it('tracks ambiguous dispatch acceptance until reconciliation instead of resubmitting', async () => {
  const fixture = await setup()
  const saved = await fixture.manager.save(fixture.root, fixture.input)
  fixture.adapter.dispatch = async ({ runId }) => {
    fixture.dispatched.push(runId)
    throw new Error('transport disconnected after acceptance')
  }
  await fixture.manager.run(fixture.root, saved.rules[0].id)
  await fixture.manager.run(fixture.root, saved.rules[0].id)
  expect(fixture.dispatched).toHaveLength(1)
  expect((await fixture.manager.list(fixture.root)).runs[0].status).toBe('running')
  fixture.adapter.status = async () => ({ status: 'completed' })
  await fixture.manager.tick()
  expect((await fixture.manager.list(fixture.root)).runs[0].status).toBe('completed')
  expect((await fixture.manager.list(fixture.root)).runs[0].error).toBeUndefined()
})

it('keeps recovered sessions tracked during transient backend errors', async () => {
  const fixture = await setup()
  const saved = await fixture.manager.save(fixture.root, fixture.input)
  await fixture.manager.run(fixture.root, saved.rules[0].id)
  fixture.adapter.status = async () => {
    throw new Error('offline')
  }
  const restarted = new AutomationManager({
    path: fixture.path,
    adapter: fixture.adapter,
    emit: () => {}
  })
  await restarted.load()
  expect((await restarted.list(fixture.root)).runs[0].status).toBe('running')
})

it('ignores files changed by automation through completion, then observes new user edits', async () => {
  const fixture = await setup()
  let fingerprint = 'initial'
  fixture.adapter.filesFingerprint = async () => fingerprint
  await fixture.manager.save(fixture.root, {
    ...fixture.input,
    trigger: { type: 'filesChanged', cooldownMinutes: 1 }
  })
  await fixture.manager.tick()
  fixture.advance(60000)
  fingerprint = 'user edit'
  await fixture.manager.tick()
  expect(fixture.dispatched).toHaveLength(1)
  fixture.advance(60000)
  fingerprint = 'automation edit'
  await fixture.manager.tick()
  fixture.adapter.status = async () => ({ status: 'completed' })
  fingerprint = 'final automation edit'
  await fixture.manager.tick()
  expect(fixture.dispatched).toHaveLength(1)
  fingerprint = 'next user edit'
  await fixture.manager.tick()
  expect(fixture.dispatched).toHaveLength(2)
})

it('continues due schedules when file monitoring fails and exposes that error', async () => {
  const fixture = await setup()
  fixture.adapter.filesFingerprint = async () => {
    throw new Error('watch unavailable')
  }
  await fixture.manager.save(fixture.root, {
    ...fixture.input,
    trigger: { type: 'filesChanged', cooldownMinutes: 1 }
  })
  await fixture.manager.save(fixture.root, fixture.input)
  fixture.advance(60000)
  await fixture.manager.tick()
  expect(fixture.dispatched).toHaveLength(1)
  expect((await fixture.manager.list(fixture.root)).error).toContain('watch unavailable')
  const broken = (await fixture.manager.list(fixture.root)).rules.find(
    (rule) => rule.trigger.type === 'filesChanged'
  )!
  const fixed = await fixture.manager.save(fixture.root, {
    ...broken,
    trigger: { type: 'interval', minutes: 30 }
  })
  expect(fixed.error).toBeUndefined()
})

it('keeps memory unchanged if a durable write fails', async () => {
  const fixture = await setup()
  await mkdir(fixture.path)
  await expect(fixture.manager.save(fixture.root, fixture.input)).rejects.toThrow()
  expect((await fixture.manager.list(fixture.root)).rules).toHaveLength(0)
})

it('does not dispatch after stop while session preparation is pending', async () => {
  const fixture = await setup()
  const saved = await fixture.manager.save(fixture.root, fixture.input)
  let release!: () => void
  let prepared!: () => void
  const ready = new Promise<void>((resolve) => {
    prepared = resolve
  })
  fixture.adapter.prepare = async ({ runId }) => {
    prepared()
    await new Promise<void>((resolve) => {
      release = resolve
    })
    return { workspaceId: 'workspace', sessionId: runId }
  }
  const pending = fixture.manager.run(fixture.root, saved.rules[0].id)
  await ready
  fixture.manager.stop()
  release()
  await pending
  await fixture.manager.dispose()
  expect(fixture.dispatched).toHaveLength(0)
  expect((await fixture.manager.list(fixture.root)).runs[0].status).toBe('interrupted')
})

it('rejects cross-project rule access and failed cancellations leave work active', async () => {
  const fixture = await setup()
  const otherRoot = await mkdtemp(join(tmpdir(), 'prime-other-project-'))
  directories.push(otherRoot)
  const saved = await fixture.manager.save(fixture.root, fixture.input)
  await expect(fixture.manager.run(otherRoot, saved.rules[0].id)).rejects.toThrow(
    'automation.notFound'
  )
  await fixture.manager.run(fixture.root, saved.rules[0].id)
  const run = (await fixture.manager.list(fixture.root)).runs[0]
  await expect(fixture.manager.cancel(otherRoot, run.id)).rejects.toThrow('automation.notFound')
  fixture.adapter.cancel = async () => {
    throw new Error('still stopping')
  }
  await expect(fixture.manager.cancel(fixture.root, run.id)).rejects.toThrow('still stopping')
  expect((await fixture.manager.list(fixture.root)).runs[0].status).toBe('running')
})

it('bounds completed history while preserving active runs and old ownership', async () => {
  const fixture = await setup()
  const saved = await fixture.manager.save(fixture.root, fixture.input)
  await fixture.manager.run(fixture.root, saved.rules[0].id)
  const state = JSON.parse(await readFile(fixture.path, 'utf8'))
  const active = state.runs[0]
  for (let index = 0; index < 210; index++) {
    const id = `previous-${index}`
    state.runs.push({ ...active, id, status: 'completed', finishedAt: 999999 })
    state.ownedRunIds.push(id)
  }
  await writeFile(fixture.path, JSON.stringify(state))
  const restarted = new AutomationManager({
    path: fixture.path,
    adapter: fixture.adapter,
    emit: () => {}
  })
  await restarted.load()
  const snapshot = await restarted.list(fixture.root)
  expect(snapshot.runs).toHaveLength(201)
  expect(snapshot.runs.find((run) => run.id === active.id)?.status).toBe('running')
  const disk = JSON.parse(await readFile(fixture.path, 'utf8'))
  expect(disk.ownedRunIds).toContain('previous-0')
})

it('preserves structurally invalid JSON instead of normalizing corrupt runs', async () => {
  const fixture = await setup()
  const saved = await fixture.manager.save(fixture.root, fixture.input)
  await fixture.manager.run(fixture.root, saved.rules[0].id)
  const state = JSON.parse(await readFile(fixture.path, 'utf8'))
  state.runs[0].sessionId = 123
  const original = JSON.stringify(state)
  await writeFile(fixture.path, original)
  const restarted = new AutomationManager({
    path: fixture.path,
    adapter: fixture.adapter,
    emit: () => {}
  })
  await restarted.load()
  expect((await restarted.list(fixture.root)).error).toBe('automation.corrupt')
  expect(await readFile(fixture.path, 'utf8')).toBe(original)
})

it('preserves a future interval occurrence across restart and advances only overdue schedules', async () => {
  const fixture = await setup()
  const saved = await fixture.manager.save(fixture.root, {
    ...fixture.input,
    trigger: { type: 'interval', minutes: 120 }
  })
  const dueAt = saved.rules[0].nextAt!
  const restarted = new AutomationManager({
    path: fixture.path,
    adapter: fixture.adapter,
    emit: () => {},
    now: () => dueAt - 3600000
  })
  await restarted.load()
  expect((await restarted.list(fixture.root)).rules[0].nextAt).toBe(dueAt)
  await restarted.tick()
  expect(fixture.dispatched).toHaveLength(0)
  const overdueNow = dueAt + 3600000
  const overdue = new AutomationManager({
    path: fixture.path,
    adapter: fixture.adapter,
    emit: () => {},
    now: () => overdueNow
  })
  await overdue.load()
  expect((await overdue.list(fixture.root)).rules[0].nextAt).toBe(overdueNow + 120 * 60000)
  await overdue.tick()
  expect(fixture.dispatched).toHaveLength(0)
})

it('dispatches all eligible file rules for the same event while suppressing own later edits', async () => {
  const fixture = await setup()
  const trigger = { type: 'filesChanged' as const, cooldownMinutes: 1 }
  await fixture.manager.save(fixture.root, { ...fixture.input, title: 'Review', trigger })
  await fixture.manager.save(fixture.root, { ...fixture.input, title: 'Test', trigger })
  fixture.advance(60000)
  await fixture.manager.event({
    root: fixture.root,
    type: 'filesChanged',
    fingerprint: 'user edit'
  })
  expect(fixture.dispatched).toHaveLength(2)
  const snapshot = await fixture.manager.list(fixture.root)
  expect(new Set(snapshot.runs.map((run) => run.ruleId)).size).toBe(2)
  fixture.advance(60000)
  await fixture.manager.event({ root: fixture.root, type: 'filesChanged', fingerprint: 'own edit' })
  expect(fixture.dispatched).toHaveLength(2)
})

it('allows managing known rules after their project directory disappears', async () => {
  const fixture = await setup()
  const projectRoot = join(fixture.root, 'deleted-project')
  await mkdir(projectRoot)
  const saved = await fixture.manager.save(projectRoot, fixture.input)
  await rm(projectRoot, { recursive: true })
  const missing = await fixture.manager.list(projectRoot)
  expect(missing.rules[0].enabled).toBe(true)
  await fixture.manager.save(projectRoot, {
    ...fixture.input,
    id: saved.rules[0].id,
    enabled: false
  })
  fixture.advance(60000)
  await fixture.manager.tick()
  expect(fixture.dispatched).toHaveLength(0)
  expect(fixture.manager.counts().enabledCount).toBe(0)
  expect((await fixture.manager.remove(projectRoot, saved.rules[0].id)).rules).toHaveLength(0)
  await expect(
    fixture.manager.save(join(fixture.root, 'unknown-project'), fixture.input)
  ).rejects.toThrow()
})
