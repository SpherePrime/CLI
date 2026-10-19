import { createHash } from 'node:crypto'
import type { RemoteConnection } from './connection'
import { shellQuote } from './security'

export async function remoteFingerprint(connection: RemoteConnection): Promise<string> {
  const command = `cd ${shellQuote(connection.root)} && find . -xdev \\( -name .git -o -name node_modules \\) -prune -o -type f -printf '%T@ %s %p\\n' 2>/dev/null | sort | sha256sum`
  const result = await connection.exec(command, { timeout: 30000, maxBytes: 4096 })
  if (result.code) throw new Error('remote.commandTimeout')
  return createHash('sha256').update(connection.profile.id).update(result.stdout).digest('hex')
}