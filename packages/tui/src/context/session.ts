import { createSignal } from "solid-js"
import type { EngineEvent, PermissionMode, StoredMessage } from "../client"
import { reduceEvent, type ChatEntry, type TimelineState } from "./timeline"

export type { ChatEntry, ToolCallInfo, TimelineState } from "./timeline"
export { lastAssistantText, transcriptText } from "./timeline"

let entryId = 0

export function nextEntryId(): string {
  return `entry-${++entryId}`
}

const [entries, setEntries] = createSignal<ChatEntry[]>([])
const [lastSequence, setLastSequence] = createSignal(0)
const [sessionId, setSessionId] = createSignal<string | undefined>()
const [permissionMode, setPermissionMode] = createSignal<PermissionMode | undefined>(undefined)
const [permissionApplied, setPermissionApplied] = createSignal(false)

export function useSession() {
  return {
    entries,
    setEntries,
    sessionId,
    setSessionId,
    permissionMode,
    setPermissionMode,
    permissionApplied,
    setPermissionApplied,
    applyEvent(event: EngineEvent) {
      const timeline = reduceEvent(
        { lastSequence: lastSequence(), entries: entries() },
        event,
      )
      setLastSequence(timeline.lastSequence)
      setEntries(timeline.entries)
    },
    seedFromTimeline(state: TimelineState) {
      setLastSequence(state.lastSequence)
      setEntries(state.entries)
    },
    addEntry(entry: ChatEntry) {
      setEntries((prev) => [...prev, entry])
    },
  }
}

export function resetSession() {
  setEntries([])
  setLastSequence(0)
  setPermissionMode(undefined)
  setPermissionApplied(false)
}

export function loadStoredMessages(messages: StoredMessage[]) {
  setEntries(
    messages.map((message) => {
      const role =
        message.role === "tool" ? "tool" : message.role === "system" ? "system" : message.role === "assistant" ? "assistant" : "user"
      const text = message.content || ""
      return { id: nextEntryId(), role, text: text as string }
    }),
  )
}