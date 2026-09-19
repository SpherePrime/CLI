import { describe, expect, test } from "bun:test"
import { nearBottom, shouldAutoScroll } from "./autoscroll"

describe("nearBottom", () => {
  test("at the bottom is near bottom", () => {
    expect(nearBottom(1000, 1000, 300)).toBe(true)
  })

  test("4px above the tolerance boundary is still near bottom", () => {
    // boundary = scrollHeight - viewHeight - tolerance = 1000 - 300 - 32 = 668
    expect(nearBottom(672, 1000, 300)).toBe(true)
  })

  test("40px above the boundary is not near bottom", () => {
    expect(nearBottom(664, 1000, 300)).toBe(false)
  })

  test("respects a custom tolerance", () => {
    expect(nearBottom(640, 1000, 300, 80)).toBe(true)
    expect(nearBottom(640, 1000, 300)).toBe(false)
  })
})

describe("shouldAutoScroll", () => {
  test("user at bottom with new entries keeps following (newUpdates stays 0)", () => {
    const next = shouldAutoScroll({ atBottom: true, newUpdates: 0 }, true, 2)
    expect(next).toEqual({ atBottom: true, newUpdates: 0 })
  })

  test("user scrolled up with 3 new entries accumulates 3", () => {
    const next = shouldAutoScroll({ atBottom: false, newUpdates: 0 }, false, 3)
    expect(next).toEqual({ atBottom: false, newUpdates: 3 })
  })

  test("scrolling back to bottom resets accumulated updates", () => {
    const scrolledUp = shouldAutoScroll({ atBottom: false, newUpdates: 0 }, false, 1)
    expect(scrolledUp.newUpdates).toBe(1)
    const backAtBottom = shouldAutoScroll(scrolledUp, true, 0)
    expect(backAtBottom).toEqual({ atBottom: true, newUpdates: 0 })
  })

  test("entries shrank while at bottom resets to 0", () => {
    const next = shouldAutoScroll({ atBottom: true, newUpdates: 0 }, true, -2)
    expect(next).toEqual({ atBottom: true, newUpdates: 0 })
  })

  test("entries shrank while scrolled up does not reset", () => {
    const next = shouldAutoScroll({ atBottom: false, newUpdates: 4 }, false, -1)
    expect(next).toEqual({ atBottom: false, newUpdates: 4 })
  })

  test("accumulated updates are clamped at 0 when scrolled up and nothing grew", () => {
    const next = shouldAutoScroll({ atBottom: false, newUpdates: 0 }, false, 0)
    expect(next).toEqual({ atBottom: false, newUpdates: 0 })
  })

  test("multiple grows while scrolled up keep accumulating", () => {
    let state = shouldAutoScroll({ atBottom: false, newUpdates: 0 }, false, 2)
    state = shouldAutoScroll(state, false, 5)
    expect(state).toEqual({ atBottom: false, newUpdates: 7 })
  })
})
