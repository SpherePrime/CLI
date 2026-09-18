import { createSignal } from "solid-js"

export type SelectDialogOption = {
  title: string
  value: string
  description?: string
}

export type FormField =
  | { kind: "text"; key: string; label: string; initial?: string; placeholder?: string; hint?: string }
  | { kind: "select"; key: string; label: string; options: { title: string; value: string }[]; initial?: string }

export type PermissionRequest = {
  sessionId?: string
  id: string
  tool: string
  scope: string
  target: string
  reason: string
}

export type DialogState =
  | { type: "none" }
  | { type: "palette" }
  | { type: "model" }
  | { type: "provider" }
  | { type: "sessions" }
  | { type: "select"; title: string; options: SelectDialogOption[]; onSelect: (value: string) => void }
  | {
      type: "form"
      title: string
      description?: string
      fields: FormField[]
      onSubmit: (values: Record<string, string>) => void
    }
  | { type: "info"; title: string; body: string }
  | {
      type: "confirm"
      title: string
      body: string
      confirmLabel: string
      confirmColor?: "primary" | "success" | "warning" | "error"
      secondaryLabel?: string
      cancelLabel?: string
      onConfirm: () => void
      onSecondary?: () => void
      onCancel?: () => void
    }
  | ({ type: "permission" } & PermissionRequest)

export const [dialog, setDialog] = createSignal<DialogState>({ type: "none" })
const [permissionQueue, setPermissionQueue] = createSignal<PermissionRequest[]>([])

function pumpPermissionQueue() {
  if (dialog().type !== "none") return
  const pending = permissionQueue()
  if (pending.length === 0) return
  const [next, ...rest] = pending
  setPermissionQueue(rest)
  if (next) setDialog({ type: "permission", ...next })
}

export function openPalette() {
  setDialog({ type: "palette" })
}

export function openSelect(input: {
  title: string
  options: SelectDialogOption[]
  onSelect: (value: string) => void
}) {
  setDialog({ type: "select", ...input })
}

export function openForm(input: {
  title: string
  description?: string
  fields: FormField[]
  onSubmit: (values: Record<string, string>) => void
}) {
  setDialog({ type: "form", ...input })
}

export function openModelDialog() {
  setDialog({ type: "model" })
}

export function openSessionsDialog() {
  setDialog({ type: "sessions" })
}

export function openProviderDialog() {
  setDialog({ type: "provider" })
}

export function openInfo(input: { title: string; body: string }) {
  setDialog({ type: "info", ...input })
}

export function openConfirm(input: {
  title: string
  body: string
  confirmLabel: string
  confirmColor?: "primary" | "success" | "warning" | "error"
  secondaryLabel?: string
  cancelLabel?: string
  onConfirm: () => void
  onSecondary?: () => void
  onCancel?: () => void
}) {
  setDialog({ type: "confirm", ...input })
}

export function openPermissionDialog(input: PermissionRequest) {
  setPermissionQueue((pending) => [...pending, input])
  pumpPermissionQueue()
}

export function closeDialog() {
  setDialog({ type: "none" })
  pumpPermissionQueue()
}

export function resetDialog() {
  setPermissionQueue([])
  setDialog({ type: "none" })
}

export function useDialog() {
  return {
    dialog,
    openPalette,
    openSelect,
    openForm,
    openInfo,
    openConfirm,
    openModelDialog,
    openProviderDialog,
    openSessionsDialog,
    openPermissionDialog,
    closeDialog,
  }
}
