import { desktopCapturer } from 'electron'
import { z } from 'zod'
import type { DesktopEvent } from '../../shared/contracts'
import type {
  WindowAccessRequest,
  WindowAction,
  WindowLogEntry,
  WindowShare,
  WindowState
} from '../../shared/computer'
import type { WorkbenchTool, WorkbenchToolContext } from '../workbench/tools'
import {
  claimWindowIdentity,
  listWindows,
  performWindowAction,
  verifyWindowIdentity,
  windowInfo
} from './win32'

type Options = {
  emit(event: DesktopEvent): void
  busySessions(workspaceId: string): Promise<string[]>
}
const hwnd = z.string().regex(/^\d{1,20}$/)
const mode = z.enum(['view', 'control'])
const coordinate = z.number().finite().int().min(0).max(100000)
const requestSchema = z.discriminatedUnion('action', [
  z.object({ action: z.enum(['list', 'state', 'revoke', 'dismiss']) }).strict(),
  z.object({ action: z.literal('snapshot'), hwnd }).strict(),
  z
    .object({
      action: z.literal('share'),
      hwnd,
      sessionId: z.string().min(1).max(200),
      mode: mode.optional(),
      expectedIdentity: z.string().min(1).max(30).optional(),
      expectedPid: z.number().int().positive().optional()
    })
    .strict()
])
const actionSchema = z.discriminatedUnion('action', [
  z.object({ action: z.literal('focus') }).strict(),
  z
    .object({
      action: z.literal('click'),
      x: coordinate,
      y: coordinate,
      button: z.enum(['left', 'right']).optional(),
      count: z.number().int().min(1).max(2).optional()
    })
    .strict(),
  z.object({ action: z.literal('type'), text: z.string().min(1).max(1000) }).strict(),
  z.object({ action: z.literal('press_key'), key: z.string().min(1).max(60) }).strict(),
  z
    .object({
      action: z.literal('scroll'),
      x: coordinate,
      y: coordinate,
      delta: z
        .number()
        .int()
        .min(-2400)
        .max(2400)
        .refine((value) => value !== 0)
    })
    .strict()
])
const text = (value: unknown) => [
  { type: 'text' as const, text: typeof value === 'string' ? value : JSON.stringify(value) }
]

