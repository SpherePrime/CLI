import { describe, expect, test } from "bun:test"
import type { EngineEvent } from "../client"
import { reduceEvent, timelineFromEvents, type ChatEntry } from "./timeline"

const meta = (sequence: number, itemId = `item-${sequence}`) => ({
  protocol: 2,
  sequence,
  turn_id: "turn-1",
  item_id: itemId,
  ts: 0,
})

const textDelta = (sequence: number, text: string, itemId?: string): Extract<EngineEvent, { type: "text_delta" }> => ({
  type: "text_delta",
  meta: meta(sequence, itemId),
  text,
})

const reasoningDelta = (sequence: number, text: string, itemId?: string): Extract<EngineEvent, { type: "reasoning_delta" }> => ({
  type: "reasoning_delta",
  meta: meta(sequence, itemId),
  text,
})

const toolStarted = (sequence: number, id: string, name: string): Extract<EngineEvent, { type: "tool_call_started" }> => ({
  type: "tool_call_started",
  meta: meta(sequence, `tool_${id}`),
  id,
  name,
  args: { command: "ls" },
})

const toolResult = (
  sequence: number,
  id: string,
  name: string,
  fileChanges: unknown[] = [],
): Extract<EngineEvent, { type: "tool_result" }> => ({
  type: "tool_result",
  meta: meta(sequence),
  id,
  name,
  ok: true,
  content: "done",
  error: null,
  duration_ms: 42,
  summary: "ran",
  details: null,
  exit_code: 0,
  truncated: false,
  file_changes: fileChanges as never,
})

function apply(events: EngineEvent[]): ChatEntry[] {
  return timelineFromEvents(events).entries
}

