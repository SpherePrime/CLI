import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Preferences } from '../../src/shared/contracts'

type EffectEntry = { deps?: unknown[]; cleanup?: () => void }

const hookState = {
  cursor: 0,
  states: [] as unknown[],
  refs: [] as { current: unknown }[],
  effects: [] as EffectEntry[],
  pendingEffects: [] as { index: number; effect: () => void | (() => void); deps?: unknown[] }[]
}

function resetHooks() {
  for (const entry of hookState.effects) entry?.cleanup?.()
  hookState.cursor = 0
  hookState.states = []
  hookState.refs = []
  hookState.effects = []
  hookState.pendingEffects = []
}

function flushTasks() {
  return new Promise<void>((resolve) => setTimeout(resolve, 0))
}

vi.mock('react', () => ({
  useState<T>(initial: T | (() => T)) {
    const index = hookState.cursor++
    if (!(index in hookState.states)) {
      hookState.states[index] = typeof initial === 'function' ? (initial as () => T)() : initial
    }
    const setState = (next: T | ((prev: T) => T)) => {
      hookState.states[index] =
        typeof next === 'function'
          ? (next as (prev: T) => T)(hookState.states[index] as T)
          : next
    }
    return [hookState.states[index] as T, setState] as const
  },
  useRef<T>(initial: T) {
    const index = hookState.cursor++
    if (!(index in hookState.refs)) {
      hookState.refs[index] = { current: initial }
    }
    return hookState.refs[index] as { current: T }
  },
  useEffect(effect: () => void | (() => void), deps?: unknown[]) {
    const index = hookState.cursor++
    const previous = hookState.effects[index]
    const changed =
      !previous ||
      !deps ||
      !previous.deps ||
      deps.length !== previous.deps.length ||
      deps.some((item, i) => !Object.is(item, previous.deps![i]))
    if (changed) {
      hookState.pendingEffects.push({ index, effect, deps })
    }
  }
}))

import { useChatPermissions } from '../../src/renderer/src/features/chat/useChatPermissions'

function runHook<T>(render: () => T): T {
  hookState.cursor = 0
  hookState.pendingEffects = []
  const result = render()
  const queued = [...hookState.pendingEffects]
  hookState.pendingEffects = []
  for (const item of queued) {
    hookState.effects[item.index]?.cleanup?.()
    const cleanup = item.effect()
    hookState.effects[item.index] = {
      deps: item.deps,
      cleanup: typeof cleanup === 'function' ? cleanup : undefined
    }
  }
  return result
}

function basePreferences(overrides: Partial<Preferences> = {}): Preferences {
  return {
    theme: 'dark',
    language: 'ru',
    recent: [],
    pinned: [],
    drafts: {},
    sidebarWidth: 256,
    inspectorWidth: 350,
    ...overrides
  }
}

describe('useChatPermissions', () => {
  afterEach(() => {
    resetHooks()
    delete (globalThis as { window?: unknown }).window
  })

  it('preserves draft Full Access across session materialization', async () => {
    const call = vi.fn().mockResolvedValue(undefined)
    ;(globalThis as { window?: unknown }).window = { prime: { call } }

    let preferences = basePreferences()
    const savePreferences = vi.fn(async (patch: Partial<Preferences>) => {
      preferences = {
        ...preferences,
        ...patch,
        skipBySession: { ...preferences.skipBySession, ...patch.skipBySession }
      }
    })
    const onError = vi.fn()

    let sessionId: string | undefined = undefined
    const render = () =>
      runHook(() =>
        useChatPermissions({
          projectId: 'proj-1',
          sessionId,
          preferences,
          savePreferences,
          onError
        })
      )

    let result = render()
    expect(result.skip).toBe(false)

    await result.setSkip(true)
    result = render()
    expect(result.skip).toBe(true)

    sessionId = 'sess-1'
    result = render()
    expect(result.skip).toBe(true)

    await flushTasks()

    expect(call).toHaveBeenCalledWith('proj-1', 'skip', { sessionId: 'sess-1', skip: true })
    expect(savePreferences).toHaveBeenCalledWith({ skipBySession: { 'sess-1': true } })

    result = render()
    expect(result.skip).toBe(true)
    expect(onError).not.toHaveBeenCalled()
  })

  it('respects preferences.defaultSkip when no per-session preference exists', async () => {
    const call = vi.fn().mockResolvedValue(undefined)
    ;(globalThis as { window?: unknown }).window = { prime: { call } }

    const preferences = basePreferences({ defaultSkip: true })
    const savePreferences = vi.fn().mockResolvedValue(undefined)
    const onError = vi.fn()

    const draftResult = runHook(() =>
      useChatPermissions({
        projectId: 'proj-1',
        sessionId: undefined,
        preferences,
        savePreferences,
        onError
      })
    )
    expect(draftResult.skip).toBe(true)

    const sessionResult = runHook(() =>
      useChatPermissions({
        projectId: 'proj-1',
        sessionId: 'sess-2',
        preferences,
        savePreferences,
        onError
      })
    )
    expect(sessionResult.skip).toBe(true)
    await flushTasks()
    expect(call).toHaveBeenCalledWith('proj-1', 'skip', { sessionId: 'sess-2', skip: true })
  })
})
