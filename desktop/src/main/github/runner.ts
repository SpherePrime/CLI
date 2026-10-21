import { execFile } from 'node:child_process'
import type { RemoteConnection } from '../remote/connection'
import { shellQuote } from '../remote/security'

export type CommandRunner = (
  command: 'gh' | 'git',
  args: string[],
  input?: string
) => Promise<string>

export class CommandError extends Error {
  constructor(
    message: string,
    readonly code: number | string | undefined,
    readonly stdout = ''
  ) {
    super(message)
  }
}

export function localRunner(root: string): CommandRunner {
  return (command, args, input) =>
    new Promise((resolve, reject) => {
      const child = execFile(
        command,
        args,
        {
          cwd: root,
          windowsHide: true,
          maxBuffer: 16 * 1024 * 1024,
          timeout: 60000,
          env: { ...process.env, GH_PROMPT_DISABLED: '1', NO_COLOR: '1' }
        },
        (error, stdout, stderr) => {
          if (error)
            reject(
              new CommandError(
                (stderr || error.message).trim().slice(0, 2000),
                (error as NodeJS.ErrnoException).code ?? error.code ?? undefined,
                stdout
              )
            )
          else resolve(stdout)
        }
      )
      child.stdin?.on('error', () => {})
      if (input !== undefined) child.stdin?.end(input)
    })
}

export function remoteRunner(connection: RemoteConnection): CommandRunner {
  return async (command, args, input) => {
    const line = [command, ...args].map(shellQuote).join(' ')
    const result = await connection.exec(
      `cd ${shellQuote(connection.root)} && GH_PROMPT_DISABLED=1 NO_COLOR=1 ${line}`,
      {
        timeout: 60000,
        maxBytes: 16 * 1024 * 1024,
        input
      }
    )
    if (result.code === 127) throw new CommandError(`${command} not found`, 'ENOENT')
    if (result.code)
      throw new CommandError(
        result.stderr.trim().slice(0, 2000) || `exit ${result.code}`,
        result.code,
        result.stdout
      )
    return result.stdout
  }
}
