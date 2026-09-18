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
      tool: { name: "shell", state: "ok", durationMs: 42, fileChanges: "" },
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

  test("file changes are summarized per tool result", () => {
    const entries = apply([
      toolStarted(1, "call_1", "edit"),
      toolResult(2, "call_1", "edit", [
        { path: "src/a.ts", change: "write", additions: 3, deletions: 0 },
      ]),
    ])
    expect(entries[0]!.tool!.fileChanges).toBe("write src/a.ts (+3)")
  })

  test("activity changes render as running system entries", () => {
    const entries = apply([
      { type: "activity_changed", meta: meta(1, "prep_0"), activity: "Preparing read_file", kind: null },
      { type: "activity_changed", meta: meta(2, "prep_0"), activity: "switching to read_file", kind: null },
    ])
    expect(entries).toHaveLength(1)
    expect(entries[0]).toMatchObject({ role: "system", text: "switching to read_file", running: true })
  })
})