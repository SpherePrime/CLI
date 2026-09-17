import { createSignal } from "solid-js"

export type ChatEntry = {
  id: string
  role: "user" | "assistant" | "error" | "system"
  text: string
}

let entryId = 0

export function nextEntryId(): string {
  return `entry-${++entryId}`
}

const [entries, setEntries] = createSignal<ChatEntry[]>([])
const [sessionId, setSessionId] = createSignal<string | undefined>()

export function useSession() {
  return {
    entries,
    setEntries,
    sessionId,
    setSessionId,
    addEntry(entry: ChatEntry) {
      setEntries((prev) => [...prev, entry])
    },
    updateEntry(id: string, update: Partial<ChatEntry>) {
      setEntries((prev) => prev.map((entry) => (entry.id === id ? { ...entry, ...update } : entry)))
    },
    appendEntry(id: string, chunk: string) {
      setEntries((prev) => prev.map((entry) => (entry.id === id ? { ...entry, text: entry.text + chunk } : entry)))
    },
  }
}

export function resetSession() {
  setEntries([])
}

export function transcriptText(): string {
  return entries()
    .map((entry) => `## ${entry.role}\n\n${entry.text}`)
    .join("\n\n")
}

export function lastAssistantText(): string | undefined {
  const all = entries()
  for (let index = all.length - 1; index >= 0; index--) {
    const entry = all[index]
    if (entry && entry.role === "assistant" && entry.text.trim()) return entry.text
  }
  return undefined
}
