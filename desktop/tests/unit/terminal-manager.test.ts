import { afterEach, describe, expect, it } from 'vitest'
import { TerminalManager } from '../../src/main/terminal/manager'
import type { DesktopEvent } from '../../src/shared/contracts'

const managers: TerminalManager[] = []
const until = async (check: () => boolean, timeout = 12000) => {
  const deadline = Date.now() + timeout
  while (!check()) {
    if (Date.now() > deadline) throw new Error('Terminal condition timed out')
    await new Promise((resolve) => setTimeout(resolve, 35))
  }
}
const setup = (maxSessions = 12) => {
  const events: DesktopEvent[] = []
  const manager = new TerminalManager({ emit: (event) => events.push(event), maxSessions })
  managers.push(manager)
  return { manager, events }
}
afterEach(async () => {
  await Promise.all(managers.splice(0).map((manager) => manager.dispose()))
})

describe('terminal manager with real PTYs', () => {
  it('round trips input, resizes and reports a real exit with ordered events', async () => {
    const { manager, events } = setup()
    const terminal = manager.create('workspace-a', process.cwd(), { cols: 80, rows: 24 })
    expect(terminal.id).toMatch(/^[a-f\d-]{36}$/)
    expect(terminal.owner).toBe('user')
    expect(terminal.status).toBe('running')
    const resized = manager.resize(terminal.id, 110, 32, 'workspace-a')
    expect([resized.cols, resized.rows]).toEqual([110, 32])
    manager.write(
      terminal.id,
      process.platform === 'win32'
        ? "Write-Output ('PTY_'+'ROUNDTRIP')\r"
        : "printf 'PTY_%s\\n' ROUNDTRIP\n",
      'user',
      'workspace-a'
    )
    await until(() => manager.read(terminal.id).output.includes('PTY_ROUNDTRIP'))
    manager.write(terminal.id, process.platform === 'win32' ? 'exit 7\r' : 'exit 7\n', 'user')
    await until(() => manager.read(terminal.id).terminal.status === 'exited')
    expect(manager.read(terminal.id).terminal.exitCode).toBe(7)
    const sequences = events.map(
      (event) => event.payload.sequence ?? event.payload.terminal.sequence
    )
    expect(sequences).toEqual([...new Set(sequences)].sort((left, right) => left - right))
    expect(events.some((event) => event.payload.action === 'exit')).toBe(true)
    expect(
      events.every((event) => event.type === 'terminal' && event.workspaceId === 'workspace-a')
    ).toBe(true)
    expect(() => manager.write(terminal.id, 'echo forbidden', 'user')).toThrow(/exited/i)
  })

  it('requires an explicit user handover before agent writes', () => {
    const { manager } = setup()
    const terminal = manager.create('workspace-a', process.cwd())
    expect(() => manager.write(terminal.id, 'echo unsafe\r', 'agent')).toThrow(/ownership/i)
    manager.setOwner(terminal.id, 'agent', 'workspace-a')
    expect(() => manager.write(terminal.id, '\r', 'agent')).not.toThrow()
    expect(() => manager.write(terminal.id, '\r', 'user')).toThrow(/ownership/i)
    manager.setOwner(terminal.id, 'user')
    expect(() => manager.write(terminal.id, '\r', 'agent')).toThrow(/ownership/i)
    expect(() => manager.write(terminal.id, '\r', 'user')).not.toThrow()
    expect(() => manager.write(terminal.id, '\r', 'invalid' as 'user')).toThrow(/source/i)
    expect(() => manager.setOwner(terminal.id, 'invalid' as 'user')).toThrow(/owner/i)
  })

  it('rejects operations from a different workspace and removes closed sessions', async () => {
    const { manager } = setup()
    const terminal = manager.create('workspace-a', process.cwd())
    expect(manager.list('workspace-b')).toEqual([])
    expect(() => manager.read(terminal.id, 'workspace-b')).toThrow(/workspace/i)
    expect(() => manager.write(terminal.id, '\r', 'user', 'workspace-b')).toThrow(/workspace/i)
    expect(() => manager.resize(terminal.id, 80, 24, 'workspace-b')).toThrow(/workspace/i)
    expect(() => manager.setOwner(terminal.id, 'agent', 'workspace-b')).toThrow(/workspace/i)
    await expect(manager.close(terminal.id, 'workspace-b')).rejects.toThrow(/workspace/i)
    await manager.close(terminal.id, 'workspace-a')
    expect(manager.list()).toEqual([])
    expect(() => manager.read(terminal.id)).toThrow(/not found/i)
    expect(() => manager.write(terminal.id, '\r', 'user')).toThrow(/not found/i)
  })

  it('enforces a session limit, validates dimensions and disposes every shell', async () => {
    const { manager } = setup(2)
    const first = manager.create('workspace-a', process.cwd(), { cols: 1, rows: 10000 })
    expect([first.cols, first.rows]).toEqual([20, 300])
    const second = manager.create('workspace-b', process.cwd())
    expect(() => manager.create('workspace-a', process.cwd())).toThrow(/limit/i)
    expect(() => manager.resize(first.id, NaN, 24)).toThrow(/dimensions/i)
    expect(() => manager.create('', process.cwd())).toThrow(/workspace/i)
    expect(() => manager.create('workspace-a', 'does-not-exist')).toThrow()
    await manager.dispose()
    for (const terminal of [first, second]) {
      expect(() => process.kill(terminal.pid, 0)).toThrow()
    }
    expect(manager.list()).toEqual([])
    expect(() => manager.create('workspace-a', process.cwd())).toThrow(/disposed/i)
  })

  it.skipIf(process.platform !== 'win32')(
    'closes child processes launched from PowerShell',
    async () => {
      const { manager } = setup()
      const terminal = manager.create('workspace-a', process.cwd())
      manager.write(
        terminal.id,
        "$child = Start-Process powershell.exe -ArgumentList '-NoLogo -NoProfile -Command Start-Sleep -Seconds 60' -PassThru; Write-Output ('CHILD_PID=' + $child.Id)\r",
        'user'
      )
      await until(() => /CHILD_PID=\d+/.test(manager.read(terminal.id).output))
      const childPid = Number(manager.read(terminal.id).output.match(/CHILD_PID=(\d+)/)![1])
      expect(() => process.kill(childPid, 0)).not.toThrow()
      await manager.close(terminal.id)
      await until(() => {
        try {
          process.kill(childPid, 0)
          return false
        } catch {
          return true
        }
      })
    }
  )

  it.skipIf(process.platform !== 'win32')(
    'awaits detached child cleanup after the root PowerShell exits naturally',
    async () => {
      const { manager } = setup()
      const terminal = manager.create('workspace-a', process.cwd())
      let childPid = 0
      try {
        manager.write(
          terminal.id,
          "$child = Start-Process powershell.exe -WindowStyle Hidden -ArgumentList '-NoLogo -NoProfile -Command Start-Sleep -Seconds 60' -PassThru; Write-Output ('ORPHAN_'+'PID=' + $child.Id)\r",
          'user'
        )
        await until(() => /ORPHAN_PID=\d+/.test(manager.read(terminal.id).output))
        childPid = Number(manager.read(terminal.id).output.match(/ORPHAN_PID=(\d+)/)![1])
        expect(() => process.kill(childPid, 0)).not.toThrow()
        manager.write(terminal.id, 'exit 0\r', 'user')
        await until(() => manager.read(terminal.id).terminal.status === 'exited')
        await manager.close(terminal.id)
        expect(() => process.kill(childPid, 0)).toThrow()
      } finally {
        if (childPid) {
          try {
            process.kill(childPid)
          } catch {}
        }
      }
    }
  )

  it('rebinds a live terminal to the recovered workspace without replacing its process', () => {
    const { manager, events } = setup()
    const terminal = manager.create('workspace-old', process.cwd())
    manager.rebindWorkspace('workspace-old', 'workspace-new')
    const rebound = manager.list('workspace-new')[0]
    expect(rebound.id).toBe(terminal.id)
    expect(rebound.pid).toBe(terminal.pid)
    expect(rebound.owner).toBe('user')
    expect(rebound.status).toBe('running')
    expect(manager.list('workspace-old')).toEqual([])
    expect(() => manager.read(terminal.id, 'workspace-old')).toThrow(/workspace/i)
    expect(manager.read(terminal.id, 'workspace-new').terminal).toEqual(rebound)
    expect(events.at(-1)).toMatchObject({
      workspaceId: 'workspace-new',
      type: 'terminal',
      payload: { action: 'state', terminal: rebound }
    })
    expect(() => process.kill(rebound.pid, 0)).not.toThrow()
    expect(() => manager.rebindWorkspace('workspace-new', '')).toThrow(/workspace/i)
  })
})
