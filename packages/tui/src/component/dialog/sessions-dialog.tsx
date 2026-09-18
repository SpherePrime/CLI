import { createResource, createMemo, createSignal, For, onMount, Show } from "solid-js"
import { useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import { theme } from "../../theme"
import { closeDialog } from "../../context/dialog"
import { navigateTo } from "../../context/route"
import { resetSession } from "../../context/session"
import { workspaceInfo } from "../../context/model"
import { groupSessions, sessionLabel, projectLabel } from "../../util/group-sessions"
import type { AgentClient, SessionInfo } from "../../client"

const dim = RGBA.fromValues(0, 0, 0, 160)

type Row =
  | { kind: "header"; id: string; title: string; count: number }
  | { kind: "session"; id: string; session: SessionInfo }

export function SessionsDialog(props: { client: AgentClient }) {
  const dimensions = useTerminalDimensions()
  const [sessions] = createResource(() => props.client.sessions())
  const [query, setQuery] = createSignal("")
  const [selected, setSelected] = createSignal(0)

  const currentProjectId = createMemo(() => workspaceInfo()?.id)

  const rows = createMemo<Row[]>(() => {
    const groups = groupSessions(sessions() ?? [], currentProjectId(), query())
    const out: Row[] = []
    for (const group of groups) {
      out.push({ kind: "header", id: group.id, title: group.title, count: group.sessions.length })
      for (const session of group.sessions) out.push({ kind: "session", id: session.id, session })
    }
    return out
  })

  function selectedRow(): Row | undefined {
    const all = rows()
    if (all.length === 0) return undefined
    const index = Math.min(selected(), all.length - 1)
    return all[Math.max(0, index)]
  }

  function move(delta: number) {
    const all = rows()
    if (all.length === 0) return
    let index = selected()
    for (let step = 0; step < all.length; step++) {
      index = (index + delta + all.length) % all.length
      const row = all[index]
      if (row && row.kind === "session") {
        setSelected(index)
        return
      }
    }
  }

  function choose() {
    const row = selectedRow()
    if (!row || row.kind !== "session") return
    closeDialog()
    resetSession()
    navigateTo({ type: "session", sessionId: row.id })
  }

  const maxRows = createMemo(() => Math.max(6, Math.min(18, dimensions().height - 8)))
  const width = createMemo(() => Math.min(88, Math.max(60, dimensions().width - 6)))
  const height = createMemo(() => maxRows() + 8)

  useKeyboard((key) => {
    if (key.name === "escape") {
      key.preventDefault()
      closeDialog()
      return
    }
    if (key.name === "up") {
      key.preventDefault()
      move(-1)
      return
    }
    if (key.name === "down") {
      key.preventDefault()
      move(1)
      return
    }
    if (key.name === "return") {
      key.preventDefault()
      choose()
      return
    }
    if (key.name === "backspace") {
      key.preventDefault()
      setQuery((value) => value.slice(0, -1))
      return
    }
    if (key.name === "space") {
      key.preventDefault()
      setQuery((value) => `${value} `)
      return
    }
    if (key.name && key.name.length === 1 && !key.ctrl && !key.meta) {
      key.preventDefault()
      setQuery((value) => value + key.name)
    }
  })

  onMount(() => setSelected(0))

  const top = createMemo(() => Math.max(1, Math.floor((dimensions().height - height()) / 2)))
  const left = createMemo(() => Math.max(1, Math.floor((dimensions().width - width()) / 2)))

  return (
    <box position="absolute" top={0} left={0} width="100%" height="100%" backgroundColor={dim} zIndex={200}>
      <box
        position="absolute"
        top={top()}
        left={left()}
        width={width()}
        backgroundColor={theme.backgroundPanel}
        border
        borderColor={theme.borderSubtle}
        flexDirection="column"
      >
        <box flexDirection="row" justifyContent="space-between" paddingLeft={2} paddingRight={2} paddingTop={1}>
          <text fg={theme.text}>Open session</text>
          <text fg={theme.textMuted}>esc</text>
        </box>
        <box paddingLeft={2} paddingRight={2} paddingTop={1}>
          <input
            width="100%"
            value={query()}
            placeholder="Search by title, project or path"
            placeholderColor={theme.textMuted}
            textColor={theme.text}
            focusedTextColor={theme.text}
            backgroundColor={theme.backgroundElement}
            focusedBackgroundColor={theme.backgroundElement}
            focused
            onInput={(value: string) => setQuery(value)}
          />
        </box>
        <Show when={rows().length === 0}>
          <box paddingLeft={2} paddingRight={2} paddingTop={1} paddingBottom={1}>
            <text fg={theme.textMuted}>no sessions match</text>
          </box>
        </Show>
        <Show when={rows().length > 0}>
          <box flexDirection="column" paddingTop={1} paddingBottom={1}>
            <For each={rows()}>
              {(row, index) => {
                if (row.kind === "header") {
                  return (
                    <box flexDirection="row" justifyContent="space-between" paddingLeft={2} paddingRight={2} height={1}>
                      <text fg={theme.primary}>{row.title}</text>
                      <text fg={theme.textMuted}>
                        {row.count} {row.count === 1 ? "session" : "sessions"}
                      </text>
                    </box>
                  )
                }
                return (
                  <box
                    flexDirection="row"
                    justifyContent="space-between"
                    paddingLeft={2}
                    paddingRight={2}
                    height={1}
                    backgroundColor={selected() === index() ? theme.primary : undefined}
                    onMouseDown={() => {
                      setSelected(index())
                      choose()
                    }}
                  >
                    <text fg={selected() === index() ? theme.background : theme.text}>{sessionLabel(row.session)}</text>
                    <text fg={selected() === index() ? theme.background : theme.textMuted}>
                      {projectLabel(row.session)}
                    </text>
                  </box>
                )
              }}
            </For>
          </box>
        </Show>
        <box paddingLeft={2} paddingRight={2} paddingBottom={1}>
          <text fg={theme.textMuted}>type to search · ↑↓ move · enter open</text>
        </box>
      </box>
    </box>
  )
}