import { run } from "./app"
import { log, logError } from "./log"

const args = process.argv.slice(2)
function argValue(name: string): string | undefined {
  const index = args.indexOf(name)
  return index !== -1 ? args[index + 1] : undefined
}

const DEFAULT_PORT = 40123
const port = Number(argValue("--port") ?? process.env.AGENT_TUI_PORT ?? DEFAULT_PORT)

process.on("uncaughtException", (error) => {
  logError(`uncaughtException: ${error?.stack ?? error}`)
})

process.on("unhandledRejection", (reason) => {
  logError(`unhandledRejection: ${reason instanceof Error ? reason.stack : String(reason)}`)
})

process.on("exit", (code) => {
  log(`process exit code=${code}`)
})

async function main() {
  log(`start port=${port}`)
  await run({ url: `http://127.0.0.1:${port}` })
}

main().catch((error) => {
  const message = error instanceof Error ? (error.stack ?? error.message) : String(error)
  logError(`main: ${message}`)
  console.error(message)
  process.exit(1)
})
