import type { EngineEvent, FileChange } from "../client"

export type ToolCallInfo = {
  name: string
  args: string
  state: "running" | "ok" | "failed"
  durationMs?: number
  summary?: string
  details?: string
  exitCode?: number | null
  truncated?: boolean
  fileChanges: string
}

export type ChatEntry = {
  id: string
  role: "user" | "assistant" | "tool" | "error" | "system"
  text: string
  reasoning?: string
  tool?: ToolCallInfo
  running?: boolean
}

export type TimelineState = {
  lastSequence: number
  entries: ChatEntry[]
}

export function emptyTimeline(): TimelineState {
  return { lastSequence: 0, entries: [] }
}

function entryIndex(entries: ChatEntry[], id: string): number {
  return entries.findIndex((entry) => entry.id === id)
}

function upsert(entries: ChatEntry[], entry: ChatEntry): ChatEntry[] {
  const index = entryIndex(entries, entry.id)
  if (index === -1) return [...entries, entry]
  const next = [...entries]
  next[index] = entry
  return next
}

function appendText(entries: ChatEntry[], id: string, chunk: string): ChatEntry[] {
  const index = entryIndex(entries, id)
  if (index === -1) return [...entries, { id, role: "assistant", text: chunk }]
  const next = [...entries]
  const current = next[index]!
  next[index] = { ...current, text: current.text + chunk }
  return next
}

function appendReasoning(entries: ChatEntry[], id: string, chunk: string): ChatEntry[] {
  const index = entryIndex(entries, id)
  const next = [...entries]
  if (index === -1) {
    next.push({ id, role: "assistant", text: "", reasoning: chunk })
    return next
  }
  const current = next[index]!
  next[index] = { ...current, reasoning: (current.reasoning ?? "") + chunk }
  return next
}

function finishReasoning(entries: ChatEntry[], id: string): ChatEntry[] {
  const index = entryIndex(entries, id)
  if (index === -1) return entries
  const next = [...entries]
  next[index] = { ...next[index]!, running: false }
  return next
}

function toolArgsPreview(args: unknown): string {
  const raw = JSON.stringify(args)
  if (!raw) return "{}"
  return raw.length > 240 ? `${raw.slice(0, 240)}…` : raw
}

function fileChangesText(changes: FileChange[]): string {
  return changes
    .map((change) => {
      const ops = [change.additions, change.deletions].filter((n) => typeof n === "number" && n > 0).join("/")
      const stats = ops ? ` (+${ops})` : ""
      return `${change.change} ${change.path}${stats}`
    })
    .join("\n")
}

function upsertTool(entries: ChatEntry[], entry: ChatEntry): ChatEntry[] {
  return upsert(entries, entry)
}

