import { createCliRenderer } from "@opentui/core"
import { render, useTerminalDimensions, useRenderer, useKeyboard } from "@opentui/solid"
import { createSignal, Switch, Match, onMount } from "solid-js"
import { AgentClient } from "./client"
import { useRoute } from "./context/route"
import { theme } from "./theme"
import { Home } from "./routes/home"
import { Session } from "./routes/session"
import { log } from "./log"

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

  const shutdown = new Promise<void>((resolve) => {
    renderer.once("destroy", () => {
      log("run: renderer destroyed")
      resolve()
    })
  })

  await render(() => <App url={input.url} />, renderer)
  log("run: mounted")

  await shutdown
  log("run: finished")
}

function App(props: { url: string }) {
  const { route } = useRoute()
  const dimensions = useTerminalDimensions()
  const renderer = useRenderer()
  const [client] = createSignal(new AgentClient(props.url))

  useKeyboard((key) => {
    if (key.ctrl && key.name === "c") {
      log("key: ctrl+c -> destroy")
      renderer.destroy()
    }
  })

  onMount(() => {
    renderer.setTerminalTitle("Agent")
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

export { useRoute } from "./context/route"
export type { Route } from "./context/route"
