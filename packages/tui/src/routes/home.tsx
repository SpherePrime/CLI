import { createResource, Show } from "solid-js"
import { theme } from "../theme"
import type { AgentClient } from "../client"

const quickActions = [
  { label: "New session", key: "c" },
  { label: "Open file", key: "o" },
] as const

export function Home(props: { client: AgentClient }) {
  const [info] = createResource(() => props.client.info())

  return (
    <box width="100%" height="100%" flexDirection="column" justifyContent="center" alignItems="center">
      <box width="100%" justifyContent="center" flexDirection="row">
        <text fg={theme.primary}>agent</text>
      </box>
      <box paddingTop={1} width="100%" justifyContent="center" flexDirection="row">
        <Show when={info()} fallback={<text fg={theme.textMuted}>Loading…</text>}>
          <text fg={theme.textMuted}>
            {info()?.version} · {info()?.model.model}
          </text>
        </Show>
      </box>
      <box paddingTop={2} flexDirection="row" gap={2}>
        {quickActions.map((action) => (
          <box
            paddingLeft={1}
            paddingRight={1}
            backgroundColor={theme.backgroundElement}
            borderColor={theme.borderSubtle}
          >
            <text fg={theme.text}>
              {action.label} <text fg={theme.textMuted}>{action.key}</text>
            </text>
          </box>
        ))}
      </box>
    </box>
  )
}