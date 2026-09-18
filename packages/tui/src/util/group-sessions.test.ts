import { describe, expect, test } from "bun:test"
import { groupSessions, sessionLabel, projectLabel } from "./group-sessions"
import type { SessionInfo } from "../client"

function session(id: string, overrides: Partial<SessionInfo> = {}): SessionInfo {
  return {
    id,
    title: undefined,
    created: "2026-09-18T00:00:00Z",
    updated: "2026-09-18T00:00:00Z",
    project_id: null,
    project_name: null,
    project_path: null,
    ...overrides,
  }
}

describe("groupSessions", () => {
  test("current project group comes first sorted by updated desc", () => {
    const a = session("a", { title: "old", project_id: "p1", project_name: "cli", updated: "2026-09-18T01:00:00Z" })
    const b = session("b", { title: "new", project_id: "p1", project_name: "cli", updated: "2026-09-18T02:00:00Z" })
    const c = session("c", { title: "other", project_id: "p2", project_name: "other", updated: "2026-09-18T03:00:00Z" })

    const groups = groupSessions([a, c, b], "p1")

    expect(groups).toHaveLength(2)
    expect(groups[0]!.title).toBe("Current project")
    expect(groups[0]!.sessions.map((s) => s.id)).toEqual(["b", "a"])
    expect(groups[1]!.title).toBe("Other projects")
    expect(groups[1]!.sessions.map((s) => s.id)).toEqual(["c"])
  })

  test("sessions without current project all go to others", () => {
    const a = session("a", { project_id: null, updated: "2026-09-18T01:00:00Z" })
    const groups = groupSessions([a], "p1")
    expect(groups).toHaveLength(1)
    expect(groups[0]!.id).toBe("others")
  })

  test("filter matches title, project name and path", () => {
    const title = session("t", { title: "Refactor login", project_id: "p1" })
    const name = session("n", { title: "x", project_name: "website", project_id: "p1" })
    const path = session("p", { title: "y", project_path: "C:/work/backend", project_id: "p1" })

    expect(groupSessions([title, name, path], "p1", "refactor").map((g) => g.sessions[0]!.id)).toEqual(["t"])
    expect(groupSessions([title, name, path], "p1", "website").map((g) => g.sessions[0]!.id)).toEqual(["n"])
    expect(groupSessions([title, name, path], "p1", "backend").map((g) => g.sessions[0]!.id)).toEqual(["p"])
    expect(groupSessions([title, name, path], "p1", "").flatMap((g) => g.sessions)).toHaveLength(3)
  })

  test("empty list yields no groups", () => {
    expect(groupSessions([], "p1")).toEqual([])
  })
})

describe("labels", () => {
  test("sessionLabel uses title or short id", () => {
    expect(sessionLabel(session("12345678-0000-0000-0000-000000000000", { title: "Hi" }))).toBe("Hi")
    expect(sessionLabel(session("12345678-0000-0000-0000-000000000000"))).toBe("12345678")
  })

  test("projectLabel falls back to 'no project'", () => {
    expect(projectLabel(session("x", { project_name: "cli" }))).toBe("cli")
    expect(projectLabel(session("x"))).toBe("no project")
  })
})