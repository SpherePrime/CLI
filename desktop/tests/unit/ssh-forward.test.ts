import { readFile } from 'node:fs/promises'
import { createServer } from 'node:http'
import { Client } from 'ssh2'
import { expect, it } from 'vitest'
import { createRemoteForward } from '../../src/main/remoteRuntime/forward'
import { createSshLinuxFixture, detectSshLinuxFixture } from '../fixtures/ssh-linux'

const support = await detectSshLinuxFixture()

async function httpFixture(body: string) {
  let holdStarted: () => void = () => {}
  const held = new Promise<void>((ready) => {
    holdStarted = ready
  })
  const server = createServer((request, response) => {
    if (request.url === '/hold') {
      response.write(body)
      holdStarted()
    } else response.end(body)
  })
  await new Promise<void>((ready) => server.listen(0, '127.0.0.1', ready))
  return {
    port: (server.address() as import('node:net').AddressInfo).port,
    held,
    close: () =>
      new Promise<void>((ready, reject) =>
        server.close((error) => (error ? reject(error) : ready()))
      )
  }
}

it.skipIf(!support.available)(
  'routes two explicit loopback services independently and cleans up on close and SSH loss',
  async () => {
    const fixture = await createSshLinuxFixture()
    const client = new Client()
    const first = await httpFixture('first')
    const second = await httpFixture('second')
    try {
      const privateKey = await readFile(fixture.keyPath)
      await new Promise<void>((ready, reject) => {
        client.once('ready', ready).on('error', reject)
        client.connect({
          host: fixture.host,
          port: fixture.port,
          username: fixture.user,
          privateKey
        })
      })
      const otherListener = () => {}
      client.on('tcp connection', otherListener)
      const baseline = client.listenerCount('tcp connection')
      const forwardA = await createRemoteForward(client, first.port)
      const forwardB = await createRemoteForward(client, second.port)
      const get = (port: number) =>
        fixture.execute(
          `/usr/local/bin/node -e 'require("node:http").get("http://127.0.0.1:${port}",r=>r.pipe(process.stdout)).on("error",()=>process.exit(2))'`
        )
      expect(await get(forwardA.port)).toBe('first')
      expect(await get(forwardB.port)).toBe('second')
      expect(client.listenerCount('tcp connection')).toBe(baseline + 1)
      let rejected = 0
      let accepted = false
      for (const [destIP, destPort] of [
        ['0.0.0.0', forwardA.port],
        ['::1', forwardB.port],
        ['127.0.0.1', 0]
      ] as const) {
        client.emit(
          'tcp connection',
          { destIP, destPort, srcIP: '127.0.0.1', srcPort: 1 },
          () => {
            accepted = true
            throw new Error('Unexpected forwarded connection')
          },
          () => {
            rejected++
          }
        )
      }
      expect(accepted).toBe(false)
      expect(rejected).toBe(3)
      const heldRequest = fixture.execute(
        `/usr/local/bin/node -e 'require("node:http").get("http://127.0.0.1:${forwardA.port}/hold",r=>{r.resume();r.on("aborted",()=>process.exit(0));r.on("end",()=>process.exit(0));r.on("error",()=>process.exit(0))}).on("error",()=>process.exit(0))'`
      )
      await first.held
      await forwardA.close()
      await heldRequest
      await expect(get(forwardA.port)).rejects.toThrow()
      expect(await get(forwardB.port)).toBe('second')
      await forwardB.close()
      expect(client.listenerCount('tcp connection')).toBe(baseline)
      const forwardC = await createRemoteForward(client, first.port)
      const closed = new Promise<void>((ready) => client.once('close', ready))
      client.destroy()
      await closed
      expect(client.listenerCount('tcp connection')).toBe(baseline)
      await forwardC.close()
    } finally {
      client.destroy()
      await fixture.close()
      await Promise.all([first.close(), second.close()])
    }
  },
  60000
)

it.each([0, -1, 65536, 1.5, Number.NaN])('rejects invalid local port %s', async (port) => {
  await expect(createRemoteForward(new Client(), port)).rejects.toThrow(/port/i)
})

it('removes its listeners when forwarding cannot start on a disconnected client', async () => {
  const client = new Client()
  const baseline = client.eventNames().map((name) => [name, client.listenerCount(name)])
  await expect(createRemoteForward(client, 12345)).rejects.toThrow()
  expect(client.eventNames().map((name) => [name, client.listenerCount(name)])).toEqual(baseline)
})
