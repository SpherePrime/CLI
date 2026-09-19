import { createEffect, createMemo, createSignal, onCleanup } from "solid-js"
import { theme } from "../theme"
import { useSession } from "../context/session"

const statusFrames = ["●", "◐", "○", "◑"]

export function StatusBar(props: { running: boolean }) {
  const session = useSession()
  const [frame, setFrame] = createSignal(0)
  const [elapsed, setElapsed] = createSignal(0)

  createEffect(() => {
    if (!props.running) {
      setElapsed(0)
      setFrame(0)
      return
    }
    const started = Date.now()
    const timer = setInterval(() => {
      setFrame((index) => (index + 1) % statusFrames.length)
      setElapsed(Math.floor((Date.now() - started) / 1000))
    }, 120)
    onCleanup(() => clearInterval(timer))
  })

  const activity = () => session.lastActivity()

  const elapsedText = createMemo(() => {
    const total = elapsed()
    const m = Math.floor(total / 60)
    const s = total % 60
    if (m > 0) return `${m}m ${s}s`
    return `${s}s`
  })

  if (!props.running) {
    return (
      <box height={1} flexDirection="row" paddingLeft={2} gap={1}>
        <text fg={theme.dim}>idle</text>
      </box>
    )
  }

  return (
    <box height={1} flexDirection="row" gap={1} paddingLeft={2}>
      <text fg={theme.primary}>{statusFrames[frame()]}</text>
      <text fg={theme.text}>{activity() ?? "working"}</text>
      <text fg={theme.textMuted}>{elapsedText()}</text>
    </box>
  )
}
