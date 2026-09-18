import { beforeEach, describe, expect, test } from "bun:test"
import { dialog, resetDialog } from "./dialog"
import { useSession, resetSession } from "./session"
import { switchPermissionMode } from "./permission"
import type { AgentClient, PermissionMode } from "../client"

function fakeClient(recorder: { calls: { mode: PermissionMode; remember: boolean }[] }): AgentClient {
  return {
    setPermissionMode: async (id: string, mode: PermissionMode, remember?: boolean) => {
      const _id = id
      recorder.calls.push({ mode, remember: remember ?? false })
      return { applied: true, mode }
    },
  } as unknown as AgentClient
}

beforeEach(() => {
  resetDialog()
  resetSession()
  useSession().setSessionId("s1")
})

describe("permission mode switch", () => {
  test("selecting full_access asks for confirmation first", () => {
    const recorder = { calls: [] as { mode: PermissionMode; remember: boolean }[] }
    switchPermissionMode(fakeClient(recorder), "full_access")
    const state = dialog()
    expect(state.type).toBe("confirm")
    expect(recorder.calls).toHaveLength(0)
  })

  test("full_access confirm enables mode for this session only", () => {
    const recorder = { calls: [] as { mode: PermissionMode; remember: boolean }[] }
    const client = fakeClient(recorder)
    switchPermissionMode(client, "full_access")
    const state = dialog()
    expect(state.type).toBe("confirm")
    if (state.type !== "confirm") return
    state.onConfirm()
    expect(recorder.calls).toEqual([{ mode: "full_access", remember: false }])
  })

  test("full_access secondary option remembers mode for this project", () => {
    const recorder = { calls: [] as { mode: PermissionMode; remember: boolean }[] }
    const client = fakeClient(recorder)
    switchPermissionMode(client, "full_access")
    const state = dialog()
    if (state.type !== "confirm") return
    state.onSecondary?.()
    expect(recorder.calls).toEqual([{ mode: "full_access", remember: true }])
  })

  test("ask and auto_edit switch without confirmation", () => {
    const recorder = { calls: [] as { mode: PermissionMode; remember: boolean }[] }
    const client = fakeClient(recorder)
    switchPermissionMode(client, "ask")
    switchPermissionMode(client, "auto_edit")
    expect(dialog().type).toBe("none")
    expect(recorder.calls).toEqual([
      { mode: "ask", remember: false },
      { mode: "auto_edit", remember: false },
    ])
  })

  test("deny asks for confirmation because it is a hard block", () => {
    const recorder = { calls: [] as { mode: PermissionMode; remember: boolean }[] }
    switchPermissionMode(fakeClient(recorder), "deny")
    expect(dialog().type).toBe("confirm")
    expect(recorder.calls).toHaveLength(0)
  })
})

describe("allow for session label", () => {
  test("permission dialog uses session-scoped wording", async () => {
    const source = await Bun.file(
      new URL("../component/dialog/permission-dialog.tsx", import.meta.url),
    ).text()
    expect(source).toContain("allow for session")
    expect(source).not.toContain("allow always")
  })
})