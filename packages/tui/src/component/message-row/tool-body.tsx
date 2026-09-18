import { For, Show } from "solid-js"
import { theme } from "../../theme"
import { Spinner } from "../spinner"
import type { ChatEntry } from "../../context/session"
import type { ToolState } from "../../context/timeline"
import { diffLines, fileChangesSummary, toolStatusLabel } from "../../util/tool-text"

function statusColor(state: ToolState) {
  switch (state) {
    case "ok":
      return theme.success
    case "failed":
    case "timed_out":
      return theme.error
    case "denied":
      return theme.warning
    case "queued":
    case "running":
    case "waiting_permission":
    case "cancelled":
      return theme.textMuted
  }
}

function active(state: ToolState): boolean {
  return state === "queued" || state === "running" || state === "waiting_permission"
}

export function ToolBody(props: { entry: ChatEntry; onToggle: () => void }) {
  const tool = props.entry.tool
  if (!tool) return <text fg={theme.text}>{props.entry.text}</text>

  const expanded = tool.expanded === true

  return (
    <box flexDirection="column" gap={1} paddingTop={1}>
      <box flexDirection="row" gap={1} onMouseDown={() => props.onToggle()}>
        <Show when={active(tool.state)}>
          <Spinner />
        </Show>
        <text fg={statusColor(tool.state)}>{toolStatusLabel(tool.state)}</text>
        <Show when={!props.entry.text || props.entry.text === tool.summary}>
          <text fg={theme.text}>{tool.name}</text>
        </Show>
        <text fg={theme.text}>{props.entry.text !== tool.summary ? props.entry.text : ""}</text>
        <Show when={tool.durationMs !== undefined}>
          <text fg={theme.textMuted}>{tool.durationMs}ms</text>
        </Show>
        <Show when={tool.exitCode !== undefined && tool.exitCode !== null}>
          <text fg={tool.exitCode === 0 ? theme.success : theme.error}>exit {tool.exitCode}</text>
        </Show>
        <Show when={tool.truncated}>
          <text fg={theme.textMuted}>(truncated)</text>
        </Show>
      </box>
      <Show when={tool.args}>
        <text fg={theme.textMuted}>{argsLabel(tool.args)}</text>
      </Show>
      <Show when={tool.summary}>
        <text fg={expanded ? theme.textMuted : theme.text}>{tool.summary}</text>
      </Show>
      <Show when={tool.details}>
        <text fg={theme.textMuted}>{tool.details}</text>
      </Show>
      <Show when={expanded && tool.fileChanges.length > 0}>
        <box flexDirection="column" gap={1}>
          <text fg={theme.info}>changed:</text>
          <For each={tool.fileChanges}>
            {(change) => (
              <box flexDirection="column" gap={1}>
                <text fg={theme.textMuted}>{fileChangesSummary([change])}</text>
                <Show when={change.diff}>
                  <DiffText diff={change.diff ?? null} />
                </Show>
              </box>
            )}
          </For>
        </box>
      </Show>
    </box>
  )
}

function argsLabel(args: string): string {
  if (args === "{}") return ""
  return `args: ${args}`
}

function DiffText(props: { diff: string | null }) {
  const lines = () => diffLines(props.diff)
  return (
    <box flexDirection="column" paddingLeft={2}>
      <For each={lines()}>
        {(line) => (
          <text fg={line.sign === "+" ? theme.success : line.sign === "-" ? theme.error : theme.textMuted}>
            {line.sign + line.line.slice(1)}
          </text>
        )}
      </For>
    </box>
  )
}