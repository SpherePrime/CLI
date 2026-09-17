import { createResource, createMemo, createSignal, For } from "solid-js"
import { useKeyboard } from "@opentui/solid"
import { theme } from "../theme"
import { useRoute } from "../context/route"
import { useDialog } from "../context/dialog"
import { modelRevision } from "../context/app"
import type { AgentClient } from "../client"

type HomeItem = {
  label: string
  hint: string
  run: () => void
}

export function Home(props: { client: AgentClient }) {
  const { navigate } = useRoute()
  const { dialog } = useDialog()
  const [info] = createResource(() => (modelRevision(), props.client.info()))
  const [recent] = createResource(() => props.client.sessions())
  const [selected, setSelected] = createSignal(0)

  const items = createMemo<HomeItem[]>(() => {
    const base: HomeItem[] = [
      { label: "New session", hint: "c", run: () => navigate({ type: "session" }) },
      { label: "Open file", hint: "o", run: () => navigate({ type: "session", draft: "@" }) },
    ]
    const sessions = (recent() ?? []).slice(0, 5).map((session) => ({
      label: session.title?.trim() || session.id.slice(0, 8),
      hint: "resume",
      run: () => navigate({ type: "session", sessionId: session.id }),
    }))
    return [...base, ...sessions]
  })

  const status = createMemo(() => {
    const value = info()
    if (value) return `${value.version} · ${value.model.model}`
    if (info.error) return `offline · ${String(info.error)}`
    return "connecting…"
  })

  function move(delta: number) {
    const count = items().length
    if (count === 0) return
    setSelected((prev) => Math.min(Math.max(prev + delta, 0), count - 1))
  }

  useKeyboard((key) => {
    if (dialog().type !== "none") return
    if (key.name === "up" || key.name === "left") {
      key.preventDefault()
      move(-1)
      return
    }
    if (key.name === "down" || key.name === "right") {
      key.preventDefault()
      move(1)
      return
    }
    if (key.name === "return" || key.name === "space") {
      key.preventDefault()
      items()[selected()]?.run()
      return
    }
    if (key.name === "c") {
      key.preventDefault()
      navigate({ type: "session" })
      return
    }
    if (key.name === "o") {
      key.preventDefault()
      navigate({ type: "session", draft: "@" })
    }
  })

  return (
    <box width="100%" height="100%" flexDirection="column" justifyContent="center" alignItems="center">
      <box width="100%" justifyContent="center" flexDirection="row">
        <text fg={theme.primary}>agent</text>
      </box>
      <box paddingTop={1} width="100%" justifyContent="center" flexDirection="row">
        <text fg={theme.textMuted}>{status()}</text>
      </box>
      <box paddingTop={2} flexDirection="column" alignItems="center" gap={1}>
        <For each={items()}>
          {(item, index) => (
            <box
              width={40}
              paddingLeft={2}
              paddingRight={2}
              flexDirection="row"
              justifyContent="space-between"
              backgroundColor={selected() === index() ? theme.primary : theme.backgroundElement}
              onMouseDown={() => {
                setSelected(index())
                item.run()
              }}
            >
              <text fg={selected() === index() ? theme.background : theme.text}>{item.label}</text>
              <text fg={selected() === index() ? theme.background : theme.textMuted}>{item.hint}</text>
            </box>
          )}
        </For>
      </box>
      <box paddingTop={2} flexDirection="row" gap={2}>
        <text fg={theme.textMuted}>↑↓ move</text>
        <text fg={theme.textMuted}>enter select</text>
        <text fg={theme.textMuted}>ctrl+c exit</text>
      </box>
    </box>
  )
}
