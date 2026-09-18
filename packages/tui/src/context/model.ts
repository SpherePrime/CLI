import { createSignal } from "solid-js"
import type { AgentClient, ModelConfig } from "../client"

export type ModelInfo = {
  provider: string
  model: string
}

export type ModelState =
  | { status: "loading" }
  | { status: "ready"; version: string; model: ModelInfo }
  | { status: "error"; error: string }

const [state, setState] = createSignal<ModelState>({ status: "loading" })

export function useModel() {
  return {
    modelState: state,
    loadModel,
    applyModel,
    resetModel,
  }
}

export function modelState() {
  return state()
}

export function modelStatus(): ModelState["status"] {
  return state().status
}

export function modelVersion(): string | undefined {
  const current = state()
  return current.status === "ready" ? current.version : undefined
}

export function modelInfo(): ModelInfo | undefined {
  const current = state()
  return current.status === "ready" ? current.model : undefined
}

export function modelError(): string | undefined {
  const current = state()
  return current.status === "error" ? current.error : undefined
}

export function modelLabel(): string | undefined {
  const current = modelInfo()
  return current ? `${current.model} · ${current.provider}` : undefined
}

export function loadModel(client: AgentClient): Promise<void> {
  setState({ status: "loading" })
  return client.info().then(
    (info) => {
      setState({
        status: "ready",
        version: info.version,
        model: { provider: info.model.provider, model: info.model.model },
      })
    },
    (error: unknown) => {
      setState({ status: "error", error: String(error) })
    },
  )
}

export function applyModel(model: ModelConfig): void {
  setState((previous) => {
    const version = previous.status === "ready" ? previous.version : ""
    return { status: "ready", version, model: { provider: model.provider, model: model.model } }
  })
}

export function resetModel(): void {
  setState({ status: "loading" })
}