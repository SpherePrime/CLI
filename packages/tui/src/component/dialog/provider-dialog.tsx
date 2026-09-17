import { createMemo, createSignal, createEffect, onMount, For, Show } from "solid-js"
import { useKeyboard, useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import fuzzysort from "fuzzysort"
import { theme } from "../../theme"
import { closeDialog, openForm, openInfo, openModelDialog, type FormField } from "../../context/dialog"
import type { AgentClient, ProviderInfo } from "../../client"

const dim = RGBA.fromValues(0, 0, 0, 160)

export function ProviderDialog(props: { client: AgentClient }) {
  const dimensions = useTerminalDimensions()
  let searchInput: { focus?: () => void } | undefined

  const [query, setQuery] = createSignal("")
  const [selected, setSelected] = createSignal(0)
  const [offset, setOffset] = createSignal(0)
  const [providers, setProviders] = createSignal<ProviderInfo[]>([])
  const [loading, setLoading] = createSignal(true)
  const [error, setError] = createSignal<string | undefined>(undefined)

  const maxRows = createMemo(() => Math.max(5, Math.min(14, dimensions().height - 8)))
  const width = createMemo(() => Math.min(78, Math.max(44, dimensions().width - 6)))
  const height = createMemo(() => maxRows() + 8)

  async function load() {
    setLoading(true)
    setError(undefined)
    try {
      setProviders(await props.client.listProviders())
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

  const rows = createMemo<ProviderInfo[]>(() => {
    const text = query().trim()
    if (text) {
      return fuzzysort
        .go(text, providers(), { key: "name", threshold: -10000, limit: 80 })
        .map((result) => result.obj)
    }
    return providers()
  })

  createEffect(() => {
    rows()
    setSelected(0)
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
    const count = list.length
    const index = (selected() + delta + count) % count
    setSelected(index)
    ensureVisible(index)
  }

  function apiKeyForm(input: { id: string; name: string; baseUrl: boolean }) {
    const fields: FormField[] = [
      { kind: "text", key: "api_key", label: "API key", placeholder: "sk-...", hint: "Stored locally, never sent anywhere else." },
    ]
    if (input.baseUrl) {
      fields.push({ kind: "text", key: "base_url", label: "Base URL", placeholder: "https://api.example.com/v1" })
    }
    openForm({
      title: `Connect ${input.name}`,
      description: "This only stores a credential for the provider.",
      fields,
      onSubmit: async (values) => {
        try {
          await props.client.connectProvider({
            id: input.id,
            api_key: values.api_key,
            base_url: values.base_url,
          })
          openModelDialog()
        } catch (cause) {
          openInfo({ title: "Connect provider", body: String(cause) })
        }
      },
    })
  }

  function connectOther() {
    openForm({
      title: "Provider id",
      description: "Add any OpenAI-compatible provider by id.",
      fields: [{ kind: "text", key: "id", label: "Provider id", placeholder: "deepseek" }],
      onSubmit: (values) => {
        const id = (values.id ?? "").trim()
        if (!id) return
        apiKeyForm({ id, name: id, baseUrl: true })
      },
    })
  }

  function choose() {
    const provider = rows()[selected()]
    if (!provider) return
    closeDialog()
    if (provider.id === "other") {
      connectOther()
      return
    }
    apiKeyForm({ id: provider.id, name: provider.name, baseUrl: false })
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
      choose()
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
          <text fg={theme.text}>Connect a provider</text>
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
              <text fg={theme.textMuted}>Loading providers…</text>
            </box>
          </Show>
          <Show when={error()}>
            <box paddingLeft={2} paddingRight={2}>
              <text fg={theme.error}>{error()}</text>
            </box>
          </Show>
          <For each={window()}>
            {(provider, index) => (
              <box
                flexDirection="row"
                justifyContent="space-between"
                paddingLeft={2}
                paddingRight={2}
                height={1}
                backgroundColor={selected() === offset() + index() ? theme.primary : undefined}
                onMouseDown={() => {
                  setSelected(offset() + index())
                  choose()
                }}
              >
                <text fg={selected() === offset() + index() ? theme.background : theme.text}>
                  {provider.connected ? "✓ " : "  "}
                  {provider.name}
                </text>
                <text fg={selected() === offset() + index() ? theme.background : theme.textMuted}>
                  {provider.custom && provider.id !== "other" ? "custom" : ""}
                </text>
              </box>
            )}
          </For>
        </box>
        <box paddingLeft={2} paddingRight={2} paddingBottom={1}>
          <text fg={theme.textMuted}>enter select · ↑↓ move</text>
        </box>
      </box>
    </box>
  )
}
