import { beforeEach, describe, expect, test } from "bun:test"
import type { AgentClient, ModelConfig, ServerInfo } from "../client"
import { applyModel, loadModel, modelLabel, modelStatus, modelVersion, resetModel } from "./model"

function fakeClient(info: ServerInfo): AgentClient {
  return {
    info: () => Promise.resolve(info),
  } as unknown as AgentClient
}

function failingClient(error: string): AgentClient {
  return {
    info: () => Promise.reject(new Error(error)),
  } as unknown as AgentClient
}

const config: ModelConfig = {
  provider: "openai-compatible",
  model: "agnes-3.0-flash",
  base_url: "http://localhost:8080",
}

const info: ServerInfo = { name: "agent", version: "0.1.0", model: config }

beforeEach(() => {
  resetModel()
})

describe("model store", () => {
  test("startup load from server shows model without a hardcoded fallback", async () => {
    expect(modelLabel()).toBeUndefined()
    await loadModel(fakeClient(info))
    expect(modelStatus()).toBe("ready")
    expect(modelVersion()).toBe("0.1.0")
    expect(modelLabel()).toBe("agnes-3.0-flash · openai-compatible")
  })

  test("failed load never leaves a stale or invented model", async () => {
    await loadModel(fakeClient(info))
    resetModel()
    await loadModel(failingClient("connection refused"))
    expect(modelStatus()).toBe("error")
    expect(modelLabel()).toBeUndefined()
  })

  test("switching model updates label synchronously", async () => {
    await loadModel(fakeClient(info))
    expect(modelLabel()).toBe("agnes-3.0-flash · openai-compatible")
    applyModel({ provider: "anthropic", model: "claude-opus-4", base_url: undefined })
    expect(modelLabel()).toBe("claude-opus-4 · anthropic")
  })

  test("applying a model before load still records it as the only source", () => {
    applyModel({ provider: "openai-compatible", model: "deepseek-r1", base_url: undefined })
    expect(modelStatus()).toBe("ready")
    expect(modelLabel()).toBe("deepseek-r1 · openai-compatible")
  })
})