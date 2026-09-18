import { Show } from "solid-js"
import { theme } from "../../theme"
import { Spinner } from "../spinner"
import type { ChatEntry } from "../../context/session"

function previewLine(text: string, max: number): string {
  const firstLine = text.split("\n").find((line) => line.trim())
  if (!firstLine) return "…"
  const trimmed = firstLine.trim()
  return trimmed.length > max ? `${trimmed.slice(0, max)}…` : trimmed
}

export function ReasoningBlock(props: { entry: ChatEntry; onToggle: () => void }) {
  const reasoning = props.entry.reasoning ?? ""
  if (props.entry.reasoningOpen) {
    return (
      <box flexDirection="column" paddingTop={1}>
        <box flexDirection="row" gap={1}>
          <Spinner />
          <text fg={theme.textMuted}>reasoning…</text>
        </box>
        <text fg={theme.textMuted}>{reasoning}</text>
      </box>
    )
  }
  const expanded = props.entry.expandedReasoning === true
  return (
    <box flexDirection="column" paddingTop={1}>
      <box flexDirection="row" gap={1} onMouseDown={() => props.onToggle()}>
        <text fg={theme.info}>{expanded ? "▼" : "▶"} thinking</text>
        {expanded && <text fg={theme.textMuted}>· {reasoning.length} chars</text>}
        <Show when={!expanded}>
          <text fg={theme.textMuted}> · click to expand</text>
        </Show>
      </box>
      <Show when={expanded}>
        <text fg={theme.textMuted}>{reasoning}</text>
      </Show>
      <Show when={!expanded}>
        <text fg={theme.textMuted}>{previewLine(reasoning, 140)}</text>
      </Show>
    </box>
  )
}