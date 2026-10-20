import type { ExecutionDefinition } from './executionSupport'
import {
  integer,
  pathProperty,
  relativePath,
  schema,
  string,
  stringProperty,
  text
} from './executionSupport'

export const sshFileTools: ExecutionDefinition[] = [
  {
    name: 'ssh_read_file',
    description:
      'Read a file on the SSH server. Text includes a revision for safe writes; offset is a zero-based line index. Images return image content.',
    inputSchema: schema(
      {
        path: pathProperty,
        offset: { type: 'integer', minimum: 0 },
        limit: { type: 'integer', minimum: 1, maximum: 2000 }
      },
      ['path']
    ),
    async run(args, connection) {
      const offset = integer(args, 'offset', 0, Number.MAX_SAFE_INTEGER, 0)
      const limit = integer(args, 'limit', 400, 2000)
      const file = await connection.readFile(relativePath(connection, string(args, 'path')))
      if (file.image) {
        const match = /^data:([^;]+);base64,(.*)$/s.exec(file.image)
        if (!match) throw new Error('Invalid remote image')
        return [{ type: 'image', mimeType: match[1], data: match[2] }]
      }
      const lines = file.content!.split('\n')
      const content = lines.slice(offset, offset + limit).join('\n')
      const maximum = 128 * 1024
      return text({
        path: file.path,
        revision: file.revision,
        offset,
        totalLines: lines.length,
        content: content.slice(0, maximum),
        truncated: offset + limit < lines.length || content.length > maximum
      })
    }
  },
  {
    name: 'ssh_write_file',
    description:
      'Create or replace a UTF-8 file on the SSH server (maximum 2 MiB). Existing files use revision checks; pass revision from ssh_read_file to reject stale content. Parent directory must exist.',
    inputSchema: schema({ path: pathProperty, content: stringProperty, revision: stringProperty }, [
      'path',
      'content'
    ]),
    async run(args, connection) {
      const path = relativePath(connection, string(args, 'path'))
      const content = string(args, 'content')
      let current: Awaited<ReturnType<typeof connection.readFile>>
      try {
        current = await connection.readFile(path)
      } catch (error: any) {
        if (![2, 'ENOENT'].includes(error.code)) throw error
        if (args.revision !== undefined) throw new Error('remote.revisionConflict')
        return text(await connection.createFile(path, content))
      }
      if (!current.revision) throw new Error('remote.binaryFile')
      const revision = args.revision === undefined ? current.revision : string(args, 'revision')
      const saved = await connection.saveFile(path, content, revision)
      return text({ path, revision: saved.revision })
    }
  },
  {
    name: 'ssh_edit_file',
    description:
      'Replace exact text in a remote UTF-8 file with concurrent-change protection. oldText must occur once unless replaceAll is true.',
    inputSchema: schema(
      {
        path: pathProperty,
        oldText: { type: 'string', minLength: 1 },
        newText: stringProperty,
        replaceAll: { type: 'boolean' }
      },
      ['path', 'oldText', 'newText']
    ),
    async run(args, connection) {
      const path = relativePath(connection, string(args, 'path'))
      const oldText = string(args, 'oldText')
      const newText = string(args, 'newText')
      if (!oldText) throw new Error('oldText must not be empty')
      if (args.replaceAll !== undefined && typeof args.replaceAll !== 'boolean')
        throw new Error('Invalid replaceAll')
      const file = await connection.readFile(path)
      if (file.content === undefined || !file.revision) throw new Error('remote.binaryFile')
      const count = file.content.split(oldText).length - 1
      if (!count) throw new Error('oldText was not found')
      if (count > 1 && !args.replaceAll)
        throw new Error('oldText matches multiple locations; provide more context or replaceAll')
      const content = args.replaceAll
        ? file.content.split(oldText).join(newText)
        : file.content.replace(oldText, () => newText)
      const saved = await connection.saveFile(path, content, file.revision)
      return text({ path, revision: saved.revision, replacements: args.replaceAll ? count : 1 })
    }
  },
  {
    name: 'ssh_list_files',
    description:
      'List a directory on the SSH server, respecting project ignore rules. At most 2000 entries are returned.',
    inputSchema: schema({ path: pathProperty }),
    async run(args, connection) {
      const path = relativePath(connection, string(args, 'path', ''))
      const entries = await connection.list(path)
      return text({ path, entries: entries.slice(0, 2000), truncated: entries.length > 2000 })
    }
  }
]
