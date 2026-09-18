import { beforeEach, describe, expect, test } from "bun:test"
import { closeDialog, dialog, openInfo, openPermissionDialog, resetDialog } from "./dialog"

const permission = (id: string) => ({
  sessionId: "s1",
  id,
  tool: "shell",
  scope: "execute",
  target: "git status",
  reason: "run command",
})

beforeEach(() => {
  resetDialog()
})

describe("permission queue", () => {
  test("openPermissionDialog shows the first request immediately", () => {
    openPermissionDialog(permission("a"))
    expect(dialog().type).toBe("permission")
  })

  test("burst requests are shown one at a time in FIFO order", () => {
    openPermissionDialog(permission("a"))
    openPermissionDialog(permission("b"))
    openPermissionDialog(permission("c"))

    expect(dialog()).toMatchObject({ type: "permission", id: "a" })
    closeDialog()
    expect(dialog()).toMatchObject({ type: "permission", id: "b" })
    closeDialog()
    expect(dialog()).toMatchObject({ type: "permission", id: "c" })
    closeDialog()
    expect(dialog().type).toBe("none")
  })

  test("closing a regular dialog reveals the next queued permission", () => {
    openInfo({ title: "t", body: "b" })
    openPermissionDialog(permission("a"))
    expect(dialog().type).toBe("info")
    closeDialog()
    expect(dialog()).toMatchObject({ type: "permission", id: "a" })
  })
})