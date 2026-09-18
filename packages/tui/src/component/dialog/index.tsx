import { createMemo } from "solid-js"
import { useDialog } from "../../context/dialog"
import { CommandPalette } from "./command-palette"
import { ModelDialog } from "./model-dialog"
import { ProviderDialog } from "./provider-dialog"
import { SessionsDialog } from "./sessions-dialog"
import { SelectDialog } from "./select-dialog"
import { FormDialog } from "./form-dialog"
import { InfoDialog } from "./info-dialog"
import { ConfirmDialog } from "./confirm-dialog"
import { PermissionDialog } from "./permission-dialog"
import { QuestionDialog } from "./question-dialog"
import type { AgentClient } from "../../client"

export function DialogHost(props: { client: AgentClient }) {
  const { dialog } = useDialog()

  const content = createMemo(() => {
    const state = dialog()
    switch (state.type) {
      case "palette":
        return <CommandPalette client={props.client} />
      case "model":
        return <ModelDialog client={props.client} />
      case "provider":
        return <ProviderDialog client={props.client} />
      case "sessions":
        return <SessionsDialog client={props.client} />
      case "select":
        return <SelectDialog state={state} />
      case "form":
        return <FormDialog state={state} />
      case "info":
        return <InfoDialog state={state} />
      case "confirm":
        return <ConfirmDialog state={state} />
      case "permission":
        return <PermissionDialog client={props.client} state={state} />
      case "question":
        return <QuestionDialog state={state} />
      default:
        return undefined
    }
  })

  return <>{content()}</>
}
