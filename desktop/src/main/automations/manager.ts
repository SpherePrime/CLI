import { randomUUID } from 'node:crypto'
import { realpath } from 'node:fs/promises'
import { isRemoteWorkspace, remoteWorkspaceIdentity } from '../../shared/remoteWorkspace'
import { resolve } from 'node:path'
import type { DesktopEvent } from '../../shared/contracts'
import type {
  AutomationEvent,
  AutomationInput,
  AutomationRequest,
  AutomationResponse,
  AutomationRule,
  AutomationRun
} from '../../shared/automations'
import { AtomicStore } from '../persistence/atomicStore'
import { nextOccurrence, validateInput } from './schedule'
import { activeRun, parseState, type AutomationState } from './state'

export type AgentAdapter = {
  prepare(input: {
    root: string
    title: string
    runId: string
  }): Promise<{ workspaceId: string; sessionId: string }>
  dispatch(input: {
    root: string
    workspaceId: string
    sessionId: string
    runId: string
    prompt: string
  }): Promise<void>
  status(
    run: AutomationRun
  ): Promise<{ status: Exclude<AutomationRun['status'], 'claimed'>; error?: string }>
  cancel(run: AutomationRun): Promise<void>
  filesFingerprint?(root: string): Promise<string>
}
type Options = {
  path: string
  adapter: AgentAdapter
  emit(event: DesktopEvent): void
  now?: () => number
}

export class AutomationManager {
  private state: AutomationState = { version: 1, rules: [], runs: [], ownedRunIds: [] }
  private readonly store: AtomicStore<AutomationState>
  private readonly now: () => number
  private queue: Promise<unknown> = Promise.resolve()
  private timer?: ReturnType<typeof setInterval>
  private loaded = false
  private stopped = false
  private error?: string
  private fingerprints = new Map<string, string>()
  private rebaseline = new Set<string>()
  private monitorErrors = new Map<string, string>()

