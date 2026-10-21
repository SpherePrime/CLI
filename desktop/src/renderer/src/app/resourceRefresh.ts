import type { Operation } from '../../../shared/contracts'

export type ResourceName = 'mcp' | 'lsp' | 'skills' | 'channels'

export const resourceOperations: Record<ResourceName, Operation> = {
  mcp: 'mcp',
  lsp: 'lsp',
  skills: 'skills',
  channels: 'channelMessages'
}

export const resourceEvents: Record<string, ResourceName> = {
  mcp_event: 'mcp',
  lsp_event: 'lsp',
  skills_event: 'skills',
  mcp_channel_message: 'channels'
}

export const resourceRefreshDelay = 300

export function createResourceRefresh(
  load: (name: ResourceName) => Promise<void>,
  delay = resourceRefreshDelay
) {
  const pending = new Set<ResourceName>()
  let timer: ReturnType<typeof setTimeout> | undefined
  let running = false
  let disposed = false
  const run = async () => {
    timer = undefined
    if (running || disposed) return
    const names = [...pending]
    pending.clear()
    running = true
    await Promise.allSettled(names.map((name) => load(name)))
    running = false
    if (pending.size && !disposed) timer ??= setTimeout(run, delay)
  }
  return {
    schedule(name: ResourceName) {
      if (disposed) return
      pending.add(name)
      if (!running) timer ??= setTimeout(run, delay)
    },
    dispose() {
      disposed = true
      if (timer) clearTimeout(timer)
      timer = undefined
      pending.clear()
    }
  }
}
