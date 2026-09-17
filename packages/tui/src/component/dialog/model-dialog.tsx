import { createMemo, createSignal, createEffect, onMount, For, Show } from "solid-js"
import { useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import fuzzysort from "fuzzysort"
import { theme } from "../../theme"
import { closeDialog, openProviderDialog } from "../../context/dialog"
import { bumpModelRevision } from "../../context/app"
import type { AgentClient, ModelGroup, ModelInfo } from "../../client"

type ModelRow = { kind: "model"; model: ModelInfo; group: ModelGroup }
type Row = { kind: "header"; title: string } | ModelRow

const dim = RGBA.fromValues(0, 0, 0, 160)

export function ModelDialog(props: { client: AgentClient }) {
  const dimensions = useTerminalDimensions()
  let searchInput: { focus?: () => void } | undefined

  const [query, setQuery] = createSignal("")
  const [selected, setSelected] = createSignal(0)
  const [offset, setOffset] = createSignal(0)
  const [groups, setGroups] = createSignal<ModelGroup[]>([])
  const [favorites, setFavorites] = createSignal<string[]>([])
  const [loading, setLoading] = createSignal(true)
  const [error, setError] = createSignal<string | undefined>(undefined)

  const maxRows = createMemo(() => Math.max(5, Math.min(14, dimensions().height - 9)))
  const width = createMemo(() => Math.min(78, Math.max(44, dimensions().width - 6)))
  const height = createMemo(() => maxRows() + 9)

  async function load() {
    setLoading(true)
    setError(undefined)
    try {
      const data = await props.client.listModels()
      setGroups(data.data)
      setFavorites(data.favorites)
    } catch (cause) {
      setError(String(cause))
    } finally {
      setLoading(false)
    }
  }

  onMount(() => {
    searchInput?.focus?.()
    void load()
  })

  const rows = createMemo<Row[]>(() => {
    const text = query().trim()
    if (text) {
      const all = groups().flatMap((group) => group.models.map((model) => ({ model, group })))
      return fuzzysort
        .go(text, all, { key: "model.name", threshold: -10000, limit: 80 })
        .map((result): Row => ({ kind: "model", model: result.obj.model, group: result.obj.group }))
    }
    const output: Row[] = []
    for (const group of groups()) {
      if (group.models.length === 0) continue
      output.push({ kind: "header", title: group.name })
      for (const model of group.models) output.push({ kind: "model", model, group })
    }
    return output
  })

  createEffect(() => {
    rows()
    const first = rows().findIndex((row) => row.kind === "model")
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
      if (row && row.kind === "model") {
        setSelected(index)
        ensureVisible(index)
        return
      }
    }
  }

  function selectedRow(): ModelRow | undefined {
    const row = rows()[selected()]
    return row && row.kind === "model" ? row : undefined
  }

  async function choose() {
    const row = selectedRow()
    if (!row) return
    closeDialog()
    try {
      await props.client.selectModel(row.model.provider, row.model.id)
      bumpModelRevision()
    } catch {}
  }

  async function toggleFavorite() {
    const row = selectedRow()
    if (!row) return
    try {
      const next = await props.client.toggleFavorite(row.model.provider, row.model.id)
      setFavorites(next)
    } catch {}
  }

  function isFavorite(model: ModelInfo) {
    return favorites().includes(`${model.provider}/${model.id}`)
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
      void choose()
      return
    }
    if (key.ctrl && key.name === "a") {
      key.preventDefault()
      openProviderDialog()
      return
    }
    if (key.ctrl && key.name === "f") {
      key.preventDefault()
      void toggleFavorite()
    }
  })

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
          <text fg={theme.text}>Select model</text>
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
          <Show when={loading()}>
            <box paddingLeft={2} height={1}>
              <text fg={theme.textMuted}>Loading models…</text>
            </box>
          </Show>
          <Show when={error()}>
            <box paddingLeft={2} paddingRight={2}>
              <text fg={theme.error}>{error()}</text>
            </box>
          </Show>
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
                    void choose()
                  }}
                >
                  <text fg={selected() === offset() + index() ? theme.background : theme.text}>
                    {row.model.name}
                  </text>
                  <text fg={selected() === offset() + index() ? theme.background : theme.textMuted}>
                    {[isFavorite(row.model) ? "★" : "", row.model.free ? "Free" : ""]
                      .filter(Boolean)
                      .join(" ")}
                  </text>
                </box>
              )
            }
          </For>
        </box>
        <box flexDirection="row" gap={2} paddingLeft={2} paddingRight={2} paddingBottom={1}>
          <text fg={theme.textMuted}>ctrl+a connect provider</text>
          <text fg={theme.textMuted}>ctrl+f favorite</text>
        </box>
      </box>
    </box>
  )
}
