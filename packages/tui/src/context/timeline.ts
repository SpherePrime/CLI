import type { EngineEvent, FileChange, PlanStep } from "../client"
import {
  argsPreview,
  deriveToolState,
  describeToolAction,
  fileChangesSummary,
} from "../util/tool-text"

export type ToolState =
  | "queued"
  | "running"
  | "waiting_permission"
  | "ok"
  | "failed"
  | "denied"
  | "cancelled"
  | "timed_out"

export type ToolCallInfo = {
  name: string
  args: string
  state: ToolState
  durationMs?: number
  summary?: string
  details?: string
  exitCode?: number | null
  truncated?: boolean
  fileChanges: FileChange[]
  expanded?: boolean
}

export type ChatEntry = {
  id: string
  role: "user" | "assistant" | "tool" | "error" | "system"
  kind?: "activity"
  text: string
  reasoning?: string
  reasoningOpen?: boolean
  expandedReasoning?: boolean
  tool?: ToolCallInfo
  running?: boolean
  stage?: string
}

export type TimelineState = {
  lastSequence: number
  entries: ChatEntry[]
  lastActivity?: string
  toolPreps: Record<number, { id?: string; name?: string; args: string }>
  plan: PlanStep[]
}

export function emptyTimeline(): TimelineState {
  return { lastSequence: 0, entries: [], lastActivity: undefined, toolPreps: {}, plan: [] }
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

function removeEntry(entries: ChatEntry[], id: string): ChatEntry[] {
  return entries.filter((entry) => entry.id !== id)
}

function updateMatch(entries: ChatEntry[], predicate: (entry: ChatEntry) => boolean, update: (entry: ChatEntry) => ChatEntry): ChatEntry[] {
  return entries.map((entry) => (predicate(entry) ? update(entry) : entry))
}

function appendText(entries: ChatEntry[], id: string, chunk: string, sequenceStart: boolean): ChatEntry[] {
  const index = entryIndex(entries, id)
  if (index === -1) {
    return [...entries, { id, role: "assistant", text: chunk, running: true, expandedReasoning: false }]
  }
  const next = [...entries]
  const current = next[index]!
  next[index] = {
    ...current,
    text: current.text + chunk,
    running: current.running ?? sequenceStart,
  }
  return next
}

function appendReasoning(entries: ChatEntry[], id: string, chunk: string): ChatEntry[] {
  const index = entryIndex(entries, id)
  const next = [...entries]
  if (index === -1) {
    next.push({
      id,
      role: "assistant",
      text: "",
      reasoning: chunk,
      reasoningOpen: true,
      running: true,
      expandedReasoning: false,
    })
    return next
  }
  const current = next[index]!
  next[index] = {
    ...current,
    reasoning: (current.reasoning ?? "") + chunk,
    reasoningOpen: true,
  }
  return next
}

function toolArgRecord(args: unknown): Record<string, unknown> {
  if (args && typeof args === "object") return args as Record<string, unknown>
  return {}
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
      return { ...state, lastSequence: sequence, entries: [...state.entries, { id, role: "user", text }] }
    }
    case "assistant_message_started": {
      const entry: ChatEntry = {
        id: event.meta.item_id,
        role: "assistant",
        text: "",
        running: true,
        expandedReasoning: false,
      }
      return { ...state, lastSequence: sequence, entries: upsert(state.entries, entry) }
    }
    case "text_delta": {
      const entries = appendText(state.entries, event.meta.item_id, event.text, false)
      return { ...state, lastSequence: sequence, entries }
    }
    case "assistant_message_completed": {
      const index = entryIndex(state.entries, event.meta.item_id)
      if (index === -1) return { ...state, lastSequence: sequence }
      const entries = [...state.entries]
      entries[index] = { ...entries[index]!, running: false }
      return { ...state, lastSequence: sequence, entries: removeEntry(entries, `activity_${event.meta.item_id}`) }
    }
    case "reasoning_started": {
      const id = event.meta.item_id
      const entries = updateMatch(state.entries, (entry) => entry.id === id && entry.role === "assistant", (entry) => ({
        ...entry,
        reasoningOpen: true,
      }))
      return { ...state, lastSequence: sequence, entries: removeEntry(entries, `activity_${id}`) }
    }
    case "reasoning_delta": {
      return { ...state, lastSequence: sequence, entries: appendReasoning(state.entries, event.meta.item_id, event.text) }
    }
    case "reasoning_completed": {
      const index = entryIndex(state.entries, event.meta.item_id)
      if (index === -1) return { ...state, lastSequence: sequence }
      const entries = [...state.entries]
      entries[index] = { ...entries[index]!, reasoningOpen: false, expandedReasoning: false }
      return { ...state, lastSequence: sequence, entries }
    }
    case "activity_changed": {
      const activityEntry: ChatEntry = {
        id: event.meta.item_id,
        role: "system",
        kind: "activity",
        text: event.activity,
        running: true,
      }
      const entries = upsert(state.entries, activityEntry)
      const staged = updateMatch(entries, (entry) => entry.role === "assistant" && !!entry.running && !entry.reasoningOpen, (entry) => ({
        ...entry,
        stage: event.activity,
      }))
      return { ...state, lastSequence: sequence, entries: staged, lastActivity: event.activity }
    }
    case "tool_call_delta": {
      const preps = { ...state.toolPreps }
      const current = preps[event.index] ?? { args: "" }
      const args = current.args + (event.args_delta ?? "")
      preps[event.index] = { id: event.id ?? current.id, name: event.name ?? current.name, args }
      let entries = state.entries
      if (event.id && entryIndex(entries, event.id) === -1) {
        const tool: ToolCallInfo = {
          name: event.name ?? `tool ${event.index}`,
          args: argsPreview(args),
          state: "queued",
          fileChanges: [],
        }
        entries = [...entries, { id: event.id, role: "tool", text: "", tool, running: true }]
      } else if (event.id) {
        entries = updateMatch(entries, (entry) => entry.id === event.id && entry.role === "tool", (entry) => {
          const info = entry.tool!
          return { ...entry, tool: { ...info, args: argsPreview(args) } }
        })
      }
      return { ...state, lastSequence: sequence, entries, toolPreps: preps }
    }
    case "tool_call_started": {
      const args = toolArgRecord(event.args)
      const tool: ToolCallInfo = {
        name: event.name,
        args: argsPreview(event.args),
        state: "running",
        fileChanges: [],
      }
      const index = entryIndex(state.entries, event.id)
      if (index !== -1) {
        const entries = [...state.entries]
        const existing = entries[index]!
        entries[index] = {
          ...existing,
          text: describeToolAction(event.name, args),
          tool: { ...existing.tool!, ...tool },
        }
        return { ...state, lastSequence: sequence, entries }
      }
      const entry: ChatEntry = {
        id: event.id,
        role: "tool",
        text: describeToolAction(event.name, args),
        tool,
        running: true,
      }
      return { ...state, lastSequence: sequence, entries: upsert(state.entries, entry) }
    }
    case "tool_call_completed": {
      return { ...state, lastSequence: sequence }
    }
    case "tool_result": {
      const summary = event.summary ?? event.content
      const previous = state.entries.find((entry) => entry.id === event.id)
      const state_ = deriveToolState(event.ok, event.error, summary)
      const action = previous?.text || summary || "tool"
      const entry: ChatEntry = {
        id: event.id,
        role: "tool",
        text: action,
        running: false,
        tool: {
          name: event.name,
          args: previous?.tool?.args ?? "{}",
          state: state_,
          durationMs: event.duration_ms,
          summary: summary ?? undefined,
          details: event.details ?? undefined,
          exitCode: event.exit_code ?? null,
          truncated: event.truncated,
          fileChanges: event.file_changes ?? [],
          expanded: previous?.tool?.expanded,
        },
      }
      const entries = upsert(removeEntry(state.entries, `tool_${event.id}`), entry)
      return { ...state, lastSequence: sequence, entries }
    }
    case "permission_requested": {
      const entries = updateMatch(state.entries, (entry) => {
        if (entry.role !== "tool" || !entry.tool) return false
        return entry.tool.name === event.tool && (entry.tool.state === "running" || entry.tool.state === "queued")
      }, (entry) => ({
        ...entry,
        tool: { ...entry.tool!, state: "waiting_permission" as ToolState },
      }))
      if (entries.length === state.entries.length) {
        // no running tool matched; mark the latest running tool instead
        for (let i = entries.length - 1; i >= 0; i--) {
          const entry = entries[i]
          if (entry?.role === "tool" && entry.tool && entry.tool.state === "running") {
            entries[i] = { ...entry, tool: { ...entry.tool, state: "waiting_permission" as ToolState } }
            break
          }
        }
      }
      return { ...state, lastSequence: sequence, entries }
    }
    case "permission_resolved": {
      const entries = updateMatch(state.entries, (entry) => entry.role === "tool" && entry.tool?.state === "waiting_permission", (entry) => ({
        ...entry,
        tool: { ...entry.tool!, state: "running" as ToolState },
      }))
      return { ...state, lastSequence: sequence, entries }
    }
    case "turn_cancelled": {
      const text = event.reason ? `turn cancelled: ${event.reason}` : "turn cancelled"
      const entries = updateMatch(state.entries, (entry) => entry.running === true, (entry) => {
        if (entry.role === "tool" && entry.tool) {
          return { ...entry, running: false, tool: { ...entry.tool, state: "cancelled" as ToolState } }
        }
        return { ...entry, running: false }
      })
      return {
        ...state,
        lastSequence: sequence,
        entries: [...entries.filter((entry) => entry.kind !== "activity"), { id: `cancel-${sequence}`, role: "system", text }],
      }
    }
    case "plan_updated":
      return { ...state, lastSequence: sequence, plan: event.steps }
    case "permission_mode_changed":
    case "usage":
    case "finished":
    case "started":
    case "turn_started":
    case "turn_completed":
    case "session_title_changed":
    case "session.created":
    case "session.project_missing":
    case "done":
      return { ...state, lastSequence: sequence }
    case "error": {
      let entries = state.entries
      const itemId = event.meta?.item_id
      if (itemId) {
        entries = updateMatch(entries, (entry) => entry.id === itemId, (entry) => {
          if (entry.role === "tool" && entry.tool) {
            return { ...entry, running: false, tool: { ...entry.tool, state: "failed" as ToolState } }
          }
          return { ...entry, running: false }
        })
        entries = removeEntry(entries, `activity_${itemId}`)
      }
      if (entries.some((entry) => entry.role === "error" && entry.id === `error-${sequence}`)) {
        return { ...state, lastSequence: sequence, entries }
      }
      const entry: ChatEntry = { id: `error-${sequence}`, role: "error", text: event.message }
      return { ...state, lastSequence: sequence, entries: [...entries, entry] }
    }
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
    .filter((entry) => entry.kind !== "activity")
    .map((entry) => {
      if (entry.role === "user") return `## user\n\n${entry.text}`
      if (entry.role === "tool" && entry.tool) {
        const parts = [
          `## tool:${entry.tool.name}`,
          entry.tool.args && `args: ${entry.tool.args}`,
          entry.text,
          entry.tool.fileChanges.length > 0 && `changed:\n${fileChangesSummary(entry.tool.fileChanges)}`,
        ].filter((part): part is string => Boolean(part))
        return parts.join("\n\n")
      }
      const reasoning = entry.reasoning ? `## reasoning\n\n${entry.reasoning}\n\n` : ""
      return `## ${entry.role}\n\n${reasoning}${entry.text}`
    })
    .join("\n\n")
}