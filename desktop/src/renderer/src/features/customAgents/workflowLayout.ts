import type { CustomAgent } from '../../../../shared/customAgents'

export interface NodePosition {
  x: number
  y: number
}

export type WorkflowLinkKind = 'next' | 'verifier'

export const WORKFLOW_NODE_WIDTH = 264
export const WORKFLOW_NODE_HEIGHT = 164
export const WORKFLOW_LAYOUT_STORAGE_KEY = 'prime.customAgents.layout.v1'

const PORT_IN_OFFSET_Y = 94
const PORT_NEXT_OFFSET_Y = 74
const PORT_VERIFIER_OFFSET_Y = 114
const COLUMN_GAP = 396
const ROW_GAP = 212
const ORIGIN_X = 52
const ORIGIN_Y = 72

export function getPortPoint(
  pos: NodePosition,
  port: 'in' | WorkflowLinkKind
): NodePosition {
  if (port === 'in') {
    return { x: pos.x, y: pos.y + PORT_IN_OFFSET_Y }
  }
  if (port === 'next') {
    return { x: pos.x + WORKFLOW_NODE_WIDTH, y: pos.y + PORT_NEXT_OFFSET_Y }
  }
  return { x: pos.x + WORKFLOW_NODE_WIDTH, y: pos.y + PORT_VERIFIER_OFFSET_Y }
}

export function buildBezierPath(
  x1: number,
  y1: number,
  x2: number,
  y2: number
): { path: string; midX: number; midY: number } {
  const dx =
    x2 >= x1
      ? Math.max(64, (x2 - x1) * 0.5)
      : Math.max(100, Math.abs(x2 - x1) * 0.45 + 64)
  const path = `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`
  return {
    path,
    midX: (x1 + x2) / 2,
    midY: (y1 + y2) / 2
  }
}

export function computeAutoLayout(agents: CustomAgent[]): Record<string, NodePosition> {
  if (agents.length === 0) return {}
  const byId = new Map(agents.map((agent) => [agent.id, agent]))
  const orderIndex = new Map(agents.map((agent, index) => [agent.id, index]))
  const ranks = new Map<string, number>()
  for (const agent of agents) {
    ranks.set(agent.id, 0)
  }

  const maxPasses = Math.max(1, agents.length)
  for (let pass = 0; pass < maxPasses; pass++) {
    let changed = false
    for (const agent of agents) {
      const currentRank = ranks.get(agent.id) ?? 0
      if (
        agent.next_agent_id &&
        agent.next_agent_id !== agent.id &&
        byId.has(agent.next_agent_id)
      ) {
        const nextRank = ranks.get(agent.next_agent_id) ?? 0
        if (nextRank <= currentRank && currentRank + 1 < agents.length) {
          ranks.set(agent.next_agent_id, currentRank + 1)
          changed = true
        }
      }
      if (
        agent.verifier_agent_id &&
        agent.verifier_agent_id !== agent.id &&
        byId.has(agent.verifier_agent_id)
      ) {
        const verifierRank = ranks.get(agent.verifier_agent_id) ?? 0
        if (verifierRank <= currentRank && currentRank + 1 < agents.length) {
          ranks.set(agent.verifier_agent_id, currentRank + 1)
          changed = true
        }
      }
    }
    if (!changed) break
  }

  const columns = new Map<number, CustomAgent[]>()
  for (const agent of agents) {
    const rank = ranks.get(agent.id) ?? 0
    const bucket = columns.get(rank) || []
    bucket.push(agent)
    columns.set(rank, bucket)
  }

  const sortedRanks = [...columns.keys()].sort((a, b) => a - b)
  const result: Record<string, NodePosition> = {}
  for (const rank of sortedRanks) {
    const bucket = columns.get(rank) || []
    bucket.sort((a, b) => {
      const aVerifier = a.role === 'verifier' ? 1 : 0
      const bVerifier = b.role === 'verifier' ? 1 : 0
      if (aVerifier !== bVerifier) return aVerifier - bVerifier
      return (orderIndex.get(a.id) ?? 0) - (orderIndex.get(b.id) ?? 0)
    })
    bucket.forEach((agent, row) => {
      result[agent.id] = {
        x: ORIGIN_X + rank * COLUMN_GAP,
        y: ORIGIN_Y + row * ROW_GAP
      }
    })
  }
  return result
}

