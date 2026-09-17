import { createSignal } from "solid-js"
import type { CliRenderer } from "@opentui/core"

let current: CliRenderer | undefined

const [modelRevision, setModelRevision] = createSignal(0)

export function bumpModelRevision() {
  setModelRevision((value) => value + 1)
}

export { modelRevision }

export function setAppRenderer(renderer: CliRenderer) {
  current = renderer
}

export function getAppRenderer(): CliRenderer | undefined {
  return current
}

export function quitApp() {
  current?.destroy()
}
