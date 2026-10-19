import { z } from 'zod'
import type { TerminalManager } from '../terminal/manager'
import type { RemoteConnection } from '../remote/connection'

const terminalRequest = z.discriminatedUnion('action', [
  z.object({ action: z.literal('list') }),
  z.object({
    action: z.literal('create'),
    options: z
      .object({
        cols: z.number().int().min(20).max(500).optional(),
        rows: z.number().int().min(5).max(300).optional(),
        shell: z.string().min(1).max(4096).optional(),
        args: z.array(z.string().max(8192)).max(128).optional(),
        title: z.string().max(120).optional()
      })
      .optional()
  }),
  z.object({ action: z.literal('read'), id: z.string().uuid() }),
  z.object({
    action: z.literal('write'),
    id: z.string().uuid(),
    data: z.string().max(1024 * 1024)
  }),
  z.object({ action: z.literal('report'), id: z.string().uuid(), data: z.string().max(256) }),
  z.object({
    action: z.literal('resize'),
    id: z.string().uuid(),
    cols: z.number().int().min(20).max(500),
    rows: z.number().int().min(5).max(300)
  }),
  z.object({ action: z.literal('close'), id: z.string().uuid() }),
  z.object({ action: z.literal('owner'), id: z.string().uuid(), owner: z.enum(['user', 'agent']) })
])

export function terminalOperation(
  manager: TerminalManager,
  workspaceId: string,
  root: string,
  input: unknown,
  remote?: RemoteConnection
) {
  const request = terminalRequest.parse(input)
  switch (request.action) {
    case 'list':
      return manager.list(workspaceId)
    case 'create':
      return remote
        ? manager.createRemote(workspaceId, remote, request.options)
        : manager.create(workspaceId, root, request.options)
    case 'read':
      return manager.read(request.id, workspaceId)
    case 'write':
      return manager.write(request.id, request.data, 'user', workspaceId)
    case 'report':
      return manager.report(request.id, request.data, workspaceId)
    case 'resize':
      return manager.resize(request.id, request.cols, request.rows, workspaceId)
    case 'close':
      return manager.close(request.id, workspaceId)
    case 'owner':
      return manager.setOwner(request.id, request.owner, workspaceId)
  }
}
