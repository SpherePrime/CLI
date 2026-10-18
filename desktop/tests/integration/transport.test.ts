import { expect, it } from 'vitest'
import { createServer } from 'node:http'
import { randomUUID } from 'node:crypto'
import { PrimeTransport } from '../../src/main/prime/transport'

it('exchanges real HTTP and SSE messages through a local named pipe', async () => {
  const path =
    process.platform === 'win32'
      ? `\\\\.\\pipe\\prime-test-${randomUUID()}`
      : `/tmp/prime-test-${randomUUID()}.sock`
  const server = createServer((request, response) => {
    if (request.url === '/events') {
      response.writeHead(200, { 'Content-Type': 'text/event-stream' })
      response.write('data: {"type":"message","text":"Привет"}\n\n')
    } else {
      response.setHeader('Content-Type', 'application/json')
      response.end(JSON.stringify({ alive: true }))
    }
  })
  await new Promise<void>((resolve) => server.listen(path, resolve))
  const transport = new PrimeTransport(path)
  let unsubscribe: (() => void) | undefined
  try {
    expect(await transport.request('GET', '/health')).toEqual({ alive: true })
    const event = await new Promise((resolve) => {
      unsubscribe = transport.subscribe('/events', resolve, () => {})
    })
    expect(event).toEqual({ type: 'message', text: 'Привет' })
  } finally {
    unsubscribe?.()
    server.closeAllConnections()
    await new Promise<void>((resolve) => server.close(() => resolve()))
  }
})
