import { randomUUID } from 'node:crypto'
import { posix } from 'node:path'
import type { RemoteConnection } from './connection'
import type { ExecutionDefinition } from './executionSupport'
import {
  integer,
  maxOutput,
  pathProperty,
  relativePath,
  schema,
  string,
  stringProperty,
  text
} from './executionSupport'
import { shellQuote } from './security'

async function execute(connection: RemoteConnection, command: string, path = '', timeout = 30000) {
  const directory = await connection.resolve(relativePath(connection, path))
  if (!(await connection.stat(relativePath(connection, directory))).isDirectory())
    throw new Error('remote.notDirectory')
  return connection.exec(`cd ${shellQuote(directory)} && (\n${command}\n)`, {
    timeout,
    maxBytes: maxOutput
  })
}

export const sshCommandTools: ExecutionDefinition[] = [
  {
    name: 'ssh_exec',
    description:
      'Execute a shell command on the selected SSH server in its project directory. timeout is milliseconds (maximum 120000), output is limited to 256 KiB. For long jobs explicitly start the command in the remote background and inspect it with ssh_exec. terminal_create/read/write/close belong to the current project and do not select profileId: never substitute a local terminal for the selected SSH server. No local execution fallback.',
    inputSchema: schema(
      {
        command: { type: 'string', minLength: 1 },
        description: stringProperty,
        workingDir: pathProperty,
        timeout: { type: 'integer', minimum: 1, maximum: 120000 }
      },
      ['command']
    ),
    async run(args, connection) {
      const command = string(args, 'command')
      if (!command.trim() || command.length > 128 * 1024) throw new Error('Invalid command')
      return text(
        await execute(
          connection,
          command,
          string(args, 'workingDir', ''),
          integer(args, 'timeout', 30000, 120000)
        )
      )
    }
  },
  {
    name: 'ssh_search',
    description:
      'Search remote file contents with a regular expression. Uses ripgrep or grep on the SSH server; output includes line numbers and is limited to 256 KiB. path is a project directory.',
    inputSchema: schema({ pattern: stringProperty, path: pathProperty }, ['pattern']),
    async run(args, connection) {
      const pattern = shellQuote(string(args, 'pattern'))
      const command = `if command -v rg >/dev/null 2>&1; then rg -n --no-heading --color never -m 100 -- ${pattern} .; else grep -RInI -m 100 --exclude-dir=.git --exclude-dir=node_modules -- ${pattern} .; fi`
      const result = await execute(connection, command, string(args, 'path', ''))
      if (result.code > 1 || result.code < 0)
        throw new Error('Remote search failed: ' + result.stderr.slice(0, 2000))
      return text(result)
    }
  },
  {
    name: 'ssh_glob',
    description:
      'Find remote file paths by glob, using ripgrep or find. path is a project directory. Without ripgrep, patterns without slashes match file names; patterns with slashes match relative paths. Output is limited to 256 KiB.',
    inputSchema: schema({ pattern: stringProperty, path: pathProperty }, ['pattern']),
    async run(args, connection) {
      const pattern = string(args, 'pattern')
      if (!pattern || pattern.length > 4096) throw new Error('Invalid pattern')
      const findPattern = pattern.includes('/')
        ? `-path ${shellQuote('./' + pattern.replace(/^\.\//, ''))}`
        : `-name ${shellQuote(pattern)}`
      const command = `if command -v rg >/dev/null 2>&1; then rg --files --hidden -g '!.git' -g ${shellQuote(pattern)} -- .; else find . -type d \\( -name .git -o -name node_modules \\) -prune -o -type f ${findPattern} -print; fi`
      const result = await execute(connection, command, string(args, 'path', ''))
      if (result.code > 1 || result.code < 0)
        throw new Error('Remote glob failed: ' + result.stderr.slice(0, 2000))
      return text(result)
    }
  },
  {
    name: 'ssh_download',
    description:
      'Download an HTTP(S) URL directly on the SSH server with curl or wget into a new project file. Existing files are never replaced. Maximum download size is 50 MiB, timeout 120 seconds. URL credentials are not accepted.',
    inputSchema: schema({ url: stringProperty, path: pathProperty }, ['url', 'path']),
    async run(args, connection) {
      const url = new URL(string(args, 'url'))
      if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password)
        throw new Error('Expected HTTP(S) URL without credentials')
      const path = relativePath(connection, string(args, 'path'))
      const absolute = await connection.resolve(path, true)
      const temporary = posix.join(posix.dirname(absolute), `.prime-download-${randomUUID()}`)
      const target = shellQuote(absolute)
      const temporaryQuoted = shellQuote(temporary)
      const source = shellQuote(url.href)
      const cleanup = shellQuote(`rm -f -- ${temporaryQuoted}`)
      const command = `umask 077; trap ${cleanup} EXIT HUP INT TERM; ( set -C; : > ${temporaryQuoted} ) && ( ulimit -f 102400; if command -v curl >/dev/null 2>&1; then curl --fail --location --silent --show-error --connect-timeout 15 --max-time 110 --max-filesize 52428800 --proto '=http,https' --proto-redir '=http,https' --output ${temporaryQuoted} -- ${source}; elif command -v wget >/dev/null 2>&1; then wget --quiet --timeout=30 --tries=1 --output-document=${temporaryQuoted} -- ${source}; else printf 'curl or wget is required' >&2; exit 127; fi ) && test "$(wc -c < ${temporaryQuoted})" -le 52428800 && ln -- ${temporaryQuoted} ${target}`
      const result = await execute(connection, command, '', 120000)
      if (result.code) throw new Error('Remote download failed: ' + result.stderr.slice(0, 2000))
      return text({ path, downloaded: true })
    }
  }
]
