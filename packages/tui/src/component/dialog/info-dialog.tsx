import { createMemo } from "solid-js"
import { useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import { theme } from "../../theme"
import { closeDialog, type DialogState } from "../../context/dialog"

type InfoState = Extract<DialogState, { type: "info" }>

const dim = RGBA.fromValues(0, 0, 0, 160)

export function InfoDialog(props: { state: InfoState }) {
  const dimensions = useTerminalDimensions()
  const lines = createMemo(() => props.state.body.split("\n"))
  const width = createMemo(() => Math.min(78, Math.max(44, dimensions().width - 6)))
  const height = createMemo(() => Math.min(dimensions().height - 4, lines().length + 7))
  const top = createMemo(() => Math.max(1, Math.floor((dimensions().height - height()) / 2)))
  const left = createMemo(() => Math.max(1, Math.floor((dimensions().width - width()) / 2)))

  useKeyboard((key) => {
    if (key.name === "escape" || key.name === "return") {
      key.preventDefault()
      closeDialog()
    }
  })

  return (
    <box position="absolute" top={0} left={0} width="100%" height="100%" backgroundColor={dim} zIndex={200}>
      <box
        position="absolute"
        top={top()}
        left={left()}
        width={width()}
        backgroundColor={theme.backgroundPanel}
        border
        borderColor={theme.borderSubtle}
        flexDirection="column"
      >
        <box flexDirection="row" justifyContent="space-between" paddingLeft={2} paddingRight={2} paddingTop={1}>
          <text fg={theme.text}>{props.state.title}</text>
          <text fg={theme.textMuted}>esc</text>
        </box>
        <box paddingLeft={2} paddingRight={2} paddingTop={1} flexDirection="column">
          {lines().map((line) => (
            <text fg={theme.text}>{line || " "}</text>
          ))}
        </box>
      </box>
    </box>
  )
}