  constructor(private readonly options: Options) {
    this.store = new AtomicStore(options.path)
    this.now = options.now ?? Date.now
  }
  private serialize<T>(operation: () => Promise<T>): Promise<T> {
    const result = this.queue.then(operation)
    this.queue = result.catch(() => {})
    return result
  }
  private async canonical(root: string) {
    if (isRemoteWorkspace(root)) {
      if (!remoteWorkspaceIdentity(root)) throw new Error('automation.invalidRoot')
      return root
    }
    const resolved = resolve(root)
    const identity = process.platform === 'win32' ? resolved.toLowerCase() : resolved
    try {
      const path = await realpath(resolved)
      return process.platform === 'win32' ? path.toLowerCase() : path
    } catch (error) {
      const code = (error as NodeJS.ErrnoException).code
      const known =
        this.state.rules.some((rule) => rule.root === identity) ||
        this.state.runs.some((run) => run.root === identity)
      if ((code === 'ENOENT' || code === 'ENOTDIR') && known) return identity
      throw error
    }
  }
  private writable() {
    if (!this.loaded) throw new Error('automation.notLoaded')
    if (this.error) throw new Error(this.error)
  }
  private snapshot(root: string): AutomationResponse {
    const error = this.error ?? this.monitorErrors.get(root)
    return structuredClone({
      root,
      rules: this.state.rules.filter((rule) => rule.root === root),
      runs: this.state.runs.filter((run) => run.root === root).toReversed(),
      ...(error ? { error } : {})
    })
  }
  counts() {
    return {
      activeCount: this.state.runs.filter(activeRun).length,
      enabledCount: this.state.rules.filter((rule) => rule.enabled).length
    }
  }
  private async persist(next: AutomationState) {
    const inactive = next.runs.filter((run) => !activeRun(run)).slice(-200)
    const retainedIds = new Set(inactive.map((run) => run.id))
    next.runs = next.runs.filter((run) => activeRun(run) || retainedIds.has(run.id))
    await this.store.write(next)
    this.state = next
    for (const root of this.monitorErrors.keys()) {
      if (
        !next.rules.some(
          (rule) => rule.root === root && rule.enabled && rule.trigger.type === 'filesChanged'
        )
      )
        this.monitorErrors.delete(root)
    }
    for (const root of new Set([
      ...next.rules.map((rule) => rule.root),
      ...next.runs.map((run) => run.root)
    ])) {
      try {
        this.options.emit({
          workspaceId: '',
          type: 'automation',
          payload: { root, ...this.counts() }
        })
      } catch {}
    }
  }
  load(): Promise<void> {
    return this.serialize(async () => {
      if (this.loaded) return
      this.loaded = true
      let state: AutomationState
      try {
        const value = await this.store.read()
        if (value === undefined) return
        state = parseState(value)
      } catch {
        this.error = 'automation.corrupt'
        return
      }
      for (const rule of state.rules) {
        if (!rule.enabled) rule.nextAt = undefined
        else if (rule.nextAt === undefined || rule.nextAt <= this.now())
          rule.nextAt = nextOccurrence(rule.trigger, this.now())
      }
      for (const run of state.runs.filter(activeRun)) {
        if (run.sessionId && run.workspaceId) {
          try {
            const status = await this.options.adapter.status(run)
            Object.assign(run, status, { error: status.error })
          } catch {
            run.status = 'running'
            run.error = 'automation.recoveryFailed'
          }
        } else {
          run.status = 'interrupted'
          run.error = 'automation.interrupted'
        }
        if (!activeRun(run)) run.finishedAt = this.now()
        this.rebaseline.add(run.root)
      }
      await this.persist(state)
    })
  }
  start() {
    this.stopped = false
    if (!this.timer)
      this.timer = setInterval(() => {
        void this.tick().catch((error) => {
          try {
            this.options.emit({
              workspaceId: '',
              type: 'automation',
              payload: { error: String(error), ...this.counts() }
            })
          } catch {}
        })
      }, 10000)
    this.timer.unref?.()
  }
  stop() {
    this.stopped = true
    if (this.timer) clearInterval(this.timer)
    this.timer = undefined
  }
  async dispose() {
    this.stop()
    await this.queue
  }
  list(root: string) {
    return this.serialize(async () => this.snapshot(await this.canonical(root)))
  }
  handle(request: AutomationRequest): Promise<AutomationResponse> {
    if (request.action === 'list') return this.list(request.root)
    if (request.action === 'save') return this.save(request.root, request.input)
    if (request.action === 'cancel') return this.cancel(request.root, request.runId)
    if (request.action === 'remove') return this.remove(request.root, request.id)
    return this.run(request.root, request.id)
  }
  save(root: string, input: AutomationInput) {
    return this.serialize(async () => {
      this.writable()
      validateInput(input)
      root = await this.canonical(root)
      const next = structuredClone(this.state)
      const previous = input.id
        ? next.rules.find((rule) => rule.id === input.id && rule.root === root)
        : undefined
      if (input.id && !previous) throw new Error('automation.notFound')
      const changedTrigger =
        !previous ||
        JSON.stringify(previous.trigger) !== JSON.stringify(input.trigger) ||
        previous.enabled !== input.enabled
      const rule: AutomationRule = {
        ...previous,
        title: input.title.trim(),
        prompt: input.prompt.trim(),
        enabled: input.enabled,
        trigger: structuredClone(input.trigger),
        id: previous?.id ?? randomUUID(),
        root,
        createdAt: previous?.createdAt ?? this.now(),
        updatedAt: this.now(),
        nextAt: input.enabled
          ? changedTrigger
            ? nextOccurrence(input.trigger, this.now())
            : previous?.nextAt
          : undefined
      }
      if (changedTrigger) {
        rule.lastFingerprint = undefined
        rule.lastTriggeredAt = this.now()
        this.fingerprints.delete(root)
      }
      next.rules = next.rules.filter((candidate) => candidate.id !== rule.id).concat(rule)
      await this.persist(next)
      return this.snapshot(root)
    })
  }
  remove(root: string, id: string) {
    return this.serialize(async () => {
      this.writable()
      root = await this.canonical(root)
      if (this.state.runs.some((run) => run.ruleId === id && run.root === root && activeRun(run)))
        throw new Error('automation.active')
      if (!this.state.rules.some((rule) => rule.id === id && rule.root === root))
        throw new Error('automation.notFound')
      const next = structuredClone(this.state)
      next.rules = next.rules.filter((rule) => rule.id !== id || rule.root !== root)
      await this.persist(next)
      this.options.emit({
        workspaceId: '',
        type: 'automation',
        payload: { root, ...this.counts() }
      })
      return this.snapshot(root)
    })
  }
  private async dispatch(
    rule: AutomationRule,
    trigger: AutomationRun['trigger'],
    fingerprint?: string
  ) {
    if (this.stopped || this.state.runs.some((run) => run.ruleId === rule.id && activeRun(run)))
      return
    const run: AutomationRun = {
      id: randomUUID(),
      ruleId: rule.id,
      root: rule.root,
      title: rule.title,
      trigger,
      status: 'claimed',
      createdAt: this.now()
    }
    const claimed = structuredClone(this.state)
    claimed.runs.push(run)
    claimed.ownedRunIds.push(run.id)
    const claimedRule = claimed.rules.find((candidate) => candidate.id === rule.id)!
    claimedRule.lastTriggeredAt = this.now()
    if (fingerprint) claimedRule.lastFingerprint = fingerprint
    if (trigger !== 'manual') claimedRule.nextAt = nextOccurrence(rule.trigger, this.now())
    await this.persist(claimed)
    let dispatchAttempted = false
    try {
      const prepared = await this.options.adapter.prepare({
        root: rule.root,
        title: rule.title,
        runId: run.id
      })
      const preparedState = structuredClone(this.state)
      Object.assign(
        preparedState.runs.find((candidate) => candidate.id === run.id)!,
        prepared
      )
      await this.persist(preparedState)
      if (this.stopped) throw new Error('automation.interrupted')
      dispatchAttempted = true
      await this.options.adapter.dispatch({
        root: rule.root,
        ...prepared,
        runId: run.id,
        prompt: rule.prompt
      })
    } catch (error) {
      const failed = structuredClone(this.state)
      Object.assign(
        failed.runs.find((candidate) => candidate.id === run.id)!,
        {
          status: dispatchAttempted ? 'running' : 'interrupted',
          error: error instanceof Error ? error.message : String(error),
          ...(dispatchAttempted ? {} : { finishedAt: this.now() })
        }
      )
      await this.persist(failed)
      return
    }
    const running = structuredClone(this.state)
    running.runs.find((candidate) => candidate.id === run.id)!.status = 'running'
    await this.persist(running)
  }
  run(root: string, id: string) {
    return this.serialize(async () => {
      this.writable()
      root = await this.canonical(root)
      const rule = this.state.rules.find(
        (candidate) => candidate.id === id && candidate.root === root
      )
      if (!rule) throw new Error('automation.notFound')
      await this.dispatch(rule, 'manual')
      return this.snapshot(root)
    })
  }
  cancel(root: string, runId: string) {
    return this.serialize(async () => {
      this.writable()
      root = await this.canonical(root)
      const run = this.state.runs.find(
        (candidate) => candidate.id === runId && candidate.root === root
      )
      if (!run) throw new Error('automation.notFound')
      if (activeRun(run)) {
        await this.options.adapter.cancel(structuredClone(run))
        const next = structuredClone(this.state)
        Object.assign(
          next.runs.find((candidate) => candidate.id === runId)!,
          { status: 'cancelled', finishedAt: this.now() }
        )
        this.rebaseline.add(root)
        await this.persist(next)
      }
      return this.snapshot(root)
    })
  }
  private async acceptEvent(event: AutomationEvent) {
    if (event.sourceRunId && this.state.ownedRunIds.includes(event.sourceRunId)) return
    const ownWorkWasActive = this.state.runs.some(
      (run) => run.root === event.root && activeRun(run)
    )
    if (event.type === 'filesChanged' && ownWorkWasActive) return
    for (const rule of this.state.rules.filter(
      (rule) => rule.root === event.root && rule.enabled && rule.trigger.type === event.type
    )) {
      const trigger = rule.trigger
      if (trigger.type !== 'filesChanged' && trigger.type !== 'runFailed') continue
      const fingerprint = event.fingerprint ?? event.sourceRunId
      if (fingerprint && fingerprint === rule.lastFingerprint) continue
      if (
        rule.lastTriggeredAt !== undefined &&
        this.now() - rule.lastTriggeredAt < trigger.cooldownMinutes * 60000
      )
        continue
      await this.dispatch(rule, event.type, fingerprint)
    }
  }
  event(event: AutomationEvent) {
    return this.serialize(async () => {
      this.writable()
      if (this.stopped) return
      await this.acceptEvent({ ...event, root: await this.canonical(event.root) })
    })
  }
  tick() {
    return this.serialize(async () => {
      this.writable()
      if (this.stopped) return
      for (const run of this.state.runs.filter(activeRun)) {
        if (!run.sessionId) continue
        let status: Awaited<ReturnType<AgentAdapter['status']>>
        try {
          status = await this.options.adapter.status(structuredClone(run))
        } catch {
          continue
        }
        if (status.status !== run.status || status.error !== run.error) {
          const next = structuredClone(this.state)
          const updated = next.runs.find((candidate) => candidate.id === run.id)!
          Object.assign(updated, status, { error: status.error })
          if (!activeRun(updated)) {
            updated.finishedAt = this.now()
            this.rebaseline.add(run.root)
          }
          await this.persist(next)
        }
      }
      if (this.options.adapter.filesFingerprint) {
        for (const root of new Set(
          this.state.rules
            .filter((rule) => rule.enabled && rule.trigger.type === 'filesChanged')
            .map((rule) => rule.root)
        )) {
          let fingerprint: string
          try {
            fingerprint = await this.options.adapter.filesFingerprint(root)
            if (this.monitorErrors.delete(root))
              this.options.emit({
                workspaceId: '',
                type: 'automation',
                payload: { root, ...this.counts() }
              })
          } catch (error) {
            this.monitorErrors.set(root, error instanceof Error ? error.message : String(error))
            this.options.emit({
              workspaceId: '',
              type: 'automation',
              payload: { root, error: this.monitorErrors.get(root), ...this.counts() }
            })
            continue
          }
          const previous = this.fingerprints.get(root)
          this.fingerprints.set(root, fingerprint)
          const reset = this.rebaseline.delete(root)
          if (previous !== undefined && previous !== fingerprint && !reset)
            await this.acceptEvent({ root, type: 'filesChanged', fingerprint })
        }
      }
      for (const rule of this.state.rules.filter(
        (rule) => rule.enabled && rule.nextAt !== undefined && rule.nextAt <= this.now()
      )) {
        const next = structuredClone(this.state)
        next.rules.find((candidate) => candidate.id === rule.id)!.nextAt = nextOccurrence(
          rule.trigger,
          this.now()
        )
        await this.persist(next)
        await this.dispatch(rule, rule.trigger.type)
      }
    })
  }
}
