import { describe, it, expect } from 'vitest'
import { EventEmitter } from 'node:events'
import type { Client, ClientChannel, SFTPWrapper } from 'ssh2'
import { RemoteConnection } from '../../src/main/remote/connection'
import type { RemoteProfile } from '../../src/shared/remote'
describe('remote command deadlines', () => {
  it('removes output listeners when an open command times out', async () => {
    let closed = false
    const stream = Object.assign(new EventEmitter(), {
      stderr: new EventEmitter(),
      close() {
        closed = true
      }
    })
    const client = {
      exec(_command: string, callback: (error: Error | null, stream: ClientChannel) => void) {
        callback(null, stream as unknown as ClientChannel)
      }
    }
    const connection = new RemoteConnection(
      client as unknown as Client,
      {} as SFTPWrapper,
      {} as RemoteProfile
    )
    await expect(connection.exec('sleep 10', { timeout: 1 })).rejects.toThrow(
      'remote.commandTimeout'
    )
    expect(closed).toBe(true)
    expect(stream.listenerCount('data')).toBe(0)
    expect(stream.stderr.listenerCount('data')).toBe(0)
  })
  it('closes a channel returned after its command deadline', async () => {
    let closed = false
    const stream = Object.assign(new EventEmitter(), {
      stderr: new EventEmitter(),
      close() {
        closed = true
      }
    })
    let deliver!: () => void
    const client = {
      exec(_command: string, callback: (error: Error | null, stream: ClientChannel) => void) {
        deliver = () => callback(null, stream as unknown as ClientChannel)
      }
    }
    const connection = new RemoteConnection(
      client as unknown as Client,
      {} as SFTPWrapper,
      {} as RemoteProfile
    )
    await expect(connection.exec('sleep 10', { timeout: 1 })).rejects.toThrow(
      'remote.commandTimeout'
    )
    deliver()
    expect(closed).toBe(true)
    expect(stream.listenerCount('data')).toBe(0)
  })
})
