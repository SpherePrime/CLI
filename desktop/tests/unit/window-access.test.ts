import { describe, it, expect, vi, beforeEach } from 'vitest'
const native = vi.hoisted(() => ({
  listWindows: vi.fn(),
  windowInfo: vi.fn(),
  claimWindowIdentity: vi.fn(),
  verifyWindowIdentity: vi.fn(),
  performWindowAction: vi.fn()
}))
vi.mock('../../src/main/computer/win32', () => native)
vi.mock('electron', () => ({ desktopCapturer: { getSources: vi.fn() } }))
import { WindowViewer } from '../../src/main/computer/viewer'
const info = {
  hwnd: '123',
  title: 'Fixture',
  pid: 55,
  bounds: { x: 0, y: 0, width: 400, height: 300 }
}
describe('window access', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    native.listWindows.mockReturnValue([info])
    native.windowInfo.mockReturnValue(info)
    native.claimWindowIdentity.mockReturnValue('identity')
    native.verifyWindowIdentity.mockReturnValue(true)
  })
  const create = (sessions = ['chat']) =>
    new WindowViewer({ emit: vi.fn(), busySessions: async () => sessions })
  const invoke = (viewer: WindowViewer, name: string, args = {}) =>
    viewer
      .tools()
      .find((t) => t.name === name)!
      .run(args, { workspaceId: 'project', sessionId: 'chat' })
  it('lists windows without granting access', async () => {
    const viewer = create()
    expect(await invoke(viewer, 'window_list')).toEqual([
      { type: 'text', text: expect.stringContaining('Fixture') }
    ])
    expect((await viewer.handle('project', { action: 'state' })).share).toBeUndefined()
  })
  const selection = {
    hwnd: '123',
    pid: 55,
    identity: 'identity',
    title: 'Fixture',
    mode: 'control'
  }
  it('grants access when the approved request executes without another panel confirmation', async () => {
    const viewer = create()
    const result = await invoke(viewer, 'window_request_access', selection)
    expect(JSON.parse(result[0].type === 'text' ? result[0].text : '')).toMatchObject({
      status: 'granted'
    })
    const state = await viewer.handle('project', { action: 'state' })
    expect(state.request).toBeUndefined()
    expect(state.share).toMatchObject({
      hwnd: '123',
      sessionId: 'chat',
      mode: 'control'
    })
    await invoke(viewer, 'window_action', { action: 'click', x: 20, y: 20 })
    expect(native.performWindowAction).toHaveBeenCalledOnce()
  })
  it('requires a control grant and revokes immediately', async () => {
    const viewer = create()
    await viewer.handle('project', { action: 'share', hwnd: '123', sessionId: 'chat' })
    await expect(
      invoke(viewer, 'window_action', { action: 'click', x: 20, y: 20 })
    ).rejects.toThrow()
    await viewer.handle('project', {
      action: 'share',
      hwnd: '123',
      sessionId: 'chat',
      mode: 'control'
    })
    await invoke(viewer, 'window_action', { action: 'click', x: 20, y: 20 })
    expect(native.performWindowAction).toHaveBeenCalledOnce()
    await viewer.handle('project', { action: 'revoke' })
    await expect(
      invoke(viewer, 'window_action', { action: 'click', x: 20, y: 20 })
    ).rejects.toThrow()
    expect(native.performWindowAction).toHaveBeenCalledOnce()
  })
  it('blocks ambiguous chat access and recycled handles', async () => {
    const viewer = create(['other'])
    await viewer.handle('project', {
      action: 'share',
      hwnd: '123',
      sessionId: 'chat',
      mode: 'control'
    })
    await expect(invoke(viewer, 'window_action', { action: 'focus' })).rejects.toThrow(
      'different chat'
    )
    const single = create()
    await single.handle('project', {
      action: 'share',
      hwnd: '123',
      sessionId: 'chat',
      mode: 'control'
    })
    native.verifyWindowIdentity.mockReturnValue(false)
    await expect(invoke(single, 'window_action', { action: 'focus' })).rejects.toThrow()
    expect(native.performWindowAction).not.toHaveBeenCalled()
  })
  it('does not release a screenshot after revocation during capture', async () => {
    const viewer = create()
    await viewer.handle('project', { action: 'share', hwnd: '123', sessionId: 'chat' })
    let release!: (value: any) => void
    const capture = vi.spyOn(viewer, 'capture').mockImplementation(
      () =>
        new Promise((resolve) => {
          release = resolve
        })
    )
    const pending = invoke(viewer, 'window_view')
    await vi.waitFor(() => expect(capture).toHaveBeenCalled())
    await viewer.handle('project', { action: 'revoke' })
    release({ info, png: Buffer.from('image'), imageSize: { width: 400, height: 300 } })
    await expect(pending).rejects.toThrow('revoked')
  })
  it('rejects recycled approval targets and malformed input without native events', async () => {
    const viewer = create()
    native.verifyWindowIdentity.mockReturnValue(false)
    await expect(invoke(viewer, 'window_request_access', selection)).rejects.toThrow()
    expect((await viewer.handle('project', { action: 'state' })).share).toBeUndefined()
    await expect(
      invoke(viewer, 'window_action', { action: 'click', x: -1, y: 0 })
    ).rejects.toThrow()
    await expect(
      invoke(viewer, 'window_action', { action: 'focus', hwnd: '999' })
    ).rejects.toThrow()
    expect(native.performWindowAction).not.toHaveBeenCalled()
  })
  it('revokes every workspace and pending request with the emergency stop', async () => {
    const viewer = create()
    await viewer.handle('project', {
      action: 'share',
      hwnd: '123',
      sessionId: 'chat',
      mode: 'control'
    })
    await invoke(viewer, 'window_request_access', { ...selection, mode: 'view' })
    viewer.revokeAll()
    expect(await viewer.handle('project', { action: 'state' })).toMatchObject({
      share: undefined,
      request: undefined
    })
    await expect(invoke(viewer, 'window_action', { action: 'focus' })).rejects.toThrow('not shared')
  })
  it('requires trusted chat context even when only one chat is busy', async () => {
    const viewer = create()
    const request = viewer.tools().find((tool) => tool.name === 'window_request_access')!
    await expect(request.run(selection, { workspaceId: 'project' })).rejects.toThrow('chat')
    expect((await viewer.handle('project', { action: 'state' })).share).toBeUndefined()
  })
  it('rejects permission claims smuggled through tool arguments', async () => {
    const viewer = create()
    await expect(
      invoke(viewer, 'window_request_access', { ...selection, skip: true })
    ).rejects.toThrow()
    await expect(
      invoke(viewer, 'window_request_access', { ...selection, sessionId: 'other' })
    ).rejects.toThrow()
    expect((await viewer.handle('project', { action: 'state' })).share).toBeUndefined()
  })
  it('includes native identity in tool listings and verifies the displayed target', async () => {
    const viewer = create()
    const list = await invoke(viewer, 'window_list')
    expect(JSON.parse(list[0].type === 'text' ? list[0].text : '')[0]).toMatchObject({
      ...info,
      identity: 'identity'
    })
    await expect(
      invoke(viewer, 'window_request_access', { ...selection, pid: 56 })
    ).rejects.toThrow()
    await expect(
      invoke(viewer, 'window_request_access', { ...selection, title: 'Another window' })
    ).rejects.toThrow()
    expect((await viewer.handle('project', { action: 'state' })).share).toBeUndefined()
  })
  it('keeps the approved grant bound to its chat when another chat is also busy', async () => {
    const viewer = create(['chat', 'other'])
    await invoke(viewer, 'window_request_access', selection)
    const action = viewer.tools().find((tool) => tool.name === 'window_action')!
    await expect(
      action.run({ action: 'focus' }, { workspaceId: 'project', sessionId: 'other' })
    ).rejects.toThrow('different chat')
    expect(native.performWindowAction).not.toHaveBeenCalled()
    await invoke(viewer, 'window_action', { action: 'focus' })
    expect(native.performWindowAction).toHaveBeenCalledOnce()
  })
})
