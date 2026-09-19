import { createMemo, createSignal, Show } from "solid-js"
import { useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import { theme } from "../../theme"
import { closeDialog, type DialogState } from "../../context/dialog"
import type { AgentClient } from "../../client"

type PermissionState = Extract<DialogState, { type: "permission" }>

const dim = RGBA.fromValues(0, 0, 0, 160)

type DecisionItem = {
  key: string
  label: string
  description: string
  decision: "allow" | "once" | "deny" | "reject"
  remember: boolean
  recommended?: boolean
}

const decisions: DecisionItem[] = [
  { key: "a", label: "allow", description: "permit this call", decision: "allow", remember: false, recommended: true },
  { key: "o", label: "allow once", description: "single use, next call will ask again", decision: "once", remember: false },
  { key: "w", label: "session", description: "remember for the whole session", decision: "allow", remember: true },
  { key: "d", label: "deny", description: "block this call", decision: "deny", remember: false },
]

export function PermissionDialog(props: { client: AgentClient; state: PermissionState }) {
  const dimensions = useTerminalDimensions()
  const [selected, setSelected] = createSignal(0)

  const width = createMemo(() => Math.min(90, Math.max(58, dimensions().width - 6)))
  const left = createMemo(() => Math.max(1, Math.floor((dimensions().width - width()) / 2)))

  async function decide(input: { decision: "allow" | "once" | "deny" | "reject"; remember?: boolean }) {
    closeDialog()
    try {
      await props.client.resolvePermission(
        props.state.sessionId,
        props.state.id,
        input.decision,
        input.remember,
      )
    } catch {}
  }

  useKeyboard((key) => {
    if (key.name === "escape") {
      key.preventDefault()
      void decide({ decision: "deny" })
      return
    }
    if (key.name === "right") {
      key.preventDefault()
      setSelected((index) => (index + 1) % decisions.length)
      return
    }
    if (key.name === "left") {
      key.preventDefault()
      setSelected((index) => (index - 1 + decisions.length) % decisions.length)
      return
    }
    if (key.name === "return" || key.name === "space") {
      key.preventDefault()
      const item = decisions[selected()]
      if (item) void decide({ decision: item.decision, remember: item.remember })
      return
    }
    for (const item of decisions) {
      if (key.name === item.key) {
        key.preventDefault()
        void decide({ decision: item.decision, remember: item.remember })
        return
      }
    }
  })

  return (
    <box position="absolute" top={0} left={0} width="100%" height="100%" backgroundColor={dim} zIndex={200}>
      <box
        position="absolute"
        top={2}
        left={left()}
        width={width()}
        backgroundColor={theme.backgroundPanel}
        border
        borderColor={theme.borderSubtle}
        flexDirection="column"
      >
        <box flexDirection="row" justifyContent="space-between" paddingLeft={2} paddingRight={2} paddingTop={1}>
          <text fg={theme.warning}>⚠ Permission required</text>
          <text fg={theme.textMuted}>esc = deny</text>
        </box>
        <box paddingLeft={2} paddingRight={2} paddingTop={1} flexDirection="column" gap={1}>
          <text fg={theme.text}>{props.state.tool}</text>
          <text fg={theme.textMuted}>{props.state.target}</text>
          <Show when={props.state.reason}>
            <text fg={theme.textMuted}>{props.state.reason}</text>
          </Show>
          <text fg={theme.dim}>(scope: {props.state.scope})</text>
        </box>
        <box paddingTop={1} flexDirection="column" gap={1}>
          {decisions.map((item, index) => (
            <box
              flexDirection="row"
              paddingLeft={2}
              paddingRight={2}
              backgroundColor={selected() === index ? theme.primary : undefined}
              onMouseDown={() => {
                setSelected(index)
                void decide({ decision: item.decision, remember: item.remember })
              }}
            >
              <text fg={selected() === index ? theme.background : theme.text} width={4}>{item.key} </text>
              <text fg={selected() === index ? theme.background : theme.text} width={12}>{item.label}</text>
              <text fg={selected() === index ? theme.background : theme.dim}>{item.description}</text>
              <Show when={item.recommended}>
                <text fg={selected() === index ? theme.background : theme.success}> ★ recommended</text>
              </Show>
            </box>
          ))}
        </box>
        <box paddingLeft={2} paddingRight={2} paddingBottom={1}>
          <text fg={theme.textMuted}>←→ move · enter confirm · a/o/w/d quick keys</text>
        </box>
      </box>
    </box>
  )
}
