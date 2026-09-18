import type { SessionInfo } from "../client"

export type SessionGroup = {
  id: string
  title: string
  sessions: SessionInfo[]
}

export function groupSessions(
  sessions: SessionInfo[],
  currentProjectId?: string | null,
  query?: string,
): SessionGroup[] {
  const needle = (query ?? "").trim().toLowerCase()
  const filtered = sessions.filter((session) => {
    if (!needle) return true
    const title = session.title ?? ""
    const name = session.project_name ?? ""
    const path = session.project_path ?? ""
    return (
      title.toLowerCase().includes(needle) ||
      name.toLowerCase().includes(needle) ||
      path.toLowerCase().includes(needle)
    )
  })
  const sorted = [...filtered].sort((a, b) => String(b.updated).localeCompare(String(a.updated)))
  const current = sorted.filter((session) => session.project_id && session.project_id === currentProjectId)
  const others = sorted.filter((session) => !(session.project_id && session.project_id === currentProjectId))
  const groups: SessionGroup[] = []
  if (current.length > 0) groups.push({ id: "current", title: "Current project", sessions: current })
  if (others.length > 0) groups.push({ id: "others", title: "Other projects", sessions: others })
  return groups
}

export function sessionLabel(session: SessionInfo): string {
  return session.title?.trim() || session.id.slice(0, 8)
}

export function projectLabel(session: SessionInfo): string {
  return session.project_name ?? "no project"
}