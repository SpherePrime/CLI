import { Show } from "solid-js"
import { theme } from "../../theme"
import { EmptyBorder } from "../../ui/border"
import { Spinner } from "../spinner"
import { MarkdownBlock } from "../markdown"
import type { ChatEntry } from "../../context/session"
import { ReasoningBlock } from "./reasoning"
import { ToolBody } from "./tool-body"

export type MessageRowProps = {
  entry: ChatEntry
  onToggleReasoning: (id: string) => void
  onToggleTool: (id: string) => void
  focused?: boolean
}

function assistantStatusText(entry: ChatEntry): string {
  if (entry.reasoningOpen) return "thinking…"
  if (entry.reasoning && !entry.text.trim()) return "writing…"
  if (entry.text.trim()) return "writing…"
  return entry.stage ?? "thinking…"
}

export function MessageRow(props: MessageRowProps) {
  const entry = props.entry
  const focused = props.focused ?? false
  if (entry.kind === "activity") return null

  if (entry.role === "user") {
    return (
      <box
        backgroundColor={theme.backgroundPanel}
        paddingLeft={1}
        paddingRight={1}
        paddingTop={1}
        paddingBottom={1}
        border={["left"]}
        borderColor={focused ? theme.activeBorder : theme.primary}
        customBorderChars={{ ...EmptyBorder, vertical: "│" }}
      >
        <text fg={theme.primary}>you</text>
        <text> </text>
        <text>{entry.text}</text>
      </box>
    )
  }

  return (
    <box
      paddingLeft={1}
      paddingRight={1}
      paddingTop={1}
      flexDirection="column"
      border={focused ? ["left"] : undefined}
      borderColor={focused ? theme.activeBorder : undefined}
      backgroundColor={focused ? theme.activeBg : undefined}
      customBorderChars={focused ? { ...EmptyBorder, vertical: "│" } : undefined}
    >
      <Show when={entry.role === "assistant" && entry.running && !entry.reasoningOpen}>
        <box flexDirection="row" gap={1}>
          <Spinner />
          <text fg={theme.textMuted}>{assistantStatusText(entry)}</text>
        </box>
      </Show>
      <Show when={entry.reasoning}>
        <ReasoningBlock entry={entry} onToggle={() => props.onToggleReasoning(entry.id)} />
      </Show>
      <Show when={entry.role === "tool"}>
        <ToolBody entry={entry} onToggle={() => props.onToggleTool(entry.id)} />
      </Show>
      <Show when={entry.role === "assistant"}>
        <text fg={theme.textMuted}>agent</text>
        <Show when={entry.text && !entry.running}>
          <MarkdownBlock text={entry.text!} />
        </Show>
        <Show when={entry.text && entry.running}>
          <text>{entry.text}</text>
        </Show>
      </Show>
      <Show when={entry.role === "error"}>
        <text fg={theme.error}>{entry.text}</text>
      </Show>
      <Show when={entry.role === "system" && !entry.running}>
        <text fg={theme.info}>{entry.text}</text>
      </Show>
      <Show when={entry.role === "system" && entry.running}>
        <box flexDirection="row" gap={1}>
          <Spinner />
          <text fg={theme.textMuted}>{entry.text}</text>
        </box>
      </Show>
    </box>
  )
}