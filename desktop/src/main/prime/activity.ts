export class ActivityTracker {
  private runs = new Map<string, string>()
  private terminal = new Set<string>()
  accept(workspaceId: string, runId: string, sessionId: string): void {
    const key = `${workspaceId}:${runId}`
    if (!this.terminal.has(key)) this.runs.set(key, sessionId)
  }
  event(workspaceId: string, type: string, payload: any): void {
    if (type === 'run_complete' && payload.run_id) this.remove(workspaceId, payload.run_id)
  }
  reconcile(workspaceId: string, records: any[]): void {
    for (const record of records) {
      if (['running', 'queued', 'accepted'].includes(record.status))
        this.accept(workspaceId, record.run_id, record.session_id)
      else this.remove(workspaceId, record.run_id)
    }
  }
  remove(workspaceId: string, runId: string): void {
    const key = `${workspaceId}:${runId}`
    this.runs.delete(key)
    this.terminal.add(key)
    if (this.terminal.size > 5000)
      for (const oldest of this.terminal) {
        this.terminal.delete(oldest)
        if (this.terminal.size <= 4000) break
      }
  }
  isBusy(): boolean {
    return this.runs.size > 0
  }
  forget(workspaceId: string, runId: string): void {
    this.runs.delete(`${workspaceId}:${runId}`)
  }
}
