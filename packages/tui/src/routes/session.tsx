import { ScrollBoxRenderable } from "@opentui/core"
import { useRenderer, useKeyboard } from "@opentui/solid"
import { createSignal, createMemo, createEffect, Show, For } from "solid-js"
import { Prompt } from "../component/prompt/index"
import { Spinner } from "../component/spinner"
import { useRoute } from "../context/route"
import { useDialog } from "../context/dialog"
import { modelLabel, workspaceName } from "../context/model"
import {
  useSession,
  resetSession,
  loadStoredMessages,
  nextEntryId,
} from "../context/session"
import {
  timelineFromEvents,
  type ChatEntry,
} from "../context/timeline"
import { permissionColor, permissionLabel, openPermissionSwitcher } from "../context/permission"
import { theme } from "../theme"
import { EmptyBorder } from "../ui/border"
import type { AgentClient, EngineEvent, PermissionMode } from "../client"

const helpText = [
  "enter — send message",
  "ctrl+p — command palette",
  "f2 — permission mode",
  "esc — back to home / cancel",
  "ctrl+c — exit",
].join("\n")

export function Session(props: { client: AgentClient }) {
  const { route, navigate } = useRoute()
  const { dialog, openPermissionDialog, openInfo, openSelect, openForm } = useDialog()
  const renderer = useRenderer()
  const session = useSession()
  const [status, setStatus] = createSignal<"idle" | "running">("idle")
  const [loading, setLoading] = createSignal(false)
  const [loadError, setLoadError] = createSignal<string | undefined>(undefined)
  const [title, setTitle] = createSignal<string | undefined>(undefined)
  let scroll: ScrollBoxRenderable

  const routeSessionId = () => {
    const current = route()
    return current.type === "session" ? current.sessionId : undefined
  }
  const draft = () => {
    const current = route()
    return current.type === "session" ? current.draft : undefined
  }

  const modelLabelMemo = createMemo(() => modelLabel() ?? "no model")
  const workspaceNameMemo = createMemo(() => workspaceName() ?? "…")

  async function loadHistory(id: string) {
    if (session.entries().length > 0) return
    setLoading(true)
    setLoadError(undefined)
    try {
      const detail = await props.client.getSessionDetail(id)
      if (detail.id !== session.sessionId()) return
      resetSession()
      if (detail.timeline && detail.timeline.length > 0) {
        session.seedFromTimeline(timelineFromEvents(detail.timeline))
      } else {
        loadStoredMessages(detail.messages)
      }
      if (detail.interrupted) {
        session.addEntry({ id: nextEntryId(), role: "system", text: "session was interrupted" })
      }
      setTitle(detail.title)
    } catch (error) {
      if (session.sessionId() !== id) return
      session.addEntry({ id: nextEntryId(), role: "error", text: String(error) })
      setLoadError(String(error))
    } finally {
      setLoading(false)
    }
  }

  createEffect(() => {
    const id = routeSessionId()
    session.setSessionId(id)
    if (id) {
      void loadHistory(id)
      void loadPermissionMode(id)
    }
  })

  function scrollToBottom() {
    if (!scroll) return
    scroll.scrollTop = scroll.scrollHeight
  }

  function refreshTitleOnce() {
    const id = session.sessionId()
    if (!id) return
    setTimeout(() => {
      if (session.sessionId() !== id) return
      void props.client.getSession(id).then((info) => {
        if (session.sessionId() === id) setTitle(info.title)
      })
    }, 600)
  }

  async function loadPermissionMode(id: string) {
    try {
      const { mode } = await props.client.getPermissionMode(id)
      session.setPermissionMode(mode)
    } catch {
      session.setPermissionMode(undefined)
    }
  }

  createEffect(() => {
    session.entries()
    scrollToBottom()
  })

  async function cancelRunning() {
    if (status() !== "running") return
    await props.client.cancelMessage(session.sessionId())
    setStatus("idle")
  }

  function appendSystem(text: string) {
    session.addEntry({ id: nextEntryId(), role: "system", text })
  }

  async function sendPrompt(text: string, readonly?: boolean) {
    setStatus("running")
    try {
      await props.client.streamMessage(text, session.sessionId(), handleEvent, {
        readonly: readonly ?? false,
      })
      void refreshTitleOnce()
    } catch (error) {
      openInfo({ title: "Request failed", body: String(error) })
    }
    setStatus("idle")
    scrollToBottom()
  }

  async function handleProjectMissing(projectName: string, projectPath: string | null) {
    appendSystem(`Project folder not found (${projectName ?? "unknown project"}).`)
    setStatus("idle")
    openSelect({
      title: "Project folder not found",
      options: [
        { title: "Locate project", value: "locate", description: "Select the current location of the project folder" },
        { title: "Open read-only", value: "readonly", description: "Continue the session without the saved project" },
        { title: "Cancel", value: "cancel", description: "Abort the message" },
      ],
      onSelect: (value) => {
        void resumeProjectMissing(value, projectPath)
      },
    })
  }

  let pendingText = ""

  async function resumeProjectMissing(value: string, projectPath: string | null) {
    const id = session.sessionId()
    if (!id) return
    if (value === "cancel") return
    if (value === "locate") {
      openForm({
        title: "Locate project",
        description: "Enter the current path of the project folder",
        fields: [
          {
            kind: "text",
            key: "path",
            label: "Project path",
            initial: projectPath ?? "",
            placeholder: "C:\\path\\to\\project",
            hint: "The session will be attached to the git root of this folder",
          },
        ],
        onSubmit: (values) => {
          const raw = (values.path ?? "").trim()
          if (!raw) {
            openInfo({ title: "Locate project", body: "A project path is required." })
            return
          }
          void relocateProject(id, raw)
        },
      })
      return
    }
    if (pendingText) await sendPrompt(pendingText, true)
  }

  async function relocateProject(id: string, raw: string) {
    try {
      await props.client.locateSession(id, raw)
    } catch (error) {
      openInfo({ title: "Locate project", body: String(error) })
      return
    }
    appendSystem(`Project relocated to ${raw}`)
    if (pendingText) await sendPrompt(pendingText, false)
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
      case "/permission":
      case "/perm":
        openPermissionSwitcher(props.client)
        return true
      case "/models":
        appendSystem(modelLabelMemo())
        return true
      default:
        return false
    }
  }

  function handleEvent(event: EngineEvent) {
    switch (event.type) {
      case "session.created":
        session.setSessionId(event.session.id)
        void loadPermissionMode(event.session.id)
        break
      case "session.project_missing":
        void handleProjectMissing(event.session.project_name ?? "", event.project_path)
        break
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
      case "permission_mode_changed":
        session.setPermissionMode(event.mode as PermissionMode)
        break
      case "session_title_changed":
        setTitle(event.title)
        break
      case "error":
        appendSystem(event.message)
        break
      case "done":
        break
      default:
        break
    }
    session.applyEvent(event)
  }

  async function handleSubmit(prompt: { input: string }) {
    const text = prompt.input.trim()
    if (!text) return
    if (text.startsWith("/") && (await runSlashCommand(text))) return

    pendingText = text
    await sendPrompt(text)
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
    if (key.name.toLowerCase() === "f2") {
      key.preventDefault()
      openPermissionSwitcher(props.client)
    }
  })

  return (
    <box width="100%" flexDirection="column" flexGrow={1} minHeight={0}>
      <box flexDirection="row" justifyContent="space-between" paddingLeft={2} paddingRight={2} paddingTop={1}>
        <text fg={theme.textMuted}>{title() ?? modelLabelMemo()}</text>
        <box flexDirection="row" gap={2}>
          <text fg={permissionColor(session.permissionMode())}>
            {status() === "running" ? "running… · " : ""}mode:{permissionLabel(session.permissionMode())}
          </text>
          <text fg={theme.textMuted}>{workspaceNameMemo()}</text>
        </box>
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
              <Show
                when={!loading()}
                fallback={
                  <box paddingLeft={2} paddingTop={1}>
                    <text fg={theme.textMuted}>Loading session…</text>
                  </box>
                }
              >
                <Show
                  when={!loadError()}
                  fallback={
                    <box paddingLeft={2} paddingTop={1} flexDirection="column" gap={1}>
                      <text fg={theme.error}>{loadError()}</text>
                      <text fg={theme.textMuted}>Press ctrl+p for commands or esc to go back.</text>
                    </box>
                  }
                >
                  <box paddingLeft={2} paddingTop={1} flexDirection="column" gap={1}>
                    <text fg={theme.text}>Ask anything, or press ctrl+p for commands.</text>
                    <text fg={theme.textMuted}>esc back · f2 permission mode · ctrl+c exit</text>
                  </box>
                </Show>
              </Show>
            </Show>
          </scrollbox>
        </box>
      </box>
      <Prompt
        client={props.client}
        initialInput={draft()}
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
          <text fg={theme.secondary}>tool: {props.entry.tool?.name ?? "?"}</text>
        </Show>
        <Show when={props.entry.role === "error"}>
          <text fg={theme.error}>error:</text>
        </Show>
        <Show when={props.entry.role === "system"}>
          <text fg={theme.info}>system:</text>
        </Show>
        <Show when={props.entry.role === "assistant" && props.entry.text === "" && props.entry.running}>
          <box flexDirection="row" gap={1} paddingTop={1}>
            <Spinner />
            <text fg={theme.textMuted}>thinking…</text>
          </box>
        </Show>
        <Show when={props.entry.role === "assistant" && props.entry.reasoning}>
          <box paddingTop={1}>
            <text fg={theme.textMuted}>
              thinking:
              {"\n"}
              {props.entry.reasoning!.slice(-1200)}
            </text>
          </box>
        </Show>
        <Show when={props.entry.role === "tool"}>
          <ToolBody entry={props.entry} />
        </Show>
        <Show when={props.entry.role !== "tool"}>
          <box paddingTop={1}>
            <Show when={props.entry.role === "system" && props.entry.running}>
              <box flexDirection="row" gap={1}>
                <Spinner />
                <text fg={theme.textMuted}>{props.entry.text}</text>
              </box>
            </Show>
            <Show when={props.entry.text}>
              <text fg={props.entry.role === "error" ? theme.error : theme.text}>{props.entry.text}</text>
            </Show>
          </box>
        </Show>
      </box>
    </box>
  )
}

