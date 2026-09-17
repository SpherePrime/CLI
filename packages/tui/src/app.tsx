import { createCliRenderer } from "@opentui/core"
import type { CliRenderer } from "@opentui/core"
import { render, useTerminalDimensions, useRenderer } from "@opentui/solid"
import { createSignal, Switch, Match, onMount, ErrorBoundary } from "solid-js"
import { AgentClient } from "./client"
import { useRoute } from "./context/route"
import { theme } from "./theme"
import { Home } from "./routes/home"
import { Session } from "./routes/session"
import { log, logError } from "./log"
import { win32DisableProcessedInput, win32FlushInputBuffer, win32InstallCtrlCGuard } from "./terminal-win32"

export type TuiInput = {
  url: string
}

export async function run(input: TuiInput): Promise<void> {
  log("run: creating renderer")
  const renderer = await createCliRenderer({
    externalOutputMode: "passthrough",
    targetFps: 60,
    exitOnCtrlC: false,
    useKittyKeyboard: {},
    autoFocus: false,
  })
  log("run: renderer created")

  win32DisableProcessedInput()
  const removeGuard = win32InstallCtrlCGuard()

  const shutdown = new Promise<void>((resolve) => {
    renderer.once("destroy", () => {
      log("run: renderer destroyed")
      resolve()
    })
  })

  renderer.keyInput.on("keypress", (key) => {
    if (key.ctrl && key.name === "c") {
      renderer.destroy()
    }
  })

  await render(
    () => (
      <ErrorBoundary
        fallback={(error) => {
          logError(`render: ${error instanceof Error ? (error.stack ?? error.message) : String(error)}`)
          return (
            <box width="100%" height="100%" flexDirection="column" padding={2} gap={1}>
              <text fg={theme.error}>TUI error</text>
              <text fg={theme.text}>{String(error)}</text>
              <text fg={theme.textMuted}>Press Ctrl+C to exit</text>
            </box>
          )
        }}
      >
        <App url={input.url} />
      </ErrorBoundary>
    ),
    renderer,
  )
  renderer.start()
  log(`run: mounted isRunning=${renderer.isRunning}`)

  const stopResizePoll = startResizePoll(renderer)

  await shutdown
  stopResizePoll()
  removeGuard?.()
  win32FlushInputBuffer()
  log("run: finished")
}

function startResizePoll(renderer: CliRenderer) {
  if (process.platform !== "win32") return () => {}
  let lastWidth = renderer.terminalWidth
  let lastHeight = renderer.terminalHeight
  const timer = setInterval(() => {
    const width = process.stdout.columns || 0
    const height = process.stdout.rows || 0
    if (width <= 0 || height <= 0) return
    if (width === lastWidth && height === lastHeight) return
    lastWidth = width
    lastHeight = height
    const target = renderer as unknown as { handleResize?: (w: number, h: number) => void }
    target.handleResize?.(width, height)
    log(`run: polled resize ${width}x${height}`)
  }, 250)
  return () => clearInterval(timer)
}

function App(props: { url: string }) {
  const { route } = useRoute()
  const dimensions = useTerminalDimensions()
  const renderer = useRenderer()
  const [client] = createSignal(new AgentClient(props.url))

  onMount(() => {
    renderer.setTerminalTitle("agent")
  })

  return (
    <box
      width={dimensions().width}
      height={dimensions().height}
      flexDirection="column"
      backgroundColor={theme.background}
    >
      <box flexGrow={1} minHeight={0} flexDirection="column">
        <Switch>
          <Match when={route().type === "home"}>
            <Home client={client()} />
          </Match>
          <Match when={route().type === "session"}>
            <Session client={client()} />
          </Match>
        </Switch>
      </box>
    </box>
  )
}

export { useRoute, navigateTo } from "./context/route"
export type { Route } from "./context/route"
