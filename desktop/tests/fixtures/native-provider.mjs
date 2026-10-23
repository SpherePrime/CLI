import { createServer } from 'node:http'

export async function startNativeProvider() {
  const requests = []
  const server = createServer(async (request, response) => {
    if (request.url === '/fixture') {
      response.writeHead(200, { 'Content-Type': 'text/html' })
      response.end('<!doctype html><title>Native browser fixture</title><input aria-label="Fixture input"><h1>Native browser fixture</h1>')
      return
    }
    let raw = ''
    for await (const chunk of request) raw += chunk
    const input = raw ? JSON.parse(raw) : {}
    requests.push(input)
    const message = 'Native engine response'
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
