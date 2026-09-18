import { createSignal } from "solid-js"
import type { EngineEvent, PermissionMode, StoredMessage } from "../client"
import { reduceEvent, emptyTimeline, type ChatEntry, type TimelineState } from "./timeline"

export type { ChatEntry, ToolCallInfo, TimelineState } from "./timeline"
export { lastAssistantText, transcriptText } from "./timeline"

let entryId = 0

export function nextEntryId(): string {
  return `entry-${++entryId}`
}

const [timeline, setTimeline] = createSignal<TimelineState>(emptyTimeline())
const [sessionId, setSessionId] = createSignal<string | undefined>()
const [permissionMode, setPermissionMode] = createSignal<PermissionMode | undefined>(undefined)
const [permissionApplied, setPermissionApplied] = createSignal(false)

export function useSession() {
  return {
    entries: () => timeline().entries,
    plan: () => timeline().plan,
    tokens: () => timeline().totalTokens,
    sessionId,
    setSessionId,
    permissionMode,
    setPermissionMode,
    permissionApplied,
    setPermissionApplied,
    applyEvent(event: EngineEvent) {
      setTimeline((previous) => reduceEvent(previous, event))
    },
    seedFromTimeline(state: TimelineState) {
      setTimeline(state)
    },
    addEntry(entry: ChatEntry) {
      setTimeline((previous) => ({ ...previous, entries: [...previous.entries, entry] }))
    },
    updateEntry(id: string, update: (entry: ChatEntry) => ChatEntry) {
      setTimeline((previous) => ({
        ...previous,
        entries: previous.entries.map((entry) => (entry.id === id ? update(entry) : entry)),
      }))
    },
  }
}

export function resetSession() {
  setTimeline(emptyTimeline())
  setPermissionMode(undefined)
  setPermissionApplied(false)
}

export function loadStoredMessages(messages: StoredMessage[]) {
  const entries: ChatEntry[] = messages.map((message) => {
    const role =
      message.role === "tool" ? "tool" : message.role === "system" ? "system" : message.role === "assistant" ? "assistant" : "user"
    const text = message.content || ""
    return { id: nextEntryId(), role, text: text as string }
  })
  setTimeline((previous) => ({ ...previous, entries }))
}