describe("timeline reducer", () => {
  test("stale events with lower sequence are deduped", () => {
    const first = apply([textDelta(1, "a"), textDelta(2, "b"), textDelta(1, "stale")])
    expect(first.map((entry) => entry.text).join("")).toBe("ab")
  })

  test("user message from message.created is appended first", () => {
    const events: EngineEvent[] = [
      {
        type: "message.created",
        message: { id: "m1", role: "user", content: "hello" },
      },
      {
        type: "assistant_message_started",
        meta: meta(1, "assistant_1"),
        id: "assistant_1",
      },
      textDelta(2, "hi", "assistant_1"),
    ]
    const entries = apply(events)
    expect(entries).toHaveLength(2)
    expect(entries[0]).toMatchObject({ role: "user", text: "hello" })
    expect(entries[1]).toMatchObject({ role: "assistant", text: "hi" })
  })

  test("reasoning and text land in the same assistant item", () => {
    const events: EngineEvent[] = [
      {
        type: "assistant_message_started",
        meta: meta(1, "assistant_1"),
        id: "assistant_1",
      },
      reasoningDelta(2, "think", "assistant_1"),
      textDelta(3, "answer", "assistant_1"),
    ]
    const entries = apply(events)
    expect(entries).toHaveLength(1)
    expect(entries[0]!.reasoning).toBe("think")
    expect(entries[0]!.text).toBe("answer")
  })

  test("assistant text is a separate entry from tool entries", () => {
    const events: EngineEvent[] = [
      textDelta(1, "partial"),
      toolStarted(2, "call_1", "shell"),
      toolResult(3, "call_1", "shell"),
      textDelta(4, "done now"),
    ]
    const entries = apply(events)
    expect(entries).toHaveLength(3)
    expect(entries[0]!).toMatchObject({ role: "assistant", text: "partial" })
    expect(entries[1]!).toMatchObject({
      role: "tool",
      tool: { name: "shell", state: "ok", durationMs: 42, fileChanges: [] },
    })
    expect(entries[2]!).toMatchObject({ role: "assistant", text: "done now" })
  })

  test("text_delta appends to the open assistant item", () => {
    const entries = apply([
      {
        type: "assistant_message_started",
        meta: meta(1, "assistant_1"),
        id: "assistant_1",
      },
      textDelta(2, "first ", "assistant_1"),
      textDelta(3, "second", "assistant_1"),
    ])
    expect(entries).toHaveLength(1)
    expect(entries[0]!.text).toBe("first second")
  })

  test("failed tool result marks tool entry failed", () => {
    const events: EngineEvent[] = [
      toolStarted(1, "call_1", "shell"),
      {
        ...toolResult(2, "call_1", "shell"),
        ok: false,
        error: "boom",
        content: "",
        summary: null,
      },
    ]
    const entries = apply(events)
    expect(entries).toHaveLength(1)
    expect(entries[0]!.tool).toMatchObject({ state: "failed" })
  })

  test("file changes are kept raw per tool result", () => {
    const change = { path: "src/a.ts", change: "write", additions: 3, deletions: 0 }
    const entries = apply([
      toolStarted(1, "call_1", "edit"),
      toolResult(2, "call_1", "edit", [change]),
    ])
    expect(entries[0]!.tool!.fileChanges).toEqual([change])
  })

  test("tool result keeps the human-readable action as the row title", () => {
    const entries = apply([toolStarted(1, "call_1", "shell"), toolResult(2, "call_1", "shell")])
    expect(entries[0]!.text).toBe("run ls")
  })

  test("denied tool result maps to the denied lifecycle state", () => {
    const events: EngineEvent[] = [
      toolStarted(1, "call_1", "shell"),
      {
        ...toolResult(2, "call_1", "shell"),
        ok: false,
        error: "tool shell is not permitted in Ask mode",
        content: "",
        summary: "denied by mode",
      },
    ]
    const entries = apply(events)
    expect(entries[0]!.tool).toMatchObject({ state: "denied" })
  })

  test("permission request marks the running tool as waiting", () => {
    const events: EngineEvent[] = [
      toolStarted(1, "call_1", "shell"),
      {
        type: "permission_requested",
        meta: meta(2),
        id: "p1",
        tool: "shell",
        scope: "Write",
        target: "src/a.ts",
        reason: "rewrite tests",
      },
    ]
    const entries = apply(events)
    expect(entries[0]!.tool).toMatchObject({ state: "waiting_permission" })
  })

  test("permission resolution puts the tool back into running", () => {
    const events: EngineEvent[] = [
      toolStarted(1, "call_1", "shell"),
      {
        type: "permission_requested",
        meta: meta(2),
        id: "p1",
        tool: "shell",
        scope: "Write",
        target: "src/a.ts",
        reason: "rewrite tests",
      },
      { type: "permission_resolved", meta: meta(3), id: "p1", decision: "allow" },
    ]
    const entries = apply(events)
    expect(entries[0]!.tool).toMatchObject({ state: "running" })
  })

  test("collapsible reasoning is collapsed again after completion", () => {
    const events: EngineEvent[] = [
      {
        type: "assistant_message_started",
        meta: meta(1, "assistant_1"),
        id: "assistant_1",
      },
      { type: "reasoning_started", meta: meta(2, "assistant_1") },
      reasoningDelta(3, "think hard", "assistant_1"),
      { type: "reasoning_completed", meta: meta(4, "assistant_1") },
    ]
    const entries = apply(events)
    expect(entries[0]).toMatchObject({
      reasoning: "think hard",
      reasoningOpen: false,
      expandedReasoning: false,
    })
  })

  test("tool activity line is removed once the tool finishes", () => {
    const events: EngineEvent[] = [
      toolStarted(1, "call_1", "shell"),
      { type: "activity_changed", meta: meta(2, "tool_call_1"), activity: "Running ls", kind: "tool" },
      toolResult(3, "call_1", "shell"),
    ]
    const entries = apply(events)
    expect(entries).toHaveLength(1)
    expect(entries[0]!.role).toBe("tool")
  })

  test("activity changes render as running system entries", () => {
    const entries = apply([
      { type: "activity_changed", meta: meta(1, "prep_0"), activity: "Preparing read_file", kind: null },
      { type: "activity_changed", meta: meta(2, "prep_0"), activity: "switching to read_file", kind: null },
    ])
    expect(entries).toHaveLength(1)
    expect(entries[0]).toMatchObject({ role: "system", text: "switching to read_file", running: true })
  })

  test("plan_updated stores steps and survives later events", () => {
    const state = timelineFromEvents([
      {
        type: "plan_updated",
        meta: meta(1, "tool_call_1"),
        steps: [
          { title: "Read code", status: "completed" },
          { title: "Patch code", status: "in_progress" },
          { title: "Run tests" },
        ],
      },
      textDelta(2, "working"),
    ])
    expect(state.plan).toHaveLength(3)
    expect(state.plan[0]).toEqual({ title: "Read code", status: "completed" })
    expect(state.plan[2]).toEqual({ title: "Run tests" })
  })

  test("question events render and record the answer", () => {
    const entries = apply([
      { type: "question_asked", meta: meta(1, "question_q1"), id: "q1", question: "Which file?", options: ["a", "b"] },
      { type: "question_answered", meta: meta(2, "question_q1"), id: "q1", answer: "a" },
    ])
    expect(entries).toHaveLength(1)
    expect(entries[0]!.text).toBe("? Which file?\n→ a")
  })

  test("reloading the same event log reconstructs an identical timeline", () => {
    const events: EngineEvent[] = [
      {
        type: "message.created",
        message: { id: "m1", role: "user", content: "read and finish" },
      },
      {
        type: "assistant_message_started",
        meta: meta(1, "assistant_0"),
        id: "assistant_0",
      },
      reasoningDelta(2, "thinking", "assistant_0"),
      toolStarted(3, "call_1", "read_file"),
      toolResult(4, "call_1", "read_file"),
      {
        type: "assistant_message_completed",
        meta: meta(5, "assistant_0"),
      },
      {
        type: "assistant_message_started",
        meta: meta(6, "assistant_2"),
        id: "assistant_2",
      },
      textDelta(7, "All set now", "assistant_2"),
      {
        type: "assistant_message_completed",
        meta: meta(8, "assistant_2"),
      },
      { type: "turn_completed", meta: meta(9, "turn-1") },
    ]
    const first = timelineFromEvents(events)
    const second = timelineFromEvents(events)
    expect(second.entries.map((entry) => entry.id)).toEqual(first.entries.map((entry) => entry.id))
    expect(second.entries.map((entry) => entry.role)).toEqual(first.entries.map((entry) => entry.role))
    expect(second.entries.map((entry) => entry.text)).toEqual(first.entries.map((entry) => entry.text))
    expect(second.plan).toEqual(first.plan)

    const toolIdx = first.entries.findIndex((entry) => entry.role === "tool")
    const finalIdx = first.entries.findIndex((entry) => entry.role === "assistant" && entry.text === "All set now")
    expect(toolIdx).toBeGreaterThanOrEqual(0)
    expect(finalIdx).toBeGreaterThan(toolIdx)
  })
})