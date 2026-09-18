import type { CliRenderer } from "@opentui/core"

let current: CliRenderer | undefined

export function setAppRenderer(renderer: CliRenderer) {
  current = renderer
}

export function getAppRenderer(): CliRenderer | undefined {
  return current
}

export function quitApp() {
  current?.destroy()
}
