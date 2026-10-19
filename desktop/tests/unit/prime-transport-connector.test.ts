import { expect, it } from 'vitest'
import { createServer } from 'node:http'
import { connect } from 'node:net'
import type { AddressInfo } from 'node:net'
import { PrimeTransport } from '../../src/main/prime/transport'

it('carries requests and event streams through an asynchronous connection factory', async () => {
  const server = createServer((request, response) => {
    if (request.url === '/events') {
      response.writeHead(200, { 'Content-Type': 'text/event-stream' })
      response.write('data: {"type":"remote","payload":{"ready":true}}\n\n')
    } else response.end(JSON.stringify({ path: request.url }))
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const port = (server.address() as AddressInfo).port
  let connections = 0
  const transport = new PrimeTransport('remote-unix-socket', callback => {
    connections++
    const socket = connect(port, '127.0.0.1')
    socket.once('error', error => callback(error))
    socket.once('connect', () => callback(null, socket))
  })
  let unsubscribe = () => {}
  try {
    expect(await transport.request('GET', '/health')).toEqual({ path: '/health' })
    const events: unknown[] = []
    unsubscribe = transport.subscribe('/events', event => events.push(event), () => {})
    await expect.poll(() => events).toEqual([{ type: 'remote', payload: { ready: true } }])
    expect(connections).toBe(2)
  } finally {
    unsubscribe()
    server.closeAllConnections()
    await new Promise<void>(resolve => server.close(() => resolve()))
  }
})
