import { describe, expect, it, vi } from 'vitest'
import { createSshExecutionTools } from '../../src/main/remote/execution'
import type { RemoteConnection } from '../../src/main/remote/connection'

function fixture() {
  const connection = {
    root: '/srv/project',
    resolve: vi.fn(async (path: string) => '/srv/project' + (path ? '/' + path : '')),
    stat: vi.fn(async () => ({ isDirectory: () => true })),
    exec: vi.fn(async (_command: string) => ({ stdout: 'server', stderr: '', code: 0 })),
    readFile: vi.fn(async () => ({
      content: 'first\nsecond\nthird',
      revision: 'revision-1',
      path: 'file.txt'
    })),
    saveFile: vi.fn(async () => ({ revision: 'revision-2' })),
    createFile: vi.fn(async () => ({ revision: 'created' })),
    list: vi.fn(async () => [])
  }
  const tools = createSshExecutionTools(() => connection as unknown as RemoteConnection)
  const run = (name: string, args: Record<string, unknown> = {}) =>
    tools.find((tool) => tool.name === name)!.run(args, { workspaceId: 'ssh-project' })
  return { connection, tools, run }
}

describe('SSH executor tools', () => {
  it('rejects missing bindings and disconnected projects without local execution', async () => {
    const tools = createSshExecutionTools(() => undefined)
    for (const tool of tools) {
      await expect(tool.run({}, {})).rejects.toThrow('binding')
      await expect(tool.run({}, { workspaceId: 'gone' })).rejects.toThrow('disconnected')
    }
  })

  it('maps server paths and bounds commands without interpolating command arguments', async () => {
    const { connection, run } = fixture()
    await run('ssh_exec', {
      command: 'pwd; printf hello',
      workingDir: '/srv/project/sub',
      timeout: 90000
    })
    expect(connection.exec).toHaveBeenCalledWith(
      "cd '/srv/project/sub' && (\npwd; printf hello\n)",
      { timeout: 90000, maxBytes: 262144 }
    )
    await expect(run('ssh_exec', { command: 'pwd', workingDir: '/etc' })).rejects.toThrow(
      'remote.pathOutside'
    )
    await expect(run('ssh_exec', { command: 'pwd', timeout: 120001 })).rejects.toThrow(
      'Invalid timeout'
    )
    expect(connection.exec).toHaveBeenCalledTimes(1)
  })

  it('returns paged text with revisions and images as MCP image content', async () => {
    const { connection, run } = fixture()
    const result = await run('ssh_read_file', {
      path: '/srv/project/file.txt',
      offset: 1,
      limit: 1
    })
    expect(result).toEqual([
      {
        type: 'text',
        text: JSON.stringify({
          path: 'file.txt',
          revision: 'revision-1',
          offset: 1,
          totalLines: 3,
          content: 'second',
          truncated: true
        })
      }
    ])
    connection.readFile.mockResolvedValueOnce({
      image: 'data:image/png;base64,aGVsbG8=',
      path: 'image.png'
    } as any)
    expect(await run('ssh_read_file', { path: 'image.png' })).toEqual([
      { type: 'image', mimeType: 'image/png', data: 'aGVsbG8=' }
    ])
  })

  it('uses safe revisions, exclusive creates and literal edit replacements', async () => {
    const { connection, run } = fixture()
    await run('ssh_write_file', { path: 'file.txt', content: 'updated', revision: 'revision-0' })
    expect(connection.saveFile).toHaveBeenCalledWith('file.txt', 'updated', 'revision-0')
    connection.readFile.mockRejectedValueOnce(Object.assign(new Error('missing'), { code: 2 }))
    await run('ssh_write_file', { path: 'new.txt', content: 'new' })
    expect(connection.createFile).toHaveBeenCalledWith('new.txt', 'new')
    connection.readFile.mockResolvedValueOnce({
      content: 'first first',
      revision: 'revision-1',
      path: 'file.txt'
    })
    await expect(
      run('ssh_edit_file', { path: 'file.txt', oldText: 'first', newText: 'x' })
    ).rejects.toThrow('multiple')
    connection.readFile.mockResolvedValueOnce({
      content: 'first first',
      revision: 'revision-1',
      path: 'file.txt'
    })
    await run('ssh_edit_file', {
      path: 'file.txt',
      oldText: 'first',
      newText: '$&',
      replaceAll: true
    })
    expect(connection.saveFile).toHaveBeenLastCalledWith('file.txt', '$& $&', 'revision-1')
    connection.saveFile.mockRejectedValueOnce(new Error('remote.revisionConflict'))
    await expect(
      run('ssh_edit_file', { path: 'file.txt', oldText: 'first', newText: 'x' })
    ).rejects.toThrow('remote.revisionConflict')
  })

  it('quotes search patterns and rejects credentials in download URLs', async () => {
    const { connection, run } = fixture()
    await run('ssh_search', { pattern: "'; touch /bad; '" })
    expect(connection.exec.mock.calls[0][0]).toContain("'\\''; touch /bad; '\\'''")
    await expect(
      run('ssh_download', { url: 'https://user:password@example.com/a', path: 'a' })
    ).rejects.toThrow('without credentials')
    await expect(run('ssh_download', { url: 'file:///etc/passwd', path: 'a' })).rejects.toThrow(
      'HTTP(S)'
    )
    expect(connection.exec).toHaveBeenCalledTimes(1)
  })
})
