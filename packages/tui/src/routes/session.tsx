import { ScrollBoxRenderable } from "@opentui/core"
import { useRenderer, useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { createSignal, createMemo, createEffect, onCleanup, Show, For } from "solid-js"
import { Prompt } from "../component/prompt/index"
import { useRoute } from "../context/route"
import { useDialog } from "../context/dialog"
import { modelLabel, workspaceName, gitBranch } from "../context/model"
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
import { Spinner } from "../component/spinner"
import { MessageRow } from "../component/message-row"
import { PlanPanel } from "../component/plan-panel"
import { contentLayout } from "../util/content-layout"
import type { AgentClient, EngineEvent, PermissionMode } from "../client"

function permissionBadge(mode: PermissionMode | undefined, running: boolean): string {
  const prefix = running ? "running… · " : ""
  if (mode === "full_access") return `${prefix}!! FULL ACCESS !!`
  return `${prefix}mode:${permissionLabel(mode)}`
}

const helpText = [
  "enter — send message",
  "ctrl+p — command palette",
  "f2 — permission mode",
  "esc — back to home / cancel",
  "ctrl+c — exit",
  "click on a tool row to expand args/output",
  "click on ▶ thinking to expand the final reasoning",
].join("\n")

export function Session(props: { client: AgentClient }) {
  const { route, navigate } = useRoute()
  const { dialog, openPermissionDialog, openInfo, openSelect, openForm, openQuestion } = useDialog()
  const renderer = useRenderer()
  const session = useSession()
  const dimensions = useTerminalDimensions()
  const [status, setStatus] = createSignal<"idle" | "running">("idle")
  const [loading, setLoading] = createSignal(false)
  const [loadError, setLoadError] = createSignal<string | undefined>(undefined)
  const [title, setTitle] = createSignal<string | undefined>(undefined)
  const [newUpdates, setNewUpdates] = createSignal(0)
  let scroll: ScrollBoxRenderable
  let lastEntryCount = 0

  const sidebar = createMemo(() => {
    const layout = contentLayout(dimensions().width ?? 80)
    return { left: layout.sidePad, right: layout.sidePad, maxContent: layout.maxContent }
  })

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
  const gitBranchMemo = createMemo(() => {
    const branch = gitBranch()
    return branch ? `[${branch}]` : undefined
  })

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

  function isAtBottom(): boolean {
    if (!scroll) return true
    const tolerance = 32
    return scroll.scrollTop >= scroll.scrollHeight - scroll.height - tolerance
  }

  function jumpToBottom() {
    scrollToBottom()
    setNewUpdates(0)
  }

  createEffect(() => {
    const entries = session.entries()
    const count = entries.length
    if (count > lastEntryCount && !isAtBottom()) {
      setNewUpdates((previous) => previous + (count - lastEntryCount))
    } else if (count <= lastEntryCount && isAtBottom()) {
      setNewUpdates(0)
    }
    lastEntryCount = count
  })

  createEffect(() => {
    const timer = setInterval(() => {
      if (isAtBottom()) setNewUpdates(0)
    }, 400)
    onCleanup(() => clearInterval(timer))
  })

  async function loadPermissionMode(id: string) {
    try {
      const { mode } = await props.client.getPermissionMode(id)
      session.setPermissionMode(mode)
    } catch {
      session.setPermissionMode(undefined)
    }
  }

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
    } catch (error) {
      openInfo({ title: "Request failed", body: String(error) })
    }
    setStatus("idle")
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
      case "question_asked":
        openQuestion({
          question: event.question,
          options: event.options ?? [],
          onAnswer: (answer) => {
            void props.client.answerQuestion(session.sessionId(), event.id, answer).catch(() => {})
          },
        })
        break
      case "question_answered":
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

  function toggleReasoning(id: string) {
    session.updateEntry(id, (entry) => ({ ...entry, expandedReasoning: !entry.expandedReasoning }))
  }

  function toggleTool(id: string) {
    session.updateEntry(id, (entry) =>
      entry.tool ? { ...entry, tool: { ...entry.tool, expanded: !entry.tool.expanded } } : entry,
    )
  }

  const [focusIndex, setFocusIndex] = createSignal(-1)

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
    if (key.name === "arrowdown") {
      key.preventDefault()
      const entries = session.entries()
      setFocusIndex((prev) => Math.min(entries.length - 1, prev + 1))
    }
    if (key.name === "arrowup") {
      key.preventDefault()
      setFocusIndex((prev) => Math.max(-1, prev - 1))
    }
    if (key.name === "enter" && focusIndex() >= 0) {
      const entries = session.entries()
      const entry = entries[focusIndex()]
      if (entry?.tool) toggleTool(entry.id)
      else if (entry?.reasoning) toggleReasoning(entry.id)
    }
  })

  const sessionIdShort = createMemo(() => {
    const id = session.sessionId()
    if (!id) return undefined
    return id.length > 8 ? `${id.slice(0, 6)}…` : id
  })

  return (
    <box width="100%" flexDirection="column" flexGrow={1} minHeight={0}>
      <box
        flexDirection="row"
        justifyContent="space-between"
        paddingLeft={sidebar().left}
        paddingRight={sidebar().right}
        paddingTop={1}
        paddingBottom={1}
        border={["bottom"]}
        borderColor={theme.borderSubtle}
      >
        <box flexDirection="row" alignItems="center" gap={2} flexGrow={1} overflow="hidden">
          <Show when={sessionIdShort()}>
            <text fg={theme.dim}>{sessionIdShort()}</text>
          </Show>
          <text fg={theme.text}>{title() ?? modelLabelMemo()}</text>
        </box>
        <box flexDirection="row" alignItems="center" gap={2}>
          <Show when={status() === "running"}>
            <box flexDirection="row" gap={1}>
              <Spinner />
              <text fg={theme.textMuted}>working</text>
            </box>
          </Show>
          <Show when={gitBranchMemo()}>
            <text fg={theme.dim}>{gitBranchMemo()}</text>
          </Show>
          <text fg={theme.textMuted}>{workspaceNameMemo()}</text>
        </box>
      </box>
      <box border={["bottom"]} borderColor={theme.borderSubtle}>
        <text fg={theme.dim}> </text>
      </box>
      <box flexDirection="row" flexGrow={1} minHeight={0}>
        <box
          flexDirection="column"
          flexGrow={1}
          minHeight={0}
          paddingLeft={sidebar().left}
          paddingRight={sidebar().right}
          paddingTop={1}
          gap={1}
          width={Math.min(sidebar().maxContent + sidebar().left + sidebar().right, 9999)}
        >
          <scrollbox
            ref={(r: ScrollBoxRenderable) => (scroll = r)}
            flexGrow={1}
            minHeight={0}
            stickyScroll
            stickyStart="bottom"
          >
            <Show
              when={session.entries().length === 0}
              fallback={
                <For each={session.entries()}>
                  {(entry, index) => (
                    <MessageRow
                      entry={entry}
                      onToggleReasoning={toggleReasoning}
                      onToggleTool={toggleTool}
                      focused={index() === focusIndex()}
                    />
                  )}
                </For>
              }
            >
              <Show when={loading()}>
                <box paddingLeft={2} paddingTop={2} flexDirection="column" gap={1}>
                  <Spinner />
                  <text fg={theme.textMuted}>Loading session…</text>
                </box>
              </Show>
              <Show when={!loading() && loadError()}>
                <box paddingLeft={2} paddingTop={2} flexDirection="column" gap={1}>
                  <text fg={theme.error}>{loadError()}</text>
                  <text fg={theme.dim}>Press esc to go back</text>
                </box>
              </Show>
              <Show when={!loading() && !loadError()}>
                <box paddingLeft={2} paddingTop={2} flexDirection="column" gap={1}>
                  <text fg={theme.primary}>Ready</text>
                  <text fg={theme.textMuted}>What do you want to build or change?</text>
                  <text fg={theme.dim}>try: "inspect this project" · "explain the auth flow" · "add a failing test"</text>
                  <text fg={theme.dim}>ctrl+p commands · f2 permission · esc back · ctrl+c exit</text>
                </box>
              </Show>
            </Show>
            <Show when={newUpdates() > 0}>
              <box flexDirection="row" flexShrink={0} justifyContent="flex-end">
                <box
                  flexShrink={0}
                  backgroundColor={theme.backgroundPanel}
                  paddingX={1}
                  onMouseDown={() => jumpToBottom()}
                >
                  <text fg={theme.primary}>{newUpdates()} new updates ↓</text>
                </box>
              </box>
            </Show>
          </scrollbox>
        </box>
      </box>
      <PlanPanel steps={session.plan()} />
      <Prompt
        client={props.client}
        initialInput={draft()}
        onSubmit={handleSubmit}
        placeholder={status() === "idle" ? "Ask anything…" : "Agent is thinking…"}
      />
    </box>
  )
}

