import { createMemo, createSignal, For, Show } from "solid-js"
import { useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import { theme } from "../../theme"
import { closeDialog, type DialogState } from "../../context/dialog"

type QuestionState = Extract<DialogState, { type: "question" }>

const dim = RGBA.fromValues(0, 0, 0, 160)

export function QuestionDialog(props: { state: QuestionState }) {
  const dimensions = useTerminalDimensions()
  const hasOptions = createMemo(() => props.state.options.length > 0)
  const [selected, setSelected] = createSignal(0)
  const [text, setText] = createSignal("")

  const width = createMemo(() => Math.min(78, Math.max(50, dimensions().width - 6)))
  const height = createMemo(() => Math.min(dimensions().height - 4, props.state.options.length + 8))
  const top = createMemo(() => Math.max(1, Math.floor((dimensions().height - height()) / 2)))
  const left = createMemo(() => Math.max(1, Math.floor((dimensions().width - width()) / 2)))

  function finish(answer: string | undefined) {
    closeDialog()
    props.state.onAnswer(answer)
  }

  function choose() {
    const option = props.state.options[selected()]
    if (option) finish(option)
  }

  useKeyboard((key) => {
    if (key.name === "escape") {
      key.preventDefault()
      finish(undefined)
      return
    }
    if (hasOptions()) {
      if (key.name === "up") {
        key.preventDefault()
        setSelected((index) => (index - 1 + props.state.options.length) % props.state.options.length)
        return
      }
      if (key.name === "down") {
        key.preventDefault()
        setSelected((index) => (index + 1) % props.state.options.length)
        return
      }
      if (key.name === "return") {
        key.preventDefault()
        choose()
      }
      return
    }
    if (key.name === "return") {
      key.preventDefault()
      const value = text().trim()
      if (value) finish(value)
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
          <text fg={theme.primary}>Question</text>
          <text fg={theme.textMuted}>esc skip</text>
        </box>
        <box paddingLeft={2} paddingRight={2} paddingTop={1}>
          <text fg={theme.text}>{props.state.question}</text>
        </box>
        <Show
          when={hasOptions()}
          fallback={
            <box paddingLeft={2} paddingRight={2} paddingTop={1} paddingBottom={1}>
              <input
                width="100%"
                placeholder="Type your answer…"
                placeholderColor={theme.textMuted}
                textColor={theme.text}
                focusedTextColor={theme.text}
                backgroundColor={theme.backgroundElement}
                focusedBackgroundColor={theme.backgroundElement}
                focused
                onInput={(value: string) => setText(value)}
              />
            </box>
          }
        >
          <box flexDirection="column" paddingTop={1} paddingBottom={1}>
            <For each={props.state.options}>
              {(option, index) => (
                <box
                  flexDirection="row"
                  paddingLeft={2}
                  paddingRight={2}
                  height={1}
                  backgroundColor={selected() === index() ? theme.primary : undefined}
                  onMouseDown={() => {
                    setSelected(index())
                    finish(option)
                  }}
                >
                  <text fg={selected() === index() ? theme.background : theme.text}>{`${index() + 1}. ${option}`}</text>
                </box>
              )}
            </For>
          </box>
        </Show>
        <box paddingLeft={2} paddingRight={2} paddingBottom={1}>
          <text fg={theme.textMuted}>
            {hasOptions() ? "↑↓ move · enter confirm · mouse click" : "enter submit · esc skip"}
          </text>
        </box>
      </box>
    </box>
  )
}
