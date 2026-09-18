import { describe, expect, test } from "bun:test"
import { contentLayout } from "./content-layout"

describe("contentLayout", () => {
  test("narrow terminals fill the full width with minimal padding", () => {
    expect(contentLayout(100)).toEqual({ maxContent: 96, sidePad: 2 })
  })

  test("wide terminals center a capped content column", () => {
    expect(contentLayout(200)).toEqual({ maxContent: 120, sidePad: 40 })
    expect(contentLayout(124)).toEqual({ maxContent: 120, sidePad: 2 })
  })

  test("very narrow terminals never collapse to negative size", () => {
    expect(contentLayout(4)).toEqual({ maxContent: 0, sidePad: 2 })
    expect(contentLayout(2)).toEqual({ maxContent: 0, sidePad: 2 })
  })
})