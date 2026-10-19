import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http'
import { randomBytes, timingSafeEqual } from 'node:crypto'
import type { BrowserPanel } from './panel'
import { browserTools } from './tools'
import type { WorkbenchTool, WorkbenchToolContext } from '../workbench/tools'

const maxBody = 1024 * 1024
const protocolVersion = '2025-03-26'

export type BrowserMcpServer = { url: string; token: string; close: () => Promise<void> }

function readBody(request: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    let size = 0
    const chunks: Buffer[] = []
    request.on('data', (chunk: Buffer) => {
      size += chunk.length
      if (size > maxBody) {
        reject(new Error('request too large'))
        request.destroy()
        return
      }
      chunks.push(chunk)
    })
    request.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')))
    request.on('error', reject)
  })
}

function authorized(request: IncomingMessage, token: string) {
  const header = Buffer.from(String(request.headers.authorization || ''))
  const expected = Buffer.from(`Bearer ${token}`)
  return header.length === expected.length && timingSafeEqual(header, expected)
}

function send(response: ServerResponse, status: number, body?: unknown) {
  response.writeHead(status, body === undefined ? {} : { 'Content-Type': 'application/json' })
  response.end(body === undefined ? undefined : JSON.stringify(body))
}

type ServerOptions = {
  tools?: () => WorkbenchTool[]
  resolveWorkspace?: (path: string) => string | undefined
}

async function handle(
  panel: BrowserPanel,
  message: any,
  options: ServerOptions,
  context: WorkbenchToolContext
) {
  const { id, method, params } = message
  const reply = (result: unknown) => ({ jsonrpc: '2.0', id, result })
  const fail = (code: number, text: string) => ({
    jsonrpc: '2.0',
    id,
    error: { code, message: text }
  })
  if (method === 'initialize')
    return reply({
      protocolVersion: params?.protocolVersion || protocolVersion,
      capabilities: { tools: { listChanged: false } },
      serverInfo: { name: 'prime-desktop-browser', version: '1.0.0' },
      instructions:
        'Prime Desktop project tools and built-in browser. Use desktop_capabilities to discover project automation, app launch, terminal, team, working context and GitHub tools. Maintain working context automatically as the task evolves. Use automation_save for requested schedules; new rules default enabled. Use terminal_create for a new agent-controlled project terminal. Existing user terminals require explicit handover. Browser navigation, DOM snapshots/evaluation, input and screenshots use the visible built-in browser. Use window_list and window_request_access to request a specific window, then wait for an explicit view/control grant. window_action may act only on the approved window and chat. Tools are bound to the current project; do not supply workspace IDs or roots. Create agent teams only when the user explicitly asks for delegation.'
    })
  if (method === 'ping') return reply({})
  const extraTools = context.workspaceId ? options.tools?.() || [] : []
  if (method === 'tools/list')
    return reply({
      tools: [...browserTools, ...extraTools].map(({ name, description, inputSchema }) => ({
        name,
        description,
        inputSchema
      }))
    })
  if (method === 'tools/call') {
    const tool = browserTools.find((item) => item.name === params?.name)
    const extraTool = extraTools.find((item) => item.name === params?.name)
    if (!tool && !extraTool) return fail(-32602, `Unknown tool ${params?.name}`)
    try {
      return reply({
        content: tool
          ? await tool.run(panel, params?.arguments || {})
          : await extraTool!.run(params?.arguments || {}, context)
      })
    } catch (error: any) {
      return reply({
        content: [{ type: 'text', text: error?.message || String(error) }],
        isError: true
      })
    }
  }
  return fail(-32601, `Method ${method} not found`)
}

export async function startBrowserMcpServer(
  panel: BrowserPanel,
  options: ServerOptions = {}
): Promise<BrowserMcpServer> {
  const token = randomBytes(32).toString('hex')
  const server: Server = createServer(async (request, response) => {
    if (!request.url?.startsWith('/mcp')) return send(response, 404)
    if (!authorized(request, token)) return send(response, 401)
    if (request.method === 'DELETE') return send(response, 200)
    if (request.method !== 'POST') return send(response, 405)
    try {
      const encodedRoot = request.headers['x-prime-workspace']
      const root =
        typeof encodedRoot === 'string' && /^[A-Za-z0-9_-]+$/.test(encodedRoot)
          ? Buffer.from(encodedRoot, 'base64url').toString('utf8')
          : ''
      const context = { workspaceId: root ? options.resolveWorkspace?.(root) : undefined }
      const payload = JSON.parse(await readBody(request))
      const messages = Array.isArray(payload) ? payload : [payload]
      const replies = []
      for (const message of messages)
        if (message?.id !== undefined && message?.method)
          replies.push(await handle(panel, message, options, context))
      if (!replies.length) return send(response, 202)
      send(response, 200, Array.isArray(payload) ? replies : replies[0])
    } catch (error: any) {
      send(response, 400, {
        jsonrpc: '2.0',
        id: null,
        error: { code: -32700, message: error?.message || 'parse error' }
      })
    }
  })
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject)
    server.listen(0, '127.0.0.1', () => resolve())
  })
  const address = server.address()
  const port = typeof address === 'object' && address ? address.port : 0
  return {
    url: `http://127.0.0.1:${port}/mcp`,
    token,
    close: () => new Promise((resolve) => server.close(() => resolve()))
  }
}
