import { ScrollBoxRenderable } from "@opentui/core"
import { useRenderer, useKeyboard } from "@opentui/solid"
import { createSignal, createResource, createMemo, createEffect, onCleanup, For, Show } from "solid-js"
import { Prompt, type PromptRef } from "../component/prompt/index"
import { useRoute } from "../context/route"
import { useDialog } from "../context/dialog"
import { modelRevision } from "../context/app"
import {
  useSession,
  nextEntryId,
  resetSession,
  loadStoredMessages,
  toolEntryId,
  type ChatEntry,
} from "../context/session"
import { theme } from "../theme"
import { EmptyBorder } from "../ui/border"
import type { AgentClient, SessionMessage } from "../client"

const helpText = [
  "enter — send message",
  "ctrl+p — command palette",
  "esc — back to home / cancel",
  "ctrl+c — exit",
].join("\n")

export function Session(props: { client: AgentClient }) {
  const { route, navigate } = useRoute()
  const { dialog, openPermissionDialog, openInfo } = useDialog()
  const renderer = useRenderer()
  const session = useSession()
  const [status, setStatus] = createSignal<"idle" | "running">("idle")
  const [info] = createResource(() => (modelRevision(), props.client.info()))
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

  const modelLabel = createMemo(() => {
    const value = info()
    if (!value) return ""
    return `${value.model.model} · ${value.model.provider}`
  })

  async function loadHistory(id: string) {
    if (session.entries().length > 0) return
    try {
      const detail = await props.client.getSessionDetail(id)
      if (detail.id !== session.sessionId()) return
      loadStoredMessages(detail.messages)
    } catch (error) {
      session.addEntry({ id: nextEntryId(), role: "error", text: String(error) })
    }
  }

  createEffect(() => {
    const id = routeSessionId()
    session.setSessionId(id)
    if (id) void loadHistory(id)
  })

  function scrollToBottom() {
    if (!scroll) return
    scroll.scrollTop = scroll.scrollHeight
  }

  createEffect(() => {
    session.entries()
    scrollToBottom()
  })

  async function cancelRunning() {
    const id = session.sessionId()
    if (status() !== "running") return
    try {
      await props.client.cancelMessage(id)
    } catch {}
  }

  useKeyboard((key) => {
    if (dialog().type !== "none") return
    if (key.name === "escape") {
      key.preventDefault()
      if (status() === "running") {
        void cancelRunning()
      } else {
        navigate({ type: "home" })
      }
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
      case "/cancel":
        await cancelRunning()
        return true
      case "/models":
        appendSystem(modelLabel() || "loading…")
        return true
      default:
        return false
    }
  }

  function handleEvent(assistantId: string, event: SessionMessage) {
    switch (event.type) {
      case "session.created":
        session.setSessionId(event.session.id)
        break
      case "text_delta":
        session.appendEntry(assistantId, event.text)
        break
      case "tool_call": {
        const entryId = toolEntryId(event.id)
        const args = JSON.stringify(event.args) ?? ""
        session.addEntry({
          id: entryId,
          role: "tool",
          text: `run ${event.name}(${args.length > 200 ? `${args.slice(0, 200)}…` : args})`,
        })
        break
      }
      case "tool_result": {
        const entryId = toolEntryId(event.id)
        const body = event.ok ? event.content || "(no output)" : (event.error ?? event.content)
        session.updateEntry(entryId, {
          role: "tool",
          text: `→ ${event.name} in ${event.ms}ms${event.ok ? "" : " (failed)"}\n${body}`,
        })
        break
      }
      case "permission_requested":
        openPermissionDialog({
          sessionId: session.sessionId(),
          id: event.id,
          tool: event.tool,
          scope: event.scope,
          target: event.target,
          reason: event.reason,
        })
        break
      case "error":
        session.updateEntry(assistantId, { role: "error", text: event.message })
        appendSystem(event.message)
        break
      case "permission_resolved":
      case "usage":
        break
      case "done":
        break
      default:
        break
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
      await props.client.streamMessage(text, session.sessionId(), (event) => handleEvent(assistantId, event))
    } catch (error) {
      openInfo({ title: "Request failed", body: String(error) })
    }

    setStatus("idle")
    scrollToBottom()
  }

  return (
    <box width="100%" flexDirection="column" flexGrow={1} minHeight={0}>
      <box flexDirection="row" justifyContent="space-between" paddingLeft={2} paddingRight={2} paddingTop={1}>
        <text fg={theme.textMuted}>{modelLabel()}</text>
        <text fg={status() === "running" ? theme.primary : theme.textMuted}>
          {status() === "running" ? "running…" : "idle"}
        </text>
      </box>
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
        <Show when={props.entry.role === "tool"}>
          <text fg={theme.info}>tool:</text>
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