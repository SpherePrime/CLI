import { run } from "./app"

const args = process.argv.slice(2)
function argValue(name: string): string | undefined {
  const index = args.indexOf(name)
  return index !== -1 ? args[index + 1] : undefined
}

const DEFAULT_PORT = 40123
const port = Number(argValue("--port") ?? process.env.AGENT_TUI_PORT ?? DEFAULT_PORT)

async function main() {
  await run({ url: `http://127.0.0.1:${port}` })
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})