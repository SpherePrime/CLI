import { createMemo, createSignal, For, Show } from "solid-js"
import { useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import { theme } from "../../theme"
import { closeDialog, type DialogState } from "../../context/dialog"

type ConfirmState = Extract<DialogState, { type: "confirm" }>

const dim = RGBA.fromValues(0, 0, 0, 160)

const colorFor = (color: ConfirmState["confirmColor"]): RGBA => {
  switch (color) {
    case "success":
      return theme.success
    case "warning":
      return theme.warning
    case "error":
      return theme.error
    default:
      return theme.primary
  }
}

export function ConfirmDialog(props: { state: ConfirmState }) {
  const dimensions = useTerminalDimensions()
  const [selected, setSelected] = createSignal(0)

  const width = createMemo(() => Math.min(78, Math.max(50, dimensions().width - 6)))
  const left = createMemo(() => Math.max(1, Math.floor((dimensions().width - width()) / 2)))

  const buttons = () =>
    [
      { label: props.state.confirmLabel, run: () => props.state.onConfirm() },
      ...(props.state.secondaryLabel
        ? [{ label: props.state.secondaryLabel, run: () => props.state.onSecondary?.() }]
        : []),
      ...(props.state.cancelLabel
        ? [{ label: props.state.cancelLabel, run: () => props.state.onCancel?.() }]
        : []),
    ].filter((button) => button.run)

  function choose(index: number) {
    closeDialog()
    const target = buttons()[index]
    target?.run()
  }

  useKeyboard((key) => {
    if (key.name === "escape") {
      key.preventDefault()
      closeDialog()
      props.state.onCancel?.()
      return
    }
    if (key.name === "right") {
      key.preventDefault()
      setSelected((index) => (index + 1) % buttons().length)
      return
    }
    if (key.name === "left") {
      key.preventDefault()
      setSelected((index) => (index - 1 + buttons().length) % buttons().length)
      return
    }
    if (key.name === "return" || key.name === "space") {
      key.preventDefault()
      choose(selected())
      return
    }
    if (key.name.toLowerCase() === "y") {
      key.preventDefault()
      choose(0)
      return
    }
    if (key.name.toLowerCase() === "n" && buttons().length > 1) {
      key.preventDefault()
      choose(buttons().length - 1)
      return
    }
  })

  return (
    <box position="absolute" top={0} left={0} width="100%" height="100%" backgroundColor={dim} zIndex={200}>
      <box
        position="absolute"
        top={4}
        left={left()}
        width={width()}
        backgroundColor={theme.backgroundPanel}
        border
        borderColor={theme.borderSubtle}
        flexDirection="column"
      >
        <box flexDirection="row" justifyContent="space-between" paddingLeft={2} paddingRight={2} paddingTop={1}>
          <text fg={colorFor(props.state.confirmColor)}>{props.state.title}</text>
          <text fg={theme.textMuted}>esc cancel</text>
        </box>
        <box paddingLeft={2} paddingRight={2} paddingTop={1} flexDirection="column" gap={1}>
          <text fg={theme.text}>{props.state.body}</text>
        </box>
        <box paddingTop={1} paddingBottom={1} flexDirection="row">
          <For each={buttons()}>
            {(button, index) => (
              <box
                flexGrow={1}
                paddingLeft={1}
                paddingRight={1}
                backgroundColor={selected() === index() ? colorFor(props.state.confirmColor) : undefined}
                onMouseDown={() => choose(index())}
              >
                <text fg={selected() === index() ? theme.background : theme.text}>{button.label}</text>
              </box>
            )}
          </For>
        </box>
        <Show when={props.state.secondaryLabel}>
          <box paddingLeft={2} paddingRight={2} paddingBottom={1}>
            <text fg={theme.textMuted}>←→ move · enter confirm · mouse click</text>
          </box>
        </Show>
      </box>
    </box>
  )
}