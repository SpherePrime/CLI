export type AgentWorkerStatus =
  'queued' | 'running' | 'done' | 'error' | 'canceled' | 'interrupted' | 'idle'

export type AgentWorker = {
  id: string
  session_id: string
  parent_session_id: string
  root_session_id: string
  name: string
  agent: string
  task: string
  status: AgentWorkerStatus
  started_at: number
  updated_at: number
}

export function workerActive(worker: AgentWorker) {
  return worker.status === 'running' || worker.status === 'queued'
}

export const workerStatusLabels: Record<AgentWorkerStatus, string> = {
  queued: 'В очереди',
  running: 'Выполняется',
  done: 'Завершён',
  error: 'Ошибка',
  canceled: 'Остановлен',
  interrupted: 'Прерван',
  idle: 'Ожидает задания'
}
