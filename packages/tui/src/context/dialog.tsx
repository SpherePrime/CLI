import { createSignal } from "solid-js"

export type SelectDialogOption = {
  title: string
  value: string
  description?: string
}

export type FormField =
  | { kind: "text"; key: string; label: string; initial?: string; placeholder?: string; hint?: string }
  | { kind: "select"; key: string; label: string; options: { title: string; value: string }[]; initial?: string }

export type DialogState =
  | { type: "none" }
  | { type: "palette" }
  | { type: "model" }
  | { type: "provider" }
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
      type: "permission"
      sessionId?: string
      id: string
      tool: string
      scope: string
      target: string
      reason: string
    }

const [dialog, setDialog] = createSignal<DialogState>({ type: "none" })

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

export function openProviderDialog() {
  setDialog({ type: "provider" })
}

export function openInfo(input: { title: string; body: string }) {
  setDialog({ type: "info", ...input })
}

export function openPermissionDialog(input: {
  sessionId?: string
  id: string
  tool: string
  scope: string
  target: string
  reason: string
}) {
  setDialog({ type: "permission", ...input })
}

export function closeDialog() {
  setDialog({ type: "none" })
}

export function useDialog() {
  return {
    dialog,
    openPalette,
    openSelect,
    openForm,
    openInfo,
    openModelDialog,
    openProviderDialog,
    openPermissionDialog,
    closeDialog,
  }
}
