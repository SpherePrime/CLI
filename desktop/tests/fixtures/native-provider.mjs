import { createServer } from 'node:http'

export async function startNativeProvider(options = {}) {
  const requests = []
  const server = createServer(async (request, response) => {
    if (request.url === '/fixture') {
      response.writeHead(200, { 'Content-Type': 'text/html' })
      response.end('<!doctype html><title>Native browser fixture</title><input aria-label="Fixture input"><h1>Native browser fixture</h1>')
      return
    }
    if (request.method === 'GET' && !request.url?.endsWith('/models')) {
      response.writeHead(404)
      response.end()
      return
    }
    let raw = ''
    for await (const chunk of request) raw += chunk
    const input = raw ? JSON.parse(raw) : {}
    requests.push({ ...input, fixturePath: request.url })
    if (request.url === '/mcp') {
      if (!input.id) { response.writeHead(202); response.end(); return }
      response.setHeader('Content-Type', 'application/json')
      const result = input.method === 'initialize'
        ? { protocolVersion: '2024-11-05', capabilities: { tools: {} }, serverInfo: { name: 'ui-fixture', version: '1' } }
        : input.method === 'tools/list' ? { tools: [] } : {}
      response.end(JSON.stringify({ jsonrpc: '2.0', id: input.id, result }))
      return
    }
    if (request.url?.endsWith('/models')) {
      response.setHeader('Content-Type', 'application/json')
      response.end(JSON.stringify({ object: 'list', data: (options.models || ['native-fixture']).map(id => ({ id, object: 'model', owned_by: 'fixture' })) }))
      return
    }
    const message = options.reply ? options.reply(input) : 'Native engine response'
    const usage = { prompt_tokens: 10, completion_tokens: 5, total_tokens: 15 }
    if (input.stream) {
      response.writeHead(200, { 'Content-Type': 'text/event-stream' })
      response.end(
        'data: ' + JSON.stringify({ id: 'fixture', object: 'chat.completion.chunk', model: 'native-fixture', choices: [{ index: 0, delta: { content: message }, finish_reason: null }] }) + '\n\n' +
        'data: ' + JSON.stringify({ id: 'fixture', object: 'chat.completion.chunk', model: 'native-fixture', choices: [{ index: 0, delta: {}, finish_reason: 'stop' }], usage }) + '\n\ndata: [DONE]\n\n'
      )
    } else {
      response.writeHead(200, { 'Content-Type': 'application/json' })
      response.end(JSON.stringify({ id: 'fixture', object: 'chat.completion', model: 'native-fixture', choices: [{ index: 0, message: { role: 'assistant', content: message }, finish_reason: 'stop' }], usage }))
    }
  })
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
  return { server, requests, url: `http://127.0.0.1:${server.address().port}` }
}
