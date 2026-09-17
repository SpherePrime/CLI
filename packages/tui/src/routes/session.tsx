import { ScrollBoxRenderable } from "@opentui/core"
import { useRenderer, useKeyboard } from "@opentui/solid"
import { createSignal, createEffect, onCleanup, For, Show } from "solid-js"
import { Prompt, type PromptRef } from "../component/prompt/index"
import { useRoute } from "../context/route"
import { theme } from "../theme"
import { EmptyBorder } from "../ui/border"
import type { AgentClient, SessionMessage } from "../client"

export type ChatEntry = {
  id: string
  role: "user" | "assistant" | "error" | "system"
  text: string
}

const helpText = [
  "/new — start a new session",
  "/clear — clear the conversation",
  "/models — show the current model",
  "/help — show this help",
  "/exit — quit the app",
].join("\n")

let entryId = 0
function nextEntryId() {
  return `entry-${++entryId}`
}

export function Session(props: { client: AgentClient }) {
  const { route, navigate } = useRoute()
  const renderer = useRenderer()
  const [entries, setEntries] = createSignal<ChatEntry[]>([])
  const [status, setStatus] = createSignal<"idle" | "running">("idle")
  let scroll: ScrollBoxRenderable
  let promptRef: PromptRef | undefined

  const sessionId = () => {
    const current = route()
    return current.type === "session" ? current.sessionId : undefined
  }
  const draft = () => {
    const current = route()
    return current.type === "session" ? current.draft : undefined
  }

  function scrollToBottom() {
    if (!scroll) return
    scroll.scrollTop = scroll.scrollHeight
  }

  createEffect(() => {
    entries()
    scrollToBottom()
  })

  useKeyboard((key) => {
    if (key.name === "escape") {
      navigate({ type: "home" })
    }
  })

  onCleanup(() => {
    promptRef = undefined
  })

  function appendSystem(text: string) {
    setEntries((prev) => [...prev, { id: nextEntryId(), role: "system", text }])
  }

  async function runSlashCommand(text: string): Promise<boolean> {
    const command = text.trim().split(/\s+/)[0]
    switch (command) {
      case "/exit":
      case "/quit":
        renderer.destroy()
        return true
      case "/clear":
        setEntries([])
        return true
      case "/new":
        navigate({ type: "session" })
        setEntries([])
        return true
      case "/help":
        appendSystem(helpText)
        return true
      case "/models":
        try {
          const info = await props.client.info()
          appendSystem(`model: ${info.model.model} · provider: ${info.model.provider}`)
        } catch (error) {
          appendSystem(`failed to load model: ${String(error)}`)
        }
        return true
      default:
        return false
    }
  }

  async function handleSubmit(prompt: { input: string }) {
    const text = prompt.input.trim()
    if (!text) return
    if (text.startsWith("/") && (await runSlashCommand(text))) return

    setEntries((prev) => [...prev, { id: nextEntryId(), role: "user", text }])
    setStatus("running")

    const assistantId = nextEntryId()
    setEntries((prev) => [...prev, { id: assistantId, role: "assistant", text: "" }])

    try {
      await props.client.streamMessage(text, sessionId(), (event: SessionMessage) => {
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
            <Show
              when={entries().length === 0}
              fallback={<For each={entries()}>{(entry) => <MessageRow entry={entry} />}</For>}
            >
              <box paddingLeft={2} paddingTop={1} flexDirection="column" gap={1}>
                <text fg={theme.text}>Ask anything, or type /help for commands.</text>
                <text fg={theme.textMuted}>esc back · ctrl+c exit</text>
              </box>
            </Show>
          </scrollbox>
        </box>
      </box>
      <Prompt
        client={props.client}
        initialInput={draft()}
        ref={(ref) => {
          promptRef = ref
        }}
        onSubmit={handleSubmit}
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
      <box paddingLeft={2} paddingTop={1} paddingRight={1} flexDirection="column">
        <Show when={props.entry.role === "user"}>
          <text fg={theme.primary}>You:</text>
        </Show>
        <Show when={props.entry.role === "assistant"}>
          <text fg={theme.textMuted}>agent:</text>
        </Show>
        <Show when={props.entry.role === "error"}>
          <text fg={theme.error}>error:</text>
        </Show>
        <Show when={props.entry.role === "system"}>
          <text fg={theme.info}>system:</text>
        </Show>
        <box paddingTop={1}>
          <text fg={props.entry.role === "error" ? theme.error : theme.text}>{props.entry.text}</text>
        </box>
      </box>
    </box>
  )
}
