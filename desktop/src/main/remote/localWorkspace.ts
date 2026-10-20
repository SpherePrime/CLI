import { createHash } from 'node:crypto'
import { mkdir } from 'node:fs/promises'
import { join } from 'node:path'
import { AtomicStore } from '../persistence/atomicStore'
import type { RemoteProfile } from '../../shared/remote'

export async function prepareLocalSshWorkspace(
  directory: string,
  profile: RemoteProfile,
  root: string
) {
  const identity = createHash('sha256')
    .update(JSON.stringify([profile.host, profile.port, profile.user, root]))
    .digest('hex')
    .slice(0, 24)
  const localRoot = join(directory, profile.id, identity)
  await mkdir(join(localRoot, '.prime'), { recursive: true })
  const store = new AtomicStore<Record<string, unknown>>(join(localRoot, '.prime', 'prime.json'))
  const existing = (await store.read()) as Record<string, unknown> | undefined
  await store.write({
    ...existing,
    options: {
      ...(existing?.options && typeof existing.options === 'object' ? existing.options : {}),
      remote_workspace: { host: profile.host, user: profile.user, root },
      auto_lsp: false,
      attribution: { trailer_style: 'none', generated_with: false }
    }
  })
  return localRoot
}
