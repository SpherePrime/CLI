import { appendFileSync } from "node:fs"
import { join } from "node:path"
import { tmpdir } from "node:os"

const logFile = join(tmpdir(), "agent-tui.log")
const enabled = Boolean(process.env.AGENT_TUI_DEBUG)

export function log(message: string) {
  if (!enabled) return
  try {
    appendFileSync(logFile, `[${new Date().toISOString()}] ${message}\n`)
  } catch {}
}

export function logError(message: string) {
  try {
    appendFileSync(logFile, `[${new Date().toISOString()}] ERROR ${message}\n`)
  } catch {}
}