export class WindowViewer {
  private shares = new Map<string, WindowShare>()
  private requests = new Map<string, WindowAccessRequest>()
  private logs = new Map<string, WindowLogEntry[]>()
  constructor(private options: Options) {}
  private state(workspaceId: string): WindowState {
    return {
      share: this.shares.get(workspaceId),
      request: this.requests.get(workspaceId),
      log: [...(this.logs.get(workspaceId) ?? [])].reverse()
    }
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
  private workspace(context: WorkbenchToolContext) {
    if (!context.workspaceId) throw new Error('Workspace binding is required')
    return context.workspaceId
  }
  private async session(context: WorkbenchToolContext) {
    const workspaceId = this.workspace(context)
    const busy = await this.options.busySessions(workspaceId)
    const explicit = context.sessionId
    if (explicit && busy.includes(explicit)) return explicit
    throw new Error(
      'The window is shared with a different chat, or the active chat cannot be identified unambiguously.'
    )
  }
  private guard(workspaceId: string, share: WindowShare) {
    if (this.shares.get(workspaceId) !== share) throw new Error('Window access was revoked.')
    if (!verifyWindowIdentity(share.hwnd, share.pid, share.identity)) {
      this.shares.delete(workspaceId)
      this.publish(workspaceId)
      throw new Error('The selected window was closed or replaced. Request access again.')
    }
  }
  private async access(context: WorkbenchToolContext, control = false) {
    const workspaceId = this.workspace(context)
    const share = this.shares.get(workspaceId)
    if (!share)
      throw new Error('The user has not shared a window with this chat. Use window_request_access.')
    if ((await this.session(context)) !== share.sessionId)
      throw new Error('The window is shared with a different chat.')
    this.guard(workspaceId, share)
    if (control && share.mode !== 'control')
      throw new Error(
        'This window is view-only. Request control and wait for the user to grant it.'
      )
    return { workspaceId, share }
  }
  async capture(target: string) {
    const info = windowInfo(target)
    if (!info) throw new Error('Окно закрыто')
    try {
      info.identity = claimWindowIdentity(target)
    } catch {}
    const width = Math.max(1, Math.min(1920, info.bounds.width))
    const height = Math.max(
      1,
      Math.round((info.bounds.height / Math.max(1, info.bounds.width)) * width)
    )
    const sources = await desktopCapturer.getSources({
      types: ['window'],
      thumbnailSize: { width, height }
    })
    const source = sources.find((item) => item.id.split(':')[1] === target)
    if (!source || source.thumbnail.isEmpty())
      throw new Error('Не удалось получить снимок окна. Разверните окно и повторите попытку.')
    if (info.identity && !verifyWindowIdentity(target, info.pid, info.identity))
      throw new Error('Окно закрыто или заменено. Обновите список окон.')
    return { info, png: source.thumbnail.toPNG(), imageSize: source.thumbnail.getSize() }
  }
  async handle(workspaceId: string, input: unknown): Promise<any> {
    const request = requestSchema.parse(input)
    switch (request.action) {
      case 'list':
        return listWindows()
      case 'state':
        return this.state(workspaceId)
      case 'snapshot': {
        const { info, png, imageSize } = await this.capture(request.hwnd)
        this.record(workspaceId, {
          title: info.title,
          source: 'user',
          action: 'snapshot',
          ok: true
        })
        return { info, imageSize, image: `data:image/png;base64,${png.toString('base64')}` }
      }
      case 'share': {
        const info = windowInfo(request.hwnd)
        if (!info) throw new Error('Окно закрыто')
        const pending = this.requests.get(workspaceId)
        const expectedIdentity =
          request.expectedIdentity ??
          (pending?.hwnd === request.hwnd && pending.sessionId === request.sessionId
            ? pending.identity
            : undefined)
        const expectedPid =
          request.expectedPid ??
          (pending?.hwnd === request.hwnd && pending.sessionId === request.sessionId
            ? pending.pid
            : undefined)
        if (
          (expectedPid !== undefined && expectedPid !== info.pid) ||
          (expectedIdentity &&
            !verifyWindowIdentity(request.hwnd, expectedPid ?? info.pid, expectedIdentity))
        )
          throw new Error('Окно закрыто или заменено. Обновите список окон.')
        const identity = claimWindowIdentity(request.hwnd)
        this.shares.set(workspaceId, {
          workspaceId,
          sessionId: request.sessionId,
          hwnd: request.hwnd,
          title: info.title,
          pid: info.pid,
          identity,
          mode: request.mode ?? 'view',
          sharedAt: Date.now()
        })
        this.requests.delete(workspaceId)
        this.record(workspaceId, {
          title: info.title,
          source: 'user',
          action: request.mode === 'control' ? 'grant_control' : 'grant_view',
          ok: true
        })
        return this.state(workspaceId)
      }
      case 'dismiss':
        this.requests.delete(workspaceId)
        this.publish(workspaceId)
        return this.state(workspaceId)
      case 'revoke':
        this.shares.delete(workspaceId)
        this.requests.delete(workspaceId)
        this.publish(workspaceId)
        return this.state(workspaceId)
    }
  }
  revokeAll() {
    const workspaces = new Set([...this.shares.keys(), ...this.requests.keys()])
    this.shares.clear()
    this.requests.clear()
    for (const id of workspaces) this.publish(id)
  }
  dispose() {
    this.revokeAll()
  }
  forget(workspaceId: string) {
    this.shares.delete(workspaceId)
    this.requests.delete(workspaceId)
    this.logs.delete(workspaceId)
    this.publish(workspaceId)
  }
  tools(): WorkbenchTool[] {
    return [
      {
        name: 'window_list',
        description:
          'List visible desktop windows with handles, titles, process IDs, identities and bounds. Does not grant screenshots or control. Copy hwnd, pid, identity and title of the intended window into window_request_access.',
        inputSchema: { type: 'object', properties: {}, additionalProperties: false },
        run: async (_args, context) => {
          this.workspace(context)
          return text(
            listWindows().map((info) => {
              try {
                return { ...info, identity: claimWindowIdentity(info.hwnd) }
              } catch {
                return info
              }
            })
          )
        }
      },
      {
        name: 'window_request_access',
        description:
          'Grant this chat view or control of one selected desktop window through the normal chat permission policy. Copy hwnd, pid, identity and title exactly from window_list. Execution means the request has passed that policy, including full access mode. A successful result grants access immediately; no Windows panel confirmation is needed.',
        inputSchema: {
          type: 'object',
          properties: {
            hwnd: { type: 'string' },
            pid: { type: 'integer' },
            identity: { type: 'string' },
            title: { type: 'string' },
            mode: { type: 'string', enum: ['view', 'control'] }
          },
          required: ['hwnd', 'pid', 'identity', 'title', 'mode'],
          additionalProperties: false
        },
        run: async (args, context) => {
          const request = z
            .object({
              hwnd,
              mode,
              pid: z.number().int().positive(),
              identity: z.string().min(1).max(30),
              title: z.string().min(1).max(32768)
            })
            .strict()
            .parse(args)
          const workspaceId = this.workspace(context)
          const sessionId = await this.session(context)
          const info = windowInfo(request.hwnd)
          if (!info) throw new Error('Window is closed')
          if (
            info.pid !== request.pid ||
            info.title !== request.title ||
            !verifyWindowIdentity(request.hwnd, request.pid, request.identity)
          )
            throw new Error(
              'The selected window was closed or replaced. Refresh window_list and request access again.'
            )
          this.shares.set(workspaceId, {
            workspaceId,
            sessionId,
            hwnd: request.hwnd,
            title: info.title,
            pid: info.pid,
            identity: request.identity,
            mode: request.mode,
            sharedAt: Date.now()
          })
          this.requests.delete(workspaceId)
          this.record(workspaceId, {
            title: info.title,
            source: 'agent',
            action: request.mode === 'control' ? 'grant_control' : 'grant_view',
            ok: true
          })
          return text({
            status: 'granted',
            window: info,
            mode: request.mode,
            sessionId
          })
        }
      },
      {
        name: 'window_view',
        description:
          'Screenshot and state of the window explicitly shared with this chat. Requires active view or control grant. Coordinates for actions are relative to actual window bounds; scale screenshot coordinates using imageSize. Does not move focus.',
        inputSchema: { type: 'object', properties: {}, additionalProperties: false },
        run: async (_args, context) => {
          const { workspaceId, share } = await this.access(context)
          try {
            const { info, png, imageSize } = await this.capture(share.hwnd)
            this.guard(workspaceId, share)
            this.record(workspaceId, {
              title: info.title,
              source: 'agent',
              action: 'snapshot',
              ok: true
            })
            return [
              ...text({ window: info, imageSize, mode: share.mode }),
              { type: 'image' as const, data: png.toString('base64'), mimeType: 'image/png' }
            ]
          } catch (error) {
            this.record(workspaceId, {
              title: share.title,
              source: 'agent',
              action: 'snapshot',
              ok: false,
              error: (error as Error).message
            })
            throw error
          }
        }
      },
      {
        name: 'window_action',
        description:
          'Control only the window explicitly granted to this chat. Actions: focus (explicitly activates window), click, type Unicode text, press_key (e.g. CTRL+A, ENTER), scroll (delta in Windows wheel units, positive up). x/y are pixels relative to actual window bounds. Input requires the selected window in foreground and points unobscured; focus is never changed implicitly. Access can be revoked immediately with Ctrl+Alt+Escape.',
        inputSchema: {
          type: 'object',
          properties: {
            action: { type: 'string', enum: ['focus', 'click', 'type', 'press_key', 'scroll'] },
            x: { type: 'integer', minimum: 0 },
            y: { type: 'integer', minimum: 0 },
            text: { type: 'string', maxLength: 1000 },
            key: { type: 'string' },
            button: { type: 'string', enum: ['left', 'right'] },
            count: { type: 'integer', minimum: 1, maximum: 2 },
            delta: { type: 'integer', minimum: -2400, maximum: 2400 }
          },
          required: ['action'],
          additionalProperties: false
        },
        run: async (args, context) => {
          const action = actionSchema.parse(args) as WindowAction
          const { workspaceId, share } = await this.access(context, true)
          try {
            performWindowAction(share.hwnd, action, () => this.guard(workspaceId, share))
            this.record(workspaceId, {
              title: share.title,
              source: 'agent',
              action: action.action,
              ok: true
            })
            return text({ status: 'ok', action: action.action, hwnd: share.hwnd })
          } catch (error) {
            this.record(workspaceId, {
              title: share.title,
              source: 'agent',
              action: action.action,
              ok: false,
              error: (error as Error).message
            })
            throw error
          }
        }
      }
    ]
  }
}
