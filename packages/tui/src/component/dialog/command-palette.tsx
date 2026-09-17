import { createMemo, createSignal, createEffect, onMount, For } from "solid-js"
import { useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import fuzzysort from "fuzzysort"
import { theme } from "../../theme"
import { closeDialog } from "../../context/dialog"
import { buildCommands, SECTION_ORDER, type Command } from "../../commands"
import type { AgentClient } from "../../client"

type Row = { kind: "header"; title: string } | { kind: "command"; command: Command }

const dim = RGBA.fromValues(0, 0, 0, 160)

export function CommandPalette(props: { client: AgentClient }) {
  const dimensions = useTerminalDimensions()
  let searchInput: { focus?: () => void } | undefined

  const [query, setQuery] = createSignal("")
  const [selected, setSelected] = createSignal(0)
  const [offset, setOffset] = createSignal(0)

  const commands = createMemo(() => buildCommands(props.client))
  const maxRows = createMemo(() => Math.max(5, Math.min(13, dimensions().height - 9)))
  const width = createMemo(() => Math.min(78, Math.max(44, dimensions().width - 6)))
  const height = createMemo(() => maxRows() + 9)

  const rows = createMemo<Row[]>(() => {
    const all = commands()
    const text = query().trim()
    if (text) {
      return fuzzysort
        .go(text, all, { key: "title", threshold: -10000, limit: 60 })
        .map((result): Row => ({ kind: "command", command: result.obj }))
    }
    const output: Row[] = []
    const suggested = all.filter((command) => command.suggested)
    if (suggested.length > 0) {
      output.push({ kind: "header", title: "Suggested" })
      for (const command of suggested) output.push({ kind: "command", command })
    }
    for (const section of SECTION_ORDER) {
      const items = all.filter((command) => command.section === section)
      if (items.length === 0) continue
      output.push({ kind: "header", title: section })
      for (const command of items) output.push({ kind: "command", command })
    }
    return output
  })

  createEffect(() => {
    rows()
    const first = rows().findIndex((row) => row.kind === "command")
    setSelected(first === -1 ? 0 : first)
    setOffset(0)
  })

  function ensureVisible(index: number) {
    const visible = maxRows()
    let next = offset()
    if (index < next) next = index
    if (index >= next + visible) next = index - visible + 1
    setOffset(Math.max(0, next))
  }

  function move(delta: number) {
    const list = rows()
    if (list.length === 0) return
    let index = selected()
    for (let step = 0; step < list.length; step++) {
      index += delta
      if (index < 0) index = list.length - 1
      if (index >= list.length) index = 0
      const row = list[index]
      if (row && row.kind === "command") {
        setSelected(index)
        ensureVisible(index)
        return
      }
    }
  }

  function runSelected() {
    const row = rows()[selected()]
    if (!row || row.kind !== "command") return
    closeDialog()
    void row.command.run()
  }

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
      runSelected()
    }
  })

  onMount(() => searchInput?.focus?.())

  const window = createMemo(() => rows().slice(offset(), offset() + maxRows()))
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
          <text fg={theme.text}>Commands</text>
          <text fg={theme.textMuted}>esc</text>
        </box>
        <box paddingLeft={2} paddingRight={2} paddingTop={1}>
          <input
            ref={(r: { focus?: () => void }) => (searchInput = r)}
            width="100%"
            placeholder="Search"
            placeholderColor={theme.textMuted}
            textColor={theme.text}
            focusedTextColor={theme.text}
            backgroundColor={theme.backgroundElement}
            focusedBackgroundColor={theme.backgroundElement}
            focused
            onInput={(value: string) => setQuery(value)}
          />
        </box>
        <box flexDirection="column" paddingTop={1}>
          <For each={window()}>
            {(row, index) =>
              row.kind === "header" ? (
                <box paddingLeft={2} height={1}>
                  <text fg={theme.textMuted}>{row.title}</text>
                </box>
              ) : (
                <box
                  flexDirection="row"
                  justifyContent="space-between"
                  paddingLeft={2}
                  paddingRight={2}
                  height={1}
                  backgroundColor={selected() === offset() + index() ? theme.primary : undefined}
                  onMouseDown={() => {
                    setSelected(offset() + index())
                    runSelected()
                  }}
                >
                  <text fg={selected() === offset() + index() ? theme.background : theme.text}>
                    {row.command.title}
                  </text>
                  <text fg={selected() === offset() + index() ? theme.background : theme.textMuted}>
                    {row.command.shortcut ?? ""}
                  </text>
                </box>
              )
            }
          </For>
        </box>
        <box paddingLeft={2} paddingRight={2} paddingBottom={1}>
          <text fg={theme.textMuted}>enter select · ↑↓ move</text>
        </box>
      </box>
    </box>
  )
}