export function reduceEvent(state: TimelineState, event: EngineEvent): TimelineState {
  if ("meta" in event && event.meta) {
    if (event.meta.sequence <= state.lastSequence) return state
  }

  let next = state
  const sequence = "meta" in event && event.meta ? event.meta.sequence : state.lastSequence

  switch (event.type) {
    case "message.created": {
      if (event.message.role !== "user") return state
      const text = event.message.content ?? ""
      const id = `user-${state.entries.length}`
      return { lastSequence: sequence, entries: [...state.entries, { id, role: "user", text }] }
    }
    case "assistant_message_started": {
      const entry: ChatEntry = { id: event.meta.item_id, role: "assistant", text: "" }
      return { lastSequence: sequence, entries: [...state.entries, entry] }
    }
    case "text_delta": {
      return { lastSequence: sequence, entries: appendText(state.entries, event.meta.item_id, event.text) }
    }
    case "assistant_message_completed": {
      const index = entryIndex(state.entries, event.meta.item_id)
      if (index === -1) return { ...state, lastSequence: sequence }
      const entries = [...state.entries]
      entries[index] = { ...entries[index]!, running: false }
      return { lastSequence: sequence, entries }
    }
    case "reasoning_started": {
      return { lastSequence: sequence, entries: state.entries }
    }
    case "reasoning_delta": {
      return { lastSequence: sequence, entries: appendReasoning(state.entries, event.meta.item_id, event.text) }
    }
    case "reasoning_completed": {
      return { lastSequence: sequence, entries: finishReasoning(state.entries, event.meta.item_id) }
    }
    case "activity_changed": {
      const entry: ChatEntry = {
        id: event.meta.item_id,
        role: "system",
        text: event.activity,
        running: true,
      }
      return { lastSequence: sequence, entries: upsertTool(state.entries, entry) }
    }
    case "tool_call_started": {
      const entry: ChatEntry = {
        id: event.id,
        role: "tool",
        text: `run ${event.name}(${toolArgsPreview(event.args)})`,
        tool: { name: event.name, args: toolArgsPreview(event.args), state: "running", fileChanges: "" },
        running: true,
      }
      return { lastSequence: sequence, entries: upsertTool(state.entries, entry) }
    }
    case "tool_call_delta": {
      return { ...state, lastSequence: sequence }
    }
    case "tool_call_completed": {
      const index = entryIndex(state.entries, event.id)
      if (index === -1) return { ...state, lastSequence: sequence }
      const entries = [...state.entries]
      entries[index] = {
        ...entries[index]!,
        tool: { ...entries[index]!.tool!, state: "running" },
      }
      return { lastSequence: sequence, entries }
    }
    case "tool_result": {
      const summary = event.summary ?? event.content
      const details = event.details
      const fileChanges = fileChangesText(event.file_changes)
      const previous = state.entries.find((entry) => entry.id === event.id)
      const text = summary
      const entry: ChatEntry = {
        id: event.id,
        role: "tool",
        text,
        running: false,
        tool: {
          name: event.name,
          args: previous?.tool?.args ?? "{}",
          state: event.ok ? "ok" : "failed",
          durationMs: event.duration_ms,
          summary: summary ?? undefined,
          details: details ?? undefined,
          exitCode: event.exit_code ?? null,
          truncated: event.truncated,
          fileChanges,
        },
      }
      return { lastSequence: sequence, entries: upsertTool(state.entries, entry) }
    }
    case "turn_cancelled": {
      const text = event.reason ? `turn cancelled: ${event.reason}` : "turn cancelled"
      return {
        lastSequence: sequence,
        entries: [...state.entries, { id: `cancel-${sequence}`, role: "system", text }],
      }
    }
    case "permission_mode_changed": {
      return { ...state, lastSequence: sequence }
    }
    case "error": {
      if (state.entries.some((entry) => entry.role === "error" && entry.id === `error-${sequence}`)) {
        return { ...state, lastSequence: sequence }
      }
      const entry: ChatEntry = { id: `error-${sequence}`, role: "error", text: event.message }
      return { lastSequence: sequence, entries: [...state.entries, entry] }
    }
    case "usage":
    case "finished":
    case "permission_requested":
    case "permission_resolved":
    case "session_title_changed":
    case "started":
    case "turn_started":
    case "turn_completed":
    case "session.created":
    case "session.project_missing":
    case "done":
      return { ...state, lastSequence: sequence }
  }
}

export function timelineFromEvents(events: EngineEvent[]): TimelineState {
  let state = emptyTimeline()
  for (const event of events) {
    state = reduceEvent(state, event)
  }
  return state
}

export function lastAssistantText(entries: ChatEntry[]): string | undefined {
  for (let index = entries.length - 1; index >= 0; index--) {
    const entry = entries[index]
    if (entry && entry.role === "assistant" && entry.text.trim()) return entry.text
  }
  return undefined
}

export function transcriptText(entries: ChatEntry[]): string {
  return entries
    .map((entry) => {
      if (entry.role === "user") return `## user\n\n${entry.text}`
      if (entry.role === "tool" && entry.tool) {
        const parts = [
          `## tool:${entry.tool.name}`,
          entry.tool.args && `args: ${entry.tool.args}`,
          entry.text,
          entry.tool.fileChanges && `changed:\n${entry.tool.fileChanges}`,
        ].filter((part): part is string => Boolean(part))
        return parts.join("\n\n")
      }
      const reasoning = entry.reasoning ? `## reasoning\n\n${entry.reasoning}\n\n` : ""
      return `## ${entry.role}\n\n${reasoning}${entry.text}`
    })
    .join("\n\n")
}