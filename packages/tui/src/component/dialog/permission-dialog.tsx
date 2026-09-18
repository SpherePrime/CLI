import { createMemo, createSignal, For, Show } from "solid-js"
import { useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import { theme } from "../../theme"
import { closeDialog, type DialogState } from "../../context/dialog"
import type { AgentClient } from "../../client"

type PermissionState = Extract<DialogState, { type: "permission" }>

const dim = RGBA.fromValues(0, 0, 0, 160)

const decisions = [
  { key: "a", label: "allow", decision: "allow" as const, remember: false },
  { key: "o", label: "allow once", decision: "once" as const, remember: false },
  { key: "w", label: "allow for session", decision: "allow" as const, remember: true },
  { key: "d", label: "deny", decision: "deny" as const, remember: false },
]

export function PermissionDialog(props: { client: AgentClient; state: PermissionState }) {
  const dimensions = useTerminalDimensions()
  const [selected, setSelected] = createSignal(decisions.length - 1)

  const width = createMemo(() => Math.min(78, Math.max(50, dimensions().width - 6)))

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

  const left = createMemo(() => Math.max(1, Math.floor((dimensions().width - width()) / 2)))

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
          <text fg={theme.primary}>Permission required</text>
          <text fg={theme.textMuted}>esc deny</text>
        </box>
        <box paddingLeft={2} paddingRight={2} paddingTop={1} flexDirection="column" gap={1}>
          <text fg={theme.text}>{props.state.tool}</text>
          <text fg={theme.textMuted}>{props.state.target}</text>
          <Show when={props.state.reason}>
            <text fg={theme.textMuted}>{props.state.reason}</text>
          </Show>
          <text fg={theme.textMuted}>({props.state.scope})</text>
        </box>
        <box paddingTop={1} paddingBottom={1} flexDirection="row">
          <For each={decisions}>
            {(item, index) => (
              <box
                flexGrow={1}
                paddingLeft={1}
                paddingRight={1}
                backgroundColor={selected() === index() ? theme.primary : undefined}
                onMouseDown={() => {
                  void decide({ decision: item.decision, remember: item.remember })
                }}
              >
                <text fg={selected() === index() ? theme.background : theme.text}>
                  {item.key}·{item.label}
                </text>
              </box>
            )}
          </For>
        </box>
        <box paddingLeft={2} paddingRight={2} paddingBottom={1}>
          <text fg={theme.textMuted}>←→ move · enter confirm · mouse click</text>
        </box>
      </box>
    </box>
  )
}