function collidesWithPlaced(
  candidate: NodePosition,
  placed: NodePosition[]
): boolean {
  return placed.some(
    (other) =>
      Math.abs(other.x - candidate.x) < WORKFLOW_NODE_WIDTH + 32 &&
      Math.abs(other.y - candidate.y) < WORKFLOW_NODE_HEIGHT + 24
  )
}

export function mergeNodePositions(
  agents: CustomAgent[],
  saved: Record<string, NodePosition>
): Record<string, NodePosition> {
  const auto = computeAutoLayout(agents)
  const merged: Record<string, NodePosition> = {}
  const placed: NodePosition[] = []

  for (const agent of agents) {
    const existing = saved[agent.id]
    if (
      existing &&
      typeof existing.x === 'number' &&
      Number.isFinite(existing.x) &&
      typeof existing.y === 'number' &&
      Number.isFinite(existing.y)
    ) {
      merged[agent.id] = { x: existing.x, y: existing.y }
      placed.push(merged[agent.id])
    }
  }

  for (const agent of agents) {
    if (merged[agent.id]) continue
    const candidate = { ...(auto[agent.id] || { x: ORIGIN_X, y: ORIGIN_Y }) }
    while (collidesWithPlaced(candidate, placed)) {
      candidate.y += ROW_GAP
    }
    merged[agent.id] = candidate
    placed.push(candidate)
  }

  return merged
}

export function loadSavedLayout(
  storage: Pick<Storage, 'getItem'> | null =
    typeof localStorage !== 'undefined' ? localStorage : null
): Record<string, NodePosition> {
  if (!storage) return {}
  try {
    const raw = storage.getItem(WORKFLOW_LAYOUT_STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as Record<string, unknown>
    if (!parsed || typeof parsed !== 'object') return {}
    const clean: Record<string, NodePosition> = {}
    for (const [key, value] of Object.entries(parsed)) {
      if (
        value &&
        typeof value === 'object' &&
        typeof (value as NodePosition).x === 'number' &&
        Number.isFinite((value as NodePosition).x) &&
        typeof (value as NodePosition).y === 'number' &&
        Number.isFinite((value as NodePosition).y)
      ) {
        clean[key] = {
          x: (value as NodePosition).x,
          y: (value as NodePosition).y
        }
      }
    }
    return clean
  } catch {
    return {}
  }
}

export function saveLayout(
  positions: Record<string, NodePosition>,
  storage: Pick<Storage, 'setItem'> | null =
    typeof localStorage !== 'undefined' ? localStorage : null
): void {
  if (!storage) return
  try {
    storage.setItem(WORKFLOW_LAYOUT_STORAGE_KEY, JSON.stringify(positions))
  } catch {}
}

export function canConnectAgents(
  agents: CustomAgent[],
  sourceId: string,
  targetId: string,
  kind: WorkflowLinkKind
): { ok: boolean; reason?: string } {
  if (!sourceId || !targetId) {
    return { ok: false, reason: 'Узел не выбран' }
  }
  if (sourceId === targetId) {
    return { ok: false, reason: 'Нельзя подключить узел к самому себе' }
  }
  const byId = new Map(agents.map((agent) => [agent.id, agent]))
  if (!byId.has(sourceId) || !byId.has(targetId)) {
    return { ok: false, reason: 'Целевой узел не найден' }
  }
  if (kind === 'next') {
    const visited = new Set<string>([sourceId])
    let cursor: string | undefined = targetId
    while (cursor) {
      if (visited.has(cursor)) {
        return { ok: false, reason: 'Связь создаёт цикл в цепочке' }
      }
      visited.add(cursor)
      cursor = byId.get(cursor)?.next_agent_id || ''
    }
  }
  return { ok: true }
}
