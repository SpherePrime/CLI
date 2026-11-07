export type ChatGoal = {
  session_id: string
  text: string
  status: 'active' | 'achieved'
  summary?: string
  created_at?: number
  updated_at?: number
}

type GoalTodo = { content?: string; status?: string; active_form?: string }

export type GoalProgress = {
  total: number
  completed: number
  current?: string
  ratio: number
}

export type GoalPhase = 'none' | 'planning' | 'working' | 'verifying' | 'paused' | 'achieved'

export function goalProgress(todos: GoalTodo[] | undefined): GoalProgress {
  const items = todos ?? []
  const completed = items.filter((todo) => todo.status === 'completed').length
  const active =
    items.find((todo) => todo.status === 'in_progress') ??
    items.find((todo) => todo.status !== 'completed')
  return {
    total: items.length,
    completed,
    current: active ? active.active_form || active.content : undefined,
    ratio: items.length ? completed / items.length : 0
  }
}

export function goalPhase(
  goal: ChatGoal | null | undefined,
  progress: GoalProgress,
  busy: boolean
): GoalPhase {
  if (!goal?.text) return 'none'
  if (goal.status === 'achieved') return 'achieved'
  if (!progress.total) return busy ? 'planning' : 'paused'
  if (!busy) return 'paused'
  return progress.completed === progress.total ? 'verifying' : 'working'
}

export function normalizeGoal(value: unknown): ChatGoal | null {
  if (!value || typeof value !== 'object') return null
  const goal = value as ChatGoal
  return typeof goal.text === 'string' && goal.text.trim() ? goal : null
}
