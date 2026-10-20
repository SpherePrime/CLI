import { randomUUID } from 'node:crypto'
import { open } from 'node:fs/promises'
import { isAbsolute, posix, resolve } from 'node:path'
import type { RemoteConnection } from './connection'
import type { WorkbenchTool } from '../workbench/tools'
import type { SshAccess } from './access'
import { relativePath, schema, string, stringProperty, text } from './executionSupport'

const uploadLimit = 50 * 1024 * 1024
const call = (operation: (done: (error?: Error | null) => void) => void) =>
  new Promise<void>((resolve, reject) => operation((error) => (error ? reject(error) : resolve())))

export async function uploadLocalFile(
  connection: RemoteConnection,
  source: string,
  path: string,
  overwrite = false
) {
  const local = await open(source, 'r')
  let bytes: Buffer
  try {
    const info = await local.stat()
    if (!info.isFile()) throw new Error('Upload source must be a regular file')
    if (info.size > uploadLimit) throw new Error('Upload exceeds 50 MiB')
    const buffer = Buffer.alloc(info.size + 1)
    let length = 0
    while (length < buffer.length) {
      const { bytesRead } = await local.read(buffer, length, buffer.length - length, null)
      if (!bytesRead) break
      length += bytesRead
    }
    if (length !== info.size) throw new Error('Local file changed during upload preparation')
    bytes = buffer.subarray(0, length)
  } finally {
    await local.close()
  }
  const destination = await connection.resolve(relativePath(connection, path), true)
  const temporary = posix.join(posix.dirname(destination), `.prime-upload-${randomUUID()}`)
  try {
    await call((done) =>
      connection.sftp.writeFile(temporary, bytes, { flag: 'wx', mode: 0o600 }, done)
    )
    if ((await connection.resolve(relativePath(connection, path), true)) !== destination)
      throw new Error('remote.pathOutside')
    if (overwrite)
      await call((done) => connection.sftp.ext_openssh_rename(temporary, destination, done))
    else await call((done) => connection.sftp.ext_openssh_hardlink(temporary, destination, done))
    return { path, bytes: bytes.length, uploaded: true }
  } finally {
    await call((done) => connection.sftp.unlink(temporary, done)).catch(() => {})
  }
}

export function sshUploadTools(
  access: SshAccess,
  localRoot: (workspaceId: string) => string
): WorkbenchTool[] {
  return [
    {
      name: 'ssh_upload_file',
      description:
        'Upload a user-requested local file (including binary files, up to 50 MiB) over SFTP to a saved SSH server, from the current chat. Select profileId using ssh_profiles. localPath is absolute or relative to the local project. path is inside the remote profile root. Existing files require explicit overwrite=true. No new chat is created.',
      inputSchema: schema(
        {
          profileId: stringProperty,
          localPath: stringProperty,
          path: stringProperty,
          overwrite: { type: 'boolean', default: false }
        },
        ['localPath', 'path']
      ),
      async run(args, context) {
        if (!context.workspaceId) throw new Error('Workspace binding is required')
        const source = string(args, 'localPath')
        if (
          args.profileId !== undefined &&
          (typeof args.profileId !== 'string' || !args.profileId.trim())
        )
          throw new Error('Invalid SSH profileId')
        if (args.overwrite !== undefined && typeof args.overwrite !== 'boolean')
          throw new Error('Invalid overwrite flag')
        const connection = await access.resolve(
          context.workspaceId,
          args.profileId as string | undefined
        )
        const local = isAbsolute(source) ? source : resolve(localRoot(context.workspaceId), source)
        return text(
          await uploadLocalFile(connection, local, string(args, 'path'), args.overwrite === true)
        )
      }
    }
  ]
}