function ToolBody(props: { entry: ChatEntry }) {
  const tool = props.entry.tool
  if (!tool) return <text fg={theme.text}>{props.entry.text}</text>
  return (
    <box flexDirection="column" gap={1} paddingTop={1}>
      <box flexDirection="row" gap={1}>
        <Show when={tool.state === "running"}>
          <Spinner />
        </Show>
        <Show when={tool.state === "ok"}>
          <text fg={theme.success}>ok</text>
        </Show>
        <Show when={tool.state === "failed"}>
          <text fg={theme.error}>failed</text>
        </Show>
        <text fg={theme.textMuted}>{tool.durationMs === undefined ? "running…" : `${tool.durationMs}ms`}</text>
        <Show when={tool.exitCode !== undefined && tool.exitCode !== null}>
          <text fg={tool.exitCode === 0 ? theme.success : theme.error}>exit {tool.exitCode}</text>
        </Show>
        <Show when={tool.truncated}>
          <text fg={theme.textMuted}>(truncated)</text>
        </Show>
      </box>
      <Show when={tool.args}>
        <text fg={theme.textMuted}>{tool.args}</text>
      </Show>
      <Show when={props.entry.text}>
        <text fg={theme.text}>{props.entry.text}</text>
      </Show>
      <Show when={tool.details}>
        <text fg={theme.textMuted}>{tool.details}</text>
      </Show>
      <Show when={tool.fileChanges}>
        <box flexDirection="column">
          <text fg={theme.info}>changed:</text>
          <text fg={theme.textMuted}>{tool.fileChanges}</text>
        </box>
      </Show>
    </box>
  )
}