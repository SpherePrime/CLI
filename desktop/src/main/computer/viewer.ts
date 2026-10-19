import { desktopCapturer } from 'electron'
import { z } from 'zod'
import type { DesktopEvent } from '../../shared/contracts'
import type { WindowLogEntry, WindowRequest, WindowShare, WindowState } from '../../shared/computer'
import type { WorkbenchTool } from '../workbench/tools'
import { listWindows, windowInfo } from './win32'

type Options = {
  emit(event: DesktopEvent): void
  busySessions(workspaceId: string): Promise<string[]>
}

const hwnd = z.string().regex(/^\d{1,20}$/)
const requestSchema = z.discriminatedUnion('action', [
  z.object({ action: z.enum(['list', 'state', 'revoke']) }),
  z.object({ action: z.literal('snapshot'), hwnd }),
  z.object({ action: z.literal('share'), hwnd, sessionId: z.string().min(1).max(200) })
])

export class WindowViewer {
  private shares = new Map<string, WindowShare>()
  private logs = new Map<string, WindowLogEntry[]>()

  constructor(private options: Options) {}

  private state(workspaceId: string): WindowState {
    return { share: this.shares.get(workspaceId), log: [...(this.logs.get(workspaceId) ?? [])].reverse() }
  }

  private publish(workspaceId: string) {
    this.options.emit({ workspaceId, type: 'windows', payload: this.state(workspaceId) })
  }

  private record(workspaceId: string, entry: Omit<WindowLogEntry, 'time'>) {
    const log = this.logs.get(workspaceId) ?? []
    log.push({ ...entry, time: Date.now() })
    if (log.length > 100) log.splice(0, log.length - 100)
    this.logs.set(workspaceId, log)
    this.publish(workspaceId)
  }

  async capture(target: string) {
    const info = windowInfo(target)
    if (!info) throw new Error('Окно закрыто')
    const width = Math.max(1, Math.min(1920, info.bounds.width))
    const height = Math.max(1, Math.round((info.bounds.height / Math.max(1, info.bounds.width)) * width))
    const sources = await desktopCapturer.getSources({ types: ['window'], thumbnailSize: { width, height } })
    const source = sources.find((item) => item.id.startsWith(`window:${target}:`))
    if (!source || source.thumbnail.isEmpty()) throw new Error('Не удалось получить снимок окна. Окно не должно быть свёрнуто.')
    return { info, png: source.thumbnail.toPNG() }
  }

  async handle(workspaceId: string, input: unknown) {
    const request = requestSchema.parse(input) as WindowRequest
    switch (request.action) {
      case 'list':
        return listWindows().filter((item) => !item.title.startsWith('Prime Desktop'))
      case 'state':
        return this.state(workspaceId)
      case 'snapshot': {
        const { info, png } = await this.capture(request.hwnd)
        this.record(workspaceId, { title: info.title, source: 'user', ok: true })
        return { info, image: `data:image/png;base64,${png.toString('base64')}` }
      }
      case 'share': {
        const info = windowInfo(request.hwnd)
        if (!info) throw new Error('Окно закрыто')
        this.shares.set(workspaceId, {
          workspaceId,
          sessionId: request.sessionId,
          hwnd: request.hwnd,
          title: info.title,
          sharedAt: Date.now()
        })
        this.publish(workspaceId)
        return this.state(workspaceId)
      }
      case 'revoke':
        this.shares.delete(workspaceId)
        this.publish(workspaceId)
        return this.state(workspaceId)
    }
  }

  forget(workspaceId: string) {
    this.shares.delete(workspaceId)
    this.logs.delete(workspaceId)
  }

  tools(): WorkbenchTool[] {
    return [
      {
        name: 'window_view',
        description:
          'Read-only screenshot of the desktop window the user shared with this chat in Prime Desktop. Fails if nothing is shared or sharing was revoked. Cannot click or type.',
        inputSchema: { type: 'object', properties: {}, additionalProperties: false },
        run: async (_args, context) => {
          const workspaceId = context.workspaceId!
          const share = this.shares.get(workspaceId)
          if (!share) throw new Error('The user has not shared a window with this chat.')
          const busy: string[] = await this.options.busySessions(workspaceId).catch(() => [])
          if (!busy.includes(share.sessionId)) throw new Error('The window is shared with a different chat.')
          try {
            const { info, png } = await this.capture(share.hwnd)
            if (this.shares.get(workspaceId) !== share) throw new Error('Sharing was revoked.')
            this.record(workspaceId, { title: info.title, source: 'agent', ok: true })
            return [
              { type: 'text', text: `Window "${info.title}" ${info.bounds.width}x${info.bounds.height}` },
              { type: 'image', data: png.toString('base64'), mimeType: 'image/png' }
            ]
          } catch (error) {
            this.record(workspaceId, { title: share.title, source: 'agent', ok: false, error: (error as Error).message })
            throw error
          }
        }
      }
    ]
  }
}
