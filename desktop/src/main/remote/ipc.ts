import { z } from 'zod'
import type { RemoteManager } from './manager'
import type { RemoteRequest } from '../../shared/remote'

const profileId = z.string().uuid()
const secret = z.string().max(4096).optional()

const requestSchema = z.discriminatedUnion('action', [
  z.object({ action: z.literal('list') }),
  z.object({ action: z.literal('save'), input: z.record(z.string(), z.unknown()) }),
  z.object({ action: z.literal('remove'), id: profileId }),
  z.object({ action: z.literal('disconnect'), id: profileId }),
  z.object({ action: z.literal('forgetCredentials'), id: profileId }),
  z.object({
    action: z.literal('connect'),
    id: profileId,
    credentials: z.object({ password: secret, passphrase: secret }).optional()
  }),
  z.object({
    action: z.literal('trust'),
    id: profileId,
    fingerprint: z.string().regex(/^SHA256:[A-Za-z0-9+/]{43}$/)
  })
])

export function remoteRequest(manager: RemoteManager, input: unknown) {
  if ((JSON.stringify(input)?.length || 0) > 64000) throw new Error('remote.invalidProfile')
  return manager.handle(requestSchema.parse(input) as RemoteRequest)
}
