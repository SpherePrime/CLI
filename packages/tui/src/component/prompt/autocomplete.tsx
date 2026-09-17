import type { BoxRenderable, ScrollBoxRenderable, TextareaRenderable } from "@opentui/core"
import fuzzysort from "fuzzysort"
import { createMemo, createResource, createSignal, createEffect, onMount, onCleanup, Index, Show } from "solid-js"
import { createStore } from "solid-js/store"
import { useTerminalDimensions } from "@opentui/solid"
import { SplitBorder } from "../../ui/border"
import { theme, selectedForeground } from "../../theme"
import type { AgentClient } from "../../client"

const slashCommands = [
  { display: "/models", description: "Switch model" },
  { display: "/new", description: "New session" },
  { display: "/help", description: "Show help" },
  { display: "/exit", description: "Exit" },
  { display: "/clear", description: "Clear prompt" },
  { display: "/sessions", description: "Switch session" },
].map((item) => ({ ...item, value: item.display }))

function removeLineRange(input: string) {
  const hashIndex = input.lastIndexOf("#")
  return hashIndex !== -1 ? input.substring(0, hashIndex) : input
}

function extractLineRange(input: string) {
  const hashIndex = input.lastIndexOf("#")
  if (hashIndex === -1) return { baseQuery: input }
  const baseName = input.substring(0, hashIndex)
  const linePart = input.substring(hashIndex + 1)
  const lineMatch = linePart.match(/^(\d+)(?:-(\d*))?$/)
  if (!lineMatch) return { baseQuery: baseName }
  const startLine = Number(lineMatch[1])
  const endLine = lineMatch[2] && startLine < Number(lineMatch[2]) ? Number(lineMatch[2]) : undefined
  return { lineRange: { baseName, startLine, endLine }, baseQuery: baseName }
}

export function mentionTriggerIndex(input: string, offset: number): number | undefined {
  let start = offset
  while (start > 0) {
    const prev = input[start - 1]
    if (prev === "@") {
      if (start === 1 || /\s/.test(input[start - 2] ?? "")) return start - 1
      return undefined
    }
    if (/\s/.test(prev ?? "")) return undefined
    start--
  }
  return undefined
}

export type AutocompleteRef = {
  visible: false | "@" | "/"
  onInput: (value: string) => void
}

export type AutocompleteOption = {
  display: string
  value?: string
  description?: string
  isDirectory?: boolean
  onSelect?: () => void
}

