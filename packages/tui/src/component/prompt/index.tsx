import { createSignal, createEffect, onCleanup, type JSX } from "solid-js"
import { createStore } from "solid-js/store"
import type { TextareaRenderable, BoxRenderable } from "@opentui/core"
import { EmptyBorder, SplitBorder } from "../../ui/border"
import { theme } from "../../theme"
import { useDialog } from "../../context/dialog"
import { modelLabel } from "../../context/model"
import { Autocomplete, type AutocompleteRef } from "./autocomplete"
import type { AgentClient } from "../../client"

export type PromptInfo = {
  input: string
}

export type PromptRef = {
  focused: boolean
  current: PromptInfo
  focus(): void
  blur(): void
  reset(): void
  submit(): void
}

export function Prompt(props: {
  client: AgentClient
  sessionID?: string
  initialInput?: string
  onSubmit?: (prompt: { input: string }) => void
  ref?: (ref: PromptRef | undefined) => void
  placeholder?: string
  disabled?: boolean
}) {
  let input: TextareaRenderable
  let anchor: BoxRenderable
  const { dialog } = useDialog()
  const [inputTarget, setInputTarget] = createSignal<TextareaRenderable | undefined>()
  let auto: AutocompleteRef | undefined
  const [store, setStore] = createStore<{ prompt: PromptInfo }>({ prompt: { input: "" } })

  const placeholder = props.placeholder ?? "Ask anything…"

  const promptCommands = {
    submit: async () => {
      if (props.disabled) return
      if (!store.prompt.input.trim()) return
      if (auto?.visible) return
      const { input: text } = store.prompt
      setStore("prompt", { input: "" })
      input.clear()
      props.onSubmit?.({ input: text })
    },
    clear: () => {
      input.clear()
      setStore("prompt", { input: "" })
    },
    interrupt: () => {
      if (store.prompt.input.trim().length === 0) {
        setStore("prompt", { input: "" })
        input.clear()
      }
    },
  }

  createEffect(() => {
    if (!input || input.isDestroyed) return
    const blocked = props.disabled || dialog().type !== "none"
    if (blocked) {
      input.blur()
      return
    }
    if (!input.focused) input.focus()
  })

  createEffect(() => {
    const target = inputTarget()
    if (!target || target.isDestroyed) return
    if (!props.initialInput) return
    if (store.prompt.input.length > 0) return
    target.setText(props.initialInput)
    setStore("prompt", "input", props.initialInput)
  })

  const ref: PromptRef = {
    get focused() {
      return input.focused
    },
    get current() {
      return store.prompt
    },
    focus() {
      input.focus()
    },
    blur() {
      input.blur()
    },
    reset() {
      input.clear()
      setStore("prompt", { input: "" })
    },
    submit() {
      void promptCommands.submit()
    },
  }

  createEffect(() => props.ref?.(ref))

  onCleanup(() => {
    setInputTarget(undefined)
    props.ref?.(undefined)
  })

  return (
    <>
      <box ref={(r: BoxRenderable) => (anchor = r)} visible={props.disabled !== true} width="100%">
        <box
          width="100%"
          border={["left"]}
          borderColor={theme.borderActive}
          customBorderChars={{
            ...SplitBorder.customBorderChars,
            bottomLeft: "╹",
          }}
        >
          <box
            paddingLeft={2}
            paddingRight={2}
            paddingTop={1}
            flexShrink={0}
            backgroundColor={theme.backgroundElement}
            flexGrow={1}
            width="100%"
          >
            <textarea
              ref={(r: TextareaRenderable) => {
                input = r
                setInputTarget(r)
              }}
              width="100%"
              placeholder={placeholder}
              placeholderColor={theme.textMuted}
              textColor={theme.text}
              focusedTextColor={theme.text}
              minHeight={1}
              maxHeight={Math.max(6, Math.floor(40 / 3))}
              keyBindings={[
                { name: "return", action: "submit" },
                { name: "return", shift: true, action: "newline" },
                { name: "linefeed", action: "newline" },
              ]}
              onContentChange={() => {
                const value = input.plainText
                setStore("prompt", "input", value)
                auto?.onInput?.(value)
              }}
              onCursorChange={() => {
                auto?.onInput?.(input.plainText)
              }}
              onSubmit={() => {
                void promptCommands.submit()
              }}
              cursorColor={theme.text}
              cursorStyle={{ style: "line" }}
              onKeyDown={(e) => {
                if (props.disabled) e.preventDefault()
              }}
            />
            <box flexDirection="row" flexShrink={0} paddingTop={1} gap={1} justifyContent="space-between">
              <box flexDirection="row" gap={1}>
                <text fg={theme.textMuted}>agent</text>
                <text fg={theme.textMuted}>·</text>
                <text fg={theme.text}>{modelLabel() ?? "no model"}</text>
              </box>
              <box flexDirection="row" gap={1}>
                <text fg={theme.textMuted}>enter send · shift+enter newline · ctrl+p commands</text>
              </box>
            </box>
          </box>
        </box>
        <box
          height={1}
          border={["left"]}
          borderColor={theme.borderActive}
          customBorderChars={{
            ...EmptyBorder,
            vertical: "╹",
          }}
        >
          <box
            height={1}
            border={["bottom"]}
            borderColor={theme.backgroundElement}
            customBorderChars={{ ...EmptyBorder, horizontal: " " }}
          />
        </box>
        <Autocomplete
          client={props.client}
          value={store.prompt.input}
          setPrompt={(update) => {
            const draft = { input: store.prompt.input }
            update(draft)
            setStore("prompt", draft)
          }}
          anchor={() => anchor}
          input={() => input}
          ref={(r) => {
            auto = r
          }}
        />
      </box>
    </>
  )
}