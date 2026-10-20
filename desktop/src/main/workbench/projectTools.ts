import { randomUUID } from 'node:crypto'
import { z } from 'zod'
import { emptyAgentContext } from '../../shared/agentContext'
import type { AutomationInput } from '../../shared/automations'
import type { WorkbenchTool } from './tools'
import type { ProjectToolOptions, ProjectToolTarget } from './projectToolOptions'
import {
  automationSave,
  automationUpdate,
  contextUpdate,
  idArguments,
  identifier,
  launchSave,
  noArguments,
  teamCreate,
  terminalCreate
} from './projectToolSchemas'

export type { ProjectToolOptions } from './projectToolOptions'

function ready<T>(manager: T | undefined): T {
  if (!manager) throw new Error('Desktop service is still starting. Retry shortly.')
  return manager
}

export function createProjectTools(options: ProjectToolOptions): WorkbenchTool[] {
  const tools: WorkbenchTool[] = []
  function add<T extends z.ZodType>(
    name: string,
    description: string,
    schema: T,
    execute: (args: z.output<T>, target: ProjectToolTarget) => unknown | Promise<unknown>
  ) {
    tools.push({
      name,
      description,
      inputSchema: z.toJSONSchema(schema, { io: 'input' }),
      run: async (args, context) => {
        if (!context.workspaceId) throw new Error('Workspace binding is required')
        const parsed = schema.parse(args)
        const target = options.target(context.workspaceId)
        if (target.workspaceId !== context.workspaceId || !target.key || !target.root)
          throw new Error('Project access denied')
        return [{ type: 'text', text: JSON.stringify(await execute(parsed, target)) }]
      }
    })
  }
  const automationRoot = (target: ProjectToolTarget) => target.key
  add(
    'automation_list',
    'List this project’s saved schedules, event triggers and run history.',
    noArguments,
    (_, target) => ready(options.automations()).list(automationRoot(target))
  )
  add(
    'automation_save',
    'Create an enabled persistent automation or replace an existing rule. Runs in independent chats; daily/weekly times use the computer’s local timezone, days are 0 Sunday–6 Saturday. Desktop must remain running.',
    automationSave,
    async (input, target) => {
      const manager = ready(options.automations())
      if (
        input.id &&
        !(await manager.list(automationRoot(target))).rules.some((rule) => rule.id === input.id)
      )
        throw new Error('Automation not found in this project')
      const result = await manager.save(automationRoot(target), input)
      return result
    }
  )
  add(
    'automation_update',
    'Update selected fields of an existing project automation, including resuming with enabled=true.',
    automationUpdate,
    async (input, target) => {
      const manager = ready(options.automations())
      const previous = (await manager.list(automationRoot(target))).rules.find(
        (rule) => rule.id === input.id
      )
      if (!previous) throw new Error('Automation not found in this project')
      return manager.save(automationRoot(target), { ...previous, ...input } as AutomationInput)
    }
  )
  add(
    'automation_pause',
    'Pause a project automation without cancelling an active run.',
    idArguments,
    async ({ id }, target) => {
      const manager = ready(options.automations())
      const previous = (await manager.list(automationRoot(target))).rules.find(
        (rule) => rule.id === id
      )
      if (!previous) throw new Error('Automation not found in this project')
      return manager.save(automationRoot(target), { ...previous, enabled: false })
    }
  )
  for (const [name, action] of [
    ['delete', 'remove'],
    ['run', 'run']
  ] as const)
    add(
      `automation_${name}`,
      `${name === 'delete' ? 'Delete an inactive rule' : 'Run a saved automation now'} in this project.`,
      idArguments,
      ({ id }, target) => ready(options.automations())[action](automationRoot(target), id)
    )
  add(
    'automation_cancel',
    'Cancel an active automation run in this project.',
    z.strictObject({ runId: identifier }),
    ({ runId }, target) => ready(options.automations()).cancel(automationRoot(target), runId)
  )
  add(
    'automation_open',
    'Open an automation run’s chat in the desktop.',
    z.strictObject({ runId: identifier }),
    async ({ runId }, target) => {
      const run = (await ready(options.automations()).list(automationRoot(target))).runs.find(
        (item) => item.id === runId
      )
      if (!run?.workspaceId || !run.sessionId) throw new Error('Run chat is not available')
      if (!options.openSession) throw new Error('Opening chats is not available')
      await options.openSession(run.workspaceId, run.sessionId)
      return { opened: true }
    }
  )
  add(
    'launch_list',
    'List saved app launch commands and their process, URL and exit status in this project.',
    noArguments,
    (_, target) => ready(options.launches()).list(target.key)
  )
  add(
    'launch_save',
    'Save a project app launch profile. Command executes in the project shell only when launch_start is called. Local Windows shell is PowerShell: open desktop programs with Start-Process "C:\\Path\\App.exe" (add -ArgumentList for arguments), never cmd syntax like start "" "...".',
    launchSave,
    async (input, target) => {
      const manager = ready(options.launches())
      if (input.id && !manager.list(target.key).configs.some((config) => config.id === input.id))
        throw new Error('Launch profile not found in this project')
      return manager.handle(target, { action: 'save', input })
    }
  )
  for (const action of ['start', 'stop', 'restart', 'remove'] as const)
    add(
      `launch_${action}`,
      `${action} a saved app launch profile in this project. Start/restart executes its saved command.`,
      idArguments,
      async ({ id }, target) => {
        const manager = ready(options.launches())
        if (!manager.list(target.key).configs.some((config) => config.id === id))
          throw new Error('Launch profile not found in this project')
        return manager.handle(target, { action, id })
      }
    )
  add(
    'launch_logs',
    'Read bounded output and diagnostics of a launched project app.',
    idArguments,
    ({ id }, target) => {
      const snapshot = ready(options.launches()).list(target.key)
      if (!snapshot.configs.some((config) => config.id === id))
        throw new Error('Launch profile not found in this project')
      const process = snapshot.processes.find((item) => item.configId === id)
      return {
        process: process ?? { status: 'stopped' },
        ...(process
          ? { terminal: ready(options.terminals()).read(process.terminalId, target.workspaceId) }
          : {})
      }
    }
  )
  add(
    'teams_list',
    'List project agent teams and each member’s task, chat and status.',
    noArguments,
    (_, target) => ready(options.teams()).handle(target.key, { action: 'list' })
  )
  add(
    'teams_create',
    'Create and start a parallel team for an explicitly requested delegation. Give each member an independent task and avoid overlapping file ownership.',
    teamCreate,
    async (input, target) => {
      const result = await ready(options.teams()).handle(target.key, { action: 'create', input })
      return result
    }
  )
  add(
    'teams_cancel',
    'Cancel a project team or one of its tasks.',
    z.strictObject({ teamId: identifier, taskId: identifier.optional() }),
    ({ teamId, taskId }, target) =>
      ready(options.teams()).handle(
        target.key,
        taskId ? { action: 'stopTask', teamId, taskId } : { action: 'stopTeam', teamId }
      )
  )
  add(
    'teams_open',
    'Open a team member’s existing chat.',
    z.strictObject({ teamId: identifier, taskId: identifier }),
    async ({ teamId, taskId }, target) => {
      const state = await ready(options.teams()).handle(target.key, { action: 'list' })
      const task = state.teams
        .find((item) => item.id === teamId)
        ?.tasks.find((item) => item.id === taskId)
      if (!task?.workspaceId || !task.sessionId) throw new Error('Team chat is not available')
      if (!options.openSession) throw new Error('Opening chats is not available')
      await options.openSession(task.workspaceId, task.sessionId)
      return { opened: true }
    }
  )
  add(
    'terminal_create',
    'Open a new agent-owned project terminal for terminal_write. Existing user-owned terminals require user handover.',
    terminalCreate,
    async (input, target) => {
      const manager = ready(options.terminals())
      const terminal = target.remote
        ? await manager.createRemote(target.workspaceId, target.remote, input)
        : manager.create(target.workspaceId, target.root, input)
      try {
        return manager.setOwner(terminal.id, 'agent', target.workspaceId)
      } catch (error) {
        await manager.close(terminal.id, target.workspaceId).catch(() => {})
        throw error
      }
    }
  )
  add(
    'terminal_close',
    'Close an agent-owned terminal in this project and stop its process tree. User-owned terminals require user handover.',
    idArguments,
    async ({ id }, target) => {
      const manager = ready(options.terminals())
      if (manager.read(id, target.workspaceId).terminal.owner !== 'agent')
        throw new Error('User-owned terminal requires user handover')
      await manager.close(id, target.workspaceId)
      return { closed: true }
    }
  )
  add(
    'terminal_release',
    'Return an agent-owned project terminal to the user. Only the user can hand existing terminals back to the agent.',
    idArguments,
    ({ id }, target) => {
      const manager = ready(options.terminals())
      if (manager.read(id, target.workspaceId).terminal.owner !== 'agent')
        throw new Error('Terminal is already user-owned')
      return manager.setOwner(id, 'user', target.workspaceId)
    }
  )
  add(
    'context_get',
    'Read persistent task goal, constraints, acceptance criteria and file sources for this project.',
    noArguments,
    (_, target) => options.settings().get().agentContexts?.[target.key] ?? emptyAgentContext()
  )
  add(
    'context_update',
    'Automatically maintain project working context as the task evolves. Patch goal, constraints or criteria; sources replaces up to 8 validated project text files or selection snapshots. This context is included in future prompts.',
    contextUpdate,
    async (patch, target) => {
      const settings = options.settings()
      const previous = settings.get().agentContexts?.[target.key] ?? emptyAgentContext()
      const sources = patch.sources
        ? await Promise.all(
            patch.sources.map(async (source) => {
              const file = await options.files(target.workspaceId).read(source.path)
              if (file.image || typeof file.content !== 'string')
                throw new Error('Context source must be a project text file')
              return { ...source, id: source.id ?? randomUUID() }
            })
          )
        : previous.sources
      const context = { ...previous, enabled: true, ...patch, sources }
      await settings.update({ agentContexts: { [target.key]: context } })
      options.onContextChanged?.(target.workspaceId, context)
      return context
    }
  )
  add(
    'desktop_capabilities',
    'Discover the project tools available in this desktop session and their usage. Browser and window tools are registered separately; SSH terminals use this project’s existing connection.',
    noArguments,
    () => ({
      tools: [...tools, ...(options.extensions ?? [])].map(({ name, description }) => ({
        name,
        description
      })),
      context: 'Project-bound; workspace/root arguments are not accepted.',
      browser:
        'Use browser tools registered in this MCP session for navigation, snapshots, DOM evaluation, input and screenshots.',
      remote:
        'Saved SSH servers are available from any chat: call ssh_profiles, then pass profileId to ssh_connect, ssh_exec, SSH file tools or ssh_upload_file. No separate chat or project is required. Without profileId, SSH tools use the current remote project. Credentials stay in the desktop vault; never request passwords in chat. Launch terminals still use the current project connection.'
    })
  )
  return [...tools, ...(options.extensions ?? [])]
}
