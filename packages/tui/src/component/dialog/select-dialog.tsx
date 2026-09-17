import { createMemo, createSignal, For, onMount } from "solid-js"
import { useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import { theme } from "../../theme"
import { closeDialog, type DialogState } from "../../context/dialog"

type SelectState = Extract<DialogState, { type: "select" }>

const dim = RGBA.fromValues(0, 0, 0, 160)

export function SelectDialog(props: { state: SelectState }) {
  const dimensions = useTerminalDimensions()
  const [selected, setSelected] = createSignal(0)
  const [offset, setOffset] = createSignal(0)

  const options = createMemo(() => props.state.options)
  const maxRows = createMemo(() => Math.max(4, Math.min(12, dimensions().height - 8)))
  const width = createMemo(() => Math.min(78, Math.max(44, dimensions().width - 6)))
  const height = createMemo(() => maxRows() + 6)

  function ensureVisible(index: number) {
    const visible = maxRows()
    let next = offset()
    if (index < next) next = index
    if (index >= next + visible) next = index - visible + 1
    setOffset(Math.max(0, next))
  }

  function move(delta: number) {
    const count = options().length
    if (count === 0) return
    let next = selected() + delta
    if (next < 0) next = count - 1
    if (next >= count) next = 0
    setSelected(next)
    ensureVisible(next)
  }

  function choose() {
    const option = options()[selected()]
    if (!option) return
    closeDialog()
    props.state.onSelect(option.value)
  }

  useKeyboard((key) => {
    if (key.name === "escape") {
      key.preventDefault()
      closeDialog()
      return
    }
    if (key.name === "up") {
      key.preventDefault()
      move(-1)
      return
    }
    if (key.name === "down") {
      key.preventDefault()
      move(1)
      return
    }
    if (key.name === "return") {
      key.preventDefault()
      choose()
    }
  })

  onMount(() => setSelected(0))

  const window = createMemo(() => options().slice(offset(), offset() + maxRows()))
  const top = createMemo(() => Math.max(1, Math.floor((dimensions().height - height()) / 2)))
  const left = createMemo(() => Math.max(1, Math.floor((dimensions().width - width()) / 2)))

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
        <box flexDirection="column" paddingTop={1} paddingBottom={1}>
          <For each={window()}>
            {(option, index) => (
              <box
                flexDirection="row"
                justifyContent="space-between"
                paddingLeft={2}
                paddingRight={2}
                height={1}
                backgroundColor={selected() === offset() + index() ? theme.primary : undefined}
                onMouseDown={() => {
                  setSelected(offset() + index())
                  choose()
                }}
              >
                <text fg={selected() === offset() + index() ? theme.background : theme.text}>{option.title}</text>
                <text fg={selected() === offset() + index() ? theme.background : theme.textMuted}>
                  {option.description ?? ""}
                </text>
              </box>
            )}
          </For>
        </box>
      </box>
    </box>
  )
}
