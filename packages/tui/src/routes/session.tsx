import { ScrollBoxRenderable } from "@opentui/core"
import { createSignal, createEffect, onCleanup, For, Show } from "solid-js"
import { Prompt, type PromptRef } from "../component/prompt/index"
import { useRoute } from "../context/route"
import { theme } from "../theme"
import { EmptyBorder } from "../ui/border"
import type { AgentClient, SessionMessage } from "../client"

export type ChatEntry = {
  id: string
  role: "user" | "assistant" | "error"
  text: string
}

let entryId = 0
function nextEntryId() {
  return `entry-${++entryId}`
}

export function Session(props: { client: AgentClient }) {
  const { route } = useRoute()
  const current = route()
  const sessionId = current.type === "session" ? current.sessionId : undefined
  const [entries, setEntries] = createSignal<ChatEntry[]>([])
  const [status, setStatus] = createSignal<"idle" | "running">("idle")
  let scroll: ScrollBoxRenderable
  let promptRef: PromptRef | undefined

  function scrollToBottom() {
    if (!scroll) return
    scroll.scrollTop = scroll.scrollHeight
  }

  createEffect(() => {
    entries()
    scrollToBottom()
  })

  onCleanup(() => {
    promptRef = undefined
  })

  async function handleSubmit(prompt: { input: string }) {
    const text = prompt.input.trim()
    if (!text) return

    setEntries((prev) => [...prev, { id: nextEntryId(), role: "user", text }])
    setStatus("running")

    let assistantId = nextEntryId()
    setEntries((prev) => [...prev, { id: assistantId, role: "assistant", text: "" }])

    try {
      await props.client.streamMessage(text, sessionId, (event: SessionMessage) => {
        if (event.type === "message.part.updated" && event.part?.text) {
          setEntries((prev) =>
            prev.map((entry) =>
              entry.id === assistantId ? { ...entry, text: entry.text + event.part!.text! } : entry,
            ),
          )
        } else if (event.type === "session.error" && event.error) {
          setEntries((prev) =>
            prev.map((entry) => (entry.id === assistantId ? { ...entry, role: "error", text: event.error! } : entry)),
          )
        }
      })
    } catch (error) {
      setEntries((prev) =>
        prev.map((entry) =>
          entry.id === assistantId ? { ...entry, role: "error", text: String(error) } : entry,
        ),
      )
    }

    setStatus("idle")
    scrollToBottom()
  }

  return (
    <box width="100%" flexDirection="column" flexGrow={1} minHeight={0}>
      <box flexDirection="row" flexGrow={1} minHeight={0}>
        <box flexGrow={1} minHeight={0} paddingBottom={1} paddingLeft={2} paddingRight={2} gap={1}>
          <scrollbox
            ref={(r: ScrollBoxRenderable) => (scroll = r)}
            backgroundColor={theme.backgroundElement}
            flexGrow={1}
            minHeight={0}
          >
            <Show when={entries().length === 0} fallback={<For each={entries()}>{(entry) => <MessageRow entry={entry} />}</For>}>
              <box height={1} />
            </Show>
          </scrollbox>
        </box>
      </box>
      <Prompt
        client={props.client}
        ref={(ref) => {
          promptRef = ref
        }}
        onSubmit={handleSubmit}
        disabled={status() === "running" ? false : false}
        placeholder={status() === "idle" ? "Ask anything…" : "Agent is thinking…"}
      />
    </box>
  )
}

function MessageRow(props: { entry: ChatEntry }) {
  return (
    <box
      borderColor={theme.backgroundPanel}
      border={["left"]}
      customBorderChars={{ ...EmptyBorder, vertical: props.entry.role === "user" ? "│" : " " }}
    >
      <box paddingLeft={2} paddingTop={1} paddingRight={1}>
        <Show when={props.entry.role === "user"}>
          <text fg={theme.primary}>You:</text>
        </Show>
        <Show when={props.entry.role === "assistant"}>
          <text fg={theme.textMuted}>agent:</text>
        </Show>
        <Show when={props.entry.role === "error"}>
          <text fg={theme.error}>error:</text>
        </Show>
        <box paddingTop={1}>
          <text fg={props.entry.role === "error" ? theme.error : theme.text}>{props.entry.text}</text>
        </box>
      </box>
    </box>
  )
}