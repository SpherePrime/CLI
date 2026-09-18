import { RGBA } from "@opentui/core"
import { useSession } from "./session"
import { openInfo, openSelect } from "./dialog"
import { theme } from "../theme"
import type { AgentClient, PermissionMode } from "../client"

export const permissionOptions: { mode: PermissionMode; label: string; description: string }[] = [
  { mode: "ask", label: "Ask", description: "Confirm every tool call" },
  { mode: "auto_edit", label: "Auto edit", description: "File edits allowed, everything else asks" },
  { mode: "full_access", label: "Full access", description: "Allow everything without asking" },
  { mode: "deny", label: "Deny", description: "Deny everything" },
]

export function permissionLabel(mode: PermissionMode | undefined): string {
  switch (mode) {
    case "ask":
      return "ask"
    case "auto_edit":
      return "auto edit"
    case "full_access":
      return "full access"
    case "deny":
      return "deny"
    default:
      return "unknown"
  }
}

export function permissionColor(mode: PermissionMode | undefined): RGBA {
  switch (mode) {
    case "ask":
      return theme.warning
    case "auto_edit":
      return theme.info
    case "full_access":
      return theme.success
    case "deny":
      return theme.error
    default:
      return theme.textMuted
  }
}

export function openPermissionSwitcher(client: AgentClient): void {
  const session = useSession()
  const id = session.sessionId()
  if (!id) return
  openSelect({
    title: "Permission mode",
    options: permissionOptions.map((option) => ({
      title: `${option.label}${option.mode === session.permissionMode() ? " (current)" : ""}`,
      value: option.mode,
      description: option.description,
    })),
    onSelect: (value) => {
      const mode = value as PermissionMode
      void client
        .setPermissionMode(id, mode)
        .then((result) => {
          session.setPermissionMode(result.mode)
          session.setPermissionApplied(result.applied)
        })
        .catch((error: unknown) => {
          openInfo({ title: "Permission mode", body: String(error) })
        })
    },
  })
}