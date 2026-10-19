import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { AutomationManager } from '../../src/main/automations/manager'
import { LaunchManager } from '../../src/main/launch/manager'
import { SettingsStore } from '../../src/main/settings/store'
import { TerminalManager } from '../../src/main/terminal/manager'
import { startBrowserMcpServer } from '../../src/main/browser/mcp-server'
import type { BrowserPanel } from '../../src/main/browser/panel'
import { createProjectTools } from '../../src/main/workbench/projectTools'
import { readProjectFile } from '../../src/main/files/browser'
import { TeamManager } from '../../src/main/teams/manager'
import { createGithubReadTools } from '../../src/main/workbench/projectGithubTools'

const temporary: string[] = []
const managerCleanup: (() => Promise<void>)[] = []
afterEach(async () => {
  await Promise.all(managerCleanup.splice(0).map((dispose) => dispose()))
  await Promise.all(temporary.splice(0).map((path) => rm(path, { recursive: true, force: true, maxRetries: 10, retryDelay: 150 })))
})
async function fixture(terminals?: TerminalManager) {
  const root = await mkdtemp(join(tmpdir(), 'prime-project-tools-'))
  temporary.push(root)
  await mkdir(join(root, 'src'))
  await writeFile(join(root, 'src', 'main.ts'), 'real source')
  const adapter = {
    prepare: async () => ({ workspaceId: 'project', sessionId: 'chat' }),
    dispatch: async () => {},
    status: async () => ({ status: 'completed' as const }),
    cancel: async () => {}
  }
  const automations = new AutomationManager({
    path: join(root, 'automations.json'),
    adapter,
    emit() {}
  })
  const launches = new LaunchManager({
    path: join(root, 'launches.json'),
    terminals: {} as never,
    emit() {}
  })
  const settings = new SettingsStore(join(root, 'settings.json'))
  const teams = new TeamManager({ path: join(root, 'teams.json'), adapter, emit() {} })
  managerCleanup.push(() => teams.dispose(), () => automations.dispose())
  await automations.load()
  await launches.load()
  await teams.load()
  const tools = createProjectTools({
    target: (workspaceId) => ({ workspaceId, root, key: root }),
    automations: () => automations,
    launches: () => launches,
    teams: () => teams,
    terminals: () => terminals,
    settings: () => settings,
    files: () => ({ read: (path) => readProjectFile(root, path) })
  })
  const run = (name: string, args: Record<string, unknown> = {}) =>
    tools.find((tool) => tool.name === name)!.run(args, { workspaceId: 'project' })
  return { root, run, tools, automations, launches, settings }
}
describe('project agent tools', () => {
  it('limits GitHub access to validated read operations in the authenticated project', async () => {
    const calls: unknown[] = []
    const tools = createGithubReadTools(async (workspaceId, input) => {
      calls.push({ workspaceId, input })
      return { repository: 'local/project' }
    })
    const status = tools.find((tool) => tool.name === 'github_status')!
    await status.run({}, { workspaceId: 'bound' })
    expect(calls).toEqual([{ workspaceId: 'bound', input: { action: 'status' } }])
    await expect(status.run({ workspaceId: 'foreign' }, { workspaceId: 'bound' })).rejects.toThrow()
    expect(tools.some((tool) => /create|comment|push/.test(tool.name))).toBe(false)
    await expect(
      tools
        .find((tool) => tool.name === 'github_pull')!
        .run({ number: -1 }, { workspaceId: 'bound' })
    ).rejects.toThrow()
  })
  it('advertises tools before managers are ready', () => {
    const tools = createProjectTools({} as never)
    expect(tools.map((tool) => tool.name)).toEqual(
      expect.arrayContaining([
        'automation_save',
        'launch_start',
        'teams_create',
        'terminal_create',
        'context_update',
        'desktop_capabilities'
      ])
    )
    for (const tool of tools) expect(JSON.stringify(tool.inputSchema)).not.toContain('$ref')
  })
  it('creates enabled durable automation from the bound project', async () => {
    const { run, root, automations } = await fixture()
    await run('automation_save', {
      title: 'Check',
      prompt: 'Run checks',
      trigger: { type: 'interval', minutes: 30 }
    })
    const rules = (await automations.list(root)).rules
    expect(rules).toHaveLength(1)
    expect(rules[0]).toMatchObject({ enabled: true, title: 'Check' })
    await expect(
      run('automation_save', {
        root: 'foreign',
        title: 'Check',
        prompt: 'Run',
        trigger: { type: 'interval', minutes: 30 }
      })
    ).rejects.toThrow()
  })
  it('rejects foreign profile ids before calling unscoped launch stop', async () => {
    const { launches, run, root } = await fixture()
    const foreign = await launches.handle(
      { workspaceId: 'foreign', key: 'foreign', root },
      { action: 'save', input: { name: 'Foreign', command: 'echo safe' } }
    )
    await expect(run('launch_remove', { id: foreign.configs[0].id })).rejects.toThrow()
    await run('launch_save', { name: 'Local', command: 'echo safe' })
    expect(launches.list(root).configs[0].name).toBe('Local')
    expect(launches.list('foreign').configs).toHaveLength(1)
  })
  it('validates context sources before persisting and uses trusted project key', async () => {
    const { settings, root, run } = await fixture()
    await run('context_update', {
      goal: 'Finish',
      sources: [{ kind: 'file', path: 'src/main.ts' }]
    })
    expect(settings.get().agentContexts?.[root]).toMatchObject({
      enabled: true,
      goal: 'Finish',
      sources: [{ path: 'src/main.ts', kind: 'file' }]
    })
    await expect(
      run('context_update', { sources: [{ kind: 'file', path: '../secret' }] })
    ).rejects.toThrow()
    await expect(run('context_update', { workspaceId: 'foreign', goal: 'bad' })).rejects.toThrow()
    expect(settings.get().agentContexts?.[root]?.goal).toBe('Finish')
  })
  it('requires an authenticated workspace binding', async () => {
    const { tools } = await fixture()
    await expect(
      tools.find((tool) => tool.name === 'automation_list')!.run({}, {})
    ).rejects.toThrow('Workspace binding')
  })
  it('updates, pauses, resumes, runs, cancels and deletes project rules', async () => {
    const { run, automations, root } = await fixture()
    await run('automation_save', {
      title: 'Before',
      prompt: 'Check',
      trigger: { type: 'interval', minutes: 2 }
    })
    const id = (await automations.list(root)).rules[0].id
    await run('automation_update', { id, title: 'After' })
    await run('automation_pause', { id })
    expect((await automations.list(root)).rules[0]).toMatchObject({
      title: 'After',
      enabled: false,
      prompt: 'Check'
    })
    await run('automation_update', { id, enabled: true })
    await run('automation_run', { id })
    const active = (await automations.list(root)).runs[0]
    expect(active.status).toBe('running')
    await run('automation_cancel', { runId: active.id })
    expect((await automations.list(root)).runs[0].status).toBe('cancelled')
    await run('automation_delete', { id })
    expect((await automations.list(root)).rules).toHaveLength(0)
  })
  it('creates a real project team and rejects invalid task batches', async () => {
    const { run } = await fixture()
    await expect(run('teams_create', { title: 'Invalid', tasks: [] })).rejects.toThrow()
    const result = await run('teams_create', {
      title: 'Review',
      tasks: [{ title: 'Inspect', instruction: 'Read source and report' }]
    })
    const team = JSON.parse((result[0] as { text: string }).text).teams[0]
    expect(team).toMatchObject({ title: 'Review', concurrency: 2, tasks: [{ title: 'Inspect' }] })
    await run('teams_cancel', { teamId: team.id })
  })
  it('creates writable agent terminals without taking user terminal ownership', async () => {
    const terminals = new TerminalManager({ emit() {} })
    const { root, run } = await fixture(terminals)
    try {
      const user = terminals.create('project', root)
      await expect(run('terminal_close', { id: user.id })).rejects.toThrow('handover')
      const foreign = terminals.create('foreign', root)
      await expect(run('terminal_close', { id: foreign.id })).rejects.toThrow('workspace')
      const result = await run('terminal_create', { title: 'Agent task' })
      const agent = JSON.parse((result[0] as { text: string }).text)
      expect(agent.owner).toBe('agent')
      expect(terminals.write(agent.id, 'echo project-tool\r', 'agent', 'project').owner).toBe(
        'agent'
      )
      await run('terminal_release', { id: agent.id })
      expect(() => terminals.write(agent.id, 'echo blocked\r', 'agent', 'project')).toThrow(
        'ownership'
      )
      expect(terminals.read(user.id, 'project').terminal.owner).toBe('user')
    } finally {
      await terminals.dispose()
    }
  })
  it('advertises and executes project tools through authenticated MCP', async () => {
    const { tools } = await fixture()
    const server = await startBrowserMcpServer({} as BrowserPanel, {
      resolveWorkspace: (root) => (root === 'bound' ? 'project' : undefined),
      tools: () => tools
    })
    try {
      const call = async (
        method: string,
        params: Record<string, unknown>,
        token = server.token,
        bound = true
      ) => {
        const response = await fetch(server.url, {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${token}`,
            ...(bound ? { 'X-Prime-Workspace': Buffer.from('bound').toString('base64url') } : {})
          },
          body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params })
        })
        return { status: response.status, body: await response.text() }
      }
      const listed = JSON.parse((await call('tools/list', {})).body).result.tools
      expect(listed.map((tool: { name: string }) => tool.name)).toEqual(
        expect.arrayContaining([
          'automation_save',
          'teams_create',
          'context_update',
          'terminal_create'
        ])
      )
      expect((await call('tools/list', {}, 'wrong')).status).toBe(401)
      const unbound = JSON.parse((await call('tools/list', {}, server.token, false)).body).result
        .tools
      expect(unbound.some((tool: { name: string }) => tool.name === 'automation_save')).toBe(false)
      const saved = JSON.parse(
        (
          await call('tools/call', {
            name: 'automation_save',
            arguments: {
              title: 'MCP rule',
              prompt: 'Check',
              trigger: { type: 'daily', time: '09:30' }
            }
          })
        ).body
      )
      expect(JSON.parse(saved.result.content[0].text).rules[0]).toMatchObject({
        title: 'MCP rule',
        enabled: true
      })
      const spoofed = JSON.parse(
        (
          await call('tools/call', {
            name: 'context_update',
            arguments: { root: 'foreign', goal: 'Override' }
          })
        ).body
      )
      expect(spoofed.result.isError).toBe(true)
    } finally {
      await server.close()
    }
  })
})
