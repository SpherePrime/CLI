const { spawn } = require('node:child_process')
const readline = require('node:readline')

const marker = process.argv[2]
spawn(process.execPath, ['-e', 'setInterval(() => {}, 1000)', marker], { stdio: 'ignore' })
setInterval(() => {}, 1000)

const reply = (id, result) => process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id, result }) + '\n')
readline.createInterface({ input: process.stdin }).on('line', (line) => {
  let message
  try {
    message = JSON.parse(line)
  } catch {
    return
  }
  if (message.id === undefined) return
  if (message.method === 'initialize')
    reply(message.id, {
      protocolVersion: message.params?.protocolVersion || '2025-06-18',
      capabilities: { tools: {} },
      serverInfo: { name: 'child-server', version: '1.0.0' }
    })
  else if (message.method === 'tools/list') reply(message.id, { tools: [] })
  else if (message.method === 'prompts/list') reply(message.id, { prompts: [] })
  else if (message.method === 'resources/list') reply(message.id, { resources: [] })
  else reply(message.id, {})
})
