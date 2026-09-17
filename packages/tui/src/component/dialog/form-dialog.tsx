import { createMemo, createSignal, For, Show } from "solid-js"
import { createStore } from "solid-js/store"
import { useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import { theme } from "../../theme"
import { closeDialog, type DialogState, type FormField } from "../../context/dialog"

type FormState = Extract<DialogState, { type: "form" }>

const dim = RGBA.fromValues(0, 0, 0, 160)

function initialValues(fields: FormField[]): Record<string, string> {
  const values: Record<string, string> = {}
  for (const field of fields) {
    if (field.kind === "select") values[field.key] = field.initial ?? field.options[0]?.value ?? ""
    else values[field.key] = field.initial ?? ""
  }
  return values
}

export function FormDialog(props: { state: FormState }) {
  const dimensions = useTerminalDimensions()
  const fields = props.state.fields
  const [values, setValues] = createStore<Record<string, string>>(initialValues(fields))
  const [focus, setFocus] = createSignal(0)

  const width = createMemo(() => Math.min(78, Math.max(48, dimensions().width - 6)))
  const height = createMemo(() => Math.min(dimensions().height - 4, fields.length * 3 + 8))
  const top = createMemo(() => Math.max(1, Math.floor((dimensions().height - height()) / 2)))
  const left = createMemo(() => Math.max(1, Math.floor((dimensions().width - width()) / 2)))

  function selectTitle(field: Extract<FormField, { kind: "select" }>): string {
    const value = values[field.key]
    return field.options.find((option) => option.value === value)?.title ?? value ?? ""
  }

  function cycle(field: Extract<FormField, { kind: "select" }>, delta: number) {
    const index = field.options.findIndex((option) => option.value === values[field.key])
    const next = (index + delta + field.options.length) % field.options.length
    const option = field.options[next]
    if (option) setValues(field.key, option.value)
  }

  function submit() {
    const snapshot: Record<string, string> = {}
    for (const field of fields) snapshot[field.key] = values[field.key] ?? ""
    closeDialog()
    props.state.onSubmit(snapshot)
  }

  function moveFocus(delta: number) {
    const count = fields.length
    if (count === 0) return
    setFocus((prev) => (prev + delta + count) % count)
  }

  useKeyboard((key) => {
    if (key.name === "escape") {
      key.preventDefault()
      closeDialog()
      return
    }
    if (key.name === "tab" || key.name === "down") {
      key.preventDefault()
      moveFocus(key.shift ? -1 : 1)
      return
    }
    if (key.name === "up") {
      key.preventDefault()
      moveFocus(-1)
      return
    }
    if (key.name === "return") {
      key.preventDefault()
      if (focus() === fields.length - 1) submit()
      else moveFocus(1)
      return
    }
    const field = fields[focus()]
    if (field && field.kind === "select" && (key.name === "left" || key.name === "right")) {
      key.preventDefault()
      cycle(field, key.name === "right" ? 1 : -1)
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
        <box flexDirection="column" paddingLeft={2} paddingRight={2} paddingTop={1} gap={1}>
          <Show when={props.state.description}>
            <text fg={theme.textMuted}>{props.state.description ?? ""}</text>
          </Show>
          <For each={fields}>
            {(field, index) => (
              <box flexDirection="column">
                <text fg={focus() === index() ? theme.primary : theme.textMuted}>{field.label}</text>
                <Show when={field.kind === "text" && field.hint}>
                  <text fg={theme.textMuted}>{field.kind === "text" ? field.hint ?? "" : ""}</text>
                </Show>
                <Show
                  when={field.kind === "text"}
                  fallback={
                    <box
                      flexDirection="row"
                      gap={1}
                      backgroundColor={focus() === index() ? theme.backgroundElement : undefined}
                    >
                      <text fg={theme.textMuted}>◀</text>
                      <text fg={focus() === index() ? theme.primary : theme.text}>
                        {selectTitle(field as Extract<FormField, { kind: "select" }>)}
                      </text>
                      <text fg={theme.textMuted}>▶</text>
                    </box>
                  }
                >
                  <input
                    width="100%"
                    value={(field as Extract<FormField, { kind: "text" }>).initial ?? ""}
                    placeholder={(field as Extract<FormField, { kind: "text" }>).placeholder ?? ""}
                    placeholderColor={theme.textMuted}
                    textColor={theme.text}
                    focusedTextColor={theme.text}
                    backgroundColor={theme.backgroundElement}
                    focusedBackgroundColor={theme.backgroundElement}
                    focused={focus() === index()}
                    onInput={(value: string) => setValues(field.key, value)}
                  />
                </Show>
              </box>
            )}
          </For>
        </box>
        <box paddingLeft={2} paddingRight={2} paddingBottom={1}>
          <text fg={theme.textMuted}>tab next · enter submit · ◀▶ change</text>
        </box>
      </box>
    </box>
  )
}
