import { ScrollBoxRenderable } from "@opentui/core"
import { useRenderer, useKeyboard } from "@opentui/solid"
import { createSignal, createEffect, onCleanup, For, Show } from "solid-js"
import { Prompt, type PromptRef } from "../component/prompt/index"
import { useRoute } from "../context/route"
import { useDialog } from "../context/dialog"
import { useSession, nextEntryId, resetSession, type ChatEntry } from "../context/session"
import { theme } from "../theme"
import { EmptyBorder } from "../ui/border"
import type { AgentClient, SessionMessage } from "../client"

const helpText = [
  "enter — send message",
  "ctrl+p — command palette",
  "esc — back to home",
  "ctrl+c — exit",
].join("\n")

export function Session(props: { client: AgentClient }) {
  const { route, navigate } = useRoute()
  const { dialog } = useDialog()
  const renderer = useRenderer()
  const session = useSession()
  const [status, setStatus] = createSignal<"idle" | "running">("idle")
  let scroll: ScrollBoxRenderable
  let promptRef: PromptRef | undefined

  const routeSessionId = () => {
    const current = route()
    return current.type === "session" ? current.sessionId : undefined
  }
  const draft = () => {
    const current = route()
    return current.type === "session" ? current.draft : undefined
  }

  createEffect(() => {
    const id = routeSessionId()
    session.setSessionId(id)
  })

  function scrollToBottom() {
    if (!scroll) return
    scroll.scrollTop = scroll.scrollHeight
  }

  createEffect(() => {
    session.entries()
    scrollToBottom()
  })

  useKeyboard((key) => {
    if (dialog().type !== "none") return
    if (key.name === "escape") {
      key.preventDefault()
      navigate({ type: "home" })
    }
  })

  onCleanup(() => {
    promptRef = undefined
  })

  function appendSystem(text: string) {
    session.addEntry({ id: nextEntryId(), role: "system", text })
  }

  async function runSlashCommand(text: string): Promise<boolean> {
    const command = text.trim().split(/\s+/)[0]
    switch (command) {
      case "/exit":
      case "/quit":
        renderer.destroy()
        return true
      case "/clear":
        resetSession()
        return true
      case "/new":
        navigate({ type: "session" })
        resetSession()
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

    session.addEntry({ id: nextEntryId(), role: "user", text })
    setStatus("running")

    const assistantId = nextEntryId()
    session.addEntry({ id: assistantId, role: "assistant", text: "" })

    try {
      await props.client.streamMessage(text, session.sessionId(), (event: SessionMessage) => {
        if (event.type === "session.created" && event.session?.id) {
          session.setSessionId(event.session.id)
        } else if (event.type === "message.part.updated" && event.part?.text) {
          session.appendEntry(assistantId, event.part.text)
        } else if (event.type === "session.error" && event.error) {
          session.updateEntry(assistantId, { role: "error", text: event.error })
        }
      })
    } catch (error) {
      session.updateEntry(assistantId, { role: "error", text: String(error) })
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
              when={session.entries().length === 0}
              fallback={<For each={session.entries()}>{(entry) => <MessageRow entry={entry} />}</For>}
            >
              <box paddingLeft={2} paddingTop={1} flexDirection="column" gap={1}>
                <text fg={theme.text}>Ask anything, or press ctrl+p for commands.</text>
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
