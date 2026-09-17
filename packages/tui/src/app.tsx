import { createCliRenderer, type CliRendererConfig } from "@opentui/core"
import { render, useTerminalDimensions } from "@opentui/solid"
import { createSignal, Switch, Match, onMount } from "solid-js"
import { AgentClient } from "./client"
import { useRoute, RouteProvider } from "./context/route"
import { theme } from "./theme"
import { Home } from "./routes/home"
import { Session } from "./routes/session"

export type TuiInput = {
  url: string
}

export async function run(input: TuiInput): Promise<void> {
  const renderer = await createCliRenderer({
    externalOutputMode: "passthrough",
    targetFps: 60,
    exitOnCtrlC: false,
    useKittyKeyboard: {},
    autoFocus: false,
  })

  await render(
    () => (
      <RouteProvider initialRoute={{ type: "home" }}>
        <App url={input.url} renderer={renderer} />
      </RouteProvider>
    ),
    renderer,
  )

  process.stdin.resume()
}

function App(props: { url: string; renderer: any }) {
  const { route } = useRoute()
  const dimensions = useTerminalDimensions()
  const [client] = createSignal(new AgentClient(props.url))

  onMount(() => {
    props.renderer.setTerminalTitle("Agent")
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