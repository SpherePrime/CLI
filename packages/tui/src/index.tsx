import { run } from "./app"

const args = process.argv.slice(2)
function argValue(name: string): string | undefined {
  const index = args.indexOf(name)
  return index !== -1 ? args[index + 1] : undefined
}

const port = Number(argValue("--port") ?? process.env.AGENT_TUI_PORT ?? "0")

async function main() {
  let url: string
  if (port > 0) {
    url = `http://127.0.0.1:${port}`
    await run({ url })
    return
  }

  const configFile = process.env.HOME + "/.config/agent/serve.json"
  try {
    const raw = await Bun.file(configFile).text()
    const saved = JSON.parse(raw)
    if (saved.port) {
      url = `http://127.0.0.1:${saved.port}`
      await run({ url })
      return
    }
  } catch {}

  console.error("agent-tui: cannot find a running server. Start it with `agent serve` first,")
  console.error("or pass --port <port> to connect to an existing instance.")
  process.exit(1)
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})