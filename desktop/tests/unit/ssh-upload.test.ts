import { expect, it, vi } from 'vitest'
import { mkdtemp, writeFile, rm } from 'node:fs/promises'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { uploadLocalFile } from '../../src/main/remote/upload'

it('uploads exact binary bytes and never overwrites an existing destination by default', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'prime-upload-'))
  const source = join(directory, 'binary.dat')
  await writeFile(source, Buffer.from([0, 255, 13, 10]))
  const sftp = {
    writeFile: vi.fn((_path, _data, _options, done) => done()),
    ext_openssh_hardlink: vi.fn((_from, _to, done) => done(new Error('File exists'))),
    ext_openssh_rename: vi.fn((_from, _to, done) => done()),
    unlink: vi.fn((_path, done) => done())
  }
  const connection = { root: '/srv/app', resolve: vi.fn(async () => '/srv/app/binary.dat'), sftp }
  try {
    await expect(uploadLocalFile(connection as any, source, 'binary.dat')).rejects.toThrow(
      'File exists'
    )
    expect(sftp.writeFile.mock.calls[0][1]).toEqual(Buffer.from([0, 255, 13, 10]))
    expect(sftp.writeFile.mock.calls[0][2]).toEqual({ flag: 'wx', mode: 0o600 })
    expect(sftp.ext_openssh_rename).not.toHaveBeenCalled()
    expect(sftp.unlink).toHaveBeenCalledTimes(1)
    await uploadLocalFile(connection as any, source, 'binary.dat', true)
    expect(sftp.ext_openssh_rename).toHaveBeenCalledWith(
      expect.any(String),
      '/srv/app/binary.dat',
      expect.any(Function)
    )
    await expect(uploadLocalFile(connection as any, directory, 'bad')).rejects.toThrow(
      'regular file'
    )
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})
