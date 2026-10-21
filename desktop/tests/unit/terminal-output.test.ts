import { describe, expect, it } from 'vitest'
import { TerminalOutput } from '../../src/main/terminal/output'
import { TerminalReplay } from '../../src/renderer/src/features/terminal/replay'

describe('bounded terminal journal', () => {
  it('releases an oversized incoming buffer instead of retaining its backing allocation', () => {
    const output = new TerminalOutput()
    output.append('x'.repeat(16 * 1024 * 1024))
    const chunks = (output as unknown as { chunks: Buffer[] }).chunks
    expect(output.read().length).toBe(1024 * 1024)
    expect(chunks.every((chunk) => chunk.buffer.byteLength <= 1024 * 1024)).toBe(true)
  })
  it('retains at most one MiB of UTF-8 output without broken characters', () => {
    const output = new TerminalOutput()
    output.append('x'.repeat(1024 * 1024))
    output.append('Привет'.repeat(120000))
    const value = output.read()
    expect(Buffer.byteLength(value)).toBeLessThanOrEqual(1024 * 1024)
    expect(value.endsWith('Привет')).toBe(true)
    expect(value.includes('\ufffd')).toBe(false)
    expect(output.truncated).toBe(true)
  })

  it('keeps ordinary ANSI output exactly', () => {
    const output = new TerminalOutput()
    output.append('\x1b[31mhello')
    output.append('\x1b[0m\r\n')
    expect(output.read()).toBe('\x1b[31mhello\x1b[0m\r\n')
    expect(output.truncated).toBe(false)
  })
})

describe('terminal subscription replay', () => {
  it('restores the journal and applies only events newer than the read watermark', () => {
    const replay = new TerminalReplay()
    expect(replay.receive({ action: 'data', id: 'a', sequence: 2, data: 'duplicate' })).toBe('')
    expect(replay.receive({ action: 'data', id: 'a', sequence: 4, data: 'new' })).toBe('')
    expect(replay.restore({ output: 'history', sequence: 3 })).toBe('historynew')
    expect(replay.receive({ action: 'data', id: 'a', sequence: 4, data: 'new' })).toBe('')
    expect(replay.receive({ action: 'data', id: 'a', sequence: 5, data: 'next' })).toBe('next')
  })

  it('bounds pending output while a read response is delayed', () => {
    const replay = new TerminalReplay()
    for (let sequence = 1; sequence <= 1200; sequence += 1)
      replay.receive({ action: 'data', id: 'a', sequence, data: 'x'.repeat(1024) })
    expect(replay.restore({ output: '', sequence: 0 }).length).toBeLessThanOrEqual(1024 * 1024)
  })
})