export function Autocomplete(props: {
  client: AgentClient
  value: string
  setPrompt: (fn: (draft: { input: string }) => void) => void
  anchor: () => BoxRenderable
  input: () => TextareaRenderable
  ref: (ref: AutocompleteRef) => void
}) {
  const dimensions = useTerminalDimensions()
  const [store, setStore] = createStore({
    index: 0,
    selected: 0,
    visible: false as AutocompleteRef["visible"],
  })
  const [positionTick, setPositionTick] = createSignal(0)

  createEffect(() => {
    if (store.visible) {
      let lastPos = { x: 0, y: 0, width: 0 }
      const interval = setInterval(() => {
        const anchor = props.anchor()
        if (anchor.x !== lastPos.x || anchor.y !== lastPos.y || anchor.width !== lastPos.width) {
          lastPos = { x: anchor.x, y: anchor.y, width: anchor.width }
          setPositionTick((t) => t + 1)
        }
      }, 50)
      onCleanup(() => clearInterval(interval))
    }
  })

  const position = createMemo(() => {
    if (!store.visible) return { x: 0, y: 0, width: 0 }
    dimensions()
    positionTick()
    const anchor = props.anchor()
    const parent = anchor.parent
    const parentX = parent?.x ?? 0
    const parentY = parent?.y ?? 0
    return { x: anchor.x - parentX, y: anchor.y - parentY, width: anchor.width }
  })

  const filter = createMemo(() => {
    if (!store.visible) return undefined
    props.value
    return props.input().getTextRange(store.index + 1, props.input().cursorOffset)
  })

  const [search, setSearch] = createSignal("")
  createEffect(() => {
    const next = filter()
    setSearch(next && next ? next : "")
  })

  const [files] = createResource(
    () => ({ query: search() }),
    async (input) => {
      if (!store.visible || store.visible === "/") return []
      const { baseQuery } = extractLineRange(input.query ?? "")
      let data: { path: string; type: string }[] = []
      try {
        const result = await props.client.findFiles(baseQuery)
        data = result
      } catch {}
      return data.map(
        (item): AutocompleteOption => ({
          display: item.type === "directory" ? item.path + "/" : item.path,
          value: item.path,
          isDirectory: item.type === "directory",
        }),
      )
    },
    { initialValue: [] },
  )

  const commands = createMemo((): AutocompleteOption[] => slashCommands)

  const options = createMemo((prev: AutocompleteOption[] | undefined) => {
    const filesValue = files()
    const commandsValue = commands()
    const searchValue = search()

    if (store.visible === "@") {
      const fileOptions = filesValue
      if (!searchValue) return fileOptions
      if (files.loading && prev && prev.length > 0) return prev
      return fileOptions.slice(0, 10)
    }

    if (store.visible === "/") {
      if (!searchValue) return commandsValue
      const results = fuzzysort.go(removeLineRange(searchValue), commandsValue, {
        keys: [(obj) => removeLineRange((obj.value ?? obj.display).trimEnd()), "description" as const],
        threshold: -Infinity,
        limit: 10,
      })
      return results.map((result) => result.obj)
    }

    return []
  })

  createEffect(() => {
    filter()
    setStore("selected", 0)
  })

  function move(direction: -1 | 1) {
    if (!store.visible) return
    if (!options().length) return
    let next = store.selected + direction
    if (next < 0) next = options().length - 1
    if (next >= options().length) next = 0
    setStore("selected", next)
  }

  function select() {
    const selected = options()[store.selected]
    if (!selected) return
    setStore("visible", false)
    selected.onSelect?.()
  }

  function hide() {
    setStore("visible", false)
  }

  function show(mode: "@" | "/") {
    setStore({ visible: mode, index: props.input().cursorOffset })
  }

  onMount(() => {
    props.ref({
      get visible() {
        return store.visible
      },
      onInput(value) {
        const auto = store.visible
        if (auto) {
          if (
            props.input().cursorOffset <= store.index ||
            props.input().getTextRange(store.index, props.input().cursorOffset).match(/\s/) ||
            (auto === "/" && value.match(/^\S+\s+\S+\s*$/))
          ) {
            hide()
          }
          return
        }
        const offset = props.input().cursorOffset
        if (offset === 0) return
        if (value.startsWith("/") && !value.slice(0, offset).match(/\s/)) {
          show("/")
          setStore("index", 0)
          return
        }
        const idx = mentionTriggerIndex(value, offset)
        if (idx !== undefined) {
          show("@")
          setStore("index", idx)
        }
      },
    })
  })

  const height = createMemo(() => {
    const count = options().length || 1
    if (!store.visible) return Math.min(10, count)
    positionTick()
    return Math.min(10, count, Math.max(1, props.anchor().y))
  })

  function insertText(text: string) {
    const input = props.input()
    const currentCursorOffset = input.cursorOffset
    input.cursorOffset = store.index
    const startCursor = input.logicalCursor
    input.cursorOffset = currentCursorOffset
    const endCursor = input.logicalCursor
    input.deleteRange(startCursor.row, startCursor.col, endCursor.row, endCursor.col)
    input.insertText(text)
    input.cursorOffset = store.index + Bun.stringWidth(text)
    props.setPrompt((draft) => {
      draft.input = input.plainText
    })
  }

  const slashOptionsWithSelect = createMemo(() =>
    commands().map((option) => ({
      ...option,
      onSelect: () => {
        const text = option.value + " "
        insertText(text)
      },
    })),
  )

  let scroll: ScrollBoxRenderable

  return (
    <box
      visible={store.visible !== false}
      position="absolute"
      top={position().y - height()}
      left={position().x}
      width={position().width}
      zIndex={100}
      {...SplitBorder}
      borderColor={theme.border}
    >
      <scrollbox
        ref={(r: ScrollBoxRenderable) => (scroll = r)}
        backgroundColor={theme.backgroundMenu}
        height={height()}
        scrollbarOptions={{ visible: false }}
      >
        <Index
          each={store.visible === "/" ? slashOptionsWithSelect() : options()}
          fallback={
            <box paddingLeft={1} paddingRight={1}>
              <text fg={theme.textMuted}>No matching items</text>
            </box>
          }
        >
          {(option: () => AutocompleteOption, index: number) => (
            <box
              paddingLeft={1}
              paddingRight={1}
              backgroundColor={index === store.selected ? theme.primary : undefined}
              flexDirection="row"
              onMouseUp={() => {
                setStore("selected", index)
                select()
              }}
            >
              <text fg={index === store.selected ? selectedForeground(theme) : theme.text} flexShrink={0}>
                {option().display}
              </text>
              <Show when={option().description}>
                <text fg={index === store.selected ? selectedForeground(theme) : theme.textMuted} wrapMode="none">
                  {" " + option().description?.trimStart()}
                </text>
              </Show>
            </box>
          )}
        </Index>
      </scrollbox>
    </box>
  )
}

export const __testing = { removeLineRange, extractLineRange, mentionTriggerIndex }