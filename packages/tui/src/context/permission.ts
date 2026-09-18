import { RGBA } from "@opentui/core"
import { useSession } from "./session"
import { openConfirm, openInfo, openSelect } from "./dialog"
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
      return theme.warning
    case "deny":
      return theme.error
    default:
      return theme.textMuted
  }
}

export function permissionChipColor(mode: PermissionMode | undefined): RGBA {
  switch (mode) {
    case "full_access":
      return theme.warning
    case "deny":
      return theme.error
    case "auto_edit":
      return theme.info
    case "ask":
      return theme.textMuted
    default:
      return theme.textMuted
  }
}

function applyMode(client: AgentClient, sessionId: string, mode: PermissionMode, rememberProject: boolean): void {
  void client
    .setPermissionMode(sessionId, mode, rememberProject)
    .then((result) => {
      const session = useSession()
      session.setPermissionMode(result.mode)
      session.setPermissionApplied(result.applied)
    })
    .catch((error: unknown) => {
      openInfo({ title: "Permission mode", body: String(error) })
    })
}

export function switchPermissionMode(client: AgentClient, mode: PermissionMode): void {
  const session = useSession()
  const id = session.sessionId()
  if (!id) return
  const current = session.permissionMode()
  if (mode === "full_access" && current !== "full_access") {
    openConfirm({
      title: "Enable full access?",
      confirmColor: "warning",
      body:
        "Full access skips confirmations for shell, edits, deletes, git, install and network tools for this session. " +
        "Secret redaction, workspace-integrity checks and deletion protection stay active.\n\n" +
        "You can go back to Ask at any time.",
      confirmLabel: "Enable for this session",
      secondaryLabel: "Remember for this project",
      cancelLabel: "Cancel",
      onConfirm: () => applyMode(client, id, "full_access", false),
      onSecondary: () => applyMode(client, id, "full_access", true),
    })
    return
  }
  if (mode === "deny") {
    openConfirm({
      title: "Deny all tools?",
      confirmColor: "error",
      body: "Deny blocks every tool call until you switch back. Turn still runs and asks again only after a mode change.",
      confirmLabel: "Enable deny",
      cancelLabel: "Cancel",
      onConfirm: () => applyMode(client, id, "deny", false),
    })
    return
  }
  applyMode(client, id, mode, false)
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
    onSelect: (value) => switchPermissionMode(client, value as PermissionMode),
  })
}