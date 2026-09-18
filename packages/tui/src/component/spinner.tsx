import { createEffect, createSignal } from "solid-js"
import { theme } from "../theme"

const frames = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"]

export function Spinner() {
  const [frame, setFrame] = createSignal(0)
  createEffect(() => {
    const timer = setInterval(() => setFrame((index) => (index + 1) % frames.length), 80)
    return () => clearInterval(timer)
  })
  return <text fg={theme.primary}>{frames[frame()]}</text>
}