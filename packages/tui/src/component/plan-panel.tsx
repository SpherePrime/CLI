import { For, Show } from "solid-js"
import type { PlanStep } from "../client"
import { theme } from "../theme"

function stepMark(status: string): string {
  if (status === "completed") return "[x]"
  if (status === "in_progress") return "[~]"
  return "[ ]"
}

function stepColor(status: string) {
  if (status === "completed") return theme.success
  if (status === "in_progress") return theme.primary
  return theme.textMuted
}

export function PlanPanel(props: { steps: PlanStep[] }) {
  return (
    <Show when={props.steps.length > 0}>
      <box flexDirection="column" flexShrink={0} paddingLeft={2} paddingRight={2} paddingBottom={1} gap={0}>
        <text fg={theme.textMuted}>plan</text>
        <For each={props.steps}>
          {(step, index) => {
            const status = step.status ?? "pending"
            return (
              <text fg={stepColor(status)}>
                {`${stepMark(status)} ${index() + 1}. ${step.title}`}
              </text>
            )
          }}
        </For>
      </box>
    </Show>
  )
}
